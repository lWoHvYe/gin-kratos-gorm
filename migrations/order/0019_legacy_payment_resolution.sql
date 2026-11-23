-- change_id: PAY-SETTLE-01
-- C-05 migration draft (NOT approved) for SPEC-0002 v1.8 (approved 2026-11-24)
-- and ADR-0007 Amendment 1 (accepted 2026-11-24, Revision 1.0) §5.1-§5.4/§6.5.
-- Owner: Order DB. Order is the only writer; Payment/ops observe through Order.
--
-- Additive only, no down migration (ADR §10):
--   rollback = keep the ledger; never delete rows or reopen a terminal state.
--   Terminal states are repaired only by audited manual compensation/close.
--
-- legacy_payment_resolution: one row per PAID order whose effective payment is
-- still unknown (settlement_payment_no = ''). orders.settlement_payment_no is
-- NOT changed here: it stays NOT NULL DEFAULT '' (order/0016) and the unique
-- candidate write-back is the CAS `UPDATE ... WHERE settlement_payment_no = ''`
-- (ADR §5.3/§5.4).
--   resolution_id is written only on a terminal transition; the unique key keeps
--   NULL open/resolving/compensating rows from colliding (MySQL allows multiple
--   NULLs). expires_at has no DB default on purpose: the writer must set
--   DB-time entered_at + 24h, and the CHECK rejects a shorter window.
CREATE TABLE IF NOT EXISTS legacy_payment_resolution (
  order_no          VARCHAR(64)  NOT NULL,
  state             VARCHAR(24)  NOT NULL DEFAULT 'OPEN',
  candidate_count   INT          NOT NULL DEFAULT 0,
  pay_cent          BIGINT       NOT NULL DEFAULT 0,
  blocked_scope     VARCHAR(32)  NOT NULL DEFAULT 'UNSHIPPED_GROUPS_AND_REFUND',
  resolution_id     VARCHAR(96)  NULL,
  resolution_kind   VARCHAR(24)  NOT NULL DEFAULT '',
  resolved_payment_no VARCHAR(64) NOT NULL DEFAULT '',
  entered_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  expires_at        DATETIME(3)  NOT NULL,
  resolved_at       DATETIME(3)  NULL,
  closed_at         DATETIME(3)  NULL,
  lease_owner       VARCHAR(64)  NOT NULL DEFAULT '',
  lease_until       DATETIME(3)  NULL,
  operator          VARCHAR(64)  NOT NULL DEFAULT '',
  authorized_by     VARCHAR(64)  NOT NULL DEFAULT '',
  reason_code       VARCHAR(64)  NOT NULL DEFAULT '',
  evidence_ref      VARCHAR(255) NOT NULL DEFAULT '',
  version           BIGINT       NOT NULL DEFAULT 1,
  created_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no),
  UNIQUE KEY uk_lpr_resolution (resolution_id),
  KEY idx_lpr_state_expiry (state, expires_at),
  KEY idx_lpr_lease (state, lease_until),
  CONSTRAINT chk_lpr_state CHECK (state IN
    ('OPEN','RESOLVING','COMPENSATING','RESOLVED','EXPIRED_REFUND','EXPIRED_ZERO','EXPIRED_FINANCE','REJECTED')),
  CONSTRAINT chk_lpr_kind CHECK (resolution_kind IN
    ('','MANUAL','EXPIRED_ZERO','EXPIRED_REFUND','EXPIRED_FINANCE','REJECTED')),
  CONSTRAINT chk_lpr_scope CHECK (blocked_scope IN
    ('UNSHIPPED_GROUPS_AND_REFUND','FINANCE_HOLD','REJECTED_HOLD','NONE')),
  CONSTRAINT chk_lpr_expiry CHECK (expires_at >= entered_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Per-order refund id sequence for legacy/payment-level refunds
-- (refund_id = legacy:<order_no>:<seq>). Numbers are allocated inside the same
-- transaction that locks the orders row (INSERT ... ON DUPLICATE KEY UPDATE
-- next_seq = LAST_INSERT_ID(next_seq + 1)); they are never reused or rolled
-- back (ADR §6.5).
CREATE TABLE IF NOT EXISTS order_refund_sequences (
  order_no   VARCHAR(64) NOT NULL,
  next_seq   BIGINT      NOT NULL DEFAULT 1,
  version    BIGINT      NOT NULL DEFAULT 1,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no),
  CONSTRAINT chk_refund_seq CHECK (next_seq >= 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
