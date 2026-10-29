# Harness ⇄ New API 网关接口交付说明（V0.9）

本文档对应 `harness-api` 分支提交 `ec3cdc0`，说明 New API 网关侧对外开放的
接口清单、鉴权配置、请求/响应契约和实测结论。

> 字段级细节与最新验收记录以 `docs/harness-delivery/` 交付包为准（V0.9.1）；
> 本文档是其摘要版本。

**本文档区分三类状态：**

- **已冻结** —— 第四轮文档（`NewAPI第四轮反馈与Schema参考-20261023`）已明确定义，双方无异议。
- **待确认** —— 网关侧实现需要对方确认的字段或语义，**尚未冻结**。
- **本期不做** —— 已明确排除在验收范围外。

---

## 一、接口清单

| 编号 | 方法 | 路径 | Scope | 状态 |
| --- | --- | --- | --- | --- |
| A1 | POST | `/v1/chat/completions` | `gateway.chat` | 已冻结（官方协议） |
| A2 | POST | `/v1/responses` | `gateway.chat` | 已冻结（官方协议） |
| A3 | GET | `/v1/models` | `gateway.catalog.read` | 已冻结 |
| A4 | POST | `/v1/messages` | `gateway.chat` | 已冻结（官方协议） |
| A9 | POST | `/v1beta/models/{tier}:{action}` | `gateway.chat` | 已冻结（官方协议） |
| B1 | GET | `/internal/v1/gateway/usage-events` | `gateway.usage.read` | 已冻结 |
| B1-batchGet | POST | `/internal/v1/gateway/usage-events:batchGet` | `gateway.usage.read` | 已冻结 |
| B3 | GET | `/internal/v1/gateway/cost-records` | `gateway.cost.read` | 已冻结 |
| B3-batchGet | POST | `/internal/v1/gateway/cost-records:batchGet` | `gateway.cost.read` | 已冻结 |
| B4 | GET | `/internal/v1/gateway/health` | `gateway.health.read` | 已冻结 |
| ~~B2~~ | — | ~~`/internal/v1/gateway/usage-events/{id}`~~ | — | **本期取消，未开发** |

**注意**：B2 单次详情本期取消，网关未注册该路由。按 Request ID 的单笔查询由
B1-batchGet / B3-batchGet 承接。

---

## 二、鉴权配置

### 2.1 Harness JWT（`/internal/v1/gateway/*`）

New API 需要三项配置，均在管理端「系统设置」写入 option 表：

| Option Key | 含义 | 示例 |
| --- | --- | --- |
| `HarnessJWKSURL` | Harness 的 JWKS 地址 | `https://harness.example.com/.well-known/jwks.json` |
| `HarnessJWTIssuer` | 期望的 `iss` | `harness` |
| `HarnessJWTAudience` | 期望的 `aud` | `new-api` |
| `HarnessSubjectTokenMap` | `sub` → 内部令牌映射 | `{"harness-gateway": 1}` |

**JWT 要求：**

- 算法：`RS256`
- `kid` 必须能在 JWKS 中找到
- `scope` claim 为**空格分隔字符串**（不是数组）
- `sub` 必须出现在 `HarnessSubjectTokenMap` 中，映射到有效的模型令牌

### 2.2 模型令牌（`/v1/*`）

A1/A2/A4/A9/A3 使用标准 `Bearer sk-...` 模型令牌。Harness JWT 会被网关映射为
内部令牌后转发，`X-Harness-Request-Id` 用于账本关联。

---

## 三、请求头

| 头 | 方向 | 必填 | 说明 |
| --- | --- | --- | --- |
| `Authorization` | Harness → New API | ✓ | `Bearer <JWT>` |
| `X-Harness-Request-Id` | Harness → New API | ✓ | 一次模型调用一个，建议 `hrq_<ULID>`，≤64 字符 |
| `X-Harness-Run-Id` | Harness → New API | ✓ | 一次任务执行一个，建议 `run_<ULID>`，≤64 字符 |
| `X-Route-Group` | Harness → New API | ✗ | 资源池 id；不传走默认池，传未知池返回 403 |
| `X-NewAPI-Request-Id` | New API → Harness | 响应头 | 网关请求 ID，账本 `gateway_request_id` 即此值 |

**缺失任一关联头时的行为**：网关在转发上游之前返回 `400 REQUEST_INVALID`，
不产生费用，也不产生账本事件（已定案，不再是待确认项）。

---

## 四、接口契约

### 4.1 A3 目录（`GET /v1/models`）

**成功响应（200）：**

```json
{
  "object": "list",
  "data": [
    {
      "id": "economy",
      "object": "model",
      "display_name": "经济档",
      "kind": "text",
      "models": ["deepseek-v4-pro", "deepseek-v4.1-flash"],
      "capabilities": ["chat", "stream", "tools"]
    }
  ],
  "catalog_version": "2",
  "etag": "W/\"e10bdaded71746aa5b1890a9a0f5e6259d3e5f744a2115f8b951694906bb7fe3\""
}
```

`data[].id` 是资源池 id（用作 `X-Route-Group`），`data[].models` 是该池可路由的
真实模型 id（用作请求的 `model`）。空数组表示该池暂无可调模型。

**响应头**：`ETag: W/"e10bdade..."`

**条件查询**：携带 `If-None-Match: <etag>`，未变化返回 `304 Not Modified`，**无响应体**。

**约束**：只返回可路由且已配置价格的模型；不返回渠道、上游厂商或采购价。
目录是能力快照，不保证当时有可用渠道。

### 4.2 B1 日常增量（`GET /internal/v1/gateway/usage-events`）

**查询参数**：`cursor`、`limit`（默认 50，上限 200）、`from`、`to`（RFC3339，窗口 ≤ 24h）

**成功响应（200）：**

```json
{
  "items": [
    {
      "event_id": "ue_01M5SCN0YC1PYC58SC77ZBME7N",
      "event_type": "usage",
      "request_id": "hrq_e2e_local_001",
      "gateway_request_id": "20261025112035229502800OD3ZtAeW4ompla36",
      "run_id": "run_e2e_local_001",
      "usage_unit": "token",
      "usage": {
        "input_tokens": 39,
        "output_tokens": 20,
        "cache_read_tokens": 0,
        "cache_write_tokens": 0,
        "total_tokens": 59,
        "split_known": true
      },
      "usage_source": "upstream",
      "is_estimated": false,
      "observed_at": "2026-10-25T11:20:37.068Z",
      "record_version": 1,
      "supersedes_event_id": null
    }
  ],
  "next_cursor": "eyJ2IjoxLCJzIjoidWUiLCJpZCI6MX0",
  "has_more": true
}
```

### 4.3 B3 日常增量（`GET /internal/v1/gateway/cost-records`）

**成功响应（200）：**

```json
{
  "items": [
    {
      "event_id": "ce_01M5SCN0YCQ6YT3BCH2Y22WGWB",
      "event_type": "cost",
      "cost_id": "cost_20261025112035229502800OD3ZtAeW4ompla36",
      "request_id": "hrq_e2e_local_001",
      "gateway_request_id": "20261025112035229502800OD3ZtAeW4ompla36",
      "status": "settled",
      "currency": "USD",
      "billing": {
        "base_amount": "0.00003304",
        "group_ratio": 0,
        "tax_rate_percent": 0,
        "total_amount": "0.00003304"
      },
      "billing_basis": "token",
      "procurement_price_version": "pw-W/\"029cbe10...\"",
      "record_version": 1,
      "supersedes_event_id": null,
      "observed_at": "2026-10-25T11:20:37.068Z"
    }
  ],
  "next_cursor": "eyJ2IjoxLCJzIjoiY2UiLCJpZCI6Mn0",
  "has_more": true
}
```

**状态语义**：`pending` 不是最终成本，`settled` 才是最终采购成本。

### 4.4 B1-batchGet（`POST /internal/v1/gateway/usage-events:batchGet`）

**请求：**

```json
{
  "request_ids": ["hrq_01H8X...", "hrq_01H8Y..."],
  "include_corrections": true
}
```

**成功响应（200）：**

```json
{
  "items": [
    {
      "request_id": "hrq_e2e_stream",
      "found": true,
      "events": [ /* 结构同 4.2 的 item */ ]
    },
    {
      "request_id": "hrq_missing_xyz",
      "found": false,
      "events": null
    }
  ],
  "missing_request_ids": ["hrq_missing_xyz"],
  "truncated": false
}
```

### 4.5 B3-batchGet（`POST /internal/v1/gateway/cost-records:batchGet`）

请求/响应结构与 B1-batchGet 对称，`events` 换为 `records`，`include_corrections`
换为 `include_status_events`。

### 4.6 B4 健康（`GET /internal/v1/gateway/health`）

```json
{
  "status": "ok",
  "interface_version": "v2",
  "upstream_baseline": "45fbb149",
  "time": "2026-10-25T12:41:35.407Z"
}
```

---

## 五、分页与限制规则

| 项 | 规则 |
| --- | --- |
| BatchGet Request ID 数量 | 单次最多 **100** 个，超限返回 400 `REQUEST_INVALID` |
| BatchGet 事件数量 | 单次最多返回 **500** 条，超限 `truncated: true` |
| BatchGet 缺失 ID | 放入 `missing_request_ids`，**不整体失败** |
| BatchGet 排序 | 按 `observed_at`、`event_id` 升序 |
| 日常增量 limit | 默认 50，上限 200 |
| 日常增量窗口 | `from`/`to` 跨度 ≤ 24h，超限 `WINDOW_TOO_LARGE` |
| `observed_at` 格式 | RFC3339 UTC，**统一三位毫秒**（`2026-10-25T11:20:37.068Z`） |

---

## 六、错误码

| HTTP | Code | 含义 |
| --- | --- | --- |
| 401 | `GATEWAY_UNAUTHORIZED` | JWT 无效/过期，或缺少必需 scope |
| 400 | `REQUEST_INVALID` | 请求体不合法（ID 数量超限、字段格式错误） |
| 400 | `CURSOR_INVALID` | 游标格式错误或流不匹配 |
| 400 | `WINDOW_TOO_LARGE` | 时间窗超过 24h |
| 404 | `REQUEST_NOT_FOUND` | 请求 ID 不存在 |
| 500 | `GATEWAY_UPSTREAM` | 网关内部错误 |
| — | `EVENT_LIMIT_EXCEEDED` | 事件数超过单次上限 |

**错误响应结构：**

```json
{
  "error": {
    "code": "REQUEST_INVALID",
    "message": "request_ids exceeds the maximum of 100",
    "retryable": false,
    "details": { "field": "request_ids", "max": 100 }
  }
}
```

---

## 七、账本事件规则

### 7.1 event_id 前缀（已冻结）

- B1 用量事件：`ue_` 前缀
- B3 成本事件：`ce_` 前缀
- 同一请求后续状态变化产生**新** `event_id`，通过 `supersedes_event_id` 关联旧事件

### 7.2 事件类型

| 流 | 场景 | event_type |
| --- | --- | --- |
| B1 | 首次用量记录 | `usage` |
| B1 | 用量被更正（有前序事件） | `usage_correction` |
| B3 | 首次成本记录 | `cost` |
| B3 | 金额变化（含税率/倍率变化导致的 total 变化） | `cost_correction` |
| B3 | 金额不变、仅状态变化 | `status_change` |
| B3 | 有前序事件后转为 refunded | `refund` |

### 7.3 版本链

- 两条流**各自独立**维护 `record_version` 与 `supersedes_event_id`
- 首次事件始终 `record_version=1`、`supersedes_event_id=null`
- 只有实际发生变化时才追加新事件；完全重复的结算不产生事件

---

## 八、需要双方确认的事项

以下前三项是网关侧实现需要对方明确的部分，**不是已冻结的正式合同**；
8.4 已定案为拒绝，网关已实施。

### 8.1 usage 字段的 null 语义

**现状**：`input_tokens`、`output_tokens`、`cache_read_tokens`、`cache_write_tokens`
在网关侧实现为可空（`null` 表示未测量）。

**第四轮文档 §4.4** 只列出字段名，未说明是否允许 `null`。

**需确认**：Harness 解析器能否接受 `null`？若使用强类型 `int` 会反序列化失败。

### 8.2 新增字段 total_tokens 与 split_known

**现状**：网关额外输出两个字段：

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `total_tokens` | `int?` | 上游报告的总量 |
| `split_known` | `bool` | 输入/输出是否分别观测到 |

**第四轮文档未定义这两个字段**。

**背景**：上游只返回 `total_tokens` 时，无法如实拆分输入输出。网关选择输出
`input_tokens: null` + `total_tokens: 1000` + `split_known: false`，而不是伪造
两个 0。

**需确认**：接受这两个字段，还是要求网关改用其他表达方式。

### 8.3 refunded 状态下 total_amount 可为非零

**现状**：退款时保留已发生的采购成本，`total_amount` **非零**。

**第四轮文档**的退款样例金额为 0，容易被理解为「退款即归零」。

**网关的判断**：网关只知道**客户**被退款，不知道**供应商**是否退款。供应商对
生成后失败的任务确实收费。强行归零会掩盖真实成本差异。

**需确认**：接受「`refunded` ⇒ `total_amount` 可非零」这一语义。

### 8.4 缺失关联头时的策略（已定案）

**结论**：拒绝请求，返回 `400 REQUEST_INVALID`。网关已实施，不再等待确认。

拒绝发生在转发上游之前，不产生费用，也不产生账本事件。`X-Harness-Request-Id`
与 `X-Harness-Run-Id` 都必填，均 ≤64 字符。

---

## 九、本期不做

| 项 | 原因 |
| --- | --- |
| B2 单次详情 | 第四轮明确取消 |
| A5 count_tokens | 本期可选，非交付阻塞项 |
| A6 Embeddings / A7 Rerank / A8 图像音频 | 第四轮后置 |
| `usage_unit` 的 image / audio_second / character / request | 本期只做 Token 计费 |
| 独立工具计费扩展 | 明确排除 |

**关于 `usage_unit`**：第四轮列出多个可选值是为未来定义，本期实际只会产生
`token` 一个值。

---

## 十、变更记录

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版交付说明，对应提交 `3a1afb154` |
