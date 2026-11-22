-- ADR 0005 §18.6: shortage timing and refund projection.
ALTER TABLE order_shortage_cases
  ADD COLUMN detected_at DATETIME(3) NULL,
  ADD COLUMN notify_deadline_at DATETIME(3) NULL,
  ADD COLUMN refund_after_sale_no VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN refunded_at DATETIME(3) NULL,
  ADD COLUMN refund_state VARCHAR(24) NOT NULL DEFAULT 'NONE';

UPDATE order_shortage_cases
SET detected_at = COALESCE(detected_at, created_at),
    notify_deadline_at = COALESCE(notify_deadline_at, DATE_ADD(COALESCE(detected_at, created_at), INTERVAL 4 HOUR))
WHERE detected_at IS NULL;

UPDATE order_shortage_cases
SET choose_deadline_at = DATE_ADD(notified_at, INTERVAL 48 HOUR)
WHERE notified_at IS NOT NULL AND choose_deadline_at IS NOT NULL;
