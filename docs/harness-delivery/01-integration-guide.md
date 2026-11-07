# Harness 网关接口接入指南

**版本**：V0.10
**对应网关版本**：`harness-api-codex` RC48 候选分支；本地验证完成，尚未部署或真实联调
**面向**：Harness 后端开发与联调

本文档给出网关侧全部对外接口的调用方法。字段语义细节见
`03-data-contract.md`，鉴权配置见 `02-auth-setup.md`。

---

## 一、接口总览

### 1.1 模型调用类（官方协议）

| 编号 | 方法 | 路径 | 所需 Scope |
| --- | --- | --- | --- |
| A1 | POST | `/v1/chat/completions` | `gateway.chat` |
| A2 | POST | `/v1/responses` | `gateway.chat` |
| A4 | POST | `/v1/messages` | `gateway.chat` |
| A9 | POST | `/v1beta/models/{model}:{action}` | `gateway.chat` |

请求/响应体遵循各自官方协议（OpenAI Chat / OpenAI Responses / Anthropic
Messages / Gemini 原生）。网关侧仅在以下方面与官方协议不同：

- `model` 字段填 **A3 目录 `models[]` 中的真实模型 id**（如 `deepseek-v4.1-flash`），
  不是池 id。**路由完全由网关负责**：Harness 只需要调一个模型 id，不需要区分
  经济/企业档，也不需要按档位选择渠道
- 请求头 `X-Route-Group` 为**可选**，仅通过 Harness JWT 校验的调用方可以使用，
  且只有 `plan`（畅用计划）有业务含义；普通 chat/video/audio 调用不传该头，使用
  默认 `chat` 池，见第二节
- 鉴权用 Harness JWT（见鉴权指南）
- 响应头返回 `X-NewAPI-Request-Id`

响应体中的 `model` 字段是**上游模型名**（如 `deepseek-flash`），与请求里的 id 不一定
相同。对账请用 `X-Harness-Request-Id` / `X-NewAPI-Request-Id`，不要用响应体的
`model` 反查。

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
| B4 | GET | `/internal/v1/gateway/health` | 无（仅需有效 JWT） |

**B2 单次详情本期取消**，网关未注册该路由，请求会返回 404。

---

## 二、请求头

| 头 | 方向 | 必填 | 说明 |
| --- | --- | --- | --- |
| `Authorization` | Harness → 网关 | ✓ | `Bearer <JWT>` |
| `X-Harness-Request-Id` | Harness → 网关 | ✓ | 一次模型调用一个，建议 `hrq_<ULID>`，**≤ 64 字符** |
| `X-Harness-Run-Id` | Harness → 网关 | ✓ | 一次任务执行一个，建议 `run_<ULID>`，**≤ 64 字符** |
| `X-Route-Group` | Harness → 网关 | ✗ | 可选。仅通过 Harness JWT 校验的调用方可以使用，且只有 `plan`（畅用计划）需要；普通 chat/video/audio 调用不传，落默认 `chat` 池 |
| `X-NewAPI-Request-Id` | 网关 → Harness | 响应头 | 网关请求 ID，即账本中的 `gateway_request_id` |

两个关联 ID 都只允许 `A-Z a-z 0-9 - _`，且都必须非空。缺失或格式不合法时，
网关在**转发上游之前**返回：

```http
HTTP/1.1 400 Bad Request
```

```json
{"error":{"code":"REQUEST_INVALID","message":"X-Harness-Run-Id is required","type":"new_api_error"}}
```

即：被拒绝的调用不会产生费用，也不会产生账本事件。长度上限是账本列的宽度，
超过 64 字符的值会被拒绝，而不是截断。

**为什么两个 ID 都必填**：`X-Harness-Request-Id` 是账本主键（B1/B3 都按它查）；
`X-Harness-Run-Id` 会写入每条账本行并在 B1 事件中返回，让同一轮任务的多次调用
可以聚合。一个 run 对应 N 次调用，两者不是重复关系。

### 2.1 资源池（可选头 `X-Route-Group`）

**路由归属网关。** 经济/企业档的划分已取消：Harness 只需调用 A3 返回的模型 id，
由网关自行决定走哪个渠道、哪个上游。Harness **不需要**为普通调用选择资源池，
也不需要感知池的划分。

**只有通过 Harness JWT 校验的调用方才能使用该头**；普通 API 令牌即使携带，也不
参与资源池选择。资源池是产品线，不是价格档位。当前有五个池：

| 池 id | 显示名 | `kind` | 说明 |
| --- | --- | --- | --- |
| `chat` | 通用对话 | `text` | **默认池**。不传 `X-Route-Group` 时的落点；A1/A2/A4/A9 的常规调用都走这里 |
| `video` | 视频 | `video` | 视频任务池。池内混合按 token 与按秒计费的模型，因此**池级不标注 `billing`**；渠道尚未接入 |
| `image` | 图片生成 | `image` | 图片生成预留池，池内混合按张、按 token 等模型，**池级不标注 `billing`**；渠道尚未接入，**本期不可调用** |
| `audio` | 语音 | `audio` | 语音/TTS 预留池，当前仅占位；渠道尚未接入，**本期不可调用** |
| `plan` | 畅用计划 | `video` | 专用于畅用计划；池级 `billing: per_call`（按次计费） |

| 情况 | 行为 |
| --- | --- |
| 不传该头 | 使用默认池 `chat` |
| 传 `plan` | 使用畅用计划池 —— 这是该头**唯一**有业务含义的用法 |
| 传其他已配置池 id（`chat` / `video` / `image` / `audio`） | 普通调用应省略该头；显式传入只会把请求硬绑定到该池，不改变模型 id |
| 传不存在的池 id | `403 access_denied`，消息 `requested resource pool is not configured` |

不存在「全渠道优先级」全局池：`global_channel_order` 已移除，资源池的渠道顺序
是唯一的路由顺序，也没有 `global` 池可供选择。池是硬绑定：选中的池里没有该模型
的可用渠道时直接失败，不会改用另一个池。

`video` 与 `audio` 的渠道尚未配置，这两个池的 `models` 可能合法地返回空数组，
此时该池不可调用。池的存在只表示产品线已定义，不表示已经可以接单。

供应商税率已取消，B3 金额不再含税额。`billing.group_ratio` 来自采购计价分组，
不是资源池的倍率；`pricing_group_order` 只判断哪些渠道+模型能力具备采购分组
资格，并给出对应倍率，当前仅 `default` 一个分组，因此 `group_ratio = 1`。

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
ETag: W/"e10bdaded71746aa5b1890a9a0f5e6259d3e5f744a2115f8b951694906bb7fe3"
```

```json
{
  "object": "list",
  "data": [
    {
      "id": "chat",
      "object": "model",
      "display_name": "通用对话",
      "kind": "text",
      "models": ["claude-opus-5", "glm-5.3", "deepseek-v4-pro", "deepseek-v4.1-flash"],
      "capabilities": ["chat", "stream", "tools"]
    },
    {
      "id": "video",
      "object": "model",
      "display_name": "视频",
      "kind": "video",
      "models": [],
      "capabilities": []
    },
    {
      "id": "image",
      "object": "model",
      "display_name": "图片生成",
      "kind": "image",
      "models": [],
      "capabilities": []
    },
    {
      "id": "audio",
      "object": "model",
      "display_name": "语音",
      "kind": "audio",
      "models": [],
      "capabilities": []
    },
    {
      "id": "plan",
      "object": "model",
      "display_name": "畅用计划",
      "kind": "video",
      "billing": "per_call",
      "models": [],
      "capabilities": []
    }
  ],
  "catalog_version": "3",
  "etag": "W/\"e10bdaded71746aa5b1890a9a0f5e6259d3e5f744a2115f8b951694906bb7fe3\""
}
```

> `etag` 与 `catalog_version` 为示例值，实际值以响应为准。

| 字段 | 说明 |
| --- | --- |
| `data[].id` | **资源池 id**（`chat` / `video` / `image` / `audio` / `plan`）；其中仅 `plan` 有 `X-Route-Group` 业务含义，普通调用不需要该头 |
| `data[].display_name` | 运营配置的显示名，仅用于界面展示 |
| `data[].kind` | `text` / `video` / `image` / `audio` |
| `data[].billing` | 仅 `plan` 池返回 `per_call`；`video` / `image` 池混合多种计费方式，不标注池级 `billing` |
| `data[].models` | 该池可路由的**真实模型 id**，直接用作请求的 `model` 字段；空数组表示该池尚无可调模型 |
| `data[].capabilities` | 该池支持的能力枚举：`chat` / `stream` / `tools`，按池聚合，不区分单个模型 |
| `catalog_version` | 单调递增的整数（字符串形式），不是时间戳 |
| `etag` | 弱 ETag，同时出现在响应体和 `ETag` 响应头 |

### 3.3 条件查询（304）

```http
GET /v1/models
Authorization: Bearer <JWT>
If-None-Match: W/"e10bdaded71746aa5b1890a9a0f5e6259d3e5f744a2115f8b951694906bb7fe3"
```

```http
HTTP/1.1 304 Not Modified
```

**304 响应无 body。** 目录未变化时，网关不重复传输内容。

### 3.4 约束

- 只返回**可路由且已配置价格**的模型：列表里的模型都可以直接调用；渠道被删除、
  价格被移除后，该模型会从列表消失
- 返回真实模型 id，但**不返回渠道、上游厂商和采购价**
- 池即使暂时没有可调模型也会返回（`models: []`），便于对接方按最终结构开发。
  `video` / `audio` 池当前就是这种情况：产品线已定义，渠道尚未配置
- 目录变更时 `etag` 与 `catalog_version` 同时变化
- 目录是**调用能力**的快照，不保证当时有可用渠道；限流或额度耗尽仍可能返回 503

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
        "group_ratio": 1,
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

**税额已取消**：`total_amount = base_amount × group_ratio`，不存在税额乘数。
`tax_rate_percent` 仍出现在报文里且恒为 `0`，只为兼容旧解析器保留，属**废弃字段**，
将在后续版本移除；Harness 不应再读取或参与计算。

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
            "group_ratio": 1,
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
| `upstream_baseline` | 网关基线提交；`45fbb149` 为 V0.9.1 构建值，本轮构建的基线待部署后确认 |
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
| 403 | `GATEWAY_UNAUTHORIZED` | 缺少所需 scope。B1/B3 在 `details.required_scope` 中给出所需 scope；A3 只返回 `code` + `message`，不带 `details` | 否 |
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
  -H 'If-None-Match: W/"e10bdade..."'
```

### 11.5 发起模型调用

```bash
# 先拉目录，从 data[].models 里选一个真实模型 id。
# 普通调用不传 X-Route-Group，使用默认 chat 池。
curl -sS -X POST "http://<gateway>/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_01H8X2K9" \
  -H "X-Harness-Run-Id: run_01H8Z3M1" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"hi"}]}'

# 流式：同一模型，加 "stream": true，同样不传池头
curl -sS -N -X POST "http://<gateway>/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_01H8X2KA" \
  -H "X-Harness-Run-Id: run_01H8Z3M1" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4.1-flash","stream":true,"messages":[{"role":"user","content":"hi"}]}'
```

> `X-Route-Group` 只用于畅用计划：需要在该计划下调用时传
> `-H "X-Route-Group: plan"`。该头仅对通过 Harness JWT 校验的调用方生效，
> 不需要额外的令牌授权。普通 chat/video/audio 调用不要传该头——传了也不会
> 改变模型 id，只会把请求硬绑定到该池。`video` / `audio` 池当前没有可用渠道，
> 不要用它们发起调用。

---

## 十二、变更记录

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版 |
| 2026-10-26 | V0.9 | 对应网关 `725ec9d6d`；样例与线上实测结果一致 |
| 2026-10-29 | V0.9.1 | 对应网关 `fd00dae`。`model` 改为真实模型 id；A3 增加 `display_name`/`kind`/`billing`/`models`；`X-Harness-Run-Id` 改为必填且两个 ID 均 ≤64；缺失关联头改为 400；补充 `X-Route-Group` 与默认池规则 |
| 2026-11-05 | V0.10 | 取消经济/企业档，路由完全归网关；资源池改为产品线 `chat`（默认）/`video`/`image`/`audio`/`plan` 五个池；`X-Route-Group` 改为仅 `plan` 有业务含义；A3 样例改为五个池；取消供应商税率，`total_amount = base_amount × group_ratio`，`tax_rate_percent` 恒 0 并标记废弃。对应本轮构建（待发布），尚未部署验证 |
| 2026-11-06 | V0.10（修订） | 路由简化：移除 `global_channel_order` / 全渠道优先级全局池与 `token_group_access` / 令牌授权，资源池成为唯一路由顺序；`X-Route-Group` 仅经验证的 Harness 调用方可使用；补充 `pricing_group_order` 是采购分组资格与倍率来源、不是渠道路由顺序。移除令牌未授权 403 行。代码已改，未构建、未部署、未验证 |
| 2026-11-07 | RC48 候选 | 基于官方 `v1.0.0-rc.48` 对齐；保留五个 Harness 产品池与账本契约；补齐 task-plugin 协议路由的 Harness scope 检查。当前候选通过本地 Go、Web 与 SQLite/MySQL/PostgreSQL 检查；尚未部署或真实联调 |
