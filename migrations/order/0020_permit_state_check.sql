-- change_id: PAY-SETTLE-01
-- C-05 migration draft (NOT approved) for SPEC-0002 v1.8 (approved 2026-11-24)
-- and ADR-0007 Amendment 1 (accepted 2026-11-24, Revision 1.0) §7.2 (D-29) and
-- §10. SPEC-0002 v1.7 §2/§5.2 remains the permit enum authority.
--
-- DEPLOY GATE (fail-closed): persisted permit states are only
-- NONE / CONSUMED / REVOKED. GRANTED is NOT a legal state. This file refuses to
-- add the CHECK while any other value exists; it never maps rows automatically
-- (mapping GRANTED->NONE allows double shipping, ->CONSUMED forges an
-- authorization). Run the read-only precheck below first, classify every dirty
-- row with evidence and an audit record (operator/time/evidence/old/new), then
-- re-run. A blocked run changes no permit/order row; the helper procedure is
-- recreated by the next attempt and can be dropped manually.
--
--   SELECT order_no, fulfillment_group_no, state, shipment_no, after_sale_no
--   FROM order_shipping_permits
--   WHERE state NOT IN ('NONE','CONSUMED','REVOKED')
--   LIMIT 100;
--
-- Additive only (a constraint, no column), no down migration (ADR §7.2/§10):
--   rollback = keep the CHECK; a wrong manual classification is corrected by a
--   new audited manual write, never by dropping the constraint or bulk-updating.
--
-- Diagnostics first: the rows that block the deploy are printed to the migrate
-- job log before the hard stop.
SELECT order_no, fulfillment_group_no, state, shipment_no, after_sale_no
FROM order_shipping_permits
WHERE state NOT IN ('NONE','CONSUMED','REVOKED')
LIMIT 100;

DROP PROCEDURE IF EXISTS c05_permit_state_precheck;
DELIMITER $$
CREATE PROCEDURE c05_permit_state_precheck()
BEGIN
  IF EXISTS (
    SELECT 1 FROM order_shipping_permits
    WHERE state NOT IN ('NONE','CONSUMED','REVOKED')
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'd29 precheck: order_shipping_permits.state outside NONE/CONSUMED/REVOKED; stop deploy, classify per ADR-0007 Amd1 7.2';
  END IF;
END$$
DELIMITER ;
CALL c05_permit_state_precheck();
DROP PROCEDURE IF EXISTS c05_permit_state_precheck;

-- Conditional so a re-run after a blocked precheck is safe.
SET @has_permit_state_check := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'order_shipping_permits'
    AND CONSTRAINT_NAME = 'chk_permit_state'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_permit_state_check = 0,
  'ALTER TABLE order_shipping_permits ADD CONSTRAINT chk_permit_state CHECK (state IN (''NONE'',''CONSUMED'',''REVOKED''))',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
