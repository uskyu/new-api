# Quick Refund（管理员快捷退款）开发交接

> 当前状态：**WIP 交接版本，不可直接部署生产**
> 开发分支：`feat/quick-refund`
> 基线分支：`HelpSelf-latest`
> 仓库：`uskyu/new-api`

## 1. 原始需求

在管理员可见权限范围内新增“快捷退款”模块：

1. 管理员选择时间范围。
2. 选择一个或多个渠道。
3. 可继续选择对应渠道/消费记录中的一个或多个模型；不选模型表示渠道全部模型。
4. 选择退款比例（快捷比例和自定义 1%～100%）。
5. 根据真实消费日志，向使用这些渠道/模型的用户退款。
6. 执行前必须预览命中记录、涉及额度、跳过记录及渠道/模型汇总。
7. 执行时需要管理员权限、高风险二次验证、限流、完整审计和幂等保护。
8. 用户应能在自己的使用记录中看到退款。
9. 第一阶段只自动处理可明确识别为钱包计费的消费；订阅原路退款后续实现。

## 2. 已确定的架构与安全原则

- 查询依据是 `logs` 中的真实消费记录，不依赖渠道当前 `Models` 配置。
- 源记录必须满足 `type=LogTypeConsume`、`quota>0`、时间/渠道/模型条件。
- 第一阶段仅处理 `Other.billing_source == "wallet"`；来源缺失、未知或 subscription 一律跳过，禁止猜测。
- 每条退款保留 `source_log_id`，不能只按用户汇总加余额。
- 使用批次幂等键，避免请求重试导致重复退款。
- 同一源消费跨批次累计成功退款不得超过原始 `quota`。
- 钱包退款与有限额度 Token 恢复在同一主库事务内完成；Token 缺失时回滚钱包调整。
- `DB` 与 `LOG_DB` 可能分离；主库退款明细是状态真源，退款日志不作为余额事务的唯一依据。
- `users.used_quota`、`channels.used_quota` 保留毛消费口径，退款通过 `LogTypeRefund` 单独体现。
- 项目必须兼容 SQLite、MySQL、PostgreSQL；当前只完成 SQLite 自动化实测。

## 3. 已完成内容

### 3.1 后端模型与迁移

新增：

- `model/refund.go`
- `model/refund_test.go`

数据模型：

- `RefundBatch`：批次筛选条件、比例、操作员、状态、统计、幂等键。
- `RefundItem`：源日志、用户、渠道、模型、Token、源额度、退款额度、状态和错误。

迁移已接入：

- `model/main.go:migrateDB`
- `model/main.go:migrateDBFast`

### 3.2 后端能力

已实现：

- 时间、多个渠道、多个模型筛选。
- 退款比例校验（1～100）。
- 渠道/模型选项读取。
- 退款预览及按渠道、模型汇总。
- 只处理明确钱包消费，未知/订阅来源跳过。
- 批次创建、列表、详情。
- `idempotency_key` 防止同请求重复退款。
- 跨批次累计退款上限。
- 用户钱包余额恢复。
- 有限额度 Token 的 `remain_quota` 恢复、`used_quota` 回退。
- Token 缺失时主库事务回滚。
- 成功后写入 `LogTypeRefund`，关联批次和源消费日志。
- 普通用户读取日志时剥离 `admin_info`。

### 3.3 API

新增 `controller/refund.go`，路由位于 `router/api-router.go`：

```text
GET  /api/refund/admin/options
POST /api/refund/admin/preview
POST /api/refund/admin/batches
GET  /api/refund/admin/batches
GET  /api/refund/admin/batches/:id
```

权限：

- 整组使用 `AdminAuth()`。
- 创建退款增加 `CriticalRateLimit()` 和 `SecureVerificationRequired()`。

### 3.4 前端 WIP

已创建但尚未完成构建验收：

```text
web/default/src/features/quick-refund/
  api.ts
  types.ts
  index.tsx
  lib/schema.ts
  components/refund-history.tsx
web/default/src/routes/_authenticated/quick-refund/index.tsx
```

已修改：

- 管理员侧边栏快捷退款入口。
- URL 到侧边栏权限模块映射。
- 使用日志退款类型、列显示和详情弹窗的部分适配。

### 3.5 测试结果

在远程高性能测试机、Go 1.25 Docker 环境中验证：

```text
退款专项测试：PASS
model 包：PASS
controller 包：PASS
```

专项覆盖：

- 参数边界。
- 时间、渠道、模型过滤。
- 明确钱包来源限制。
- 退款取整与跳过。
- 幂等请求。
- 跨批次累计上限。
- 钱包和有限 Token 同步调整。
- Token 缺失事务回滚。
- 用户日志隐藏管理员信息。

同时修复了原基线已有测试 `TestListModelsTokenLimitIncludesTieredBillingModel` 的隔离问题：补充真实中间件会提供的用户组上下文，并初始化独立测试数据库；未修改该功能的生产逻辑。

## 4. 未完成内容 / 已知限制

### 4.1 上线前阻塞项

1. **前端尚未执行并通过**：
   - `bun run typecheck`
   - `bun run lint`
   - `bun run build`
   - `bun run i18n:sync`
2. 前后端 API 字段、分页结构、时间单位需要逐项核对。
3. 管理员页面尚未进行浏览器真实交互验收。
4. 用户端退款日志显示尚未在浏览器验证。
5. Controller 接口专项测试尚不完整：权限、非法输入、安全验证、分页、详情等。
6. MySQL、PostgreSQL 尚未进行实际迁移和退款事务测试。
7. Redis/多节点下用户和 Token 缓存刷新尚未验证。
8. 并发退款压力测试尚未完成。
9. 当前 `CreateAndRunRefund` 同步执行，命中大量日志时可能导致 HTTP 超时。
10. `DB` 成功、`LOG_DB` 日志失败时目前没有正式 Outbox 自动重试 Worker。
11. 第一阶段不支持订阅额度原路退款；未知来源和 subscription 会跳过。
12. 当前扫描上限 `RefundScanLimit=10000`，大范围退款需要缩小条件或后台分页任务。

### 4.2 生产化建议

优先把同步执行改成：

```text
创建批次/快照 -> 后台 worker 分批认领 -> 事务退款 -> 进度查询 -> 失败项重试
```

并新增：

- `pending/processing/success/failed/skipped` 明确状态机。
- 条件更新或行锁认领，支持多实例 worker。
- 退款日志 Outbox，主库提交后异步写 `LOG_DB`。
- 失败项重试接口。
- MySQL/PostgreSQL/SQLite 三数据库集成测试。

## 5. 下一位开发者建议执行顺序

1. 拉取并切换分支：

```bash
git fetch origin
git checkout feat/quick-refund
git pull --ff-only origin feat/quick-refund
```

2. 先阅读：

```text
docs/plans/quick-refund-handoff.md
model/refund.go
model/refund_test.go
controller/refund.go
router/api-router.go
web/default/src/features/quick-refund/
```

3. 先完成前端 API 契约核对与 typecheck/build，不要直接部署。
4. 补 Controller 权限和输入测试。
5. 用 MySQL/PostgreSQL 容器跑迁移、预览、执行、幂等、回滚测试。
6. 处理缓存失效和多节点一致性。
7. 将同步批次执行改为后台任务，增加失败重试与 Outbox。
8. 浏览器实开管理员页面和用户日志页面，保存可见 UI 证据。
9. 完成独立资金安全代码审查后，再考虑合并回 `HelpSelf-latest`。

## 6. 禁止事项

- 不要把来源未知的历史记录默认当成钱包消费。
- 不要按用户汇总后直接循环 `IncreaseUserQuota`。
- 不要移除幂等键或累计退款上限。
- 不要把订阅消费退进钱包。
- 不要假设 `DB == LOG_DB`。
- 不要在测试环境执行真实生产退款。
- 在前端构建、三数据库测试和独立审查完成前，不要直接合并部署。

## 7. 当前交接结论

这是一个**可供下一位开发者继续的 WIP 分支**：后端钱包退款核心和 SQLite 自动化测试已经成型，前端已留下可继续完善的页面骨架；但生产化、三数据库、缓存、后台任务和完整 UI 验收仍需完成。
