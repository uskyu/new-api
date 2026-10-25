# Harness 网关接口接入指南

**版本**：V0.9
**对应网关版本**：`harness-api` 分支 `bfec3fe2e`
**面向**：Harness 后端开发与联调

本文档给出网关侧全部对外接口的调用方法。字段语义细节见
`harness-contract-data-v0.9.md`，鉴权配置见 `harness-auth-setup-v0.9.md`。

---

## 一、接口总览

### 1.1 模型调用类（官方协议）

| 编号 | 方法 | 路径 | 所需 Scope |
| --- | --- | --- | --- |
| A1 | POST | `/v1/chat/completions` | `gateway.chat` |
| A2 | POST | `/v1/responses` | `gateway.chat` |
| A4 | POST | `/v1/messages` | `gateway.chat` |
| A9 | POST | `/v1beta/models/{tier}:{action}` | `gateway.chat` |

请求/响应体遵循各自官方协议（OpenAI Chat / OpenAI Responses / Anthropic
Messages / Gemini 原生）。网关侧仅在以下方面与官方协议不同：

- `model` 字段填**业务档位名**（如 `economy`、`enterprise`），非真实模型名
- 鉴权用 Harness JWT（见鉴权指南）
- 响应头返回 `X-NewAPI-Request-Id`

### 1.2 目录类

| 编号 | 方法 | 路径 | 所需 Scope |
| --- | --- | --- | --- |
| A3 | GET | `/v1/models` | `gateway.catalog.read` |

### 1.3 账本类

| 编号 | 方法 | 路径 | 所需 Scope |
| --- | --- | --- | --- |
| B1 | GET | `/internal/v1/gateway/usage-events` | `gateway.usage.read` |
| B1-batchGet | POST | `/internal/v1/gateway/usage-events:batchGet` | `gateway.usage.read` |
| B3 | GET | `/internal/v1/gateway/cost-records` | `gateway.cost.read` |
| B3-batchGet | POST | `/internal/v1/gateway/cost-records:batchGet` | `gateway.cost.read` |
| B4 | GET | `/internal/v1/gateway/health` | `gateway.health.read` |

**B2 单次详情本期取消**，网关未注册该路由，请求会返回 404。

---

## 二、请求头

| 头 | 方向 | 必填 | 说明 |
| --- | --- | --- | --- |
| `Authorization` | Harness → 网关 | ✓ | `Bearer <JWT>` |
| `X-Harness-Request-Id` | Harness → 网关 | ✓ | 关联一次模型调用，建议 `hrq_<ULID>` |
| `X-Harness-Run-Id` | Harness → 网关 | ✗ | 关联任务执行过程，建议 `run_<ULID>` |
| `X-NewAPI-Request-Id` | 网关 → Harness | 响应头 | 网关请求 ID，即账本中的 `gateway_request_id` |

> 缺失 `X-Harness-Request-Id` 的处理策略见「待确认事项」第 4 项，尚未冻结。

---

## 三、A3 目录

### 3.1 请求

```http
GET /v1/models
Authorization: Bearer <JWT>
```

**查询参数**：无

### 3.2 首次响应（200）

```http
HTTP/1.1 200 OK
Content-Type: application/json
ETag: W/"57cf3c14fd04a4c6d3b0e62f40b648c78861d74fb38e64bf2a4546a04700bf28"
```

```json
{
  "object": "list",
  "data": [
    {
      "id": "economy",
      "object": "model",
      "capabilities": ["chat", "stream", "tools"]
    },
    {
      "id": "enterprise",
      "object": "model",
      "capabilities": ["chat", "stream", "tools"]
    }
  ],
  "catalog_version": "1",
  "etag": "W/\"57cf3c14fd04a4c6d3b0e62f40b648c78861d74fb38e64bf2a4546a04700bf28\""
}
```

### 3.3 条件查询（304）

```http
GET /v1/models
Authorization: Bearer <JWT>
If-None-Match: W/"57cf3c14fd04a4c6d3b0e62f40b648c78861d74fb38e64bf2a4546a04700bf28"
```

```http
HTTP/1.1 304 Not Modified
```

**304 响应无 body。** 目录未变化时，网关不重复传输内容。

### 3.4 约束

- 只返回运营配置的**业务档位**，不含真实上游模型名、渠道信息、采购价
- 目录变更时 `etag` 与 `catalog_version` 同时变化

---

## 四、B1 用量事件（日常增量）

### 4.1 请求

```http
GET /internal/v1/gateway/usage-events?cursor=&limit=50&from=&to=
Authorization: Bearer <JWT>
```

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `cursor` | ✗ | 上一页返回的 `next_cursor`，首次不传 |
| `limit` | ✗ | 默认 50，上限 200 |
| `from` | ✗ | 起始时间，RFC3339，与 `to` 跨度 ≤ 24h |
| `to` | ✗ | 结束时间，RFC3339 |

### 4.2 成功响应（200）

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

### 4.3 翻页

`has_more` 为 `true` 时，用 `next_cursor` 作为下次请求的 `cursor`。
`next_cursor` 为 `null` 表示已到末尾。

**游标与流绑定**：B1 的游标只能用于 B1，B3 的游标只能用于 B3。混用返回
`CURSOR_INVALID`。

---

## 五、B3 成本记录（日常增量）

### 5.1 请求

```http
GET /internal/v1/gateway/cost-records?cursor=&limit=50&from=&to=
Authorization: Bearer <JWT>
```

参数同 B1。

### 5.2 成功响应（200）

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
      "procurement_price_version": "pw-W/\"029cbe10260ffbdac4d70413d49e084c8d39972449c27dfff3b2231a117e1989\"",
      "record_version": 1,
      "supersedes_event_id": null,
      "observed_at": "2026-10-25T11:20:37.068Z"
    }
  ],
  "next_cursor": "eyJ2IjoxLCJzIjoiY2UiLCJpZCI6Mn0",
  "has_more": true
}
```

**金额为字符串十进制**，不可当作 float 解析。

---

## 六、B1-batchGet 批量补查

### 6.1 请求

```http
POST /internal/v1/gateway/usage-events:batchGet
Authorization: Bearer <JWT>
Content-Type: application/json
```

```json
{
  "request_ids": ["hrq_01H8X...", "hrq_01H8Y..."],
  "include_corrections": true
}
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `request_ids` | ✓ | Harness Request ID 列表，**最多 100 个** |
| `include_corrections` | ✗ | 是否包含更正事件，默认 `true` |

### 6.2 成功响应（200）

```json
{
  "items": [
    {
      "request_id": "hrq_a9_gemini",
      "found": true,
      "events": [
        {
          "event_id": "ue_01M5SHT1FCX48TS5YJ4K95SY3Z",
          "event_type": "usage",
          "request_id": "hrq_a9_gemini",
          "gateway_request_id": "20261025125042636632500OD3ZtAeWEjZjlh3m",
          "run_id": "run_e2e_multi",
          "usage_unit": "token",
          "usage": {
            "input_tokens": 32,
            "output_tokens": 15,
            "cache_read_tokens": 0,
            "cache_write_tokens": 0,
            "total_tokens": 47,
            "split_known": true
          },
          "usage_source": "upstream",
          "is_estimated": false,
          "observed_at": "2026-10-25T12:50:44.332Z",
          "record_version": 1,
          "supersedes_event_id": null
        }
      ]
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

### 6.3 规则

| 项 | 规则 |
| --- | --- |
| Request ID 数量 | 最多 100，超限 `400 REQUEST_INVALID` |
| 事件数量 | 最多 500，超限 `truncated: true` |
| 缺失 ID | 放入 `missing_request_ids`，**不整体失败** |
| 多事件排序 | 按 `observed_at`、`event_id` 升序 |
| 幂等 | 只读，重复调用结果一致 |

---

## 七、B3-batchGet 批量成本补查

### 7.1 请求

```http
POST /internal/v1/gateway/cost-records:batchGet
Authorization: Bearer <JWT>
Content-Type: application/json
```

```json
{
  "request_ids": ["hrq_01H8X...", "hrq_01H8Y..."],
  "include_status_events": true
}
```

与 B1-batchGet 的差异：`include_corrections` 换为 `include_status_events`。

### 7.2 成功响应（200）

```json
{
  "items": [
    {
      "request_id": "hrq_e2e_stream",
      "found": true,
      "records": [
        {
          "event_id": "ce_01M5SCPXRNZ16M1PHCEZD6DZ4W",
          "event_type": "cost",
          "cost_id": "cost_20261025112137689604000OD3ZtAeWfqRW4XUy",
          "request_id": "hrq_e2e_stream",
          "gateway_request_id": "20261025112137689604000OD3ZtAeWfqRW4XUy",
          "status": "settled",
          "currency": "USD",
          "billing": {
            "base_amount": "0.00003696",
            "group_ratio": 0,
            "tax_rate_percent": 0,
            "total_amount": "0.00003696"
          },
          "billing_basis": "token",
          "procurement_price_version": "pw-W/\"029cbe10260ffbdac4d70413d49e084c8d39972449c27dfff3b2231a117e1989\"",
          "record_version": 1,
          "supersedes_event_id": null,
          "observed_at": "2026-10-25T11:21:39.349Z"
        }
      ]
    },
    {
      "request_id": "hrq_missing_xyz",
      "found": false,
      "records": null
    }
  ],
  "missing_request_ids": ["hrq_missing_xyz"],
  "truncated": false
}
```

`events` 换为 `records`，其余规则与 B1-batchGet 一致。

---

## 八、B4 健康检查

### 8.1 请求

```http
GET /internal/v1/gateway/health
Authorization: Bearer <JWT>
```

### 8.2 成功响应（200）

```json
{
  "status": "ok",
  "interface_version": "v2",
  "upstream_baseline": "45fbb149",
  "time": "2026-10-25T14:07:29.745Z"
}
```

| 字段 | 说明 |
| --- | --- |
| `status` | `ok` 或 `degraded` |
| `interface_version` | 固定 `v2` |
| `upstream_baseline` | 网关基线提交，固定 `45fbb149` |
| `time` | 服务器时间，RFC3339 UTC 毫秒 |

---

## 九、分页与限制

| 项 | 规则 |
| --- | --- |
| 日常增量默认 limit | 50 |
| 日常增量上限 limit | 200 |
| 日常增量时间窗 | ≤ 24 小时 |
| BatchGet Request ID 上限 | 100 |
| BatchGet 事件上限 | 500 |
| 时间格式 | RFC3339 UTC，统一三位毫秒：`2026-10-25T11:20:37.068Z` |

---

## 十、错误码

### 10.1 错误响应结构

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

### 10.2 错误码表

| HTTP | code | 含义 | 是否可重试 |
| --- | --- | --- | --- |
| 401 | `GATEWAY_UNAUTHORIZED` | JWT 无效或过期 | 否 |
| 403 | `GATEWAY_UNAUTHORIZED` | 缺少所需 scope（`details.required_scope`） | 否 |
| 400 | `REQUEST_INVALID` | 请求体不合法 | 否 |
| 400 | `CURSOR_INVALID` | 游标格式错误或流不匹配 | 否 |
| 400 | `WINDOW_TOO_LARGE` | 时间窗超过 24h | 否 |
| 404 | `REQUEST_NOT_FOUND` | 请求 ID 不存在 | 否 |
| 500 | `GATEWAY_UPSTREAM` | 网关内部错误 | 是 |

---

## 十一、调用示例

### 11.1 拉取健康状态

```bash
curl -sS "http://<gateway>/internal/v1/gateway/health" \
  -H "Authorization: Bearer $HARNESS_JWT"
```

### 11.2 增量拉取用量

```bash
curl -sS "http://<gateway>/internal/v1/gateway/usage-events?limit=50" \
  -H "Authorization: Bearer $HARNESS_JWT"
```

### 11.3 批量补查成本

```bash
curl -sS -X POST "http://<gateway>/internal/v1/gateway/cost-records:batchGet" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "Content-Type: application/json" \
  -d '{"request_ids":["hrq_01H8X"],"include_status_events":true}'
```

### 11.4 条件拉取目录

```bash
# 首次
curl -sS -D- "http://<gateway>/v1/models" \
  -H "Authorization: Bearer $HARNESS_JWT"

# 拿到 ETag 后
curl -sS -D- "http://<gateway>/v1/models" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H 'If-None-Match: W/"57cf3c14..."'
```

### 11.5 发起模型调用

```bash
curl -sS -X POST "http://<gateway>/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_01H8X2K9" \
  -H "X-Harness-Run-Id: run_01H8Z3M1" \
  -H "Content-Type: application/json" \
  -d '{"model":"economy","messages":[{"role":"user","content":"hi"}]}'
```

---

## 十二、变更记录

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版，对应网关 `bfec3fe2e` |
