-- DTM server storage schema, applied once on an empty data volume after
-- 00-databases.sql and 01-schemas.sh.
--
-- The DTM image ships only the server binary; it does not create its own
-- tables unless started with `-r` (which drops them). Keeping the DDL here
-- makes a fresh compose stack usable without a manual `dtm -r` step.
--
-- Sources: dtm-labs/dtm sqls/dtmsvr.storage.mysql.sql and
-- sqls/dtmcli.barrier.mysql.sql, adapted to idempotent CREATE TABLE IF NOT
-- EXISTS so re-running this script never drops transaction history.

USE `dtm`;

CREATE TABLE IF NOT EXISTS `trans_global` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `gid` VARCHAR(128) NOT NULL COMMENT 'global transaction id',
  `trans_type` VARCHAR(45) NOT NULL COMMENT 'saga | xa | tcc | msg',
  `status` VARCHAR(12) NOT NULL COMMENT 'prepared | submitted | aborting | succeed | failed',
  `query_prepared` VARCHAR(1024) NOT NULL COMMENT 'url to check for msg|workflow',
  `protocol` VARCHAR(45) NOT NULL COMMENT 'http | grpc | json-rpc',
  `create_time` DATETIME DEFAULT NULL,
  `update_time` DATETIME DEFAULT NULL,
  `finish_time` DATETIME DEFAULT NULL,
  `rollback_time` DATETIME DEFAULT NULL,
  `options` VARCHAR(1024) DEFAULT '' COMMENT 'TimeoutToFail, RequestTimeout, ...',
  `custom_data` VARCHAR(1024) DEFAULT '',
  `next_cron_interval` INT DEFAULT NULL,
  `next_cron_time` DATETIME DEFAULT NULL,
  `owner` VARCHAR(128) NOT NULL DEFAULT '',
  `ext_data` TEXT,
  `result` VARCHAR(1024) DEFAULT '',
  `rollback_reason` VARCHAR(1024) DEFAULT '',
  PRIMARY KEY (`id`),
  UNIQUE KEY `gid` (`gid`),
  KEY `owner` (`owner`),
  KEY `status_next_cron_time` (`status`, `next_cron_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `trans_branch_op` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `gid` VARCHAR(128) NOT NULL,
  `url` VARCHAR(1024) NOT NULL,
  `data` TEXT,
  `bin_data` BLOB,
  `branch_id` VARCHAR(128) NOT NULL,
  `op` VARCHAR(45) NOT NULL COMMENT 'action | compensate | try | confirm | cancel',
  `status` VARCHAR(45) NOT NULL COMMENT 'prepared | succeed | failed',
  `finish_time` DATETIME DEFAULT NULL,
  `rollback_time` DATETIME DEFAULT NULL,
  `create_time` DATETIME DEFAULT NULL,
  `update_time` DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `gid_uniq` (`gid`, `branch_id`, `op`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `kv` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `cat` VARCHAR(45) NOT NULL,
  `k` VARCHAR(128) NOT NULL,
  `v` TEXT,
  `version` BIGINT DEFAULT 1,
  `create_time` DATETIME DEFAULT NULL,
  `update_time` DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_k` (`cat`, `k`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Branch barrier shared by every service that runs DTM branch transactions.
USE `dtm_barrier`;

CREATE TABLE IF NOT EXISTS `barrier` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `trans_type` VARCHAR(45) DEFAULT '',
  `gid` VARCHAR(128) DEFAULT '',
  `branch_id` VARCHAR(128) DEFAULT '',
  `op` VARCHAR(45) DEFAULT '',
  `barrier_id` VARCHAR(45) DEFAULT '',
  `reason` VARCHAR(45) DEFAULT '',
  `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `create_time` (`create_time`),
  KEY `update_time` (`update_time`),
  UNIQUE KEY `gid_branch_op_barrier` (`gid`, `branch_id`, `op`, `barrier_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
