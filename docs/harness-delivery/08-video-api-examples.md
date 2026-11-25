# 视频调用样例与模型差异

**版本**：V0.14（2026-11-23 修订）
**对应网关版本**：`harness-api` 分支，82 实例 `harness-api-20261117-b040241`
**日期**：2026-11-14（2026-11-15、2026-11-17、2026-11-23 修订）
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
| 内容（可选） | `HEAD /v1/videos/{task_id}/content` | 探测大小/类型；历史 TA token 模型曾出现 HEAD 502，相关模型已下架，见第 4、7 节 |

所有接口使用 Harness JWT，scope 为 `gateway.chat`。**提交接口**还必须带
`X-Harness-Request-Id` 和 `X-Harness-Run-Id`（均 ≤64 字符）；查询和内容接口
只需要 JWT。

当前模型池与模型 id 以 A3 `GET /v1/models` 和
[12-retest-model-list-20261123.md](./12-retest-model-list-20261123.md) 为准。
2026-11-25 渠道 `models` 已与上游 `/v1/models` 对齐，A3 直接返回可调集合：

| 池 | 2026-11-25 A3 当前返回 | 当前用法 |
| --- | ---: | --- |
| `video` | **49** | 按秒视频；提交时显式传 `X-Route-Group: video` |
| `plan` | **17** | 按次视频；提交时显式传 `X-Route-Group: plan` |
| `image` | 10 | 图片模型，见 [09-image-api-examples.md](./09-image-api-examples.md) |
| `chat` | 26 | 默认池；11-23 有 3 条上游失败记录，详见第 12 号文档 |
| `audio` | 0 | 预留池，本期不可调用 |

A3 已与可调清单一致，但仍不代表 49/17 个模型已在同一窗口全部完成生成验收；
2026-11-25 的 7 个上游分组代表模型实测均最终 SUCCESS，详见第 14 号文档。精确模型
ID 与复测备注仍以第 12 号文档为准。

**跨池重复本身合法。** 同一供应商或同一模型可以出现在多个池，不应以“交集必须为 0”
作为验收标准。当前需要区分的是目录声明、池内启用渠道和真实供货，而不是删除合法重复。
A3 的 `capabilities` 是池级端点类型汇总，也不能据此判断某个图片/视频模型的真实能力。

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
| TA 按秒 | `wan3.0-video-480p` | 按输出/输入秒 | 整数，按 family 范围；`wan` 支持 `-1` 自动 | 按 family 限制 | 由模型 id 锁定 | `output_seconds` / `input_seconds` |
| TA token（历史，已下架） | 见第 11 节 | 旧记录按 token | 旧合同 4–15 秒；2.5 系列 4–30 秒 | 旧合同支持图片/参考视频 | 旧合同由模型 id 锁定 | **不得作为当前样例**；历史 token 数不能回补当前 B1 |

补充：

- `duration` 是可选字段；如果发送，必须与 `seconds` 相等。PAI 上游需要
  `seconds` 字符串 + `duration` 数字，由网关自动转换，客户端无需关心。
- PAI 模型仍不接受 `resolution`、`size`，分辨率由模型 id 锁定；PAI 按秒已支持
  `image_with_roles` / `video_with_roles` / `audio_with_roles`，PAI 按次以模型声明
  为准（Seedance 2.5 Flash 按次声明支持参考素材，最多 10 图 / 5 视频 / 5 音频）。
- TA token 是历史合同，当前规格已下架，不能再作为当前调用样例；旧字段约束仅用于
  识别历史请求和任务，完整禁用清单见第 11 节。
- `with-video` 模型必须提供至少一个 `reference_video`；`no-video` 模型拒绝参考视频。
  该规则只对声明对应能力的当前模型适用，不代表所有按秒/按次模型都支持视频参考。
- TA 按秒模型按 family 支持部分音频输入；是否可用仍以第 12 号文档的当前模型清单为准。
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

## 4. 历史 TA token 样例（已下架，禁止当前调用）

原 `seedance2-mini-480p-token-no-video`、`seedance2-480p-token-with-video` 等
TA token 样例已从当前调用示例移除。2026-11-23 核实，TA Seedance Token 22 档在
上游当前目录中已不可用；旧渠道声明或旧文档仍可能展示这些 id，但提交出现
400/500/503 属于预期拒绝，不应作为当前可上线模型。

历史完成响应中的 `usage.total_tokens` 只对当时那笔任务有效，不能直接回补到当前
B1，尤其不能用于推断已下架模型当前仍有 token 用量。当前异步视频在上游不返回
usage 时，B1 使用 `usage_source=unavailable`，Token 字段为 `null`，
`split_known=false`，`total_tokens` 省略；这不是“实测为 0”，也不代表免费。
费用以 B3 最新 `status=settled` 记录的 `total_amount` 和提交时价格版本为准。

旧 token 请求中的 `aspect_ratio`、首尾帧、参考图和参考视频约束仅保留为历史排障
背景，不再提供可复制的当前调用样例。已下架/停用模型完整清单见第 11 节。

---

## 5. 其他按秒模型样例（video 池）

以下仅展示请求形状，是否能调以第 12 号文档的 49 个 `video` 模型清单为准。
所有提交都必须显式传 `X-Route-Group: video`。

### 5.1 TA 按秒

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
  "model": "grok-video1.5-720p-second",
  "prompt": "A cat leaping between two rooftops",
  "seconds": 1,
  "aspect_ratio": "16:9"
}
```

`grok-video1.5-480p-second` 已下架，不再作为当前样例；不要与仍可复测的
`grok-video1.5-720p-second` 混淆。

TA 按秒模型的 `seconds` 范围、aspect 和素材限制按 family 不同：

| family | 秒数 | aspect | 参考视频 | 参考图 | 音频 |
| --- | --- | --- | --- | --- | --- |
| happyhorse | 3–15 | 16:9 / 9:16 / 1:1 / 4:3 / 3:4 | 不支持 | 最多 9，first_frame / reference_image | 不支持 |
| grok | 1–15 | 1:1 / 16:9 / 9:16 / 3:2 / 2:3 | 不支持 | 最多 7，first_frame / reference_image | 不支持 |
| wan | 2–30，支持 `-1` 自动 | adaptive / 16:9 / 9:16 / 1:1 / 4:3 / 3:4 | 可选，最多 5 | 最多 10，first/last/reference | 最多 5，需要图片或视频素材 |
| kling | 3–30 | auto / 16:9 / 9:16 / 1:1 / 21:9 | 按模型：with-video 必须，no-video 禁止 | 最多 10，first/last/reference | 不支持 |
| minimax | 5–15 | 21:9 / 16:9 / 4:3 / 1:1 / 3:4 / 9:16 / adaptive | 可选，最多 3 | 最多 9，first/last/reference | 最多 3，需要图片或视频素材 |

### 5.2 PAI 按秒

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

## 6. 已完成实测记录（当前模型）

以下为 2026-11-07 前后在联调环境实际跑通并结算、且仍列入 2026-11-23 可复测清单的
模型；金额是当时配置的采购价，只用于历史核对，当前费用以 B3 为准。

| 模型 | 类型 | 请求参数 | 结果 | 历史金额（USD） |
| --- | --- | --- | --- | --- |
| `wan3.0-video-480p` | TA 按秒 | 2 秒 | SUCCESS，settled | $0.08 |
| `happyhorse1.1-720p-second` | TA 按秒 | 3 秒 | SUCCESS，settled | $0.24 |
| `seedance2-mini-480p-second` | PAI 按秒 | 4 秒 | SUCCESS，settled | $0.16 |
| `seedance2-fast-720p-once-1` | PAI 按次 | 按次 | SUCCESS，settled | $0.52 |

TA token 与 `grok-video1.5-480p-second` 的历史成功记录已从当前样例移除：这些
规格当前已下架，历史成功不能证明现在可调，历史 token 数也不能用于当前 B1。
这些记录的共同结论：

- 提交、轮询、内容下载、B1/B3 结算均跑通
- TA 按秒任务完成后返回 `usage.output_seconds` / `input_seconds`
- PAI 完成响应可能没有 `usage`；B1 为 `usage_source=unavailable`、Token 字段为
  `null`，不代表免费；按次模型按次结算，按秒模型目前按请求秒数结算
- 费用以 B3 最新 `status=settled` 记录的 `total_amount` 和提交时价格版本为准，
  不以 B1 的 token 字段或历史样例金额替代

---

## 7. 已知差异与坑

1. **先选当前模型，再构造请求。** 2026-11-25 对齐后 A3 直接返回 `video=49`、
   `plan=17`；精确清单见第 12 号文档。跨池重复合法，但以该模型在指定池有启用
   渠道为准。
2. **PAI 的 `seconds` 在客户端仍是整数。** 网关会在提交时转成上游需要
   `seconds` 字符串 + `duration` 数字；不要自己传字符串 `seconds` 或额外构造
   `duration`（传了也必须与 `seconds` 相等）。
3. **PAI 不接受 `resolution` / `size`。** 分辨率由模型 id 决定；PAI 按秒已支持
   `image_with_roles` / `video_with_roles` / `audio_with_roles`，按次以模型声明为准。
4. **`with-video` / `no-video` 不能混用。** 仅对声明这类能力的模型适用；
   `with-video` 必须提供 `reference_video`，`no-video` 传了参考视频会 400。
5. **异步视频 B1 可能没有真实 usage。** 上游不返回 usage 时，B1 为
   `usage_source=unavailable`，Token 字段为 `null`，`split_known=false`，
   `total_tokens` 省略。`null` 不是 0，也不代表免费；不得用历史 TA token 数补齐。
6. **B3 金额是结算依据。** 按次、按秒、按 token 的历史价格说明都不能替代 B3
   最新 `status=settled` 记录的 `total_amount` 和提交时价格版本；B1 的
   `usage_unit=token` 只是当前字段限制，不表示视频按 token 计费。
7. **PAI 完成响应可能没有 `usage`。** PAI 按秒有 `usage` 时按
   `output_seconds` / `input_seconds` 结算；Flash `second-1` 带参考时上游当前不返回
   `usage`，网关按请求 `seconds` 估算参考秒并按两段式价格结算；上游后续返回实际
   `input_seconds` 时以实际值为准。
8. **`capabilities` 是池级汇总。** 不能拿它判断池内每个模型都支持 `tools` 等能力；
   也不能据此判断图片/视频生成能力。
9. **`models` 是字典序数组。** A3 返回的模型顺序不等于后台渠道配置顺序；不要依赖
   渠道顺序，按当前清单中的模型 id 选择即可。
10. **95% 不是失败。** 2026-11-25 实测 `wan3-720p-once` 在 95% 停留约 12 分钟，
   17m46s 后完成 100%；继续轮询原 task id，不要重提。

---

## 8. 2026-11-23 复测与对账要求

1. 每次模型调用携带 `X-Harness-Request-Id` 和 `X-Harness-Run-Id`，均为 1–64 字符。
2. `video` / `plan` 必须显式传对应 `X-Route-Group`；先核对第 12 号文档的精确
   模型 id，不要把 A3 条目数当成可调数量。
3. 视频提交成功后只轮询原 task id，不因轮询超时盲目重提。
4. 使用请求 ID 关联 B1/B3，不要用响应体 `model` 反查账本。
5. B1 的 `usage_source=unavailable` / Token `null` 不表示零用量或免费；费用以
   B3 最新 `status=settled` 记录的金额为准。

---

## 9. 2026-11-17 新视频规格与编辑合同（历史验收记录）

本节保留 2026-11-17 全模型验收覆盖的请求形状；完整历史结果见
[10-full-model-acceptance-20261117.md](./10-full-model-acceptance-20261117.md)。
**本节出现不等于当前可调**，当前调用必须再核对第 12 号文档；已下架 id 不得因
历史验收通过而重新加入调用清单。

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

### 9.7 2026-11-17 已知上游阻塞（历史记录）

以下只描述 11-17 验收时的现象，不代表 2026-11-23 的当前状态；当前是否可调以
第 12 号文档和现场复测为准：

- MiniMax 4 个 `video-line-v1-second` 模型：上游 401 `fail_to_fetch_task` /
  `unauthorized`；网关已声明，直连上游复现。
- `wan3-1080p-once`：上游任务停在 95%；`wan3-720p-once` 通过。
- `seedance2-720p-second-1`：上游生成失败；同组其他 Seedance 模型通过。

---

## 10. Seedance 2.5 Flash（2026-11-17 新增）

2026-11-16 供应侧新增 5 个已启用的 Seedance 2.5 Flash 规格，2026-11-17 同步到本文。
这 5 个 id 均列入 2026-11-23 的可复测清单。清晰度由模型 id 锁定，客户端不要再传
`resolution` / `size`。配置启用不等于生成、健康或计费已全量验收；下面的价格与边界
以当前生效配置为准，实际结算金额以 B3 为准。

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

## 11. 已下架/停用模型（禁止作为当前调用清单）

以下 29 个模型在供应商当前目录中已下架或停用，部分仍可能出现在旧 A3 快照、旧渠道
声明或历史文档中。调用出现 400/500/503 属于预期拒绝，不应作为当前可上线模型；
历史成功记录、token 数或金额也不能证明其当前可用。

```text
grok-video1.5-480p-second
minimax-h3-max-480p-second
minimax-h3-max-768p-second
minimax-h3-max-1080p-second
seedance2-480p-once
seedance2-1080p-once
seedance2-fast-720p-once-2
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

渠道声明校正后，以新版本 A3 和第 12 号文档的后续修订为准；在收到新的可复测依据前，
不得自行将上述 id 恢复为当前样例。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| V0.13 | 2026-11-17 | 新增第 10 节 Seedance 2.5 Flash（3 个按秒 / 2 个按次）的池、价格、请求规则与 3 个 JSON 样例；修正第 2 节 PAI 参考素材输入与按秒结算描述。 |
| V0.14 | 2026-11-23 | 按第 11、12 号文档校准 A3 83/80 与初步核实可调 49/17，明确跨池重复合法、A3 候选不等于可调；移除 TA token 当前样例并列出 29 个已下架/停用模型；明确异步视频 B1 `unavailable` / `null`、B3 金额为结算依据。 |
| V0.15 | 2026-11-25 | 渠道声明与上游对齐，A3 直接返回 video 49 / plan 17；补充 `wan3-720p-once` 95% 停留约 12 分钟、17m46s 后完成的轮询注意。 |
