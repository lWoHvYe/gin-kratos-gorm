-- Archive terminal outbox rows before pruning them; the live table stays small
-- while dead-letter payloads remain auditable/replayable.
CREATE TABLE IF NOT EXISTS outbox_dead_letters (
  id BIGINT NOT NULL AUTO_INCREMENT,
  event_id VARCHAR(64) NOT NULL,
  topic VARCHAR(128) NOT NULL,
  aggregate_type VARCHAR(64) NOT NULL,
  aggregate_id VARCHAR(64) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  payload JSON NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1024) NOT NULL DEFAULT '',
  created_at DATETIME(3) NOT NULL,
  published_at DATETIME(3) NULL,
  dead_lettered_at DATETIME(3) NOT NULL,
  archived_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_outbox_dead_event_id (event_id),
  KEY idx_outbox_dead_archived (archived_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
