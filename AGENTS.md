# AGENTS.md

本文件是仓库中 AI Agent 的行为规范，每次会话自动加载。这里只保留长期有效的约束与
入口；架构、契约、端口、事件和排障细节以 docs/ 为准，不要在本文件重复。

## 项目

Go 1.27（go.mod 1.27.0，toolchain 1.27.1）微服务商城：Kratos v3 gRPC 服务 + Gin BFF,
MySQL 事务核心用 database/sql, GORM 只做读侧, NATS JetStream transactional outbox,
DTM saga/TCC, Redis（seckill 准入、AI 会话/限流/RAG）, K8s + Istio Ambient,
Docker Compose 为本地回退。

布局: cmd/<services>、internal/services/<services>、internal/platform、
api/proto/mall/v1、api/gen（已提交）、migrations/<service>、frontend/、deploy/。

## 先读什么

- 实现契约（表结构、事件、DTM、错误）: docs/contracts.md
- 架构取舍（Wire、database/sql vs GORM）: docs/architecture.md
- AI 助手、RAG 与模型配置: docs/ai-design.md
- 部署、故障恢复与运行手册: docs/ops.md
- 仅并行多 agent 开发时读 docs/ownership.md（文件归属与契约上报流程）。

docs/handoff.md 是历史会话交接，可能过期，不作为规范或必读入口。

## 分析行为（强制）

1. 证据链：结论必须给出 '文件:行号' 与实际值；禁止"大概/应该/可能/类似"。
2. 读取优先：分析某方法前，先读完整实现、调用方和 import 来源；禁止凭片段或记忆下结论。
3. 假设必验：需要假设时先写明"待验证"，读代码确认后才能作为后续推理依据。
4. 不确定即暂停：找不到对应数据时标 '[不确定]' 并指出需读哪个文件确认。
5. 交叉验证：关键数字必须能追溯到具体记录或查询；确认没有跨表/跨域混用；
   缺失数据要写进结论；不要用单一数字支撑结论。

## 领域铁律（违反即错）

- 新表：金额列一律 '_cent BIGINT'，状态列一律 VARCHAR；可变聚合必须有 version 与
  created_at/updated_at（明细/日志表可省略 updated_at）。
- DTM Saga 分支必须走 'dtm.Branch'，业务写与 'outbox.Append' 必须共用同一个 '*sql.Tx'。
- 仅限 DTM 分支的方法（order 的 CreateOrderInner / CancelOrderInner）在非 DTM 上下文必须
  返回 UNAVAILABLE；双写路径（inventory 的 write、pricing/payment 的 inTx）无分支元数据时
  走本地事务，供消费者/TCC/直连调用，必须幂等。
- gid 固定 'order-create:<order_no>' / 'payment-confirm:<payment_no>'。
  order-cancel 不是 DTM gid：取消是本地状态迁移 + shop.order.cancelled.v1 outbox 补偿。
- 事件 subject 为 'shop.<domain>.<event>.v1'，必须经 outbox 在同一事务内写入；
  调用 outbox.Append 必须传业务 ctx，替换 ctx 会丢失 traceparent 链路关联。
- 事务核心（order/inventory/payment/pricing）保持显式 SQL 与条件 UPDATE。
- 服务错误一律用 'apperr'；'grpcx' 把非 apperr 错误压成 INTERNAL（含已带 gRPC code 的
  status.Error）；请求路径不得 panic。
- 改 api/proto 后必须重新生成并提交 api/gen（'make proto'，需要 protoc）；
  全部服务共享单一 api/proto/mall/v1/mall.proto，改任一 service 会触发全量重生成；
  api/gen 是生成物，不得手改。
- 改 wire provider 后必须 'make wire' 重生成并提交 cmd/*/wire_gen.go（不得手改；verify
  不检查此项）。
- skus.stock 只是展示投影，真实库存在 inventory，经 shop.inventory.stock.changed.v1 投影。
- seckill 订单号必须用 id.OrderForSeckill(tokenHash) 由资格令牌哈希确定性派生，禁止
  id.Order()；JetStream 可能重复投递，确定性订单号让第二次投递幂等命中已有订单与库存预留。

## AI 子系统约束

- `ai` 是可选面：AI、模型或 RAG 故障不得影响浏览、下单、支付；网关在禁用时隐藏入口。
- 工具默认只读；写操作必须"草案 → 人工确认 → 调用原业务 RPC"，不得直接改库。
- 工具身份只能来自服务端请求上下文（BFF 从 JWT 注入），不得采用模型生成的 user_id；
  admin 标记只能由 admin BFF 设置。
- 模型/embedding key 只能来自 Secret（如 mall-ai-secrets），不得写入 values、代码或文档；
  secret/ 已被忽略。
- 模型与 embedding provider 由 AI_*/EMBEDDING_* 配置；RAG 索引在启动时重建，商品变更
  尚未增量同步，不得宣称实时。

## 静态守卫（make verify 一环，违反即失败）

arch — scripts/check-architecture.sh:
- 服务代码不得 import 其它服务包；跨服务调用必须走生成的 gRPC client。
- internal/platform 不得依赖 internal/services。
- GORM 只允许出现在 internal/services/catalog/gorm_repo.go 与
  internal/services/cart/gorm_repo.go；其它位置引入 GORM 或 sqlite 依赖即失败。
- 禁止 MustGenGid / MustGetDtmClient；DTM gid 必须由业务键确定性派生。

migrations-check — scripts/check-migrations.sh:
- 迁移文件必须使用 4 位数字前缀；同一服务内 >=0008 的前缀不得重复，<=0007 的重复保持
  原样（ledger 按 '<service>/<filename>' 记账，重编号会让已应用的迁移重跑）。
- outbox 相关迁移只能放在 0001_init.sql 已建 outbox_events 表的服务库。

dtm-contract — scripts/check-dtm-contract.sh:
- Go 代码不得出现 'discovery:///' 或 'GRPC_ENDPOINT'，DTM 配置不得启用 etcd
  MicroService 注册；跨服务与 DTM 地址用显式 URL
  （如 inventory:9003/mall.v1.InventoryService）。

## 验证

- 单元测试不依赖 MySQL/NATS/etcd/DTM/Redis: 'make test'。
- 全量检查: 'make verify'（组成见 Makefile 的 verify 目标）。
- AI 改动: 'go test ./internal/services/ai'；RAG 集成测试需要 Redis 8（带 search 模块）并设置
  TEST_REDIS_ADDR；真实模型调用不属于单测。
- 改 frontend/ 后跑 'pnpm -C frontend typecheck' 与 'pnpm -C frontend build'；改 BFF 契约后
  'pnpm -C frontend generate:api' 并提交生成物。
- 需要真实依赖的（test-integration / e2e-phase15 / concurrency-phase15 / chaos / k8s-*/
  dtm-smoke）不要假设可用。
- 无本地 Go/K8s/DB（含 Windows 无 bash/make）时只做静态修改，须列出未验证项，不得声称
  已运行。

## 禁止

- 单节点 dev 集群不得跑 100k/1M RPS 压测。
- 文档与代码冲突时以代码为准，并在回复中指出文档漂移。
- 一律使用仓库相对路径，不要写死本机绝对路径。
