# V2.3 复测问题修复与证据报告（2026-11-27）

**文档版本**：V1.0（2026-11-27）\
**性质**：V2.3 复测问题的证据归档与 P0 修复范围；不是永久合同清单。\
**当前生产基线**：`harness-api-20261127-7868b83`（2026-11-27 已部署；P0 修复版本）。\
**A3 当前口径**：截至 2026-11-27 实测 `catalog_version=v12`，`chat 30 / video 49 / image 10 / plan 17 / audio 0`；以实际接口响应为准。

---

## 1. TVC 四笔订单与八次提交的归档口径

### 1.1 结论

TVC 侧提供了 4 个 Harness 订单 UUID：

```text
2714d043-2dc1-48c4-95a3-a92cf5b61972
78f19973-62b8-4707-b436-17660ac8b073
0bc76132-e9c3-4cf3-bc19-601adc2780bd
77b18787-3f4a-4d95-9706-ffd31e4e5db9
```

这四个 UUID 在网关的 `harness_settlements`、`harness_ledger_events`、`logs`、`tasks` 中均无记录。原因是当时网关只持久化自己的网关 Request ID，没有把 Harness 侧 UUID 写入数据库；因此**不能声称四个 Harness UUID 已全部在网关侧查到**。

可归档的客观事实是：**2026-11-24 20:13:13–20:13:19，网关共收到 8 次 `POST /v1/videos`**，同一模型 `seedance2-fast-720p-once-1`、同一 token `codeplanvip`，每次按 plan 池顺序尝试渠道 69 → 80 → 82。

### 1.2 八个网关 Request ID

| # | 网关 Request ID | 时间（UTC+8） | 最终状态 |
|---:|---|---|---|
| 1 | `202611241213133535722668268d9d6ftvyn2qe` | 20:13:13 | HTTP 500 |
| 2 | `202611241213149936346428268d9d6ykireKln` | 20:13:15 | HTTP 500 |
| 3 | `202611241213158242695958268d9d6Xebhge1H` | 20:13:16 | HTTP 500 |
| 4 | `202611241213165356182688268d9d6oKQIFOcR` | 20:13:16 | HTTP 500 |
| 5 | `202611241213172544507478268d9d64i6RBSYj` | 20:13:17 | HTTP 500 |
| 6 | `202611241213181159493288268d9d6R3JMK06O` | 20:13:18 | HTTP 500 |
| 7 | `202611241213188538244238268d9d6BLnBD3DF` | 20:13:19 | HTTP 500 |
| 8 | `202611241213194691396888268d9d6HsU781YD` | 20:13:19 | HTTP 500 |

每次的三个渠道结果一致：

| 渠道 | 分组 | 结果 |
|---:|---|---|
| 69 | pai-seedance-按次 | 上游 Cloudflare `503 Service Temporarily Unavailable` |
| 80 | pai-wan3.0-按次 | 上游 `model_not_found`；该分组只有 2 个 `wan3` 按次模型 |
| 82 | video-line-v1-once | 上游 `model_not_found`；该分组只有 6 个 `gemini-omni` 模型 |

服务端日志每次都有 `retry: 69->80->82`，可作为“同模型同池切换渠道”的记录。

### 1.3 费用与账本

- 每次提交都发生临时预扣 **¥0.52**。
- 8 次合计临时预扣 **¥4.16**。
- 8 次均在失败后执行退款，**最终净扣费 ¥0**。
- 数据库该 10 秒窗口 `tasks=0`。
- 数据库 `logs` 中 24 条 `type=5` 渠道错误日志（8×3），全部 `quota=0`。
- 该窗口 `harness_settlements` 的 4 条记录属于同时间窗内无关的 `deepseek-flash` 调用，不是 TVC 订单。
- TVC 订单没有正式 B1 usage 事件和 B3 cost 事件。

对外表述应为：

> 八次请求均发生过临时预扣，但失败后已全部退款，最终没有实际扣费；未创建视频任务，也没有 B1/B3 正式账本记录。

### 1.4 逐 UUID 对应说明

如果必须把 4 个 Harness UUID 与 8 个网关 Request ID 一一对应，需要 Harness 提供每个 UUID 的提交时间（毫秒级）或对应网关 Request ID。网关侧目前只能按时间窗、模型、token 和 24 条渠道日志归档，不能从数据库反查出 UUID 映射。

---

## 2. 目录与 22+7 停用清单

### 2.1 当前 A3 实测

截至 2026-11-27，生产 A3：

| 池 | 模型数 | `catalog_version` |
|---|---:|---:|
| `chat` | 30 | v12 |
| `video` | 49 | v12 |
| `image` | 10 | v12 |
| `plan` | 17 | v12 |
| `audio` | 0 | v12 |

`chat 30` 是 2026-11-26 上游同步后的当前值。2026-11-25 对齐报告中的 `chat 26` 是同步前快照；11-26 新增了 `deepseek-v4-pro-0813`、`deepseek-v4.1-flash-1017`、`gpt-6.2-astra`、`gpt-6.2-sol` 四个 chat 模型。两个数字是不同时间点，不是同一口径的矛盾。

### 2.2 TA Seedance Token 22 档（旧渠道 66，当前已移除）

这 22 个模型在 2026-11-25 对齐前声明于渠道 66（`TA-seedance分组`），对齐后渠道 66 为空，当前 A3 `video` / `plan` 均不包含：

```text
seedance2-480p-token-no-video
seedance2-480p-token-with-video
seedance2-720p-token-no-video
seedance2-720p-token-with-video
seedance2-1080p-token-no-video
seedance2-1080p-token-with-video
seedance2-4k-token-no-video
seedance2-4k-token-with-video
seedance2-mini-480p-token-no-video
seedance2-mini-480p-token-with-video
seedance2-mini-720p-token-no-video
seedance2-mini-720p-token-with-video
seedance2-fast-480p-token-no-video
seedance2-fast-480p-token-with-video
seedance2-fast-720p-token-no-video
seedance2-fast-720p-token-with-video
seedance2.5-480p-token-no-video
seedance2.5-480p-token-with-video
seedance2.5-720p-token-no-video
seedance2.5-720p-token-with-video
seedance2.5-1080p-token-no-video
seedance2.5-1080p-token-with-video
```

### 2.3 另 7 个旧规格（当前已移除）

| 模型 | 旧声明渠道 | 当前 A3 |
|---|---|---|
| `grok-video1.5-480p-second` | 67/80/82/83 等 | 不包含 |
| `minimax-h3-max-480p-second` | 67/80/82/83 等 | 不包含 |
| `minimax-h3-max-768p-second` | 67/80/82/83 等 | 不包含 |
| `minimax-h3-max-1080p-second` | 67/80/82/83 等 | 不包含 |
| `seedance2-480p-once` | 69/80/82/83 等 | 不包含 |
| `seedance2-1080p-once` | 69/80/82/83 等 | 不包含 |
| `seedance2-fast-720p-once-2` | 69/80/82/83 等 | 不包含 |

### 2.4 池级差异

- `video`：对齐前 83 → 对齐后 49，移除 34，新增 0。
- `plan`：对齐前 80 → 对齐后 17，移除 65，新增 2（`seedance2.5-flash-480p-once-2`、`seedance2.5-flash-720p-once-2`）。
- 渠道 69：12 → 9；移除 `seedance2-480p-once`、`seedance2-1080p-once`、`seedance2-fast-720p-once-2`、`seedance2.5-flash-480p-once-1`、`seedance2.5-flash-720p-once-1`；新增两个 `-once-2`。

> 上述数字来自 2026-11-25 对齐前备份 `models-align-20261125-024736.sql.gz` 与 2026-11-27 生产渠道表比对。备份含密钥字段，不对外分发。

---

## 3. Pricing Group 资格审计（2026-11-27 时点）

只读核对生产 `IntelligentRouting`、`GroupRatio`、`abilities`、`channels`、`billing_expr`、`ModelPrice`、`ModelRatio`：

| 池 | 声明模型 | 启用 Ability | 定价组资格 | 有效价格 | Ability 缺失 | 定价组不合格 |
|---|---:|---:|---:|---:|---:|---:|
| `chat` | 30 | 30 | 30 | 30 | 0 | 0 |
| `video` | 49 | 49 | 49 | 49 | 0 | 0 |
| `image` | 10 | 10 | 10 | 10 | 0 | 0 |
| `plan` | 17 | 17 | 17 | 17 | 0 | 0 |
| `audio` | 0 | 0 | 0 | 0 | 0 | 0 |

结论：**截至 2026-11-27，生产没有因 Pricing Group 资格导致 A3 广告不可调用模型。** P0 仍会补“按池启用 Ability 过滤”，但 Pricing Group 资格不额外改生产逻辑；保留审计脚本/回归测试，后续渠道或价格变动时重跑。

---

## 4. P0 修复范围（本次提交）

> 本节记录 P0 实际提交内容；代码提交为 `7868b83e2`（`feat/harness-v23-p0` → `harness-api`），此前的地基提交为 `241181d59`。镜像与部署版本以 GHCR 构建结果为准。

### 4.1 Run ID 80

- `X-Harness-Request-Id`：保持 1–64 字符。
- `X-Harness-Run-Id`：放宽到 1–80 字符。
- `harness_ledger_events.run_id`、`harness_settlements.harness_run_id`：`varchar(64)` → `varchar(96)`。
- 部署顺序：备份数据库 → 执行 DDL 扩列 → 部署新镜像；避免新代码写 80 字符时旧列报 `data too long` 导致账本事务回滚。
- 验收：64、65、80 字符 Run ID 均 200；81 字符返回 400；B1 原样回显 80 字符；B3 同 request id 有记录。
- 已补测试：`TestHarnessLedgerStoresEightyCharacterRunId`、`TestHarnessSettlementStoresEightyCharacterRunId`、`TestRequestIdKeepsEightyCharacterRunId`；`TestHarnessGatewayEndToEnd` 用 80 字符 Run ID 验证 B1 原样回显。

### 4.2 错误状态与追踪

对外保持 V2.3 wire 契约，不把 `503 model_not_found` 改成 `404` 或 `no_available_channel`。

| 场景 | 对外 | 内部归集 |
|---|---|---|
| 模型不在池内 | 503 `model_not_found` | `model_absent` |
| 无启用 Ability | 503 `model_not_found` | `ability_disabled` |
| 无候选渠道 | 503 `model_not_found` | `no_available_channel` |
| 候选都返回 model_not_found | 503 `model_not_found` | `upstream_model_not_found` |
| 上游 429 | 429 + `Retry-After` | `upstream_rate_limited` |
| 上游 5xx | 502/503 | `upstream_unavailable` |
| 上游认证失败 | 502/503 | `upstream_auth_failed`，绝不透传为调用方 401/403 |

归集依据每次渠道尝试记录，而不是只看最后一次错误。429/5xx 只有在真实发往上游后才进入对应归集；模型调用保留原生 OpenAI/Claude 外形，B1–B4 使用结构化 `code/message/retryable/request_id`。

已实现：`middleware/harness_contract_error.go`、`controller/harness_relay_normalize.go`；B1/B3 的 401 走 contract envelope，A3/B4 错误补 `request_id/details`，verb guard 与 `/internal` 未知路径返回 contract 404；错误日志 `logs.other` 写入 `harness_request_id`/`harness_run_id`，供未来失败提交按原编号对账。

### 4.3 A3 Ability 过滤

- `ResourcePoolModelsWithError` 的返回条件改为：池内启用渠道 + 该渠道该模型 `abilities.enabled=true` + 有效价格配置。
- 不把“金额是否为 0”当作无效价格；沿用现有 `ModelRatio` / `ModelPrice` / `billing_expr` 配置存在性判断。
- DB 查询失败返回 500，不把空目录当正常目录，不推进 `catalog_version`。
- 验收：禁用某模型 Ability 后 A3 不再返回该模型，ETag 变化，`catalog_version` 递增；重新启用后恢复。
- 已补测试：`TestResourcePoolModelsRequiresEnabledAbility`、`TestHarnessTierCatalogAbilityFilteringIsPerPool`、`TestGetHarnessTierCatalogRenderFailureIsRetryable`。

### 4.4 MIME 归一化

- 视频内容接口：GET / HEAD / Range 206 都按 artifact 声明 → 上游明确媒体类型 → URL/Content-Disposition 扩展名 → 必要时 sniff 的顺序归一化 `Content-Type`。
- 206 中间段不 sniff；200 或 `Content-Range: bytes 0-` 才读 512 字节前缀，并用 `io.MultiReader` 回放，保持 `Content-Length`、`Content-Range` 和状态码不变。
- Nano Banana：`candidates[].content.parts[].inlineData.mimeType` 按 base64 首字节识别 JPEG/PNG/WebP；同步与流式都修正，原生 JSON 用 `ojson` 精确 patch，保留未知字段。
- 验收矩阵：视频 MP4 的 GET / HEAD / Range 206；图片 JPEG / PNG / WebP；不得只测一个 GET 就宣布完成。
- 已实现：`relaykit/types/media_type.go`、`controller/video_proxy.go`、`controller/task.go`、`plugins/tasks/hailuo/plugin.js`、`relaykit/dto/gemini.go`、`relaykit/relayconvert/internal/gemini_chat/to_oai_chat_resp.go`、`relay/channel/gemini/*`；已补 MIME 的 HEAD/GET/206/base64/JPEG/PNG/WebP 测试。

### 4.5 三层验证

1. 代码层：单元测试、三库迁移测试、异常分支测试、`go build ./...`。
2. 接口层：实际 HTTP 请求检查状态码、body、`X-NewAPI-Request-Id`、`Retry-After`、ETag/304、MIME/Range。
3. Harness 联调层：使用真实服务主体和测试渠道完成端到端验证；7 个代表模型只能写“冒烟”，不能写 49/17 全量通过。

### 4.6 已执行验证（2026-11-27）

| 检查 | 结果 |
|---|---|
| `go build ./...`（根模块） | 通过 |
| `cd relaykit && GOWORK=off go build ./...` | 通过 |
| `go test ./router` | 通过（含 `TestHarnessGatewayEndToEnd`、`TestHarnessTierCatalogConditionalGet`、`TestHarnessP0ModelCallE2E`） |
| `go test ./model` | 通过（含 80 字符 Run ID 落库/回显） |
| `go test ./service` | 通过 |
| `go test ./middleware -run 'Harness|RequestId|RateLimit'` | 通过 |
| `go test ./controller -run 'TestProxyTaskMedia|TestTaskArtifactContent|TestLegacyVideo|TestHarnessTierCatalogAbility|TestGetHarnessHealth|TestNormalizeHarnessRelayError|TestSetRelayRetryAfter'` | 通过 |
| `go test ./relay/channel/gemini` | 通过 |
| `go test ./plugins -run TestHailuo` | 通过 |
| `cd relaykit && GOWORK=off go test ./types ./dto ./relayconvert/...` | 通过 |

已知环境问题：Windows 上 `go test ./controller` 全量仍有若干 `TempDir RemoveAll cleanup` 失败（SQLite 文件句柄未释放），失败用例集中在 Security Enrollment / Telegram OAuth / APITokenAudit，与本 P0 改动无关；`go test ./middleware` 全量同样有已知的 `TestAccessAndPanicLogsFollowLogRotation` 临时文件清理失败。定向测试均通过。

本地已补真实 HTTP + 自签 Harness JWT 的端到端用例 `TestHarnessP0ModelCallE2E`（提交 `a453ca77e`）：SQLite + mock OpenAI 上游，用 80 字符 Run ID 完成模型调用，B1 batchGet 原样回显 80 字符；81 字符在 dispatch 前返回 400 且 mock 上游调用数不增加；同一 engine 的 A3 返回 ETag，带 `If-None-Match` 请求得到 304。

### 4.7 数据库迁移说明

生产 MySQL 先执行：

```sql
ALTER TABLE harness_ledger_events
  MODIFY COLUMN run_id varchar(96) DEFAULT '';
ALTER TABLE harness_settlements
  MODIFY COLUMN harness_run_id varchar(96) DEFAULT '';
```

PostgreSQL 使用 `ALTER COLUMN ... TYPE varchar(96)`；SQLite 由 GORM AutoMigrate 重建表。执行前备份数据库；执行后确认两列宽度 ≥80，再部署新镜像。

### 4.8 生产接口实测（2026-11-27）

**部署版本**：`ghcr.io/uskyu/new-api:harness-api-20261127-7868b83`（digest `sha256:425289d36375958420705b43d757e133db1b0aa1d9eaa2c4afb8da5979725cc5`）。\
**数据库**：`harness_ledger_events.run_id`、`harness_settlements.harness_run_id` 已从 `varchar(64)` 扩到 `varchar(96)`。\
**备份**：`/opt/new-api/backups/newapi_db_before_p0_20261127-145749.sql.gz`、`docker-compose.yml.before-p0-20261127-145749`。

| 实测 | 结果 |
|---|---|
| `POST /v1/chat/completions` 不存在的模型 | 503，`code=model_not_found`，响应头 `X-NewAPI-Request-Id` 与 body 内 request id 一致 |
| `POST /v1/chat/completions` `gpt-6.2-sol` | 200，内容 `OK` |
| `GET /internal/v1/gateway/usage-events` 无凭证 | 401 contract body，含 `GATEWAY_UNAUTHORIZED` / `retryable=false` / `request_id` |
| `GET /internal/v1/gateway/health` 无凭证 | 401 contract body，含 `GATEWAY_UNAUTHORIZED` / `retryable=false` / `request_id` |
| `POST /internal/v1/gateway/usage-eventsXYZ` | 404 contract body，含 `REQUEST_NOT_FOUND` / `retryable=false` / `request_id` |
| MiniMax `task_MOLgzFAGo10aBU4AZrTd0CHWcIDF7P4v` HEAD | 200，`Content-Type: video/mp4`，`X-Content-Type-Options: nosniff` |
| 同任务 `Range: bytes=0-15` | 206，`Content-Type: video/mp4`，`Content-Range: bytes 0-15/525952`，body 前缀 `00000020 66747970 69736f6d`（MP4 `ftypisom`） |
| 同任务完整 GET | 200，`Content-Type: video/mp4`，`Content-Length: 525952` |
| Nano Banana `Nano Banana 2.1 1K` 原生 generateContent | 200，`inlineData.mimeType=image/jpeg`，base64 实际首字节 `ffd8ffe0`（JPEG） |

仍需 Harness 持真实 JWT 复测：64/65/80 字符 Run ID 与 B1 原样回显、81 字符 400、A3 ETag/304 与按池 Ability、401/403 错误体、429 `Retry-After`。这些用例无法用网关自己的 sk 测试 token 代替 Harness JWT。

---

## 5. P1（暂缓，不纳入本次 P0）

以下内容不进入本次提交，也不扩大原合同范围：

- B1 `video_second` / `image_count` 与 B3 `billing_basis` / `billing_quantity` / `billing_quantity_source`。
- Settlement/Ledger 媒体字段扩展、异步任务用量修正。
- Pending 状态机：`pending_dispatch` / `not_dispatched` / `dispatched_unknown` / `settled` / `refunded` / `unreconciled`。
- 请求丢失、重复提交、退款和最终结算的状态管理。

P1 开工前必须先冻结字段契约和状态转换；不得边改数据库边和 Harness 商量字段。

---

## 6. 文档修正记录

| 文档 | 修正 |
|---|---|
| `14-alignment-smoke-test-20261125.md` | 删除“4 个 Harness UUID 均在网关侧有记录”的错误表述；改 8 个网关 Request ID + 时间窗；费用改为预扣 ¥4.16 全部退款、净扣 0；7 个代表模型标注为冒烟，不是全量验收 |
| `README.md` | A3 版本改为 `v12`（以实际响应为准）；当前镜像为 `harness-api-20261127-7868b83`；新增 15 号报告入口 |
| `01-integration-guide.md` | A3 版本 `10` → `v12`；当前镜像更新；补充“以实际响应为准” |
| `12-retest-model-list-20261123.md` | `seedance2.5-flash-*-once-1` → `-once-2`，注明 11-26 上游同步替换 |
| 本文档 | 新增 TVC 8 次请求凭证、22+7 清单、Pricing Group 审计、P0/P1 边界 |
