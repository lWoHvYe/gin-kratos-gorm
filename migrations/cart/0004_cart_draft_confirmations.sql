CREATE TABLE IF NOT EXISTS cart_draft_confirmations (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  draft_id VARCHAR(64) NOT NULL,
  cart_id BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_cart_draft_user (user_id, draft_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
