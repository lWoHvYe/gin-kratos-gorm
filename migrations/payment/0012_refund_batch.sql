-- PAY-SETTLE-01 T1.4b1-R / ADR-0007 Amendment 1 C-04 §2.8 + C-05.
-- change_id: PAY-SETTLE-01
--
-- Payment.RefundBatch (C-04) is atomic all-or-none: every item's refund row and
-- payment.refunded outbox event commits in one transaction. The batch idempotency
-- ledger has to be persistent, otherwise "same batch_id, different items" cannot
-- be detected and a retry could silently refund twice.
--
-- This migration:
--   1. adds refunds.batch_id (the atomic batch that wrote the row);
--   2. drops the legacy uk_refunds_idem(payment_no, amount_cent, reason) key:
--      two legitimate group refunds on the same payment may share amount/reason;
--   3. keeps uk_refunds_idempotency(payment_no, idempotency_key) as the single
--      per-refund idempotency key. refund_id is the caller-supplied stable id
--      and is stored in refunds.idempotency_key (migration 0002); no duplicate
--      identity column is introduced, so refund_no/refund_id can never diverge;
--   4. adds uk_refunds_batch_item(payment_no, batch_id, idempotency_key), i.e.
--      UNIQUE(payment_no, batch_id, refund_id), so one refund_id appears at most
--      once in a batch;
--   5. adds payment_refund_batches, which stores the canonical item-set
--      fingerprint and the per-item reply (refund_id/refund_no/status/amount)
--      for idempotent replay and IDEMPOTENCY_CONFLICT detection.
--
-- MySQL DDL is not transactional and the migration ledger may re-run a file
-- after a crash, so every statement is guarded and re-runnable.

-- 1. refunds.batch_id -------------------------------------------------------
SET @c := (SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND COLUMN_NAME = 'batch_id');
SET @s := IF(@c = 0,
  'ALTER TABLE refunds ADD COLUMN batch_id VARCHAR(96) NOT NULL DEFAULT '''' AFTER idempotency_key',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. drop the legacy (payment_no, amount_cent, reason) key -------------------
SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND INDEX_NAME = 'uk_refunds_idem');
SET @s := IF(@i > 0, 'ALTER TABLE refunds DROP INDEX uk_refunds_idem', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 3. keep (payment_no, idempotency_key) unique -------------------------------
-- payment/0002 creates uk_refunds_idempotency. A non-unique index with the same
-- name (e.g. an interrupted manual repair) is replaced instead of trusted.
SET @nu := (SELECT NON_UNIQUE FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds'
    AND INDEX_NAME = 'uk_refunds_idempotency' LIMIT 1);
SET @s := IF(@nu = 1, 'ALTER TABLE refunds DROP INDEX uk_refunds_idempotency', 'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND INDEX_NAME = 'uk_refunds_idempotency');
SET @s := IF(@i = 0,
  'ALTER TABLE refunds ADD UNIQUE KEY uk_refunds_idempotency (payment_no, idempotency_key)',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 4. UNIQUE(payment_no, batch_id, refund_id) --------------------------------
-- refunds.idempotency_key is refund_id by contract. batch_id defaults to '' for
-- non-batch (legacy Refund) rows; they remain unique through
-- uk_refunds_idempotency above.
SET @i := (SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refunds' AND INDEX_NAME = 'uk_refunds_batch_item');
SET @s := IF(@i = 0,
  'ALTER TABLE refunds ADD UNIQUE KEY uk_refunds_batch_item (payment_no, batch_id, idempotency_key)',
  'SELECT 1');
PREPARE stmt FROM @s; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 5. batch idempotency / per-item reply ledger -------------------------------
-- items_fingerprint is SHA-256 over the sorted (refund_id, amount_cent) pairs;
-- a repeat of (payment_no, batch_id) with a different fingerprint must be
-- rejected with IDEMPOTENCY_CONFLICT and zero writes.
CREATE TABLE IF NOT EXISTS payment_refund_batches (
  payment_no VARCHAR(64) NOT NULL,
  batch_id VARCHAR(96) NOT NULL,
  items_fingerprint CHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  refunded_cent BIGINT NOT NULL,
  reason VARCHAR(255) NOT NULL DEFAULT '',
  item_results JSON NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (payment_no, batch_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
