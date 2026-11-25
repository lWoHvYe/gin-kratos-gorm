-- change_id: PAY-SETTLE-01
-- R0.4c-impl migration 0023 (branch A) for ADR-0007 Rev 2.3 §6.2/§6.5 and
-- Addendum-r0.4 §2.2/§2.3.3/§2.5. Owner: Order DB.
--
-- Branch A (human decision 2026-11-26): order/0022_r04_checks.sql is FROZEN at
-- its pre-edit bytes (blob 240b45fc44a9be5b266901406357d0945ad6c0c5,
-- sha256 8339df0741584ef67c76e7daf417c2d6dee57ed26a7d1cf95ba0ef88cf574626,
-- `git show 90b502c:migrations/order/0022_r04_checks.sql`). All semantic
-- corrections live in THIS file; 0022 stays byte-identical so its checksum
-- ledger/replay contract cannot be invalidated.
--
-- What 0022 got wrong (kept in history, never re-edited):
--   * its discriminator copied the full 0017 source enum instead of only
--     expressing set membership;
--   * its 24h CHECK was a lower bound (`expires_at >= entered_at + 24h`), which
--     makes 48h/25h deadlines legal and lets the queue stay OPEN forever
--     (violates INV-46).
-- Its header also claimed "the conditional ALTERs make a re-run after cleanup
-- safe". That claim is WRONG and is superseded here: MySQL DDL commits
-- implicitly, so a blocked/interrupted run can leave one CHECK installed and the
-- other missing while the runner records an attempt as FAILED/RUNNING and then
-- refuses an automatic replay (Addendum §2.5). Do not "just re-run".
--
-- Replacement contract (no missing-constraint window): install the NEW names
-- first (`_v2`), and only after both ADDs succeed, DROP the old names. While
-- both exist the intersection is enforced; the old names are removed in the same
-- file, so a fresh apply ends with exactly one discriminator and one 24h CHECK.
-- If an ADD fails, the file stops before any DROP: the old constraint is still
-- in force and the FAILED attempt must go through the operator repair flow.
--
-- No down migration (ADR §10): rollback = a new forward fix migration. A wrong
-- manual row is corrected by an audited write, never by dropping the constraint
-- or bulk-updating data to hide it.
--
-- Failure/re-run contract (Addendum §2.5; same wording as docs/ops.md):
--   1. Stop. Locate the blocking row(s) from the precheck output or error 3819.
--   2. Repair data with an audited write only.
--   3. After operator sign-off delete the attempt row for this filename with
--      status IN ('FAILED','RUNNING').
--   4. Re-run.
--   5. Verify both _v2 CHECK clauses (CHECK_CLAUSE) and a zero precheck count.
-- Re-running this file without that flow is not "safe": the runner still
-- refuses a FAILED/RUNNING attempt.
--
-- Online DDL: on MySQL 8.4 `ADD CHECK` does not support
-- `ALGORITHM=INPLACE, LOCK=NONE` (error 1845 -> COPY, exclusive MDL, table
-- rebuild), while `DROP CHECK` is online. Therefore the two ADDs run in a
-- write-frozen maintenance window; see docs/ops.md "Migrating order/0022/0023"
-- for the UAT-as-prod and production windows and the rollback path.
--
-- 0023-1: refund_applications.group_no is the application-row discriminator.
-- Payment-level rows (payment/legacy_expired) carry no fulfillment group;
-- every other source is group-level and must name exactly one non-empty group.
-- This does NOT copy the 0017 source enum: the closed source set stays owned by
-- 0017's chk_refund_app_source, and this predicate only classifies the set
-- membership needed for the discriminator. Adding a source requires updating
-- 0017 and, if it changes which side of the discriminator it belongs to, the
-- membership set here -- never a copy of the full enum.

SELECT refund_id, source, group_no
FROM refund_applications
WHERE NOT (
  ((group_no IS NULL) = (source IN ('payment','legacy_expired')))
  AND (source IN ('payment','legacy_expired') OR group_no <> '')
)
LIMIT 100;

SET @has_r04_group_discriminator_v2 := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'refund_applications'
    AND CONSTRAINT_NAME = 'chk_refund_app_group_discriminator_v2'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_r04_group_discriminator_v2 = 0,
  'ALTER TABLE refund_applications ADD CONSTRAINT chk_refund_app_group_discriminator_v2 CHECK (((group_no IS NULL) = (source IN (''payment'',''legacy_expired''))) AND (source IN (''payment'',''legacy_expired'') OR group_no <> ''''))',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 0023-2: legacy_payment_resolution has a 24h resolution SLA. 0019 already
-- enforces expires_at >= entered_at; this replaces the lower bound with the
-- exact equality: expires_at MUST be entered_at + 24h on the database clock.
-- No tolerance band: the writer sets both values with NOW(3) in the same
-- statement (Addendum §2.3.2), so the equality is exactly reproducible. The
-- status machine (OPEN/RESOLVING -> terminal) is driven by the Order
-- expiry/compensation worker (T2), not by this CHECK: a DB constraint can only
-- reject a late deadline, it cannot move a row out of OPEN/RESOLVING.
SELECT order_no, state, entered_at, expires_at
FROM legacy_payment_resolution
WHERE expires_at <> DATE_ADD(entered_at, INTERVAL 24 HOUR)
LIMIT 100;

SET @has_r04_lpr_24h_v2 := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'legacy_payment_resolution'
    AND CONSTRAINT_NAME = 'chk_lpr_expiry_24h_v2'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_r04_lpr_24h_v2 = 0,
  'ALTER TABLE legacy_payment_resolution ADD CONSTRAINT chk_lpr_expiry_24h_v2 CHECK (expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR))',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Only after both v2 CHECKs exist are the old names dropped.
SET @has_r04_group_discriminator := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'refund_applications'
    AND CONSTRAINT_NAME = 'chk_refund_app_group_discriminator'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_r04_group_discriminator = 1,
  'ALTER TABLE refund_applications DROP CHECK chk_refund_app_group_discriminator',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @has_r04_lpr_24h := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'legacy_payment_resolution'
    AND CONSTRAINT_NAME = 'chk_lpr_expiry_24h'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_r04_lpr_24h = 1,
  'ALTER TABLE legacy_payment_resolution DROP CHECK chk_lpr_expiry_24h',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
