-- MySQL 8.0+ schema for gin-kratos-gorm.
-- Apply with: mysql -u root -p gin_kratos < migrations/001_init.sql
--
-- The application can also auto-migrate in local/dev mode
-- (data.database.auto_migrate: true), but production deployments should use
-- this file so schema changes are reviewed and versioned.

CREATE TABLE IF NOT EXISTS `users` (
  `id`            INT NOT NULL AUTO_INCREMENT,
  `created_at`    DATETIME(3) NULL,
  `updated_at`    DATETIME(3) NULL,
  `deleted_at`    DATETIME(3) NULL,
  `name`          VARCHAR(64) NOT NULL,
  `nickname`      VARCHAR(64) NULL,
  `desc`          VARCHAR(255) NULL,
  `phone`         VARCHAR(32) NULL,
  `password_hash` VARCHAR(255) NOT NULL DEFAULT '',
  `status`        INT NOT NULL DEFAULT 1 COMMENT '1 enabled, 0 disabled',
  `last_login_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_name` (`name`),
  KEY `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `roles` (
  `id`   INT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(32) NULL,
  `name` VARCHAR(64) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_roles_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_roles` (
  `id`      INT NOT NULL AUTO_INCREMENT,
  `user_id` INT NULL,
  `role_id` INT NULL,
  `status`  VARCHAR(8) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_roles_user_id` (`user_id`),
  KEY `idx_user_roles_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `product_spus` (
  `id`          BIGINT NOT NULL AUTO_INCREMENT,
  `created_at`  DATETIME(3) NULL,
  `updated_at`  DATETIME(3) NULL,
  `deleted_at`  DATETIME(3) NULL,
  `name`        VARCHAR(255) NOT NULL,
  `description` TEXT NULL,
  `category_id` BIGINT NULL,
  `brand_id`    BIGINT NULL,
  `main_image`  VARCHAR(512) NULL,
  `is_on_sale`  TINYINT(1) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_product_spus_category_id` (`category_id`),
  KEY `idx_product_spus_brand_id` (`brand_id`),
  KEY `idx_product_spus_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `product_skus` (
  `id`         BIGINT NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  `spu_id`     BIGINT NULL,
  `sku_code`   VARCHAR(64) NULL,
  `attr_value` JSON NULL,
  `price`      DECIMAL(10,2) NULL,
  `stock`      BIGINT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_product_skus_sku_code` (`sku_code`),
  KEY `idx_product_skus_spu_id` (`spu_id`),
  KEY `idx_product_skus_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `stock_logs` (
  `id`         VARCHAR(128) NOT NULL COMMENT 'deduct:<order_sn>:<sku_id> / restore:<order_sn>:<sku_id>',
  `order_sn`   VARCHAR(64) NULL,
  `sku_id`     BIGINT NOT NULL,
  `num`        INT NOT NULL,
  `action`     VARCHAR(16) NOT NULL,
  `created_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_stock_logs_order_sn` (`order_sn`),
  KEY `idx_stock_logs_sku_id` (`sku_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `orders` (
  `id`                BIGINT NOT NULL AUTO_INCREMENT,
  `created_at`        DATETIME(3) NULL,
  `updated_at`        DATETIME(3) NULL,
  `deleted_at`        DATETIME(3) NULL,
  `order_sn`          VARCHAR(64) NOT NULL,
  `user_id`           BIGINT NOT NULL,
  `status`            INT NOT NULL DEFAULT 1 COMMENT '1 pending payment, 2 pending shipment, 3 shipped, 4 completed, 5 cancelled, 6 closed',
  `total_amount`      DECIMAL(10,2) NULL,
  `pay_amount`        DECIMAL(10,2) NULL,
  `freight_amount`    DECIMAL(10,2) NULL,
  `receiver_name`     VARCHAR(64) NULL,
  `receiver_phone`    VARCHAR(32) NULL,
  `receiver_address`  VARCHAR(512) NULL,
  `remark`            VARCHAR(255) NULL,
  `pay_time`          DATETIME(3) NULL,
  `expire_at`         DATETIME(3) NULL,
  `cancel_reason`     VARCHAR(255) NULL,
  `client_request_id` VARCHAR(128) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_orders_order_sn` (`order_sn`),
  UNIQUE KEY `idx_orders_user_request` (`user_id`, `client_request_id`),
  KEY `idx_orders_user_id` (`user_id`),
  KEY `idx_orders_status` (`status`),
  KEY `idx_orders_expire_at` (`expire_at`),
  KEY `idx_orders_created_at` (`created_at`),
  KEY `idx_orders_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `order_items` (
  `id`         BIGINT NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  `order_id`   BIGINT NULL,
  `order_sn`   VARCHAR(64) NULL,
  `spu_id`     BIGINT NULL,
  `sku_id`     BIGINT NULL,
  `sku_name`   VARCHAR(255) NULL,
  `sku_pic`    VARCHAR(512) NULL,
  `price`      DECIMAL(10,2) NULL,
  `num`        INT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_order_items_order_id` (`order_id`),
  KEY `idx_order_items_order_sn` (`order_sn`),
  KEY `idx_order_items_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `outbox_events` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `event_id`       VARCHAR(64) NULL,
  `aggregate_type` VARCHAR(32) NULL,
  `aggregate_id`   VARCHAR(64) NULL,
  `event_type`     VARCHAR(64) NULL,
  `payload`        TEXT NULL,
  `status`         VARCHAR(16) NULL,
  `attempts`       INT NOT NULL DEFAULT 0,
  `last_error`     VARCHAR(1024) NULL,
  `available_at`   DATETIME(3) NULL,
  `created_at`     DATETIME(3) NULL,
  `published_at`   DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_outbox_event_id` (`event_id`),
  KEY `idx_outbox_aggregate` (`aggregate_type`, `aggregate_id`),
  KEY `idx_outbox_event_type` (`event_type`),
  KEY `idx_outbox_status` (`status`),
  KEY `idx_outbox_available_at` (`available_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Casbin GORM adapter also creates this table automatically; included here so
-- the schema is reproducible from a single file.
CREATE TABLE IF NOT EXISTS `casbin_rule` (
  `id`    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `ptype` VARCHAR(100) NULL,
  `v0`    VARCHAR(100) NULL,
  `v1`    VARCHAR(100) NULL,
  `v2`    VARCHAR(100) NULL,
  `v3`    VARCHAR(100) NULL,
  `v4`    VARCHAR(100) NULL,
  `v5`    VARCHAR(100) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_casbin_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
