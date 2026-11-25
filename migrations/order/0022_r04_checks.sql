-- change_id: PAY-SETTLE-01
-- R0.4c migration for ADR-0007 Amendment 1 Addendum-r0.4 §2.2/§2.3 (accepted
-- 2026-11-26, DRAFT 0.3 r3). Owner: Order DB.
--
-- Branch B (in-place edit) per Addendum §2.3.3: the R0.4c-impl preflight on
-- 2026-11-26 found no schema_migrations row and no schema_migration_attempts
-- row for order/0022_r04_checks.sql in any reachable persistent environment
-- (k3s mall-data, k3s mall-uat-data, local mysql-unicorn; compose dev had no
-- MySQL volume). The file therefore was never applied anywhere and may be
-- corrected before its first apply. Checksum/ledger impact: no environment
-- holds a 0022 ledger checksum, so no runner checksum mismatch can exist; the
-- corrected file will be recorded on its first apply.
--
-- Additive constraints only. This file does NOT modify 0017/0018/0019/0020/0021;
-- it must run after them. No down migration (ADR §10): a wrong manual row is
-- corrected by a new audited write, never by dropping the constraint or
-- bulk-updating.
--
-- Failure/re-run contract (Addendum §2.5): MySQL DDL commits implicitly, so a
-- blocked or interrupted run can leave the first CHECK installed and the second
-- missing while the runner records the attempt as FAILED/RUNNING and refuses an
-- automatic replay. Operators MUST: (1) locate the row(s) from the precheck
-- output or error 3819; (2) repair data with an audited write only; (3) delete
-- the attempt row for this filename (status IN ('FAILED','RUNNING')) after
-- operator sign-off; (4) re-run; (5) verify both CHECK clauses and a zero
-- precheck count. A re-run is safe only through that explicit repair flow; it
-- is NOT unconditionally re-runnable.
--
-- A constraint with the expected name but different semantics is not detected
-- here (this file only checks existence by name/type). R0.4d2's verification
-- script must compare information_schema.CHECK_CONSTRAINTS.CHECK_CLAUSE; the
-- name is not proof of the semantics.
--
-- 0022-1: refund_applications.group_no is the application-row discriminator.
-- Payment-level rows (payment/legacy_expired) carry no fulfillment group;
-- group-level sources must name exactly one non-empty group. This is not the
-- group lifecycle (order_fulfillment_groups is the identity owner, 0011) and it
-- does not touch commitment composition (0018). It does NOT copy the 0017
-- source enum: the closed source set stays owned by 0017's chk_refund_app_source
-- and this predicate only classifies the set membership needed for the
-- discriminator. Adding a new source requires updating 0017 and, if it changes
-- which side of the discriminator it belongs to, this file too.

SELECT refund_id, source, group_no
FROM refund_applications
WHERE NOT (
  ((group_no IS NULL) = (source IN ('payment','legacy_expired')))
  AND (source IN ('payment','legacy_expired') OR group_no <> '')
)
LIMIT 100;

SET @has_r04_group_discriminator := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'refund_applications'
    AND CONSTRAINT_NAME = 'chk_refund_app_group_discriminator'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_r04_group_discriminator = 0,
  'ALTER TABLE refund_applications ADD CONSTRAINT chk_refund_app_group_discriminator CHECK (((group_no IS NULL) = (source IN (''payment'',''legacy_expired''))) AND (source IN (''payment'',''legacy_expired'') OR group_no <> ''''))',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 0022-2: legacy_payment_resolution has a 24h resolution SLA. 0019 already
-- enforces expires_at >= entered_at; this adds the exact equality: expires_at
-- MUST be entered_at + 24h on the database clock. A lower-bound (>=) would make
-- a 48h/25h deadline legal and let the queue stay OPEN indefinitely, violating
-- INV-46. There is deliberately no tolerance band: the writer sets both values
-- with NOW(3) in the same statement (Addendum §2.3.2), so the equality is
-- exactly reproducible. A client clock/default outside the writer is rejected.
-- The status machine (OPEN/RESOLVING -> terminal) is driven by the Order
-- expiry/compensation worker (T2), not by this CHECK: a DB constraint can only
-- reject a late deadline, it cannot move a row out of OPEN/RESOLVING.
SELECT order_no, state, entered_at, expires_at
FROM legacy_payment_resolution
WHERE expires_at <> DATE_ADD(entered_at, INTERVAL 24 HOUR)
LIMIT 100;

SET @has_r04_lpr_24h := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'legacy_payment_resolution'
    AND CONSTRAINT_NAME = 'chk_lpr_expiry_24h'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_r04_lpr_24h = 0,
  'ALTER TABLE legacy_payment_resolution ADD CONSTRAINT chk_lpr_expiry_24h CHECK (expires_at = DATE_ADD(entered_at, INTERVAL 24 HOUR))',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
