# Harness × New API｜A3 模型目录结构更新

**日期**：2026-11-14（2026-11-08 初版，2026-11-14 更新，2026-11-17 增量补充，2026-11-23 口径校准）
**适用对象**：Harness 后端与模型列表页面
**变更范围**：仅模型目录 A3 及模型选择方式；B1/B3 账本接口沿用现有对接规范。

> 2026-11-17 增量（部署 revision `b04024190` 后实测 `GET /v1/models`）：`catalog_version` 由 `"7"` 变为 `"10"`。
> 当前各资源池数量为：`chat` 26 个、`video` 83 个、`plan` 80 个、`image` 10 个、`audio` 0 个。
> `video` 池新增 3 个 Seedance 2.5 Flash 按秒规格，`plan` 池新增 2 个 Seedance 2.5 Flash 按次规格；
> 新增模型均已配置价格表达式，当时将 A3 返回集合理解为可路由且可计费；该口径已在 2026-11-23 校准，见下文“2026-11-23 口径校准”。
> 价格见下文“2026-11-17 增量”表。

> 2026-11-23 校准：A3 v10 的 `video=83`、`plan=80` 是**目录候选数**，不等于当前可调；按上游实时目录与抽样调用初步核实可调分别为 **49/17**。Harness 在渠道声明校正前，以 [`12-retest-model-list-20261123.md`](./12-retest-model-list-20261123.md) 为复测清单。

> 2026-11-14 更新：`image` 池已配置 10 个图片模型（4 个 GPT Image + 6 个 Nano Banana），
> 返回样例中的 `catalog_version` 为 `"7"`；`audio` 池仍为空。图片调用方式见
> [09-image-api-examples.md](./09-image-api-examples.md)。2026-11-08 的目录结构与
> 分区规则不变。

## 1. 本次调整

双方已沟通取消原有的 **economy / enterprise 业务档位**及模型列表的**二级分组**。Harness 直接读取模型 ID，不再根据供应商、价格、渠道或能力二次分组；上游渠道及路由由 New API 负责。

- **模型列表接口**：`GET /v1/models`（Harness JWT，需 `gateway.catalog.read`）。
- **前台四个展示分区**：文字模型、图片生成模型、视频模型、Plan 计划。
- **API 当前仍返回五个资源池**：`chat`、`video`、`image`、`audio`、`plan`。其中 `audio` 为预留，不进入本期四个展示分区。
- **实际调用**：使用对应 `models[]` 中的真实模型 ID 作为请求 `model`；普通文字调用不传 `X-Route-Group`，`video` 池模型传 `X-Route-Group: video`，`image` 池模型传 `X-Route-Group: image`，Plan 计划模型传 `X-Route-Group: plan`。
- **`capabilities`**：目前是**资源池级汇总**，不能代表池内每个模型均支持这些能力；尤其不能据此判断图片、视频的真实媒体能力。本期列表展示可忽略，媒体接口和可调模型以具体接口说明及第 12 号复测清单为准。
- **目录候选不等于可调**：`models[]` 中的条目只表示目录声明；是否可调取决于对应池内是否存在能服务该模型的启用渠道，以及上游当前是否仍提供该模型。
- **跨池重复合法**：同一模型可以合法出现在多个资源池，不应以“跨池交集必须为 0”作为验收标准；需要校正的是过期、跨组或已下架的渠道声明。

> 这里取消的是旧档位和列表二级分组，并不是删除网关内部按产品线配置的资源池。`models[]` 只表示目录候选，不保证当前可调；可调状态以对应池启用渠道、上游实时目录及 [`12-retest-model-list-20261123.md`](./12-retest-model-list-20261123.md) 为准。

### 2026-11-17 增量

2026-11-17 部署 revision `b04024190` 后实测 `GET /v1/models`，`catalog_version = "10"`，新增以下 5 个 Seedance 2.5 Flash 规格：

| 模型 ID | 资源池 | 计费方式 | 价格摘要 |
| --- | --- | --- | --- |
| `seedance2.5-flash-480p-second-1` | `video` | 按秒 | 0.05 / 生成秒；带参考两段式 0.09 + 0.05 |
| `seedance2.5-flash-720p-second-1` | `video` | 按秒 | 0.06 / 生成秒；带参考 0.13 + 0.065 |
| `seedance2.5-flash-720p-second-2` | `video` | 按秒 | 0.06 / 生成秒 |
| `seedance2.5-flash-480p-once-1` | `plan` | 按次 | 0.78 / 条 |
| `seedance2.5-flash-720p-once-1` | `plan` | 按次 | 0.95 / 条 |

- 3 个 `second` 规格进入 `video` 池（按秒计费），2 个 `once` 规格进入 `plan` 池（按次计费）；调用头分别沿用 `X-Route-Group: video` 与 `X-Route-Group: plan`。
- 这 5 个模型均已配置价格表达式。2026-11-17 当时认为未配置价格或暂无可用渠道的模型不会出现在 `models[]` 中；2026-11-23 复核发现，过期/跨组渠道声明仍可能进入 A3，因此出现在 `models[]` 中不等于当前可调。
- 本次实测各池目录候选数量：`chat` 26、`video` 83、`plan` 80、`image` 10、`audio` 0。该数量是 2026-11-17 快照；当前可调状态见下节和第 12 号复测清单。

### 2026-11-23 口径校准

| 池 | A3 v10 当前目录候选 | 2026-11-23 初步核实可调 |
| --- | ---: | ---: |
| `video` | 83 | **49** |
| `plan` | 80 | **17** |

- **目录候选不等于可调**：其余 `video` 34 条、`plan` 63 条当前不可调或已下架。渠道声明校正完成前，不要仅凭 A3 条目直接提交付费任务，应使用 [`12-retest-model-list-20261123.md`](./12-retest-model-list-20261123.md)。
- **跨池重复合法**：两个池可以引用同一供应商或同一模型；当前需要修正的不是“重复”，而是渠道声明了上游不提供或已下架的模型。
- **能力标签不能代表媒体能力**：A3 的 `capabilities` 是池级端点类型汇总，可能包含 `chat/stream/tools` 等通用标签，不能据此判断图片、视频的真实能力。
- TA Seedance Token 22 档已下架；历史 JSON 中保留这些 ID 仅用于说明旧响应结构，不得作为当前调用清单。

## 2. 请求示例

```http
GET /v1/models
Authorization: Bearer <HARNESS_JWT>
```

首次读取返回 HTTP 200；可保存响应头 `ETag`，下次携带 `If-None-Match`，目录未变时返回 HTTP 304（无响应体）。

## 3. 返回样例

以下为 **2026-11-14 的 82 实例真实样例（历史样例，已过时）**：`chat` 15 个模型，`video` 8 个 PAI 按次模型，`image` 10 个图片模型（4 个 GPT Image + 6 个 Nano Banana），`plan` 22 个 TA token 模型；`audio` 为空。`models` 按模型 id 字典序返回；模型 ID、版本及 ETag 以实际联调环境响应为准，模型集合会随渠道与价格变化。

> 历史样例提示：该 JSON 仅用于说明响应**结构**，其中 `catalog_version` 为 `"7"`、各池模型数量均为 2026-11-14 的旧值。2026-11-17 实测 `catalog_version = "10"`；2026-11-23 校准后，目录候选与初步可调清单分别见上文及 [`12-retest-model-list-20261123.md`](./12-retest-model-list-20261123.md)，本节 JSON 不再逐一重写。

> **TA token 下架标记**：下方历史 JSON 的 `plan.models[]` 中 22 个 `*-token-*` 模型均已下架，禁止作为当前调用清单。当前 `plan` 的 80 条目录候选中仅初步核实 17 条可调，复测时必须使用第 12 号清单。

```json
{
  "object": "list",
  "data": [
    {
      "id": "chat",
      "object": "model",
      "display_name": "通用对话",
      "kind": "text",
      "models": ["claude-fable-5", "claude-fable-5-1", "claude-haiku-5-5", "claude-opus-4-8", "claude-opus-5", "claude-opus-5-5", "claude-sonnet-5", "claude-sonnet-5-5", "deepseek-flash", "deepseek-v4-pro", "glm-5.3", "glm-5.3-flash", "kimi-k3", "kimi-k3.1-pro", "kimi-k3.1-pro-highspeed"],
      "capabilities": ["chat", "stream", "tools"]
    },
    {
      "id": "video",
      "object": "model",
      "display_name": "视频",
      "kind": "video",
      "models": ["seedance2-1080p-once", "seedance2-480p-once", "seedance2-720p-once-1", "seedance2-720p-once-2", "seedance2-720p-once-3", "seedance2-fast-720p-once-1", "seedance2-fast-720p-once-2", "seedance2.5-720p-once"],
      "capabilities": []
    },
    {
      "id": "image",
      "object": "model",
      "display_name": "图片生成",
      "kind": "image",
      "models": ["Nano Banana 2.1 1K", "Nano Banana 2.1 2K", "Nano Banana 2.1 4K", "Nano Banana Pro 1K", "Nano Banana Pro 2K", "Nano Banana Pro 4K", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "gpt-image-2.6-flare", "gpt-image-2.6-sunburst"],
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
      "models": ["seedance2-1080p-token-no-video", "seedance2-1080p-token-with-video", "seedance2-480p-token-no-video", "seedance2-480p-token-with-video", "seedance2-4k-token-no-video", "seedance2-4k-token-with-video", "seedance2-720p-token-no-video", "seedance2-720p-token-with-video", "seedance2-fast-480p-token-no-video", "seedance2-fast-480p-token-with-video", "seedance2-fast-720p-token-no-video", "seedance2-fast-720p-token-with-video", "seedance2-mini-480p-token-no-video", "seedance2-mini-480p-token-with-video", "seedance2-mini-720p-token-no-video", "seedance2-mini-720p-token-with-video", "seedance2.5-1080p-token-no-video", "seedance2.5-1080p-token-with-video", "seedance2.5-480p-token-no-video", "seedance2.5-480p-token-with-video", "seedance2.5-720p-token-no-video", "seedance2.5-720p-token-with-video"],
      "capabilities": []
    }
  ],
  "catalog_version": "7",
  "etag": "W/\"example-catalog-etag\""
}
```

## 4. Harness 接入要点

| 展示分区 | 使用的 `data[].id` | 识别规则 | 调用头 |
| --- | --- | --- | --- |
| 文字模型 | `chat` | `kind=text` | 不传 `X-Route-Group` |
| 图片生成模型 | `image` | `kind=image` | 当前 10 个模型；传 `X-Route-Group: image`。GPT Image 4 个走同步 `POST /v1/images/generations`，Nano Banana 6 个走 Gemini 同步 `POST /v1beta/models/{URL编码模型}:generateContent` |
| 视频模型 | `video` | `kind=video`，无 `per_call` | `X-Route-Group: video` |
| Plan 计划 | `plan` | `billing=per_call`，独立于视频列表 | `X-Route-Group: plan` |

- 每个分区直接展开 `models[]`，无需再建立第二层分组；`models: []` 表示暂无可用模型。
- `audio` 目前仅保留接口结构，不进入展示。未来如果接入语音，优先根据实际业务引入 TTS；**本次不要求定义 TTS/ASR 二级分类，也不承诺语音接口可用**。
- 旧 `/api/harness/catalog` 的 `groups[]` 结构**不再作为 Harness 的模型列表对接依据**。
- B1/B3 账本、关联 ID 和 JWT 的详细规则请继续参考既有联调文档；本说明只替换模型列表部分。

**完整接口指南**：[`01-integration-guide.md`](./01-integration-guide.md)
