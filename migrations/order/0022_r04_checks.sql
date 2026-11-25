-- change_id: PAY-SETTLE-01
-- R0.4c migration 0022 for ADR-0007 Rev 2.3 §6.2/§6.5 and
-- Addendum-r0.4 §2.2/§2.3. Owner: Order DB.
--
-- Additive constraints only. This file does NOT modify 0017/0018/0019; it must
-- run after them. No down migration (ADR §10): a wrong manual row is corrected
-- by a new audited write, never by dropping the constraint or bulk-updating.
-- The ALTERs are the hard stop: if a dirty row already exists, MySQL rejects
-- the statement (error 3819) and the deploy stops. The SELECTs below only print
-- the blocking rows to the migration log first. A blocked run changes nothing;
-- the conditional ALTERs make a re-run after cleanup safe.
--
-- 0022-1: refund_applications.group_no is the application-row discriminator.
-- Payment-level rows (payment/legacy_expired) carry no fulfillment group;
-- group-level sources must name exactly one group. This is not the group
-- lifecycle (order_fulfillment_groups is the identity owner, 0011) and it does
-- not touch commitment composition (0018).

SELECT refund_id, source, group_no
FROM refund_applications
WHERE NOT (
  (source IN ('payment','legacy_expired') AND group_no IS NULL)
  OR (source IN ('aftersale','tcc','system_shortage','system_zero')
      AND group_no IS NOT NULL AND group_no <> '')
)
LIMIT 100;

SET @has_r04_group_discriminator := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'refund_applications'
    AND CONSTRAINT_NAME = 'chk_refund_app_group_discriminator'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_r04_group_discriminator = 0,
  'ALTER TABLE refund_applications ADD CONSTRAINT chk_refund_app_group_discriminator CHECK ((source IN (''payment'',''legacy_expired'') AND group_no IS NULL) OR (source IN (''aftersale'',''tcc'',''system_shortage'',''system_zero'') AND group_no IS NOT NULL AND group_no <> ''''))',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 0022-2: legacy_payment_resolution has a 24h resolution SLA. 0019 already
-- enforces expires_at >= entered_at; this adds the 24h lower bound on DB time.
-- The writer must set expires_at = entered_at + 24h using the database clock;
-- a client clock or a default value is not acceptable. The expiry state machine
-- is driven by the Order expiry/compensation worker, not by a CHECK reading
-- NOW(): a DB constraint cannot move a row out of OPEN/RESOLVING.
SELECT order_no, state, entered_at, expires_at
FROM legacy_payment_resolution
WHERE expires_at < DATE_ADD(entered_at, INTERVAL 24 HOUR)
LIMIT 100;

SET @has_r04_lpr_24h := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'legacy_payment_resolution'
    AND CONSTRAINT_NAME = 'chk_lpr_expiry_24h'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_r04_lpr_24h = 0,
  'ALTER TABLE legacy_payment_resolution ADD CONSTRAINT chk_lpr_expiry_24h CHECK (expires_at >= DATE_ADD(entered_at, INTERVAL 24 HOUR))',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
