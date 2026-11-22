-- ADR 0005 phase D: controlled carriers, monotonic tracking, delivery evidence,
-- receipts, disputes and durable auto-confirm jobs.

ALTER TABLE shipments
  ADD COLUMN carrier_code VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN track_status VARCHAR(24) NOT NULL DEFAULT 'CREATED',
  ADD COLUMN shipped_at DATETIME(3) NULL,
  ADD COLUMN delivered_at DATETIME(3) NULL,
  ADD COLUMN delivery_source VARCHAR(24) NOT NULL DEFAULT '',
  ADD COLUMN exception_state VARCHAR(16) NOT NULL DEFAULT 'NONE',
  ADD COLUMN tracking_key VARCHAR(192) GENERATED ALWAYS AS (
    IF(tracking_no = '' OR carrier_code = '', NULL, CONCAT(carrier_code, ':', tracking_no))
  ) STORED,
  ADD UNIQUE KEY uk_shipments_tracking (tracking_key);

UPDATE shipments
SET carrier_code = LEFT(TRIM(carrier), 32)
WHERE carrier_code = '' AND carrier <> '';

UPDATE shipments
SET track_status = CASE
      WHEN status = 'SHIPPED' OR status = 'RECEIVED' THEN 'SHIPPED'
      ELSE 'CREATED'
    END,
    shipped_at = CASE
      WHEN status = 'SHIPPED' OR status = 'RECEIVED' THEN updated_at
      ELSE NULL
    END
WHERE track_status = 'CREATED';

CREATE TABLE IF NOT EXISTS carriers (
  code VARCHAR(32) NOT NULL,
  name VARCHAR(64) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  sort_order INT NOT NULL DEFAULT 0,
  webhook_secret_ref VARCHAR(128) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS shipment_track_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  shipment_no VARCHAR(64) NOT NULL,
  carrier_code VARCHAR(32) NOT NULL,
  tracking_no VARCHAR(128) NOT NULL,
  event_id VARCHAR(128) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT '',
  exception_type VARCHAR(24) NOT NULL DEFAULT '',
  occurred_at DATETIME(3) NOT NULL,
  received_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  raw_json JSON NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_shipment_track_event (carrier_code, tracking_no, event_id),
  KEY idx_shipment_track_events_shipment (shipment_no, occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS delivery_evidence (
  id BIGINT NOT NULL AUTO_INCREMENT,
  shipment_no VARCHAR(64) NOT NULL,
  source VARCHAR(24) NOT NULL,
  evidence_ref VARCHAR(128) NOT NULL,
  occurred_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_delivery_evidence (shipment_no, source, evidence_ref),
  KEY idx_delivery_evidence_shipment (shipment_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS shipment_receipts (
  shipment_no VARCHAR(64) NOT NULL,
  source VARCHAR(24) NOT NULL,
  evidence_ref VARCHAR(128) NOT NULL DEFAULT '',
  confirmed_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (shipment_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS delivery_auto_confirm_jobs (
  shipment_no VARCHAR(64) NOT NULL,
  reason VARCHAR(24) NOT NULL,
  due_at DATETIME(3) NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  lease_owner VARCHAR(64) NOT NULL DEFAULT '',
  lease_until DATETIME(3) NULL,
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1024) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (shipment_no),
  KEY idx_delivery_auto_confirm_due (state, due_at, lease_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS delivery_disputes (
  id BIGINT NOT NULL AUTO_INCREMENT,
  shipment_no VARCHAR(64) NOT NULL,
  order_no VARCHAR(64) NOT NULL,
  user_id BIGINT NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'OPEN',
  reason VARCHAR(512) NOT NULL DEFAULT '',
  resolution_reason VARCHAR(512) NOT NULL DEFAULT '',
  resolved_by VARCHAR(64) NOT NULL DEFAULT '',
  deadline_at DATETIME(3) NOT NULL,
  opened_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  resolved_at DATETIME(3) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_delivery_disputes_open (shipment_no, status),
  KEY idx_delivery_disputes_order (order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS shipment_operation_cases (
  id BIGINT NOT NULL AUTO_INCREMENT,
  shipment_no VARCHAR(64) NOT NULL,
  exception_type VARCHAR(24) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'OPEN',
  responsibility VARCHAR(24) NOT NULL DEFAULT '',
  resolved_by VARCHAR(64) NOT NULL DEFAULT '',
  resolution VARCHAR(512) NOT NULL DEFAULT '',
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  resolved_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_shipment_operation_case (shipment_no, exception_type),
  KEY idx_shipment_operation_cases_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS shipment_change_logs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  shipment_no VARCHAR(64) NOT NULL,
  field VARCHAR(32) NOT NULL,
  old_value VARCHAR(255) NOT NULL DEFAULT '',
  new_value VARCHAR(255) NOT NULL DEFAULT '',
  reason VARCHAR(255) NOT NULL DEFAULT '',
  operator VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_shipment_change_logs_shipment (shipment_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS carrier_audit_logs (
  id BIGINT NOT NULL AUTO_INCREMENT,
  carrier_code VARCHAR(32) NOT NULL,
  action VARCHAR(32) NOT NULL,
  operator VARCHAR(64) NOT NULL DEFAULT '',
  reason VARCHAR(255) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_carrier_audit_logs_carrier (carrier_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Legacy shipped parcels get a T+7 job only when they have no track event yet;
-- the worker re-checks the parcel and dispute state before confirming.
INSERT INTO delivery_auto_confirm_jobs (shipment_no, reason, due_at, state)
SELECT s.shipment_no, 'T7_NO_TRACK', DATE_ADD(COALESCE(s.shipped_at, s.updated_at), INTERVAL 7 DAY), 'PENDING'
FROM shipments s
WHERE s.status = 'SHIPPED'
  AND s.track_status IN ('CREATED', 'SHIPPED')
  AND NOT EXISTS (SELECT 1 FROM delivery_auto_confirm_jobs j WHERE j.shipment_no = s.shipment_no);
