# Harness 网关接口接入指南

**版本**：V1.2（2026-11-23 修订）\
**测试 Base URL**：`https://testapi.soloai.chat`\
**对应网关版本**：`harness-api-20261117-b040241`（revision `b04024190`）\
**A3 目录版本**：`catalog_version=10`\
**B4 接口版本**：`interface_version=v2`（与本文档 V1.2 不是同一版本号）\
**面向**：Harness 后端开发与联调

本文档给出网关侧对外接口的当前调用方法。当前反馈结论见
[11-feedback-resolution-20261123.md](./11-feedback-resolution-20261123.md)，模型复测以
[12-retest-model-list-20261123.md](./12-retest-model-list-20261123.md) 为准；字段语义细节见
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
Messages / Gemini 原生）。网关侧在以下方面与官方协议不同：

- 所有请求使用测试 Base URL `https://testapi.soloai.chat`。
- `model` 字段填 **A3 目录 `models[]` 中的真实模型 id**（如 `deepseek-flash`），
  不是池 id。**路由完全由网关负责**：Harness 不需要区分经济/企业档，也不需要按
  档位选择渠道。A3 条目不保证当前均可调用，模型选择以第 12 号可复测清单为准。
- `X-Route-Group` 仅对通过 Harness JWT 校验的调用方生效。`chat` 模型不传（默认
  `chat` 池）；`video` / `image` / `plan` 池模型必须显式传对应池 id，见第二节。
- 每次模型调用必须携带 `X-Harness-Request-Id` 和 `X-Harness-Run-Id`，均为 1–64
  字符，合法字符为 `A-Z`、`a-z`、`0-9`、`-`、`_`；不强制 `hrq_` / `run_` 前缀，
  不校验 ULID 格式。
- 鉴权用 Harness JWT（见鉴权指南）。
- 响应头返回 `X-NewAPI-Request-Id`。

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
| `X-Harness-Request-Id` | Harness → 网关 | ✓ | 一次模型调用一个，**1–64 字符**，只允许 `A-Z`、`a-z`、`0-9`、`-`、`_` |
| `X-Harness-Run-Id` | Harness → 网关 | ✓ | 一次任务执行一个，**1–64 字符**，只允许 `A-Z`、`a-z`、`0-9`、`-`、`_` |
| `X-Route-Group` | Harness → 网关 | 视池而定 | `chat` 可不传（默认池）；`video` / `image` / `plan` 必须传对应池 id；仅经验证的 Harness 调用方可用 |
| `X-NewAPI-Request-Id` | 网关 → Harness | 响应头 | 网关请求 ID，即账本中的 `gateway_request_id` |

两个关联 ID 都必须非空，不强制 `hrq_` / `run_` 前缀，也不校验 ULID 格式。缺失、
空值、非法字符或超过 64 字符时，网关在**转发上游之前**返回：

```http
HTTP/1.1 400 Bad Request
```

```json
{"error":{"code":"REQUEST_INVALID","message":"X-Harness-Run-Id is required","type":"new_api_error"}}
```

即：被拒绝的调用不会产生费用，也不会产生 B1/B3 事件。超过 64 字符的值会被拒绝，
而不是截断；旧文档中的 80 字符上限已废止。

**为什么两个 ID 都必填**：`X-Harness-Request-Id` 是账本主键（B1/B3 都按它查）；
`X-Harness-Run-Id` 会写入每条账本行并在 B1 事件中返回，让同一轮任务的多次调用
可以聚合。一个 run 对应 N 次调用，两者不是重复关系。

### 2.1 资源池（`X-Route-Group`）

**路由归属网关。** 经济/企业档的划分已取消：Harness 调用真实模型 id，由网关自行
决定走哪个渠道、哪个上游。`chat` 模型可不传 `X-Route-Group`，使用默认池；`video` /
`image` / `plan` 模型必须显式传对应池 id，否则请求落到默认 `chat` 池并因模型不在
该池而失败。

**只有通过 Harness JWT 校验的调用方才能使用该头**；普通 API 令牌即使携带，也不
参与资源池选择。资源池是产品线，不是价格档位。当前有五个池：

| 池 id | 显示名 | `kind` | 说明 |
| --- | --- | --- | --- |
| `chat` | 通用对话 | `text` | **默认池**。不传 `X-Route-Group` 时的落点；A1/A2/A4/A9 的常规调用都走这里 |
| `video` | 视频 | `video` | 视频任务池，按秒视频模型（PAI 按秒、video-line 按秒等）；**池级不标注 `billing`** |
| `image` | 图片生成 | `image` | 图片生成池。A3 声明 10 个图片模型（4 个 GPT Image 同步 + 6 个 Nano Banana Gemini 同步），按张计价，**池级不标注 `billing`** |
| `audio` | 语音 | `audio` | 语音/TTS 预留池，当前仅占位；渠道尚未接入，**本期不可调用** |
| `plan` | 畅用计划 | `video` | 按次视频模型（PAI 按次、Wan 按次、video-line 按次等）；池级 `billing: per_call`（按次计费） |

| 情况 | 行为 |
| --- | --- |
| 不传该头 | 使用默认池 `chat` |
| 传 `video` | 使用视频池；调用 `video` 池的按秒视频模型时必须传 |
| 传 `plan` | 使用畅用计划池；调用 `plan` 池的按次视频模型时必须传 |
| 传 `chat` | 显式绑定 chat 池 |
| 传 `image` | 使用图片生成池；调用 `image` 池模型时必须传 |
| 传 `audio` | 使用语音池；该池当前无可用模型，调用会失败 |
| 传不存在的池 id | `403 access_denied`，消息 `requested resource pool is not configured` |

2026-11-25 渠道声明已与上游 `/v1/models` 对齐，A3 当前返回
`chat=26`、`video=49`、`plan=17`、`image=10`、`audio=0`。A3 已与可调清单一致；
精确模型 ID、复测备注与已下架清单仍以第 12 号文档为准。

跨池重复本身合法，同一模型可以出现在多个池；池只限制可选渠道集合，不会重写模型
ID。A3 的 `capabilities` 是池级端点类型汇总，可能包含 `chat` / `stream` / `tools`
等通用标签，不能据此判断图片或视频的真实能力。

不存在「全渠道优先级」全局池：`global_channel_order` 已移除，资源池的渠道顺序
是唯一的路由顺序，也没有 `global` 池可供选择。池是硬绑定：选中的池里没有该模型
的可用渠道时直接失败，不会改用另一个池。

`audio` 的渠道尚未配置，该池的 `models` 合法地返回空数组，此时该池不可调用。
`video`、`image` 与 `plan` 已配置渠道与模型，但具体可调集合以第 12 号文档和最新
现场确认为准。池的存在只表示产品线已定义，不表示已经可以接单。

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
ETag: W/"example-catalog-etag"
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
      "models": ["claude-fable-5", "deepseek-flash"],
      "capabilities": ["chat", "stream", "tools"]
    },
    {
      "id": "video",
      "object": "model",
      "display_name": "视频",
      "kind": "video",
      "models": ["MiniMax-H3-Max-480p", "seedance2-1080p-second-1"],
      "capabilities": []
    },
    {
      "id": "image",
      "object": "model",
      "display_name": "图片生成",
      "kind": "image",
      "models": ["Nano Banana 2.1 1K", "gpt-image-2.6-flare"],
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
      "models": ["gemini-omni-1.2-flash-720p-10s", "seedance2-720p-once-1"],
      "capabilities": []
    }
  ],
  "catalog_version": "10",
  "etag": "W/\"example-catalog-etag\""
}
```

> `etag` 与 `models` 均为示例值；JSON 仅说明响应结构，不是完整目录快照。
> 2026-11-25 对齐后 A3 为 `chat=26`、`video=49`、`plan=17`、`image=10`、
> `audio=0`；11-23 的 `video=83`、`plan=80` 为历史目录声明。模型选择以
> [12-retest-model-list-20261123.md](./12-retest-model-list-20261123.md) 为准，
> 不要按样例数量做校验。`models` 按模型 id 字典序返回，模型集合会随渠道与价格变化。

| 字段 | 说明 |
| --- | --- |
| `data[].id` | **资源池 id**（`chat` / `video` / `image` / `audio` / `plan`）；`chat` 为默认池，调用 `video` / `image` / `plan` 模型时需传对应 `X-Route-Group` |
| `data[].display_name` | 运营配置的显示名，仅用于界面展示 |
| `data[].kind` | `text` / `video` / `image` / `audio` |
| `data[].billing` | 仅 `plan` 池返回 `per_call`；`video` 池为按秒视频模型、`image` 池按张计价，均不标注池级 `billing` |
| `data[].models` | 该池渠道声明的**真实模型 id**，可用作请求的 `model` 字段；不保证当前均可调用，复测以第 12 号文档为准；空数组表示该池没有目录候选 |
| `data[].capabilities` | 池级端点类型汇总，可能包含 `chat` / `stream` / `tools` 等通用标签；不区分单个模型，不能据此判断图片或视频的真实能力 |
| `catalog_version` | 单调递增的整数（字符串形式），不是时间戳 |
| `etag` | 弱 ETag，同时出现在响应体和 `ETag` 响应头 |

### 3.3 模型列表的四个对外分区

A3 的 `data[]` 仍以资源池为单位返回，但平台侧模型列表现按四个对外分区展示：

| 分区 | 对应池 | 判别字段 | 说明 |
| --- | --- | --- | --- |
| 文字模型 | `chat` | `kind: text` | 默认池；A1/A2/A4/A9 常规调用 |
| 图片生成模型 | `image` | `kind: image` | 图片生成池；当前 10 个图片模型（4 个 GPT Image + 6 个 Nano Banana） |
| 视频模型 | `video` | `kind: video` | 视频任务池；按秒视频模型（PAI 按秒、video-line 按秒等） |
| plan 计划 | `plan` | `kind: video` + `billing: per_call` | 畅用计划；按次视频模型（PAI 按次、Wan 按次、video-line 按次等）；单独展示为 plan 分区，不并入视频分区 |

> 2026-11-25 对齐后 A3 为 `chat` 26 / `video` 49 / `plan` 17 / `image` 10 /
> `audio` 0；11-23 的 `video` 83 / `plan` 80 为历史目录声明。模型选择以第 12 号文档为准。

`audio` 仍是预留资源池（`kind: audio`），本期不进入这四个对外分区；有音频渠道后再定义它的展示位置。

**分区内不再二次分组。** 每个分区的 `models` 是一个扁平的真实模型 id 数组；不要再按供应商、计价分组、能力、渠道、模型子类或 upstream 模型名拆成二级分组。平台直接按数组渲染模型行；`capabilities` 是池级聚合标签，不是模型分组字段。

### 3.4 条件查询（304）

```http
GET /v1/models
Authorization: Bearer <JWT>
If-None-Match: W/"e10bdaded71746aa5b1890a9a0f5e6259d3e5f744a2115f8b951694906bb7fe3"
```

```http
HTTP/1.1 304 Not Modified
```

**304 响应无 body。** 目录未变化时，网关不重复传输内容。

### 3.5 约束

- 返回资源池和渠道声明的真实模型 id，但**不返回渠道、上游厂商和采购价**。
- A3 条目不等于当前均可直接调用。渠道 `models` 中的过期或跨组声明可能导致目录候选
  多于真实可调集合；历史 TA Token 模型和已下架 ID 不得当作当前在售模型。
- 池即使暂时没有目录候选也会返回（`models: []`），便于对接方按最终结构开发。
  `audio` 池当前就是这种情况：产品线已定义，渠道尚未配置。
- 跨池重复合法，不应以“交集必须为 0”作为验收标准；2026-11-25 已清理渠道声明中
  上游不提供或已下架的模型。
- 目录变更时 `etag` 与 `catalog_version` 同时变化。
- A3 是**目录声明**的快照，不是调用能力保证；限流、额度耗尽、线路故障或模型下架
  仍可能返回 4xx / 5xx。2026-11-25 目录已对齐；仍以第 12 号可复测清单和最新现场
  确认为准。

### 3.6 图片生成（简要）

图片模型统一传 `X-Route-Group: image`，有两种协议：

| 模型 | 提交 | 完成方式 | 价格（每张，`n=1`） |
| --- | --- | --- | --- |
| GPT Image（`gpt-image-2.5-*` / `gpt-image-2.6-*`，4 个） | 同步 `POST /v1/images/generations` | 响应 `data[0]` 直接返回图片，可能是 `b64_json` | 0.018 USD |
| Nano Banana（`Nano Banana 2.1 *` / `Nano Banana Pro *`，6 个） | Gemini 同步 `POST /v1beta/models/{URL编码模型}:generateContent` | 响应 `candidates[].content.parts[].inlineData.data` 直接返回 Base64 图片；无轮询 | 2.1 1K/2K 0.009、2.1 4K 0.011、Pro 0.016 USD |

GPT Image 使用 `model` / `prompt` / `n: 1` / `size`（1K 为 `1024x1024`），不要依赖
`response_format=url`；Nano Banana 使用 `contents` / `generationConfig.imageConfig.imageSize`
（1K/2K/4K），当前只验证 `aspectRatio: "1:1"` 与 `candidateCount: 1`。

**Nano Banana 超时与重试**：该接口为同步图片协议，不返回 task id。1K/4K 长请求
可能触发 524/500；客户端应使用足够长的超时。超时后先核对 B3 的 `request_id` 和
产物，再决定是否有限重试；没有统一 task id，不能盲目重提。

**MIME 校验**：成功样本可能出现实际 JPEG 内容被标为 PNG 的情况。保存文件时应按
文件头判断真实格式，不要只信 MIME 或扩展名。视频下载同样可能透传上游通用
`Content-Type`，接收端也应结合文件头校验。

图片按张计费，B1 没有状态字段且 Token 数不代表费用（Nano Banana 同步请求可能为
`usage_source=estimated`；GPT Image 同步可为 `usage_source=upstream`）。B1
`usage_unit=token` 与 B3 `billing_basis=token` 是当前字段限制，不表示图片按 Token
计费；完成与成本以 B3 `status_change` → `settled`、`total_amount` 和提交时价格版本
为准。完整样例见 [09-image-api-examples.md](./09-image-api-examples.md)。

> 视频任务（`video` / `plan` 池）的请求字段差异、轮询和内容下载样例，见
> [08-video-api-examples.md](./08-video-api-examples.md)。

### 3.7 2026-11-23 目录校准

2026-11-25 对齐后 A3 直接返回可调集合：

| 池 | 2026-11-23 A3（历史） | 2026-11-25 A3（当前） | 说明 |
| --- | ---: | ---: | --- |
| `chat` | 26 | 26 | 11-21 实测 26/26；11-23 有 3 条上游失败记录，见第 12 号文档 |
| `image` | 10 | 10 | GPT Image 4/4；Nano Banana 4/6，MIME 仍待整改 |
| `video` | 83 | **49** | 渠道声明已与上游对齐 |
| `plan` | 80 | **17** | TA token 22 档已移除 |
| `audio` | 0 | 0 | 预留池，本期不可调用 |

- A3 目录声明不等于实测可调集合；`video=49`、`plan=17` 也不等于所有模型已在同一
  窗口完成生成验收。完整模型 ID、Chat 复测备注和已下架清单见
  [12-retest-model-list-20261123.md](./12-retest-model-list-20261123.md)。
- `plan` 的池级语义为按次视频；具体价格、时长和素材限制以每个精确模型的目录价格
  和接口说明为准。历史 TA Token 模型不得当作当前在售模型。
- 上游不返回 `usage` 的异步视频任务，B1 使用 `usage_source=unavailable`、Token
  字段 `null`、`split_known=false`，`total_tokens` 省略；费用以 B3 `settled` 为准。

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
| `limit` | ✗ | 默认 50，最大 200；超过 200 时静默 clamp 到 200，并继续返回 `next_cursor` |
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

**异步视频无上游 usage 时**，B1 事件使用以下口径：

```json
{
  "usage_unit": "token",
  "usage": {
    "input_tokens": null,
    "output_tokens": null,
    "cache_read_tokens": null,
    "cache_write_tokens": null,
    "split_known": false
  },
  "usage_source": "unavailable",
  "is_estimated": false
}
```

`total_tokens` 省略，四个 Token 分项为 `null`，表示未测量，**不是实测 0 Token**。
历史 TA Token 完成记录中的 Token 数不能直接回补到当前 B1。B1 `usage_unit=token`
是当前字段限制，不表示图片或视频按 Token 计费；B3 `settled` 和金额仍是费用依据。

Nano Banana 同步图片请求可能返回 `usage_source=estimated`，表示估算 Token，不等同
于上游官方 usage。Gemini thinking tokens 是否计入 output，以当前归一化实现和 B1
实际返回为准，不另行相加。

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

**单位字段限制**：B3 `billing_basis` 当前固定为 `token`，不表示图片或视频按 Token
计费，也不存在 B3 `usage_unit` 字段。图片按张、视频按秒 / 按次的金额以 B3 最新版本
`total_amount`、`status=settled` 和提交时价格版本为准。`pending` 或零额退款记录的
`procurement_price_version` 可能为空，是否为有效终态需结合事件类型和后续版本链判断。

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
| 跨请求事件总数 | 合计最多 500；超过时返回前 500 条并标 `truncated=true` |
| 单请求事件数 | 单个请求自身超过 500 条时返回 `400 EVENT_LIMIT_EXCEEDED` |
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
| `upstream_baseline` | 网关基线提交；`45fbb149` 为 V0.9.1 构建值，`bb266025` 为 RC48 合入时的基线；当前构建的 B4 实际返回值待现场复核 |
| `time` | 服务器时间，RFC3339 UTC 毫秒 |

---

## 九、分页与限制

| 项 | 规则 |
| --- | --- |
| 日常增量默认 limit | 50 |
| 日常增量最大 limit | 200；超过 200 静默 clamp 到 200，并返回 `next_cursor` |
| 日常增量时间窗 | ≤ 24 小时 |
| BatchGet Request ID 上限 | 100 |
| BatchGet 跨请求事件上限 | 合计 500；超过时返回前 500 条并标 `truncated=true` |
| BatchGet 单请求事件上限 | 单个请求自身超过 500 条返回 `400 EVENT_LIMIT_EXCEEDED` |
| 时间格式 | RFC3339 UTC，统一三位毫秒：`2026-10-25T11:20:37.068Z` |

---

## 十、错误码

### 10.1 错误响应结构

错误体尚未完全统一，Harness 应按接口和错误类型分别解析。

**JWT 401：OpenAI 风格**

```json
{
  "error": {
    "message": "Invalid authentication credentials",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

不保证包含 `retryable` 或 `request_id`。

**Scope 403：模型调用**

```json
{
  "error": {
    "code": "access_denied",
    "message": "requested resource pool is not configured",
    "type": "new_api_error"
  }
}
```

A3/B1/B3 的 scope 错误可能使用 `GATEWAY_UNAUTHORIZED`，并带 `retryable=false` 或
`details.required_scope`；不要假设所有 403 都是同一种结构。

**网关参数 / 游标 / 窗口错误**

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

**任务插件错误**：部分错误可能使用顶层 `{code,message,data}` 或插件原生错误体。
Harness 应同时兼容嵌套 `error` 与顶层字段，并保留原始状态码和消息用于诊断。

### 10.2 错误码表

| HTTP | code / 类型 | 含义 | 是否可重试 |
| --- | --- | --- | --- |
| 401 | OpenAI 风格 `error.code/type/message` | JWT 无效或过期；字段不保证齐全 | 否 |
| 403 | `access_denied` | 模型调用缺少资源池或调用权限 | 否 |
| 403 | `GATEWAY_UNAUTHORIZED` | A3/B1/B3 缺少所需 scope；可能带 `details.required_scope` | 否 |
| 400 | `REQUEST_INVALID` | 请求体、关联 ID 或 BatchGet 参数不合法 | 否 |
| 400 | `CURSOR_INVALID` | 游标格式错误或流不匹配 | 否 |
| 400 | `WINDOW_TOO_LARGE` | 时间窗超过 24h | 否 |
| 400 | `EVENT_LIMIT_EXCEEDED` | BatchGet 中单个请求自身超过 500 条事件 | 否 |
| 404 | `REQUEST_NOT_FOUND` | 请求 ID 不存在 | 否 |
| 500 | `GATEWAY_UPSTREAM` / 插件原生错误 | 网关、上游或任务插件错误 | 视错误类型 |

### 10.3 A5 与 `/api/pricing`

- **A5 `count_tokens` 当前未交付**：Claude 路由未注册，Gemini `:countTokens`
  返回 404。Harness 不应把 404 当作临时故障自动重试。
- **`/api/pricing` 不是合同接口**：它是通用定价展示接口，默认公开但可配置关闭或
  鉴权；不是 A3 目录，也不是 B3 对账依据。目录以 A3 + 第 12 号清单为准，费用以
  B3 `settled` 记录为准。

---

## 十一、调用示例

### 11.1 拉取健康状态

```bash
curl -sS "https://testapi.soloai.chat/internal/v1/gateway/health" \
  -H "Authorization: Bearer $HARNESS_JWT"
```

### 11.2 增量拉取用量

```bash
curl -sS "https://testapi.soloai.chat/internal/v1/gateway/usage-events?limit=50" \
  -H "Authorization: Bearer $HARNESS_JWT"
```

### 11.3 批量补查成本

```bash
curl -sS -X POST "https://testapi.soloai.chat/internal/v1/gateway/cost-records:batchGet" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "Content-Type: application/json" \
  -d '{"request_ids":["hrq_01H8X"],"include_status_events":true}'
```

### 11.4 条件拉取目录

```bash
# 首次
curl -sS -D- "https://testapi.soloai.chat/v1/models" \
  -H "Authorization: Bearer $HARNESS_JWT"

# 拿到 ETag 后
curl -sS -D- "https://testapi.soloai.chat/v1/models" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H 'If-None-Match: W/"e10bdade..."'
```

### 11.5 发起模型调用

```bash
# 先拉目录，并按第 12 号可复测清单选择真实模型 id。
# 普通 chat 调用不传 X-Route-Group，使用默认 chat 池。
curl -sS -X POST "https://testapi.soloai.chat/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_01H8X2K9" \
  -H "X-Harness-Run-Id: run_01H8Z3M1" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-flash","messages":[{"role":"user","content":"hi"}]}'

# 流式：同一模型，加 "stream": true，同样不传池头
curl -sS -N -X POST "https://testapi.soloai.chat/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_01H8X2KA" \
  -H "X-Harness-Run-Id: run_01H8Z3M1" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-flash","stream":true,"messages":[{"role":"user","content":"hi"}]}'
```

> `X-Route-Group` 用于绑定非默认池：`video` 池模型传
> `-H "X-Route-Group: video"`，`image` 池模型传 `-H "X-Route-Group: image"`，
> `plan` 池模型传 `-H "X-Route-Group: plan"`；`chat` 模型可不传。该头仅对通过
> Harness JWT 校验的调用方生效，不需要额外的令牌授权。`audio` 池当前没有可用渠道，
> 不要用它发起调用；`video` / `image` / `plan` 的具体可调模型以第 12 号清单为准，
> 图片调用样例见 [09-image-api-examples.md](./09-image-api-examples.md)。

---

## 十二、变更记录

> 历史版本记录仅用于追溯，不代表当前目录、协议或可调范围；与 V1.2 不一致时，以本版及后续修订为准。

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版 |
| 2026-10-26 | V0.9 | 对应网关 `725ec9d6d`；样例与线上实测结果一致 |
| 2026-10-29 | V0.9.1 | 对应网关 `fd00dae`。`model` 改为真实模型 id；A3 增加 `display_name`/`kind`/`billing`/`models`；`X-Harness-Run-Id` 改为必填且两个 ID 均 ≤64；缺失关联头改为 400；补充 `X-Route-Group` 与默认池规则 |
| 2026-11-05 | V0.10 | 取消经济/企业档，路由完全归网关；资源池改为产品线 `chat`（默认）/`video`/`image`/`audio`/`plan` 五个池；`X-Route-Group` 改为仅 `plan` 有业务含义；A3 样例改为五个池；取消供应商税率，`total_amount = base_amount × group_ratio`，`tax_rate_percent` 恒 0 并标记废弃。对应本轮构建（待发布），尚未部署验证 |
| 2026-11-06 | V0.10（修订） | 路由简化：移除 `global_channel_order` / 全渠道优先级全局池与 `token_group_access` / 令牌授权，资源池成为唯一路由顺序；`X-Route-Group` 仅经验证的 Harness 调用方可使用；补充 `pricing_group_order` 是采购分组资格与倍率来源、不是渠道路由顺序。移除令牌未授权 403 行。代码已改，未构建、未部署、未验证 |
| 2026-11-07 | RC48 候选 | 基于官方 `v1.0.0-rc.48` 对齐；保留五个 Harness 产品池与账本契约；补齐 task-plugin 协议路由的 Harness scope 检查。当前候选通过本地 Go、Web 与 SQLite/MySQL/PostgreSQL 检查；尚未部署或真实联调 |
| 2026-11-08 | RC48 部署 | `harness-api` 合入 RC48 后构建 `harness-api-20261108-bb26602`，已部署 82 实例并通过容器健康检查与 `/api/status` 探测；真实 Harness JWT / 供应商 E2E 待执行 |
| 2026-11-14 | V0.11 | `image` 池配置 10 个图片模型（4 个 GPT Image 同步 + 6 个 Nano Banana 异步），新增 `X-Route-Group: image` 与按张计价；A3 样例 `catalog_version` 更新为 `"7"`；新增 3.6 图片生成小节与 `09-image-api-examples.md`；图片 B1 仅 usage、B3 `settled` 为准。部署 `harness-api-20261114-c29f1a6` |
| 2026-11-15 | V0.11（修订） | 异步任务无 Token 用量时 B1 改为 `usage_source=unavailable`、Token 字段 `null`、`split_known=false`；已有 `total_tokens` 保持 `upstream`；撤销 B1 结算收尾事件。部署 `harness-api-20261115-de7fce8`，三模型 E2E 复测通过 |
| 2026-11-17 | V0.12 | Nano Banana 改为 Gemini 同步 `generateContent`（无轮询）；B1 为 `estimated` prompt Token，B3 固定按张价；`supplyvideo` 1.4.0 补齐新视频模型；全模型验收 语言 35/38、图片 10/10、视频 55/61。部署 `harness-api-20261116-df87870` |
| 2026-11-17 | V0.13 | 资源池语义更新：`video` 为按秒视频模型（PAI 按秒、TA 按秒、video-line 按秒），`plan` 为按次视频模型（TA token、PAI 按次、Wan 按次、video-line 按次）；新增 Seedance 2.5 Flash 5 个规格（3 个按秒进 `video`、2 个按次进 `plan`）及价格摘要；补充 Flash 按秒上游不返回 `usage` 时网关按请求 `seconds` 估算参考秒的结算口径。对应网关 `harness-api-20261117-b040241`（revision `b04024190`），A3 实测 `catalog_version` `"10"`：chat 26 / video 83 / plan 80 / image 10 / audio 0 |
| 2026-11-23 | V1.2 | Base URL 更新为 `https://testapi.soloai.chat`；Run ID 校准为必填 1–64 字符；明确 `X-Route-Group` 必填规则；区分 A3 v10 目录声明与初步核实可调 `chat=26` / `image=10` / `video=49` / `plan=17`；补充 Nano Banana 同步协议、超时与 MIME、异步视频 B1 `unavailable/null`、B1/B3 `token` 单位字段限制、实际错误体、分页 clamp 200、BatchGet 100 ID / 500 事件、A5 404 与 `/api/pricing` 非合同口径。事实源见第 11、12 号文档 |
| 2026-11-25 | V1.3 | 渠道 `models` 与上游 `/v1/models` 对齐，A3 变为 `chat=26` / `video=49` / `image=10` / `plan=17`；7 个上游分组代表视频模型实测最终全部 SUCCESS；OpenResty 代理超时放宽到 600 秒；Nano Banana 1K/4K 公网通过，MIME 仍待修正 |
