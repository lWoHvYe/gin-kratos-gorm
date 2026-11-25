-- change_id: PAY-SETTLE-01
-- C-05 follow-up for SPEC-0002 v1.10 (approved 2026-11-25, INV-49) and
-- ADR-0007 Rev 2.3 (accepted 2026-11-25) §6.2/§12.5.6:
-- `settlement_switch_approvals.target_mode='settled'` MUST carry a non-NULL
-- `expires_at`; `shadow` rows MAY keep NULL. Fulfillment DB only.
--
-- fulfillment/0010 already ships the table and is merged/applied elsewhere, so
-- the missing CHECK is added by this new migration instead of editing 0010
-- (checksum stability, ADR §12.5.6 "C-05 迁移必须包含该 CHECK").
--
-- DEPLOY GATE (fail-closed): an existing settled row without expires_at blocks
-- the deploy. It MUST be re-signed by the Director as a new/updated approval
-- row; no default/backfill is legal (ADR §12.5.6). A blocked run changes no row
-- and adds no constraint; the helper procedure is recreated by the next attempt.
--
-- Diagnostics first: every blocking row is printed to the migrate job log
-- before the hard stop.
SELECT approval_id, target_mode, status, expires_at, approved_at
FROM settlement_switch_approvals
WHERE target_mode = 'settled' AND expires_at IS NULL
LIMIT 100;

DROP PROCEDURE IF EXISTS c05_switch_expiry_precheck;
DELIMITER $$
CREATE PROCEDURE c05_switch_expiry_precheck()
BEGIN
  IF EXISTS (
    SELECT 1 FROM settlement_switch_approvals
    WHERE target_mode = 'settled' AND expires_at IS NULL
  ) THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'd27 precheck: settled approval with NULL expires_at; Director re-sign required (ADR-0007 Rev 2.3 12.5.6)';
  END IF;
END$$
DELIMITER ;
CALL c05_switch_expiry_precheck();
DROP PROCEDURE IF EXISTS c05_switch_expiry_precheck;

-- Conditional so a re-run after a blocked precheck is safe and so a second
-- deploy does not fail on an already-present CHECK.
SET @has_switch_expiry_check := (SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
  WHERE CONSTRAINT_SCHEMA = DATABASE()
    AND TABLE_NAME = 'settlement_switch_approvals'
    AND CONSTRAINT_NAME = 'chk_switch_settled_expiry'
    AND CONSTRAINT_TYPE = 'CHECK');
SET @ddl := IF(@has_switch_expiry_check = 0,
  'ALTER TABLE settlement_switch_approvals ADD CONSTRAINT chk_switch_settled_expiry CHECK (target_mode <> ''settled'' OR expires_at IS NOT NULL)',
  'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
