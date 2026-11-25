# 全模型验收报告与接入样例（2026-11-17）

**版本**：V0.13（2026-11-17；2026-11-23 增补）
**对应网关版本**：`harness-api` 分支，82 实例 `harness-api-20261117-b040241`（revision `b04024190`）
**适用对象**：Harness 后端联调、供应商问题跟踪
**测试范围**：语言、图片、视频全部当前模型；视频参考图、参考视频、参考音频、首尾帧和视频编辑

本文是 2026-11-16 至 2026-11-17 的全模型实测结果。所有调用均使用真实上游密钥、真实生成和真实下载；没有使用 mock。测试 token 和上游密钥不写入本文。第 8 节为 2026-11-23 后续增补，不改变本文作为 2026-11-17 历史验收报告的性质；第 1、4.3、5 节的统计和阻塞结论均只代表当时快照。

---

## 1. 结论（2026-11-17 历史快照）

> 历史快照：下表通过数、阻塞数和模型状态均以 2026-11-17 测试窗口为准，不代表 2026-11-23 的当前目录或可调状态。当前口径见第 8 节及 [`12-retest-model-list-20261123.md`](./12-retest-model-list-20261123.md)。

| 类别 | 通过 | 未通过/阻塞 | 说明 |
| --- | --- | --- | --- |
| 语言 | 35/38 条调用记录 | 3 | 3 条是 `国模号池1` 上游 token 不服务的模型（预期 503），不是网关问题 |
| 图片 | 10/10 | 0 | GPT Image 4/4、Nano Banana 6/6 |
| 视频 | 57/66 个模型 | 6 阻塞 + 3 未测 | 6 个上游阻塞：MiniMax 4、Wan 1080P 按次 1、Seedance 720P 按秒 1；新增 Flash 5 个中已测 2 个，另 3 个未测（见 4.5） |
| 参考素材 | 通过 | 部分模型上游失败 | Seedance 参考图/音频、Kling 参考视频、Omni/MiniMax 首尾帧、HappyHorse 编辑均已按对应合同提交；Flash 参考视频生成通过，但上游不返回参考秒（见 4.5）；失败集中在 MiniMax 上游和 `seedance2-720p-second-1` 上游生成 |

**网关侧结论（2026-11-17 快照）**：`invalid api platform: 60` 已修复；当时声明的模型已进入 `supplyvideo` 插件并可通过对应渠道提交。当时观察到的剩余失败按上游供应网关/供应商问题记录；该结论不代表这些模型在 2026-11-23 仍可调。

---

## 2. 语言模型

### 2.1 接口

| 模型家族 | 接口 | 认证 |
| --- | --- | --- |
| GPT | `POST /v1/responses`（推荐）；`POST /v1/chat/completions`（兼容） | `Authorization: Bearer <gateway-token>` |
| Claude | `POST /v1/messages` | `x-api-key: <gateway-token>` + `anthropic-version: 2023-06-01` |
| Gemini | `POST /v1beta/models/{model}:generateContent` | `x-goog-api-key: <gateway-token>` |
| Grok / Kimi / GLM / DeepSeek | `POST /v1/chat/completions` | `Authorization: Bearer <gateway-token>` |

### 2.2 结果

- GPT：`gpt-5.6-terra`、`gpt-6-astra`、`gpt-6-sol`、`gpt-6.1-sol`、`gpt-6.1-terra`，Responses 和 Chat 均通过。
- Claude：`claude-fable-5`、`claude-fable-5-1`、`claude-fable-5-2`、`claude-haiku-5-5`、`claude-opus-4-8`、`claude-opus-5`、`claude-opus-5-5`、`claude-sonnet-5`、`claude-sonnet-5-5` 全部通过。
- Gemini：`gemini-3.8-flash`、`gemini-4-argon` 原生 `generateContent` 通过。
- Grok：`grok-4.5`、`grok-4.6`、`grok-4.7` 通过。
- 国模：`kimi-k3`、`kimi-k3.1-pro`、`kimi-k3.1-pro-highspeed`、`glm-5.3`、`glm-5.3-flash`、`deepseek-flash`、`deepseek-v4-pro` 在 `国模官方直连` 全部通过；`国模号池1` 仅 `kimi-k3`、`kimi-k3.1-pro`、`glm-5.3`、`glm-5.3-flash` 可用。

> 注意：`国模号池1` 的 `deepseek-flash`、`deepseek-v4-pro`、`kimi-k3.1-pro-highspeed` 上游 token 不服务，返回 503 `model_not_found`。需要这三个模型时请使用 `国模官方直连`，或后续给 `国模号池1` 配置日期别名映射。

### 2.3 请求示例

```bash
curl -sS -X POST "http://<gateway>/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "Content-Type: application/json" \
  -d '{"model":"glm-5.3-flash","messages":[{"role":"user","content":"Say OK"}],"max_tokens":64}'
```

```bash
curl -sS -X POST "http://<gateway>/v1/messages" \
  -H "x-api-key: $HARNESS_JWT" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-opus-5","max_tokens":64,"messages":[{"role":"user","content":"Say OK"}]}'
```

```bash
curl -sS -X POST "http://<gateway>/v1beta/models/gemini-3.8-flash:generateContent" \
  -H "x-goog-api-key: $HARNESS_JWT" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"role":"user","parts":[{"text":"Say OK"}]}],"generationConfig":{"maxOutputTokens":64}}'
```

---

## 3. 图片模型

### 3.1 模型与接口

| 模型 | 接口 | 价格（USD/张） |
| --- | --- | --- |
| `gpt-image-2.5-flare` / `gpt-image-2.5-sunburst` / `gpt-image-2.6-flare` / `gpt-image-2.6-sunburst` | `POST /v1/images/generations` | 0.018 |
| `Nano Banana 2.1 1K` / `2K` | `POST /v1beta/models/{URL编码模型}:generateContent` | 0.009 |
| `Nano Banana 2.1 4K` | 同上 | 0.011 |
| `Nano Banana Pro 1K` / `2K` / `4K` | 同上 | 0.016 |

Nano Banana 已改为 **Gemini 同步接口**；旧的异步 `/v1/videos` 合同已停用，不要再使用任务轮询和 `/v1/tasks/{id}/artifacts/image/content`。

### 3.2 Nano Banana 请求

模型名含空格，路径里必须 URL 编码，例如 `Nano%20Banana%20Pro%201K`。

```bash
curl -sS -X POST "http://<gateway>/v1beta/models/Nano%20Banana%20Pro%201K:generateContent" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Route-Group: image" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role":"user","parts":[{"text":"A red apple on a white table"}]}],
    "generationConfig": {
      "candidateCount": 1,
      "responseModalities": ["IMAGE"],
      "imageConfig": {"aspectRatio":"1:1","imageSize":"1K"}
    }
  }'
```

成功时 HTTP 200，图片在：

```
candidates[].content.parts[].inlineData.data   # Base64
```

- 同步返回，不需要轮询。
- 实际字节可能是 JPEG 或 PNG；`inlineData.mimeType` 可能误标 `image/png`，必须按文件头识别格式。
- 1K/2K/4K 的 `imageSize` 必须按模型显式填写；当前只验证 `aspectRatio: "1:1"`、`candidateCount: 1`。
- 4K 响应体可能很大（实测约 8.7 MB Base64），客户端要允许大响应和较长超时；上游偶发 524 超时，重试前先确认是否已生成。

### 3.3 GPT Image 请求

```bash
curl -sS -X POST "http://<gateway>/v1/images/generations" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Route-Group: image" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-image-2.6-flare","prompt":"A red paper lantern","n":1,"size":"1024x1024"}'
```

图片在 `data[0].b64_json`；4 个模型均为 0.018 USD/张。

### 3.4 图片 B1/B3

| 流 | GPT Image 同步 | Nano Banana 同步 |
| --- | --- | --- |
| B1 用量 | `usage_source=upstream`，带上游实际 input/output Token | `usage_source=estimated`；Gemini 路径按请求估算 prompt Token（实测 `total_tokens=10`、`completion_tokens=0`） |
| B3 成本 | 固定 0.018/张，`status_change` → `settled` | 固定 0.009/0.011/0.016/张，`status_change` → `settled` |
| 完成依据 | B3 最新 `record_version` / `settled` | B3 最新 `record_version` / `settled` |

B1 的 Token 数不代表图片费用；图片按张计费，费用只看 B3。`image_count` 当前只验证 `n=1`，不在 B1 中暴露。

---

## 4. 视频模型

### 4.1 通用接口

| 操作 | 接口 |
| --- | --- |
| 提交 | `POST /v1/videos` |
| 查询 | `GET /v1/videos/{task_id}` |
| 内容 | `GET /v1/videos/{task_id}/content`（支持 `Range`/`HEAD`，按模型能力） |

所有提交带 `Authorization: Bearer <gateway-token>`；任务提交还需 `X-Harness-Request-Id` / `X-Harness-Run-Id`（≤64 字符）。轮询和下载只需要 token。

### 4.2 各家族请求体

**Seedance / Wan / Kling / HappyHorse / Grok 按秒（通用）**

```json
{"model":"<model>","prompt":"...","seconds":5,"aspect_ratio":"16:9"}
```

`seconds` 是 JSON 数字（整数）；模型 ID 锁定分辨率和是否支持参考视频。

**Seedance 按次 / Wan 按次**

```json
{"model":"<model>","prompt":"...","seconds":5}
```

**MiniMax / Muse（video-line-v1-second）**

```json
{"model":"MiniMax-H3-Max-480p","prompt":"...","seconds":"5","resolution":"480p","aspect_ratio":"16:9"}
```

首尾帧模型追加：

```json
{"model":"MiniMax-H3-Max-480p-first-last-frame","prompt":"...","seconds":"5","resolution":"480p","aspect_ratio":"16:9","images":["https://.../first.png","https://.../last.png"]}
```

Muse：

```json
{"model":"muse-video-720p","prompt":"...","seconds":"10","aspect_ratio":"16:9"}
```

**Omni（video-line-v1-once，按次）**

720P：

```json
{"model":"gemini-omni-1.1-flash-720p-10s","prompt":"...","size":"1280x720"}
```

1080P：

```json
{"model":"gemini-omni-1.1-flash-1080p-10s","prompt":"..."}
```

首尾帧追加 `"images":["https://.../first.png","https://.../last.png"]`；两张图必须不同 URL。

**Kling 参考视频**

```json
{"model":"kling4-720p-second-with-video","prompt":"...","seconds":5,"video_with_roles":[{"role":"reference_video","url":"https://.../reference-720p.mp4"}]}
```

参考视频必须是可访问的 720P 级 MP4；低分辨率/不可达 URL 会被上游拒绝。

**Seedance 参考图/视频/音频**

```json
{"model":"seedance2-480p-second-1","prompt":"...","seconds":5,"aspect_ratio":"16:9","image_with_roles":[{"role":"reference_image","url":"https://.../ref.png"}],"audio_with_roles":[{"role":"reference_audio","url":"https://.../ref.mp3"}]}
```

参考视频用 `video_with_roles`（`role: "reference_video"`）；参考音频用 `audio_with_roles`（`role: "reference_audio"`）。

**HappyHorse 视频编辑（独立合同）**

```json
{
  "model": "happyhorse1.1-720p-edit-second",
  "action": "video-edit",
  "url": "https://.../source-720p.mp4",
  "prompt": "将画面光线调整为暖色黄昏",
  "resolution": "720P",
  "audio_setting": "origin"
}
```

编辑合同不要传 `seconds`、`duration`、`size`、`ratio`、`aspect_ratio`、`image_with_roles`、`images`、`reference_images`；`url` 是参考视频，不是图片。

### 4.3 实测结果（2026-11-17 历史快照）

| 分组 | 模型数 | 通过 | 未通过 | 说明 |
| --- | --- | --- | --- | --- |
| `pai-seedance-按秒` | 25 | 22 | 1 | `seedance2-720p-second-1` 上游生成失败；其余通过，含新增 `seedance2-480p-second-3`、`seedance2.5-1080p-second-1`；Flash `720p-second-1/2` 未测 |
| `pai-seedance-按次` | 9 | 8 | 0 | 含新增 `seedance2-fast-720p-once-3/4`、Flash `720p-once-1`；Flash `480p-once-1` 未测 |
| `pai-wan3.0-按秒` | 5 | 5 | 0 | `wan3-480p/720p/1080p-second` 全部通过 |
| `pai-wan3.0-按次` | 2 | 1 | 1 | `wan3-720p-once` 通过；`wan3-1080p-once` 上游任务停在 95% |
| `video-line-v1-second` | 5 | 1 | 4 | `muse-video-720p` 通过；MiniMax 4 个上游 401 |
| `video-line-v1-once` | 6 | 6 | 0 | Omni 6 个全部通过，含首尾帧 |
| `TA-其他视频模型分组` | 14 | 14 | 0 | Wan3.0-video 6、Kling 4、HappyHorse 3、Grok 1 全部通过 |
| **合计** | **66** | **57** | **6** | 6 个上游阻塞；另 3 个 Flash 规格未测 |

### 4.4 参考素材结果

| 测试 | 模型 | 结果 |
| --- | --- | --- |
| 参考图 | `seedance2-480p-second-1` | 通过（1.10 MB MP4） |
| 参考音频 + 参考图 | `seedance2-480p-second-1` | 通过（1.17 MB MP4） |
| 参考图/音频 | `seedance2-720p-second-1` | 网关接受请求；上游生成失败（模型问题） |
| 参考视频 | `kling4-720p-second-with-video` | 通过（10.76 MB MP4） |
| 参考视频 | `kling4-1080p-second-with-video` | 通过（19.49 MB MP4） |
| 首尾帧 | `gemini-omni-1.1-flash-720p-10s-first-last-frame` | 通过（5.31 MB MP4） |
| 首尾帧 | `gemini-omni-1.1-flash-1080p-10s-first-last-frame` | 通过（6.47 MB MP4） |
| 首尾帧 | `MiniMax-H3-Max-480p-first-last-frame` | 上游 401，未通过 |
| 视频编辑 | `happyhorse1.1-720p-edit-second` | 通过（7.51 MB MP4） |
| 参考视频 | `seedance2.5-flash-480p-second-1` | 生成通过（4.04 MB MP4）；上游完成响应不返回 `usage`，B3 按请求秒数估算参考秒，结算 0.70 USD（见 4.5） |

### 4.5 Seedance 2.5 Flash 实测（2026-11-17 新增）

2026-11-17 部署 `harness-api-20261117-b040241`（revision `b04024190`）后实测 3 笔：

| 用例 | 模型 | 结果 | 下载 | B3 |
| --- | --- | --- | --- | --- |
| 按次 720P | `seedance2.5-flash-720p-once-1` | completed | 6,139,463 bytes MP4 | 0.95 USD |
| 按秒 480P 无参考 | `seedance2.5-flash-480p-second-1` | completed | 4,529,223 bytes MP4 | 0.25 USD（5×0.05） |
| 按秒 480P 带参考 | `seedance2.5-flash-480p-second-1` | completed | 4,037,357 bytes MP4 | 0.70 USD（5×0.09 + 5×0.05） |

- 3 笔 B1 均为 `usage_source=unavailable`、Token 字段 `null`、`split_known=false`；费用以 B3 为准。
- **上游用量缺口**：Flash 按秒任务完成后，供应网关的 `/v1/videos/{id}` 和 `/v1/tasks/{id}` 都不返回 `usage`；直连上游用 `extra_videos` 提交同样不返回。因此参考秒无法按上游实测值结算。
- 网关当前在带参考请求中按请求 `seconds` 估算参考秒并预扣/结算；上游后续返回 `input_seconds` 时以实际值为准。上例供应商公式按 5.055 秒参考、`ceil(5.055)=6` 计算应为 0.75 USD，当前 0.70 USD 少 0.05 USD。
- `seedance2.5-flash-480p-once-1`、`seedance2.5-flash-720p-second-1`、`seedance2.5-flash-720p-second-2` 本轮未测；不要写成已验收。

---

## 5. 已知上游阻塞（2026-11-17 历史快照）

> 以下现象与建议保留 2026-11-17 的现场记录；部分线路和模型状态已在后续复测中变化。不要将本表直接作为 2026-11-23 的当前阻塞清单，当前可调模型以第 12 号复测清单为准。

| 模型 | 现象 | 证据 | 建议 |
| --- | --- | --- | --- |
| `MiniMax-H3-Max-480p` / `-first-last-frame` / `-768p` / `-768p-first-last-frame` | 提交返回 401 `fail_to_fetch_task`，内层 `unauthorized` | 直连供应网关 `POST /v1/videos` 同样复现；同一 `video-line-v1-second` 的 Muse 通过 | 供应网关修复 MiniMax 线路后再测 |
| `wan3-1080p-once` | 上游任务停在 95%，超过 40 分钟 | `wan3-720p-once` 同一渠道通过；重试仍停在 95% | 供应网关检查该规格 |
| `seedance2-720p-second-1` | 上游 `video_generation_failed`（“视频生成失败，请稍后重试”） | 同组其他 Seedance 模型通过；`seedance2-480p-second-1` 的参考图/音频通过 | 供应网关检查该规格；联调可先用 480P/其他 720P 规格 |
| `seedance2.5-flash-*-second-*` | 完成响应不返回 `usage`，参考秒无法按实际值结算 | `/v1/videos/{id}` 与 `/v1/tasks/{id}` 均无 usage；直连上游改用 `extra_videos` 同样无 | 供应网关补 `usage.input_seconds` / `reference_seconds`；当前网关按请求 `seconds` 估算参考秒并结算 |
| `国模号池1` 的 `deepseek-flash` / `deepseek-v4-pro` / `kimi-k3.1-pro-highspeed` | 503 `model_not_found` | 上游 token 的 `/v1/models` 不含这三个模型 | 使用 `国模官方直连`，或后续配置日期别名映射 |

---

## 6. 测试方法与证据

- 每个模型至少一次真实提交；视频任务轮询到终态后下载完整内容，并检查 MP4 `ftyp`；图片解码后检查 PNG/JPEG 文件头。
- 测试 token 按分组创建，测试结束后删除；本文不包含任何 token 或上游密钥。
- 测试覆盖的网关版本：`harness-api-20261116-df87870`（revision `df8787085`，主体验收）与 `harness-api-20261117-b040241`（revision `b04024190`，Flash 复测）。
- 语言/图片/视频的通过数以实际 HTTP 200 + 有效内容为准；上游任务失败、上游超时和上游 401 单独列为“上游阻塞”，不混入网关通过数。
- 参考图/音频/视频使用公开可访问的 HTTPS 素材；Kling 参考视频必须使用 720P 级 MP4。

---

## 7. 联调建议（2026-11-17 历史口径）

> 以下保留原报告建议；涉及 Base URL、请求头、目录可调状态和下架模型的当前要求，以第 8 节增补及第 12 号复测清单为准。

1. 先按本文第 2–4 节的接口和请求体接入；Nano Banana 不要再走 `/v1/videos`。
2. 图片按 B3 `settled` 作为完成和成本依据；B1 只作用量参考。
3. 视频提交后保存 `task_id`，轮询到 `completed` 再下载；失败任务先查上游原因，不要盲目重提。
4. MiniMax、`wan3-1080p-once`、`seedance2-720p-second-1` 作为已知上游阻塞，联调时先跳过或替换为同组可用模型。
5. Seedance 2.5 Flash 带参考任务当前上游不返回参考秒，网关按请求 `seconds` 估算并结算；联调对账时不要把 B3 金额与上游实际参考秒公式逐分对齐，待上游补 `usage` 后再校准。
6. 需要新增模型或调整价格时，以 A3 返回的模型 id 和实际 B3 金额为准。

---

## 8. 2026-11-23 增补（当前联调口径）

本节用于校准历史报告的当前使用方式，不回写或改变 2026-11-17 的实测统计。反馈处理依据见 [`11-feedback-resolution-20261123.md`](./11-feedback-resolution-20261123.md)。

### 8.1 测试环境与请求头

| 项 | 当前值 |
| --- | --- |
| HTTPS Base URL | `https://testapi.soloai.chat` |
| A3 | `GET /v1/models`，`catalog_version=10` |
| `Authorization` | 按接口使用 `Bearer $HARNESS_JWT` 或对应认证头；不在文档中写入真实 JWT/Key |
| `X-Harness-Request-Id` | 必填，1–64 字符 |
| `X-Harness-Run-Id` | 必填，1–64 字符 |
| `X-Route-Group` | `chat` 为默认池；`image`、`video`、`plan` 调用必须显式传对应池 |

两个 Harness ID 的合法字符均为 `A-Z`、`a-z`、`0-9`、`-`、`_`，不强制 `hrq_` / `run_` 前缀，也不校验 ULID。缺失、空值、非法字符或超过 64 字符会在转发上游前返回 `400 REQUEST_INVALID`，不产生费用和 B1/B3 事件。

### 8.2 A3 目录候选与初步可调（2026-11-23 历史快照）

2026-11-23 时 A3 v10 的 `video` 返回 83 条、`plan` 返回 80 条，这是历史目录候选数，不是当前可调承诺。2026-11-25 渠道声明已与上游对齐，A3 直接返回 `video` 49 / `plan` 17；本节保留历史快照。2026-11-23 初步核实结果为：

| 池 | A3 当前目录候选 | 初步核实可调 |
| --- | ---: | ---: |
| `video` | 83 | **49** |
| `plan` | 80 | **17** |

其余 `video` 34 条、`plan` 63 条在 2026-11-23 时不可调或已下架。2026-11-25 渠道 `models` 已与上游对齐，A3 直接返回 video 49 / plan 17；本节保留历史快照。复测仍使用 [`12-retest-model-list-20261123.md`](./12-retest-model-list-20261123.md)。

同一模型可以合法出现在多个池，跨池重复本身不是错误，不应要求跨池交集为 0。A3 的 `capabilities` 只是池级端点类型汇总，**不能据此判断图片或视频的真实媒体能力**。

### 8.3 已下架/停用模型

- TA Seedance Token 22 档在上游当前目录中已不可用；历史文档和历史 A3 JSON 中的这些 `*-token-*` ID 均已下架，禁止作为当前调用清单。
- 另有 7 个视频规格已下架/停用：`grok-video1.5-480p-second`、3 个旧小写 MiniMax second、`seedance2-480p-once`、`seedance2-1080p-once`、`seedance2-fast-720p-once-2`。
- 这些模型继续调用出现 500/503/400 属于预期拒绝；完整下架名单和当前 `video=49`、`plan=17` 复测清单见第 12 号文档。

### 8.4 异步视频 B1/B3 口径

| 项目 | 当前口径 |
| --- | --- |
| 异步视频 B1 | `usage_source=unavailable`；token 字段为 `null`，`split_known=false`，`total_tokens` 省略。这是用量不可得，不是“实测为 0” |
| 历史 TA Token 用量 | 不得直接回补到当前 B1；模型已下架时尤其不能作为当前用量证据 |
| B3 费用依据 | 以最新版本的 `status=settled` 和 `total_amount` 为准；B1 缺失 usage 不影响 B3 结算 |
| B3 `billing_basis` | 当前固定为 `token`，属于已知字段限制，**不表示**图片或视频按 token 计费；图片仍按张、视频按秒/按次执行模型价格表达式 |

本节不包含真实上游 Key、Harness JWT、证书私钥或内部转接组件凭据。
