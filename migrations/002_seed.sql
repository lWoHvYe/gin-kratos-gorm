-- Development seed data. Do NOT use these passwords in production.
-- Roles, demo accounts, sample products and baseline Casbin policies.

INSERT IGNORE INTO `roles` (`id`, `code`, `name`) VALUES
  (1, 'admin', '管理员'),
  (2, 'user', '普通用户');

-- admin123 / user123 (bcrypt, cost 10)
INSERT IGNORE INTO `users` (`id`, `created_at`, `updated_at`, `name`, `nickname`, `desc`, `password_hash`, `status`) VALUES
  (1, NOW(3), NOW(3), 'admin', '管理员', 'seed admin', '$2a$10$kbeq.fAUyBcH6fX4fRhoG.2GDgIysuSZORFBlJruc0ixF8mIvMq3u', 1),
  (2, NOW(3), NOW(3), 'demo', '演示用户', 'seed user', '$2a$10$22d5vF5PUGpU/e2F26UZ7ugdSzYMAKxdlHlaH2sS.4X6L4nW1c9iu', 1);

INSERT IGNORE INTO `user_roles` (`user_id`, `role_id`, `status`) VALUES
  (1, 1, '1'),
  (2, 2, '1');

INSERT IGNORE INTO `product_spus` (`id`, `created_at`, `updated_at`, `name`, `description`, `category_id`, `brand_id`, `main_image`, `is_on_sale`) VALUES
  (1, NOW(3), NOW(3), 'Kratos 机械键盘', '87 键热插拔机械键盘，适合开发者。', 1, 1, 'https://example.com/products/keyboard.png', 1),
  (2, NOW(3), NOW(3), 'Go 语言实战', '从入门到云原生微服务实践。', 2, 2, 'https://example.com/products/go-book.png', 1);

INSERT IGNORE INTO `product_skus` (`id`, `created_at`, `updated_at`, `spu_id`, `sku_code`, `attr_value`, `price`, `stock`) VALUES
  (1, NOW(3), NOW(3), 1, 'KB-RED-87', '{"轴体":"红轴"}', 399.00, 100),
  (2, NOW(3), NOW(3), 1, 'KB-BROWN-87', '{"轴体":"茶轴"}', 429.00, 80),
  (3, NOW(3), NOW(3), 2, 'BOOK-GO-01', '{"装帧":"平装"}', 89.00, 200),
  (4, NOW(3), NOW(3), 2, 'BOOK-GO-02', '{"装帧":"精装"}', 129.00, 50);

-- Role-based policies. Subjects are "r:<role code>" and are derived from JWT
-- claims; per-user grouping (u:<id> -> r:<role>) is optional but supported.
INSERT IGNORE INTO `casbin_rule` (`ptype`, `v0`, `v1`, `v2`) VALUES
  ('p', 'r:admin', '/api/users', '*'),
  ('p', 'r:admin', '/api/users*', '*'),
  ('p', 'r:admin', '/api/products', '*'),
  ('p', 'r:admin', '/api/products*', '*'),
  ('p', 'r:admin', '/api/orders', '*'),
  ('p', 'r:admin', '/api/orders*', '*'),
  ('p', 'r:user', '/api/users/me', 'GET'),
  ('p', 'r:user', '/api/products', 'GET'),
  ('p', 'r:user', '/api/products*', 'GET'),
  ('p', 'r:user', '/api/orders', '*'),
  ('p', 'r:user', '/api/orders*', '*');
