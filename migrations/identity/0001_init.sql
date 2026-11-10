CREATE TABLE IF NOT EXISTS users (
  id BIGINT NOT NULL AUTO_INCREMENT,
  username VARCHAR(64) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  nickname VARCHAR(64) NOT NULL DEFAULT '',
  phone VARCHAR(32) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS roles (
  id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(32) NOT NULL,
  name VARCHAR(64) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_roles_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_roles (
  user_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  PRIMARY KEY (user_id, role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_addresses (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  receiver VARCHAR(64) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  province VARCHAR(64) NOT NULL DEFAULT '',
  city VARCHAR(64) NOT NULL DEFAULT '',
  district VARCHAR(64) NOT NULL DEFAULT '',
  detail VARCHAR(255) NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_addresses_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS refresh_tokens (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  token_hash VARCHAR(128) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  revoked_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_refresh_hash (token_hash),
  KEY idx_refresh_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO roles (id, code, name)
SELECT * FROM (SELECT 1 AS id, 'admin' AS code, 'Administrator' AS name
  UNION ALL SELECT 2, 'user', 'Customer') AS seed
WHERE NOT EXISTS (SELECT 1 FROM roles);

-- Development accounts: admin/admin123, demo/user123.
-- These are seed-only credentials; rotate/remove them outside local dev.
INSERT IGNORE INTO users (id, username, password_hash, nickname, status)
SELECT * FROM (
  SELECT 1 AS id, 'admin' AS username,
    '$2a$10$kbeq.fAUyBcH6fX4fRhoG.2GDgIysuSZORFBlJruc0ixF8mIvMq3u' AS password_hash,
    'Administrator' AS nickname, 'ACTIVE' AS status
  UNION ALL
  SELECT 2, 'demo',
    '$2a$10$22d5vF5PUGpU/e2F26UZ7ugdSzYMAKxdlHlaH2sS.4X6L4nW1c9iu',
    'Demo Customer', 'ACTIVE'
) AS seed
WHERE NOT EXISTS (SELECT 1 FROM users);
INSERT IGNORE INTO user_roles (user_id, role_id)
SELECT * FROM (SELECT 1 AS user_id, 1 AS role_id UNION ALL SELECT 2, 2) AS seed
WHERE NOT EXISTS (SELECT 1 FROM user_roles);
