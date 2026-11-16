# 全模型验收报告与接入样例（2026-11-17）

**版本**：V0.12（2026-11-17）
**对应网关版本**：`harness-api-20261116-df87870`（revision `df8787085`）
**适用对象**：Harness 后端联调、供应商问题跟踪
**测试范围**：语言、图片、视频全部当前模型；视频参考图、参考视频、参考音频、首尾帧和视频编辑

本文是 2026-11-16 至 2026-11-17 的全模型实测结果。所有调用均使用真实上游密钥、真实生成和真实下载；没有使用 mock。测试 token 和上游密钥不写入本文。

---

## 1. 结论

| 类别 | 通过 | 未通过/阻塞 | 说明 |
| --- | --- | --- | --- |
| 语言 | 35/38 条调用记录 | 3 | 3 条是 `国模号池1` 上游 token 不服务的模型（预期 503），不是网关问题 |
| 图片 | 10/10 | 0 | GPT Image 4/4、Nano Banana 6/6 |
| 视频 | 55/61 个模型 | 6 | 全部为上游阻塞：MiniMax 4、Wan 1080P 按次 1、Seedance 720P 按秒 1 |
| 参考素材 | 通过 | 部分模型上游失败 | Seedance 参考图/音频、Kling 参考视频、Omni/MiniMax 首尾帧、HappyHorse 编辑均已按对应合同提交；失败集中在 MiniMax 上游和 `seedance2-720p-second-1` 上游生成 |

**网关侧结论**：`invalid api platform: 60` 已修复；所有当前模型已声明到 `supplyvideo` 插件并可通过对应渠道提交。剩余失败均为上游供应网关/供应商问题，不是网关未声明或未路由。

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

### 4.3 实测结果

| 分组 | 模型数 | 通过 | 未通过 | 说明 |
| --- | --- | --- | --- | --- |
| `pai-seedance-按秒` | 22 | 21 | 1 | `seedance2-720p-second-1` 上游生成失败；其余通过，含新增 `seedance2-480p-second-3`、`seedance2.5-1080p-second-1` |
| `pai-seedance-按次` | 7 | 7 | 0 | 含新增 `seedance2-fast-720p-once-3/4` |
| `pai-wan3.0-按秒` | 5 | 5 | 0 | `wan3-480p/720p/1080p-second` 全部通过 |
| `pai-wan3.0-按次` | 2 | 1 | 1 | `wan3-720p-once` 通过；`wan3-1080p-once` 上游任务停在 95% |
| `video-line-v1-second` | 5 | 1 | 4 | `muse-video-720p` 通过；MiniMax 4 个上游 401 |
| `video-line-v1-once` | 6 | 6 | 0 | Omni 6 个全部通过，含首尾帧 |
| `TA-其他视频模型分组` | 14 | 14 | 0 | Wan3.0-video 6、Kling 4、HappyHorse 3、Grok 1 全部通过 |
| **合计** | **61** | **55** | **6** | 6 个均为上游阻塞 |

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

---

## 5. 已知上游阻塞

| 模型 | 现象 | 证据 | 建议 |
| --- | --- | --- | --- |
| `MiniMax-H3-Max-480p` / `-first-last-frame` / `-768p` / `-768p-first-last-frame` | 提交返回 401 `fail_to_fetch_task`，内层 `unauthorized` | 直连供应网关 `POST /v1/videos` 同样复现；同一 `video-line-v1-second` 的 Muse 通过 | 供应网关修复 MiniMax 线路后再测 |
| `wan3-1080p-once` | 上游任务停在 95%，超过 40 分钟 | `wan3-720p-once` 同一渠道通过；重试仍停在 95% | 供应网关检查该规格 |
| `seedance2-720p-second-1` | 上游 `video_generation_failed`（“视频生成失败，请稍后重试”） | 同组其他 Seedance 模型通过；`seedance2-480p-second-1` 的参考图/音频通过 | 供应网关检查该规格；联调可先用 480P/其他 720P 规格 |
| `国模号池1` 的 `deepseek-flash` / `deepseek-v4-pro` / `kimi-k3.1-pro-highspeed` | 503 `model_not_found` | 上游 token 的 `/v1/models` 不含这三个模型 | 使用 `国模官方直连`，或后续配置日期别名映射 |

---

## 6. 测试方法与证据

- 每个模型至少一次真实提交；视频任务轮询到终态后下载完整内容，并检查 MP4 `ftyp`；图片解码后检查 PNG/JPEG 文件头。
- 测试 token 按分组创建，测试结束后删除；本文不包含任何 token 或上游密钥。
- 测试覆盖的网关版本：`harness-api-20261116-df87870`（revision `df8787085`）。
- 语言/图片/视频的通过数以实际 HTTP 200 + 有效内容为准；上游任务失败、上游超时和上游 401 单独列为“上游阻塞”，不混入网关通过数。
- 参考图/音频/视频使用公开可访问的 HTTPS 素材；Kling 参考视频必须使用 720P 级 MP4。

---

## 7. 联调建议

1. 先按本文第 2–4 节的接口和请求体接入；Nano Banana 不要再走 `/v1/videos`。
2. 图片按 B3 `settled` 作为完成和成本依据；B1 只作用量参考。
3. 视频提交后保存 `task_id`，轮询到 `completed` 再下载；失败任务先查上游原因，不要盲目重提。
4. MiniMax、`wan3-1080p-once`、`seedance2-720p-second-1` 作为已知上游阻塞，联调时先跳过或替换为同组可用模型。
5. 需要新增模型或调整价格时，以 A3 返回的模型 id 和实际 B3 金额为准。
