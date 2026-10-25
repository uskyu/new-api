# Harness 网关接口验收报告（V0.9）

对应提交：`harness-api` 分支 `3a1afb154`（本轮增量）+ `6443764fd`
测试时间：2026-10-25
测试环境：本地 `http://127.0.0.1:3000`，真实上游（地址脱敏，见内部配置）

## 状态说明

| 标记 | 含义 |
| --- | --- |
| **真实联调** | 经真实 HTTP 请求、真实上游模型、真实账本写入验证 |
| **集成测试** | 真实 JWT/JWKS 签名验证 + 真实数据库，但上游为模拟 |
| **协议测试** | 仅验证路由注册与鉴权边界，不发起真实上游调用 |
| **未完成** | 尚未验证 |

---

## 一、A 类：模型调用协议

### A1 OpenAI Chat Completions（真实联调 ✅）

```
POST /v1/chat/completions
Authorization: Bearer sk-***（网关令牌，已脱敏）
X-Harness-Request-Id: hrq_e2e_local_001
```

请求体：
```json
{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"Reply with exactly: GATEWAY-OK"}],"max_tokens":20}
```

实测结果：

| 项 | 值 |
| --- | --- |
| HTTP | `200` |
| 响应内容 | `GATE<截断>` |
| usage | `prompt=39, completion=20, total=59` |
| `X-NewAPI-Request-Id` | `20261025112035229502800OD3ZtAeW4ompla36` |

账本自动写入（已验证）：
```
stream=ue type=usage  src=upstream  tokens=39/20  splitUnknown=false  v=1
stream=ce type=cost   src=upstream  base=0.00003304  total=0.00003304  v=1
```

### A2 OpenAI Responses（真实联调 ✅）

```
POST /v1/responses
{"model":"deepseek-v4.1-flash","input":"Say OK"}
```

| 项 | 值 |
| --- | --- |
| HTTP | `200` |
| 响应 | `{"id":"5cf73b13-...","object":"response","model":"deepseek-...` |

### A4 Anthropic Messages（真实联调 ✅）

```
POST /v1/messages
anthropic-version: 2023-06-01
{"model":"deepseek-v4.1-flash","max_tokens":20,"messages":[{"role":"user","content":"Say OK"}]}
```

| 项 | 值 |
| --- | --- |
| HTTP | `200` |
| 响应 | `{"id":"59a10e10-...","type":"message","role":"assistant",...}` |
| usage | `input_tokens: 32` |

### A9 Gemini 原生（真实联调 ✅）

```
POST /v1beta/models/deepseek-v4.1-flash:generateContent
{"contents":[{"parts":[{"text":"Say OK"}]}]}
```

| 项 | 值 |
| --- | --- |
| HTTP | `200` |
| 响应 | `{"candidates":[{"content":{"role":"model","parts":[{"text":"OK"}]}...` |
| usageMetadata | `promptTokenCount: 32, candidatesTokenCount: 15, totalTokenCount: 47` |

**说明**：A2/A4/A9 均通过上游的 OpenAI 兼容层转换，非供应商原生端点。
响应格式已符合各自原生协议。

### 协议路由与鉴权（协议测试 ✅）

`TestRelayProtocolRoutesRequireAuthentication`（`router/relay_router_test.go`）

| 路由 | 未认证响应 |
| --- | --- |
| `POST /v1/chat/completions` | `401`（非 404，证明路由已注册） |
| `POST /v1/responses` | `401` |
| `POST /v1/messages` | `401` |
| `POST /v1beta/models/{tier}:generateContent` | `401` |

**测试命令**：
```bash
go test ./router/ -run TestRelayProtocolRoutesRequireAuthentication -v
```

### 流式（真实联调 ✅）

```
POST /v1/chat/completions  (stream=true, stream_options.include_usage=true)
```

| 项 | 值 |
| --- | --- |
| SSE 事件数 | `32` |
| 末尾 usage | `36/30` |

---

## 二、A3 目录与条件查询（集成测试 ✅）

`TestHarnessTierCatalogConditionalGet`（`router/harness_gateway_e2e_test.go`）

采用最小路由装配（仅 JWT 中间件 + catalog handler），避免引入 Redis 与上游分发依赖。

| 子用例 | 结果 |
| --- | --- |
| 首次请求返回 200 + ETag | ✅ ETag 非空 |
| 响应体 `object=list`、`catalog_version` 非空、`etag` 与响应头一致 | ✅ |
| 相同 ETag 条件请求返回 304 且无响应体 | ✅ |
| 缺少 `gateway.catalog.read` scope → 403 | ✅ |
| 未签名调用 → 401 | ✅ |

**测试命令**：
```bash
go test ./router/ -run TestHarnessTierCatalogConditionalGet -v
```

---

## 三、B 类：账本接口（集成测试 ✅）

使用真实 RSA 签名 JWT + 真实 JWKS 服务 + 真实 SQLite 数据库。

### B4 Health

```json
{"status":"ok","interface_version":"v2","upstream_baseline":"45fbb149","time":"2026-10-25T12:41:35.407Z"}
```
`HTTP 200` —— 与第四轮 §5.4 逐字一致。

### B1 日常增量

`HTTP 200`，返回 `items`、`next_cursor`、`has_more`。字段与第四轮 §4.4 一致。

### B3 日常增量

`HTTP 200`，`billing` 四个金额字段齐全，`currency: "USD"`，金额为字符串十进制。

### B1-batchGet

| 项 | 结果 |
| --- | --- |
| 存在的 ID | `found: true` + `events` 数组 |
| 缺失的 ID | `found: false`，进入 `missing_request_ids` |
| `truncated` | `false` |
| 101 个 ID | `HTTP 400 REQUEST_INVALID`，`details.max: 100` |

### B3-batchGet

结构与 B1-batchGet 对称，`events` → `records`。同样验证通过。

### 分页与上限

| 项 | 实测 |
| --- | --- |
| BatchGet 100 ID 上限 | ✅ 生效（101 → 400） |
| 缺失 ID 不整体失败 | ✅ |
| 事件数量上限 500 | 未触发（数据量不足） |

---

## 四、账本事件类型（单元测试 ✅，本轮修复）

### 修复的两个真实缺陷

**缺陷 1：用量更正被标成 `usage`**

复现证据：
```
expected: "usage_correction"
actual  : "usage"
```
根因：`harnessUsageEventTypeForStatus` 只在 `refunded` 时返回更正，
状态仍为 `settled` 的用量更正被漏掉。

修复后测试：`TestLedgerUsageCorrectionTypingForNonRefundSettles` ✅

**缺陷 2：金额更正被标成 `status_change`**

复现证据：
```
expected: "cost_correction"
actual  : "status_change"
```
根因：`harnessCostEventTypeForStatus` 只比较 `BaseAmount`。采购价 =
base × 采购倍率 × (1+税率)，改倍率或税率时 base 不变但 total 变。

修复后测试：`TestLedgerChangeDetectionIsPerStream/base_unchanged_but_total_changes` ✅

### 事件类型覆盖矩阵

| 验证项 | 测试 | 结果 |
| --- | --- | --- |
| 首次事件 `record_version=1`、`supersedes=null` | `TestLedgerCostEventTypesFollowTheChain` | ✅ |
| 纯状态变化 → `status_change` | 同上 | ✅ |
| 金额变化 → `cost_correction` | 同上 | ✅ |
| total 变化（base 不变）→ `cost_correction` | `TestLedgerChangeDetectionIsPerStream` | ✅ |
| 用量更正 → `usage_correction` | `TestLedgerUsageCorrectionTypingForNonRefundSettles` | ✅ |
| B1/B3 独立版本链 | `TestLedgerCorrectionChainsStayInTheirOwnStream` | ✅ |
| 完全重复结算不产生事件 | `TestLedgerRepeatedSettleAppendsNothing` | ✅ |
| 退款作首事件仍为 `cost` | `TestLedgerCostEventTypesCoverTheRiskyBranches` | ✅ |

**测试命令**：
```bash
go test ./service/ -run 'TestLedger' -v
```

---

## 五、权限与拒绝（集成测试 ✅）

| 场景 | 响应 |
| --- | --- |
| 无 Authorization 头访问 `/internal/v1/gateway/*` | `403 GATEWAY_UNAUTHORIZED` |
| 缺 `gateway.usage.read` scope 访问 B1 | `403`，`details.required_scope` |
| 缺 `gateway.cost.read` scope 访问 B3 | `403` |
| 缺 `gateway.catalog.read` scope 访问 A3 | `403` |
| 普通用户（role=1）访问 `/api/harness/ledger/` | `403` |
| 无效模型令牌 | `401 token.invalid` |
| 不存在的模型 | `503` |

### B3 `failed` 状态可达性（代码审查 ✅）

全仓库搜索 `HarnessSettlementStatusFailed`，仅 3 处引用：

| 位置 | 用途 |
| --- | --- |
| `model/harness_settlement.go:21` | 常量定义 |
| `controller/harness_settlement.go:251` | 管理员列表筛选参数校验 |
| `service/harness_ledger_invariant_test.go:563` | 测试 |

**追踪所有写入路径**：`recordTaskHarnessSettlement` 的全部 9 个调用点，传入状态
只可能是 `pending` / `settled` / `refunded`。

**结论**：生产 B3 **不会输出 `failed`**。内部保留该常量，不映射为 `adjusted`
（两者业务含义不同）。

---

## 六、数据库方言

| 方言 | 版本 | 结果 |
| --- | --- | --- |
| SQLite | 内置 | ✅ 全部测试通过 |
| MySQL | 8.0.43（真实实例） | ✅ `model` 包全量通过，`AutoMigrate` 幂等 |
| PostgreSQL | — | ⚠️ **未验证**（本机无实例） |

**MySQL 关键修复**（提交 `57be7ed60`）：结算 upsert 原先手写 `excluded.col`，
这是 PostgreSQL 专属语法，MySQL 报 `Unknown column`，导致所有结算写入失败。
已改为 GORM 方言渲染。

**PostgreSQL 说明**：`excluded.` 是 PG 原生语法，修复后仍生成该形式，理论上
兼容，但**未经实测**。部署前需在有 PG 的环境补验。

---

## 七、交付文档清单

| 文件 | 内容 |
| --- | --- |
| `docs/harness-gateway-delivery-v0.9.md` | 接口清单、鉴权配置、契约、错误码、待确认事项 |

---

## 八、未完成事项

| 项 | 状态 | 阻塞原因 |
| --- | --- | --- |
| Harness 真实 JWKS 联调 | ❌ 未开始 | 待对方提供 JWKS URL、`iss`/`aud`、服务主体 |
| PostgreSQL 实测 | ❌ 未开始 | 本机无 PostgreSQL 实例 |
| Docker 镜像 | ❌ 暂缓 | GitHub Actions 被禁用；本机无 Docker |
| 500 事件上限触发 | ⚠️ 未触发 | 测试数据量不足 |
| A5 count_tokens | — | 本期不做 |

---

## 九、契约差异（需双方确认，尚未冻结）

详见 `docs/harness-gateway-delivery-v0.9.md` 第八节：

1. usage 字段是否允许 `null`
2. 是否接受新增 `total_tokens` / `split_known`
3. `refunded` 时 `total_amount` 是否可为非零
4. 缺失 `X-Harness-Request-Id` 时的策略

**这四项不是已冻结的正式合同**，需 Harness 明确后才能定稿。

---

## 十、测试命令汇总

```bash
# 协议路由与鉴权
go test ./router/ -run 'TestRelayProtocolRoutesRequireAuthentication' -v

# A3 目录条件查询
go test ./router/ -run 'TestHarnessTierCatalogConditionalGet' -v

# 账本事件类型
go test ./service/ -run 'TestLedger' -v

# 账本读取接口
go test ./service/ -run 'TestHarnessLedger' -v

# 结算持久化（含 MySQL，需 TEST_MYSQL_DSN）
TEST_MYSQL_DSN='root:@tcp(127.0.0.1:3306)/newapi_harness_test?charset=utf8mb4&parseTime=True&loc=Local' \
  go test ./model/ -run 'TestHarnessSettlementPersistenceAcrossDialects' -v

# 全量
go build ./... && go vet ./... && go test ./service/ ./model/ ./router/ -count=1
```
