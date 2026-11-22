-- ADR 0005 §6.4/§19.1 #12: per-line in-flight reservation must carry both
-- quantity and amount so refund/reject clears the exact pending amount.
ALTER TABLE after_sale_items
  ADD COLUMN pending_amount_cent BIGINT NOT NULL DEFAULT 0;
