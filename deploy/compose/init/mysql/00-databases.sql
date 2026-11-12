-- Databases for the mall stack. Executed once by the mysql:8.4 entrypoint on
-- an empty data volume (see `make db-reset` to re-run).
--
-- Tables live in the per-service databases and are applied by 01-schemas.sh
-- from the mounted /migrations directory.

CREATE DATABASE IF NOT EXISTS mall_identity    CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_catalog     CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_inventory   CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_cart        CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_pricing     CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_order       CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_payment     CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_fulfillment CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_aftersales  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_seckill     CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS mall_admin       CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- DTM server storage (trans_global / trans_branch_op, created by DTM itself).
CREATE DATABASE IF NOT EXISTS dtm CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Shared branch-barrier database used by the DTM client. The order migration
-- also creates it defensively; keeping it here makes every branch service work
-- even when only a subset of migrations is applied manually.
CREATE DATABASE IF NOT EXISTS dtm_barrier CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
