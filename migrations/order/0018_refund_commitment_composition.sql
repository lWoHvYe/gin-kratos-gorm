-- change_id: PAY-SETTLE-01
-- C-05 migration for SPEC-0002 v1.10 (approved 2026-11-25) and ADR-0007
-- Rev 2.3 (accepted 2026-11-25) §4.3/§6.3/§6.4.
-- Owner: Order DB. Commitment headers/lines stay the single quota owner.
--
-- Additive only, no down migration (ADR §10):
--   rollback = code keeps the new columns/tables and old code ignores them; the
--   legacy backfill is one-way, quota and composition are repaired only by
--   audited manual work (never by resetting state or deleting lines).
--
-- Re-entry / partial failure (R0.1): MySQL 8.4 has no ADD COLUMN IF NOT EXISTS,
-- and a run interrupted after the atomic ALTER leaves the columns present while
-- the backfill and the two new tables may be missing. The column block below
-- therefore adds only the columns that are still missing, as one ALTER (still
-- atomic), so re-running the real file after a partial failure converges; the
-- UPDATE is idempotent and the tables use IF NOT EXISTS. The file must be
-- executed on a single session (the runner already does; the Go test helper
-- uses one connection) because PREPARE is session-scoped.
--
-- Header composition. amount_cent = goods_cent + freight_cent for new
-- commitments; legacy rows keep composition_known = 0 and may only be refunded
-- as a whole group until an operator backfills their lines (fail-closed,
-- REFUND_ADMISSION_CONFLICT — ADR §4.3/§6.3).
SET @c05_alter := (
  SELECT IFNULL(CONCAT('ALTER TABLE order_refund_commitments ',
    GROUP_CONCAT(want.definition ORDER BY want.seq SEPARATOR ', ')), 'SELECT 1')
  FROM (
    SELECT 1 AS seq, 'goods_cent' AS column_name,
           'ADD COLUMN goods_cent BIGINT NOT NULL DEFAULT 0 AFTER amount_cent' AS definition
    UNION ALL
    SELECT 2, 'freight_cent',
           'ADD COLUMN freight_cent BIGINT NOT NULL DEFAULT 0 AFTER goods_cent'
    UNION ALL
    SELECT 3, 'composition_known',
           'ADD COLUMN composition_known TINYINT(1) NOT NULL DEFAULT 0 AFTER freight_cent'
    UNION ALL
    SELECT 4, 'composition_source',
           'ADD COLUMN composition_source VARCHAR(16) NOT NULL DEFAULT ''legacy'' AFTER composition_known'
  ) AS want
  WHERE NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS existing
    WHERE existing.TABLE_SCHEMA = DATABASE()
      AND existing.TABLE_NAME = 'order_refund_commitments'
      AND existing.COLUMN_NAME = want.column_name
  )
);
PREPARE c05_commitment_columns FROM @c05_alter;
EXECUTE c05_commitment_columns;
DEALLOCATE PREPARE c05_commitment_columns;

-- Historical rows: quota only, no line attribution; they must not be used for
-- line-level partial refunds (ADR §6.3). Idempotent: goods = amount, freight = 0.
UPDATE order_refund_commitments
SET goods_cent = amount_cent, freight_cent = 0,
    composition_known = 0, composition_source = 'legacy'
WHERE composition_source = 'legacy';

-- Line composition. line_no = 0 is the freight line (goods_cent = 0);
-- line_no > 0 is a goods line (freight_cent = 0). SUM(goods_cent) =
-- header.goods_cent, SUM(freight_cent) = header.freight_cent and
-- header.amount_cent = goods + freight are validated inside the writing
-- transaction (ADR §6.3); the DB checks below only reject impossible rows.
CREATE TABLE IF NOT EXISTS order_refund_commitment_lines (
  refund_id     VARCHAR(128) NOT NULL,
  line_no       INT          NOT NULL,             -- 0=运费行；>0=商品行
  quantity      INT          NOT NULL DEFAULT 0,
  goods_cent    BIGINT       NOT NULL DEFAULT 0,
  freight_cent  BIGINT       NOT NULL DEFAULT 0,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (refund_id, line_no),
  CONSTRAINT chk_commitment_line_amount CHECK (goods_cent >= 0 AND freight_cent >= 0),
  CONSTRAINT chk_commitment_line_kind CHECK (
    (line_no = 0 AND goods_cent = 0) OR (line_no > 0 AND freight_cent = 0)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- One release per commitment and one release per authorization; amount equals
-- the commitment header (no partial release). Release is only legal on the two
-- AC-62 triggers with D-24 authorization: >= 100000 cent (1000 CNY) requires a
-- second, different reviewer (ADR §4.3/§6.4).
CREATE TABLE IF NOT EXISTS refund_commitment_releases (
  release_id      VARCHAR(96)  NOT NULL,
  refund_id       VARCHAR(128) NOT NULL,
  amount_cent     BIGINT       NOT NULL,
  authorization_id VARCHAR(96) NOT NULL,
  operator        VARCHAR(64)  NOT NULL,
  authorized_by   VARCHAR(64)  NOT NULL,
  reviewed_by     VARCHAR(64)  NOT NULL DEFAULT '',
  reason_code     VARCHAR(64)  NOT NULL,
  evidence_ref    VARCHAR(255) NOT NULL,
  authorized_at   DATETIME(3)  NOT NULL,
  reviewed_at     DATETIME(3)  NULL,
  expires_at      DATETIME(3)  NULL,
  released_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  version         BIGINT       NOT NULL DEFAULT 1,
  PRIMARY KEY (release_id),
  UNIQUE KEY uk_release_authorization (authorization_id),
  UNIQUE KEY uk_release_refund (refund_id),
  CONSTRAINT chk_release_amount CHECK (amount_cent > 0),
  CONSTRAINT chk_release_review CHECK (
    amount_cent < 100000 OR (reviewed_by <> '' AND reviewed_by <> authorized_by)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
