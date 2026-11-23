-- change_id: PAY-SETTLE-01
-- C-05 migration draft (NOT approved) for SPEC-0002 v1.8 (approved 2026-11-24)
-- and ADR-0007 Amendment 1 (accepted 2026-11-24, Revision 1.0) §4.1/§4.2/§6.1/§6.2.
-- Owner: Order DB. Order owns the admission ledger; Payment facts are not copied.
--
-- Additive only, no down migration (ADR §10):
--   rollback = code keeps these tables and old code ignores them; state/data
--   rollback is audited manual repair only, never an automatic state remap.
--
-- refund_batches: one TCC compensation attempt. (source, order_no, payment_no)
-- is deliberately NOT unique so failed prechecks stay auditable; at most one
-- active batch (FROZEN/ADMITTING/ADMITTED/PAYING) exists at a time, enforced by
-- the Tx1 order row lock + state CAS, not by an index (ADR §6.1).
CREATE TABLE IF NOT EXISTS refund_batches (
  batch_id        VARCHAR(96) NOT NULL,             -- tcc:<payment_no>:<attempt_seq>
  source          VARCHAR(24) NOT NULL,             -- tcc | legacy | ...
  order_no        VARCHAR(64) NOT NULL,
  payment_no      VARCHAR(64) NOT NULL,
  attempt_seq     INT NOT NULL DEFAULT 1,
  state           VARCHAR(24) NOT NULL,             -- PRECHECKED/FROZEN/ADMITTING/ADMITTED/PAYING/DONE/PRECHECK_FAILED/MANUAL_HOLD/FAILED_PERMANENT
  captured_snapshot_cent BIGINT NOT NULL DEFAULT 0,
  refunded_snapshot_cent BIGINT NOT NULL DEFAULT 0,
  sum_group_cap_cent     BIGINT NOT NULL DEFAULT 0,
  remaining_captured_cent BIGINT NOT NULL DEFAULT 0,
  attempts        INT NOT NULL DEFAULT 0,
  lease_owner     VARCHAR(64) NOT NULL DEFAULT '',
  lease_until     DATETIME(3) NULL,
  last_error      VARCHAR(1024) NOT NULL DEFAULT '',
  version         BIGINT NOT NULL DEFAULT 1,
  created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (batch_id),
  KEY idx_refund_batch_lookup (source, order_no, payment_no, state),
  KEY idx_refund_batch_state (state, updated_at),
  CONSTRAINT chk_refund_batch_state CHECK (state IN
    ('PRECHECKED','FROZEN','ADMITTING','ADMITTED','PAYING','DONE','PRECHECK_FAILED','MANUAL_HOLD','FAILED_PERMANENT'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- refund_applications: the durable refund intent per batch/group (or payment
-- level). It must be persisted BEFORE Payment.Refund; Payment success is the
-- only path to DONE (ADR §4.5/§5.3). batch_id/group_no are NULL for payment
-- level rows: MySQL unique keys allow multiple NULLs, and refund_id remains the
-- real uniqueness backstop (ADR §6.2). non_effective_capture rows are projected
-- out of the order captured/refunded facts (INV-37).
CREATE TABLE IF NOT EXISTS refund_applications (
  refund_id       VARCHAR(128) NOT NULL,
  batch_id        VARCHAR(96)  NULL,
  source          VARCHAR(24)  NOT NULL,            -- aftersale/tcc/system_shortage/system_zero/payment/legacy_expired
  order_no        VARCHAR(64)  NOT NULL,
  group_no        VARCHAR(64)  NULL,
  non_effective_capture TINYINT(1) NOT NULL DEFAULT 0, -- payment/legacy_expired=1；组级=0
  amount_cent     BIGINT       NOT NULL DEFAULT 0,
  goods_cent      BIGINT       NOT NULL DEFAULT 0,
  freight_cent    BIGINT       NOT NULL DEFAULT 0,
  state           VARCHAR(24)  NOT NULL DEFAULT 'PENDING',
  admission_result VARCHAR(24) NOT NULL DEFAULT '',  -- PRE_SHIP_REVOKED/POST_SHIP_ALLOWED/PENDING_REMOTE
  attempts        INT          NOT NULL DEFAULT 0,
  lease_owner     VARCHAR(64)  NOT NULL DEFAULT '',
  lease_until     DATETIME(3)  NULL,
  last_error      VARCHAR(1024) NOT NULL DEFAULT '',
  version         BIGINT       NOT NULL DEFAULT 1,
  created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (refund_id),
  UNIQUE KEY uk_refund_app_batch_group (batch_id, group_no),
  KEY idx_refund_app_order_state (order_no, state),
  KEY idx_refund_app_source_state (source, state),
  CONSTRAINT chk_refund_app_state CHECK (state IN
    ('PENDING','PROCESSING','DONE','FAILED_PERMANENT','CANCELLED','REJECTED')),
  CONSTRAINT chk_refund_app_non_effective CHECK (non_effective_capture IN (0,1)),
  CONSTRAINT chk_refund_app_non_effective_source CHECK (
    (source IN ('payment','legacy_expired') AND non_effective_capture = 1)
    OR (source NOT IN ('payment','legacy_expired') AND non_effective_capture = 0)
  ),
  CONSTRAINT chk_refund_app_amount CHECK (amount_cent >= 0 AND goods_cent >= 0 AND freight_cent >= 0
    AND goods_cent + freight_cent = amount_cent)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
