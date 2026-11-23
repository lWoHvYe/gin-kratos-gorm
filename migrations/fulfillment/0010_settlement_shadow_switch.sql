-- change_id: PAY-SETTLE-01
-- C-05 migration draft (NOT approved) for SPEC-0002 v1.8 (approved 2026-11-24)
-- and ADR-0007 Amendment 1 (accepted 2026-11-24, Revision 1.0) §3.1-§3.4/§6.6
-- (D-27). Owner: Fulfillment DB. R1 keeps FULFILLMENT_SETTLE_CONSUMER_MODE=paid;
-- these tables only make shadow comparison and the later ON switch auditable.
--
-- Additive only, no down migration (ADR §10):
--   rollback = mode switch back to shadow/paid, data is never deleted; already
--   created shipments/permits are never rolled back, only forward-compatible.
--   Approval rows are written by controlled ops/Director action, never by the
--   application; the app only reads them at startup (fail-closed to paid).
--
-- shadow diff ledger: one row per (order_no, payment_no); a repeated event only
-- accumulates version/updates last_seen_at. Diff fields are alerting/reconciling
-- inputs and never block settlement or shipping (ADR §3.3).
CREATE TABLE IF NOT EXISTS settlement_shadow_diffs (
  order_no        VARCHAR(64) NOT NULL,
  payment_no      VARCHAR(64) NOT NULL,
  source_subject  VARCHAR(64) NOT NULL DEFAULT 'shop.order.settled.v1',
  schema_version  INT         NOT NULL DEFAULT 1,
  state           VARCHAR(16) NOT NULL DEFAULT 'MATCH',
  diff_fields     JSON        NULL,
  detail          JSON        NULL,
  first_seen_at   DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_seen_at    DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  version         BIGINT      NOT NULL DEFAULT 1,
  PRIMARY KEY (order_no, payment_no),
  KEY idx_shadow_state (state, last_seen_at),
  CONSTRAINT chk_shadow_state CHECK (state IN ('MATCH','MISMATCH','ERROR'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Director switch approvals. A 'settled' (ON) row must carry both the
-- order.paid admission retirement timestamp and the subject removal date;
-- 'shadow' rows leave both NULL. The startup check only accepts
-- target_mode='settled' AND status='APPROVED' AND expires_at not passed
-- (ADR §3.2/§3.4/§6.6).
CREATE TABLE IF NOT EXISTS settlement_switch_approvals (
  approval_id    VARCHAR(96)  NOT NULL,
  target_mode    VARCHAR(16)  NOT NULL,             -- shadow | settled
  approved_by    VARCHAR(64)  NOT NULL,
  approved_at    DATETIME(3)  NOT NULL,
  release_range  VARCHAR(128) NOT NULL DEFAULT '',
  evidence_ref   VARCHAR(255) NOT NULL,
  expires_at     DATETIME(3)  NULL,
  order_paid_admission_retired_at DATETIME(3) NULL, -- settled/ON 时写；shadow 行 NULL
  order_paid_subject_removal_date DATE        NULL, -- settled/ON 时必填；移除日期
  status         VARCHAR(16)  NOT NULL DEFAULT 'APPROVED',
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (approval_id),
  KEY idx_switch_target (target_mode, status),
  CONSTRAINT chk_switch_mode CHECK (target_mode IN ('shadow','settled')),
  CONSTRAINT chk_switch_status CHECK (status IN ('APPROVED','REVOKED','EXPIRED')),
  CONSTRAINT chk_switch_removal CHECK (
    target_mode <> 'settled' OR (
      order_paid_admission_retired_at IS NOT NULL AND order_paid_subject_removal_date IS NOT NULL
    )
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
