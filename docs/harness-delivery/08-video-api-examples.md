# 视频调用样例与模型差异

**版本**：V0.13（2026-11-17 修订）
**对应网关版本**：`harness-api` 分支，82 实例 `harness-api-20261117-b040241`
**日期**：2026-11-14（2026-11-15、2026-11-17 修订）
**适用对象**：Harness 后端与视频调用联调

本文只说明视频生成调用。所有视频模型都使用同一个 OpenAI Video 入口，差异在
请求字段约束、计费方式和任务完成后的用量字段；不需要为不同厂商切换路径。

---

## 1. 统一入口

| 操作 | 方法与路径 | 用途 |
| --- | --- | --- |
| 提交 | `POST /v1/videos` | 创建视频任务 |
| 查询 | `GET /v1/videos/{task_id}` | 轮询状态与最终用量 |
| 内容 | `GET /v1/videos/{task_id}/content` | 下载视频；支持 `Range` |
| 内容（可选） | `HEAD /v1/videos/{task_id}/content` | 探测大小/类型；部分 TA token 模型不支持 HEAD，见第七节 |

所有接口使用 Harness JWT，scope 为 `gateway.chat`。**提交接口**还必须带
`X-Harness-Request-Id` 和 `X-Harness-Run-Id`（均 ≤64 字符）；查询和内容接口
只需要 JWT。

当前模型池与模型 id 以 [A3 模型目录更新](./A3-model-catalog-update.md) 为准：

- `video` 池：按秒视频模型（PAI 按秒、TA 按秒、video-line 按秒线路）
- `plan` 池：按次视频模型（TA token、PAI 按次、Wan 按次、video-line 按次）
- `image` 池已配置 10 个图片模型（见 [09-image-api-examples.md](./09-image-api-examples.md)）；`audio` 仍为空

**池选择头**：

- `chat` 模型：不传 `X-Route-Group`（默认池）
- `video` 池模型：`X-Route-Group: video`
- `plan` 池模型：`X-Route-Group: plan`

不传时请求落到默认 `chat` 池；视频模型不在 `chat` 池，会因模型不可用失败。

---

## 2. 模型类型差异

| 类型 | 代表模型 | 计费 | 客户端 `seconds` | 媒体输入 | `resolution` / `size` | 完成后的 usage |
| --- | --- | --- | --- | --- | --- | --- |
| PAI 按次 | `seedance2-fast-720p-once-1` | 按次 | 整数，按模型 4–15 或 1–60/5–30 | 按模型声明；Seedance 2.5 Flash 按次支持参考素材（≤10 图/5 视频/5 音频） | 不允许发送 | 通常无 `usage`；B1 为 `usage_source=unavailable`、Token 字段为 `null` |
| PAI 按秒 | `seedance2-mini-480p-second` | 按秒 | 整数，按模型范围 | 已支持参考图/视频/音频（按模型声明） | 不允许发送 | 可能无 `usage`；B1 为 `usage_source=unavailable`、Token 字段为 `null`；有 `usage` 时按 `output_seconds` / `input_seconds` 结算，无 `usage` 时按请求生成秒数估算参考秒 |
| TA token | `seedance2-mini-480p-token-no-video` | 按 token | 整数，4–15；2.5 系列 4–30 | 支持图片/参考视频 | 可选，但必须与模型 id 一致 | `total_tokens` |
| TA 按秒 | `wan3.0-video-480p` | 按输出/输入秒 | 整数，按 family 范围；`wan` 支持 `-1` 自动 | 按 family 限制 | 由模型 id 锁定 | `output_seconds` / `input_seconds` |

补充：

- `duration` 是可选字段；如果发送，必须与 `seconds` 相等。PAI 上游需要
  `seconds` 字符串 + `duration` 数字，由网关自动转换，客户端无需关心。
- PAI 模型仍不接受 `resolution`、`size`，分辨率由模型 id 锁定；PAI 按秒已支持
  `image_with_roles` / `video_with_roles` / `audio_with_roles`，PAI 按次以模型声明
  为准（Seedance 2.5 Flash 按次声明支持参考素材，最多 10 图 / 5 视频 / 5 音频）。
- TA token 模型的分辨率也由模型 id 决定；发送冲突的 `resolution` / `size` 会被
  拒绝。`size` 只接受 `1280x720` / `720x1280`，且只对 720p 模型有效。
- `with-video` 模型必须提供至少一个 `reference_video`；`no-video` 模型拒绝参考视频。
- TA token 当前不支持 `audio_with_roles`；TA 按秒模型按 family 支持部分音频输入。
- PAI 按秒有 `usage` 时按 `output_seconds` / `input_seconds` 结算；Seedance 2.5
  Flash `second-1` 带参考时，上游当前完成响应不返回 `usage`，网关按请求
  `seconds` 估算参考秒并按两段式价格结算；上游后续返回实际 `input_seconds`
  时以实际值为准。

---

## 3. 样例 A：PAI 按次模型（plan 池）

以 `seedance2-fast-720p-once-1` 为例。该模型最少 4 秒、最多 15 秒；本例用 5 秒。

```bash
curl -sS -X POST "http://<gateway>/v1/videos" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_video_pai_001" \
  -H "X-Harness-Run-Id: run_video_pai_001" \
  -H "X-Route-Group: plan" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance2-fast-720p-once-1",
    "prompt": "A red bicycle moving through a sunlit street, cinematic",
    "seconds": 5
  }'
```

首次响应（HTTP 200）：

```json
{
  "id": "task_xxxxxxxx",
  "object": "video",
  "model": "seedance2-fast-720p-once-1",
  "status": "queued",
  "progress": 0,
  "created_at": 1794112000
}
```

轮询：

```bash
curl -sS "http://<gateway>/v1/videos/task_xxxxxxxx" \
  -H "Authorization: Bearer $HARNESS_JWT"
```

完成时：

```json
{
  "id": "task_xxxxxxxx",
  "object": "video",
  "model": "seedance2-fast-720p-once-1",
  "status": "completed",
  "progress": 100,
  "created_at": 1794112000
}
```

PAI 按次模型通常没有 `usage`；计费在任务成功时按次价格结算，不由返回的秒数推算。
这类任务的 B1 为 `usage_source=unavailable`，Token 字段为 `null`，
`split_known=false`，`total_tokens` 省略，不代表免费；费用以 B3 为准。下载：

```bash
curl -sS "http://<gateway>/v1/videos/task_xxxxxxxx/content" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -o output.mp4
```

---

## 4. 样例 B：TA token 文字生成（plan 池）

以 `seedance2-mini-480p-token-no-video` 为例。模型 id 已锁定 480p，不需要再传
`resolution`；`no-video` 模型不接受参考视频。

```bash
curl -sS -X POST "http://<gateway>/v1/videos" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_video_token_001" \
  -H "X-Harness-Run-Id: run_video_token_001" \
  -H "X-Route-Group: plan" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance2-mini-480p-token-no-video",
    "prompt": "A paper boat drifting down a quiet stream, soft morning light",
    "seconds": 4,
    "aspect_ratio": "16:9"
  }'
```

完成响应会带 token 用量，例如：

```json
{
  "id": "task_xxxxxxxx",
  "object": "video",
  "model": "seedance2-mini-480p-token-no-video",
  "status": "completed",
  "progress": 100,
  "created_at": 1794112000,
  "usage": {
    "total_tokens": 40594
  }
}
```

`aspect_ratio` 可选，默认 `16:9`；允许
`21:9` / `16:9` / `4:3` / `1:1` / `3:4` / `9:16` / `adaptive`。
`seedance2.5` 系列最长 30 秒，其他 token 模型最长 15 秒。

---

## 5. 样例 C：TA token 带参考视频（plan 池）

`with-video` 模型必须带至少一个 `reference_video`。素材 URL 必须是公网可访问的
HTTP/HTTPS 地址，不能带网关不认识的鉴权头。

```bash
curl -sS -X POST "http://<gateway>/v1/videos" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_video_video_001" \
  -H "X-Harness-Run-Id: run_video_video_001" \
  -H "X-Route-Group: plan" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance2-480p-token-with-video",
    "prompt": "Keep the subject and camera motion, replace the background with a snowy forest",
    "seconds": 4,
    "aspect_ratio": "16:9",
    "video_with_roles": [
      {
        "url": "https://cdn.example.com/reference.mp4",
        "role": "reference_video"
      }
    ]
  }'
```

也支持参考图 / 首尾帧：

```json
{
  "model": "seedance2-480p-token-no-video",
  "prompt": "Animate the still image with a slow zoom",
  "seconds": 4,
  "aspect_ratio": "adaptive",
  "image_with_roles": [
    { "url": "https://cdn.example.com/first.png", "role": "first_frame" },
    { "url": "https://cdn.example.com/last.png", "role": "last_frame" }
  ]
}
```

规则：

- `image_with_roles` 的 role 只能是 `first_frame` / `last_frame` / `reference_image`
- `last_frame` 必须和 `first_frame` 成对；帧模式不能和 `reference_image` 混用
- 帧模式不能和 `reference_video` 混用
- `seedance2.5` 使用 `last_frame` 时，`aspect_ratio` 必须是 `adaptive`

---

## 6. 其他已支持但当前未上池的模型

这些模型已在网关侧适配并有实测记录，但当前没有加入 `video` / `plan` 池；
加入池后按对应池 id 传 `X-Route-Group`。

### 6.1 TA 按秒

```json
{
  "model": "wan3.0-video-480p",
  "prompt": "A drone shot over a calm lake at sunrise",
  "seconds": 2,
  "aspect_ratio": "16:9"
}
```

```json
{
  "model": "happyhorse1.1-720p-second",
  "prompt": "A fashion model walking through a neon-lit alley",
  "seconds": 3,
  "aspect_ratio": "16:9"
}
```

```json
{
  "model": "grok-video1.5-480p-second",
  "prompt": "A cat leaping between two rooftops",
  "seconds": 1,
  "aspect_ratio": "16:9"
}
```

TA 按秒模型的 `seconds` 范围、aspect 和素材限制按 family 不同：

| family | 秒数 | aspect | 参考视频 | 参考图 | 音频 |
| --- | --- | --- | --- | --- | --- |
| happyhorse | 3–15 | 16:9 / 9:16 / 1:1 / 4:3 / 3:4 | 不支持 | 最多 9，first_frame / reference_image | 不支持 |
| grok | 1–15 | 1:1 / 16:9 / 9:16 / 3:2 / 2:3 | 不支持 | 最多 7，first_frame / reference_image | 不支持 |
| wan | 2–30，支持 `-1` 自动 | adaptive / 16:9 / 9:16 / 1:1 / 4:3 / 3:4 | 可选，最多 5 | 最多 10，first/last/reference | 最多 5，需要图片或视频素材 |
| kling | 3–30 | auto / 16:9 / 9:16 / 1:1 / 21:9 | 按模型：with-video 必须，no-video 禁止 | 最多 10，first/last/reference | 不支持 |
| minimax | 5–15 | 21:9 / 16:9 / 4:3 / 1:1 / 3:4 / 9:16 / adaptive | 可选，最多 3 | 最多 9，first/last/reference | 最多 3，需要图片或视频素材 |

### 6.2 PAI 按秒

```json
{
  "model": "seedance2-mini-480p-second",
  "prompt": "A slow pan across a desert canyon",
  "seconds": 4
}
```

PAI 按秒与 PAI 按次使用同一基础请求形状：`model` / `prompt` / `seconds` /
可选 `aspect_ratio`；按秒模型还接受 `image_with_roles` / `video_with_roles` /
`audio_with_roles`，按次模型以模型声明为准。两者都不接受 `resolution` / `size`，
分辨率由模型 id 锁定。

---

## 7. 已完成实测记录

以下为 2026-11-07 前后在联调环境实际跑通并结算的模型；金额是当时配置的采购价，
当前价格以 B3 / 账本为准。

| 模型 | 类型 | 请求参数 | 结果 | 金额（USD） |
| --- | --- | --- | --- | --- |
| `seedance2-mini-480p-token-no-video` | TA token | 480p, no-video | 40,594 tokens，SUCCESS，settled | $0.09296026 |
| `seedance2-480p-token-no-video` | TA token | 480p, no-video | 40,594 tokens，SUCCESS，settled | $0.3227223 |
| `wan3.0-video-480p` | TA 按秒 | 2 秒 | SUCCESS，settled | $0.08 |
| `happyhorse1.1-720p-second` | TA 按秒 | 3 秒 | SUCCESS，settled | $0.24 |
| `grok-video1.5-480p-second` | TA 按秒 | 1 秒 | SUCCESS，settled | $0.02 |
| `seedance2-mini-480p-second` | PAI 按秒 | 4 秒 | SUCCESS，settled | $0.16 |
| `seedance2-fast-720p-once-1` | PAI 按次 | 按次 | SUCCESS，settled | $0.52 |

这些记录的共同结论：

- 提交、轮询、内容下载、B1/B3 结算均跑通
- TA token 任务完成后返回 `usage.total_tokens`
- TA 按秒任务完成后返回 `usage.output_seconds` / `input_seconds`
- PAI 完成响应可能没有 `usage`；B1 为 `usage_source=unavailable`、Token 字段为
  `null`，不代表免费，费用以 B3 为准；按次模型按次结算，按秒模型目前按请求秒数结算
- PAI 内容可直接从结果 URL 代理下载；TA token 内容通过网关 `/content` 读取

---

## 8. 已知差异与坑

1. **PAI 的 `seconds` 在客户端仍是整数。** 网关会在提交时转成上游需要
   `seconds` 字符串 + `duration` 数字；不要自己传字符串 `seconds` 或额外构造
   `duration`（传了也必须与 `seconds` 相等）。
2. **PAI 不接受 `resolution` / `size`。** 分辨率由模型 id 决定；PAI 按秒已支持
   `image_with_roles` / `video_with_roles` / `audio_with_roles`，按次以模型声明为准。
3. **TA token 的 `resolution` / `size` 必须和模型 id 一致。** 例如
   `seedance2-mini-480p-token-no-video` 只能 480p；传 `720p` 会 400。最稳妥的做法
   是直接省略这两个字段。
4. **`with-video` / `no-video` 不能混用。** `with-video` 必须提供
   `reference_video`；`no-video` 传了参考视频会 400。
5. **TA token 当前不支持音频输入。** `audio_with_roles` 会 400；TA 按秒的部分
   family 才支持音频。
6. **TA token 的 `content` 建议用 GET。** 部分模型的 HEAD 会返回 502；GET/带
   `Range` 的 GET 正常（可返回 206）。PAI 的 HEAD 正常。
7. **PAI 完成响应可能没有 `usage`。** 此时 B1 为 `usage_source=unavailable`，
   Token 字段为 `null`，不要据此判断免费；按次/按秒的价格和结算在网关 B3 账本里。
   PAI 按秒有 `usage` 时按 `output_seconds` / `input_seconds` 结算；Flash `second-1`
   带参考时上游当前不返回 `usage`，网关按请求 `seconds` 估算参考秒并按两段式价格
   结算；上游后续返回实际 `input_seconds` 时以实际值为准。
8. **`capabilities` 是池级汇总。** 不能拿它判断池内每个模型都支持 `tools` 等能力；
   视频池当前 `capabilities` 为空。
9. **`models` 是字典序数组。** A3 返回的模型顺序不等于后台渠道配置顺序；不要依赖
   渠道顺序，按模型 id 选择即可。

---

## 9. 2026-11-17 新视频规格与编辑合同

本节补充 2026-11-17 全模型验收覆盖的新规格；完整结果和模型清单见
[10-full-model-acceptance-20261117.md](./10-full-model-acceptance-20261117.md)。

### 9.1 Wan 3.0 按秒 / 按次

`pai-wan3.0-按秒` 5 个模型（`wan3-480p-second-1/2`、`wan3-720p-second-1/2`、
`wan3-1080p-second`）：

```json
{"model":"wan3-480p-second-1","prompt":"...","seconds":5,"aspect_ratio":"16:9"}
```

`pai-wan3.0-按次` 2 个模型（`wan3-720p-once`、`wan3-1080p-once`）：

```json
{"model":"wan3-720p-once","prompt":"...","seconds":5}
```

### 9.2 MiniMax / Muse（video-line-v1-second）

```json
{"model":"MiniMax-H3-Max-480p","prompt":"...","seconds":"5","resolution":"480p","aspect_ratio":"16:9"}
```

首尾帧模型追加 `images`（两张不同 URL）：

```json
{"model":"MiniMax-H3-Max-480p-first-last-frame","prompt":"...","seconds":"5","resolution":"480p","aspect_ratio":"16:9","images":["https://.../first.png","https://.../last.png"]}
```

Muse：

```json
{"model":"muse-video-720p","prompt":"...","seconds":"10","aspect_ratio":"16:9"}
```

### 9.3 Omni（video-line-v1-once）

720P：

```json
{"model":"gemini-omni-1.1-flash-720p-10s","prompt":"...","size":"1280x720"}
```

1080P：

```json
{"model":"gemini-omni-1.1-flash-1080p-10s","prompt":"..."}
```

首尾帧模型追加 `images`（两张不同 URL）。按次价格不乘秒数。

### 9.4 Seedance 新增变体

`seedance2-480p-second-3`、`seedance2.5-1080p-second-1` 按秒；
`seedance2-fast-720p-once-3/4` 按次。请求体与同家族现有模型一致。

### 9.5 HappyHorse 视频编辑

`happyhorse1.1-720p-edit-second` 使用独立 `video-edit` 合同：

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

不要传 `seconds`、`duration`、`size`、`ratio`、`aspect_ratio`、`image_with_roles`、
`images` 或 `reference_images`；`url` 是参考视频。

### 9.6 参考素材

- Seedance 参考图/视频/音频：`image_with_roles` / `video_with_roles` /
  `audio_with_roles`；PAI 按秒模型已支持这些字段。
- Kling `with-video`：`video_with_roles` 的 `reference_video` 必须是可访问的
  720P 级 MP4；低分辨率/不可达 URL 会被上游拒绝。
- 首尾帧：`images` 必须两张不同 URL；同一 URL 会 400。

### 9.7 已知上游阻塞

- MiniMax 4 个 `video-line-v1-second` 模型：上游 401 `fail_to_fetch_task` /
  `unauthorized`；网关已声明，直连上游复现。
- `wan3-1080p-once`：上游任务停在 95%；`wan3-720p-once` 通过。
- `seedance2-720p-second-1`：上游生成失败；同组其他 Seedance 模型通过。

---

## 10. Seedance 2.5 Flash（2026-11-17 新增）

2026-11-16 供应侧新增 5 个已启用的 Seedance 2.5 Flash 规格，2026-11-17 同步到本文。
清晰度由模型 id 锁定，客户端不要再传 `resolution` / `size`。配置启用不等于生成、
健康或计费已全量验收；下面的价格与边界以当前生效配置为准。

### 10.1 模型与价格

| 模型 | 池 | 价格（USD，分组倍率 1 已含） | 参考视频 |
| --- | --- | --- | --- |
| `seedance2.5-flash-480p-second-1` | `video` | 无参考 0.05/生成秒；带参考 0.09×生成秒 + ceil(实际参考总秒)×0.05 | 最多 5 段（能力声明） |
| `seedance2.5-flash-720p-second-1` | `video` | 无参考 0.06/生成秒；带参考 0.13×生成秒 + ceil(实际参考总秒)×0.065 | 最多 5 段（能力声明） |
| `seedance2.5-flash-720p-second-2` | `video` | 0.06/生成秒 | 不支持 |
| `seedance2.5-flash-480p-once-1` | `plan` | 0.78/条 | 最多 5 段（能力声明） |
| `seedance2.5-flash-720p-once-1` | `plan` | 0.95/条 | 最多 5 段（能力声明） |

按秒模型的参考秒由服务端计量，不能用“总时长 × 单一单价”估算；按次模型每次生成
计 1 条，价格不乘秒数。上游当前对 Flash 按秒任务的完成响应不返回 `usage`，网关在
带参考请求中按请求 `seconds` 估算参考秒，先按两段式价格预扣/结算；上游后续返回
实际 `input_seconds` 时以实际值为准。按次模型的参考素材不额外收费为能力声明，未做
本模型账单实测。

### 10.2 请求规则

- 仍使用统一入口 `POST /v1/videos`；按秒模型带 `X-Route-Group: video`，按次模型
  带 `X-Route-Group: plan`。
- `seconds` 为整数，5–30。
- 图片最多 10 张（`image_with_roles`）、音频最多 5 段（`audio_with_roles`）；
  参考视频除 `720p-second-2` 外最多 5 段（`video_with_roles`）。以上均为能力声明，
  未做全组合实测。
- `seedance2.5-flash-720p-second-2` 不支持参考视频，传 `video_with_roles` 会被拒绝。
- 不接受 `resolution` / `size`；清晰度由模型 id 锁定。
- 按次价格不乘秒数；不要给按次模型套用按秒计费表达式。

### 10.3 示例

按次 720p（`plan` 池，0.95 USD/条，不乘秒）：

```json
{
  "model": "seedance2.5-flash-720p-once-1",
  "prompt": "A hummingbird hovering beside a red flower, macro shot",
  "seconds": 5
}
```

按秒 480p，无参考（`video` 池，0.05×5 = 0.25 USD）：

```json
{
  "model": "seedance2.5-flash-480p-second-1",
  "prompt": "A paper lantern drifting over a night river, cinematic",
  "seconds": 5
}
```

按秒 480p，带参考视频（`video` 池）：

```json
{
  "model": "seedance2.5-flash-480p-second-1",
  "prompt": "Keep the subject and camera motion, replace the scene with a neon city at night",
  "seconds": 5,
  "video_with_roles": [
    {
      "url": "https://cdn.example.com/reference.mp4",
      "role": "reference_video"
    }
  ]
}
```

供应网关的参考视频历史样例使用 `extra_videos`（1 段视频 URL，prompt 中用
`@Video1` 指代）；下游合同统一走 `video_with_roles` + `role=reference_video`，
不要传 `extra_videos`。

计费示例：生成 5 秒 + 单段 5.055 秒参考，`ceil(5.055)=6`；480P 为
5×0.09 + 6×0.05 = 0.75 USD，720P 为 5×0.13 + 6×0.065 = 1.04 USD。上游当前不返回
Flash 的参考秒，网关按请求 `seconds` 估算：5 秒生成 + 5 秒参考估算，480P 为
5×0.09 + 5×0.05 = 0.70 USD；上游补 `input_seconds` 后按实际值校准。该示例是公式
计算，不是新增付费扣账实测；实际金额以 B3 为准。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| V0.13 | 2026-11-17 | 新增第 10 节 Seedance 2.5 Flash（3 个按秒 / 2 个按次）的池、价格、请求规则与 3 个 JSON 样例；修正第 2 节 PAI 参考素材输入与按秒结算描述。 |
