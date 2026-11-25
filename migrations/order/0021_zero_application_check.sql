-- change_id: PAY-SETTLE-01
-- C-05 migration 0021 (T1.4a) for SPEC-0002 v1.9 (approved 2026-11-24) AC-67/AC-68,
-- ADR-0007 Amendment 1 Addendum 1 (zero after-sale, accepted 2026-11-24) §7.
-- Owner: Order DB. Zero-amount after-sale execution ledger only.
--
-- Additive constraint only (no table/column/enum, no down migration):
--   rollback = keep the CHECK; a wrong manual write is corrected by a new
--   audited manual write, never by dropping the constraint or bulk-updating.
--   order/0017 is NOT modified.
--
-- `refund_applications(source='system_zero')` is the zero-amount execution
-- fact: it must always carry amount 0 with empty goods/freight composition and
-- must never be a payment-level (non-effective capture) row. Positive-amount
-- refunds keep the existing 0017 constraints unchanged.
--
-- Precheck: print the rows that would block the deploy before the ALTER; the
-- ALTER itself is the hard stop (a dirty row makes the statement fail).
SELECT refund_id, order_no, group_no, amount_cent, goods_cent, freight_cent, non_effective_capture
FROM refund_applications
WHERE source = 'system_zero'
  AND (amount_cent <> 0 OR goods_cent <> 0 OR freight_cent <> 0 OR non_effective_capture <> 0)
LIMIT 100;

-- Conditional so a re-run after a blocked/failed attempt is safe.
SET @has_zero_app_check := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'refund_applications'
    AND CONSTRAINT_NAME = 'chk_refund_app_system_zero'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_zero_app_check = 0,
  'ALTER TABLE refund_applications ADD CONSTRAINT chk_refund_app_system_zero CHECK (source <> ''system_zero'' OR (amount_cent = 0 AND goods_cent = 0 AND freight_cent = 0 AND non_effective_capture = 0))',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
