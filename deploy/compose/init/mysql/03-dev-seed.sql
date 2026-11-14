-- Development-only demo accounts.
--
-- This file is mounted by Docker Compose init
-- (deploy/compose/init/mysql/ -> /docker-entrypoint-initdb.d) and is NOT copied
-- into the Kubernetes migration image, so a fresh production database never
-- gets known credentials. Never reference these accounts outside local dev.
--
--   admin / admin123   (role admin)
--   demo  / user123    (role user)

USE `mall_identity`;

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
