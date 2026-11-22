-- ADR 0005 phase A/B: line-level paid amounts and order-level freight shares.
-- Quote.freight_cent remains the single user-facing freight; group freight is
-- an internal allocation whose sum must equal it (INV-20).

ALTER TABLE quote_lines
  ADD COLUMN line_paid_cent BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN unit_paid_cent BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN split_seq INT NOT NULL DEFAULT 0;

ALTER TABLE quote_groups
  ADD COLUMN freight_share_cent BIGINT NOT NULL DEFAULT 0;

UPDATE quote_lines
SET line_paid_cent = price_cent * quantity - discount_cent,
    unit_paid_cent = FLOOR((price_cent * quantity - discount_cent) / quantity)
WHERE line_paid_cent = 0 AND quantity > 0;

UPDATE quote_groups
SET freight_share_cent = freight_cent
WHERE freight_share_cent = 0;
