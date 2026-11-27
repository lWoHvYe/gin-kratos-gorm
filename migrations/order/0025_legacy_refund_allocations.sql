-- AC-60: idempotent legacy:<order_no>:<seq> allocation for a historical refund
-- with no after-sale number. The caller supplies client_request_id.
CREATE TABLE IF NOT EXISTS legacy_refund_allocations (
  order_no      VARCHAR(64)  NOT NULL,
  request_id    VARCHAR(128) NOT NULL,
  refund_id     VARCHAR(128) NOT NULL,
  created_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (order_no, request_id),
  UNIQUE KEY uk_legacy_refund_allocation_refund (refund_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
