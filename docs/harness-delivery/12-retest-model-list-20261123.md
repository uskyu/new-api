# Harness 可复测模型清单（2026-11-23 校准）

**文档版本**：V1.3（2026-11-25 更新）\
**对应网关版本**：`harness-api-20261117-b040241`\
**A3 目录版本**：对齐后自动递增（2026-11-23 为 10）\
**性质**：现场快照，不是永久合同清单。2026-11-25 渠道声明已对齐，A3 直接返回 49/17；后续以新版本目录和本清单的修订为准。

## 1. 当前目录与真实可调的区别

| 池 | 2026-11-23 A3（历史） | 2026-11-25 A3（当前） | 说明 |
| --- | ---: | ---: | --- |
| `chat` | 26 | 26 | Harness 11-21 实测 26/26；11-23 复测存在 3 条上游瞬时/线路失败，见第 6 节 |
| `image` | 10 | 10 | GPT Image 4/4；Nano Banana 4/6，MIME 仍待整改 |
| `video` | 83 | **49** | 渠道声明已与上游对齐，A3 与可调清单一致 |
| `plan` | 80 | **17** | 渠道声明已与上游对齐；TA token 22 档已移除 |
| `audio` | 0 | 0 | 预留池，本期不可调用 |

**跨池重复本身合法。** 两个池可以引用同一供应商或同一模型。2026-11-25 已把“渠道声明了上游不提供或已下架的模型”清理完毕；A3 现在与可调清单一致，Harness 可直接按 A3 选择模型，但仍以本清单为当前快照。

## 2. Chat（26）

```text
claude-fable-5
claude-fable-5-1
claude-fable-5-2
claude-haiku-5-5
claude-opus-4-8
claude-opus-5
claude-opus-5-5
claude-sonnet-5
claude-sonnet-5-5
deepseek-flash
deepseek-v4-pro
gemini-3.8-flash
gemini-4-argon
glm-5.3
glm-5.3-flash
gpt-5.6-terra
gpt-6-astra
gpt-6-sol
gpt-6.1-sol
gpt-6.1-terra
grok-4.5
grok-4.6
grok-4.7
kimi-k3
kimi-k3.1-pro
kimi-k3.1-pro-highspeed
```

调用时默认使用 `chat` 池；也可显式传 `X-Route-Group: chat`。

## 3. Image（10）

```text
Nano Banana 2.1 1K
Nano Banana 2.1 2K
Nano Banana 2.1 4K
Nano Banana Pro 1K
Nano Banana Pro 2K
Nano Banana Pro 4K
gpt-image-2.5-flare
gpt-image-2.5-sunburst
gpt-image-2.6-flare
gpt-image-2.6-sunburst
```

调用必须传 `X-Route-Group: image`。Nano Banana 使用同步图片协议，不返回 task id；长请求超时和 MIME 注意事项见 `09-image-api-examples.md`。

## 4. Video（49，初步核实可调）

```text
MiniMax-H3-Max-480p
MiniMax-H3-Max-480p-first-last-frame
MiniMax-H3-Max-768p
MiniMax-H3-Max-768p-first-last-frame
grok-video1.5-720p-second
happyhorse1.1-1080p-second
happyhorse1.1-720p-edit-second
happyhorse1.1-720p-second
kling4-1080p-second-no-video
kling4-1080p-second-with-video
kling4-720p-second-no-video
kling4-720p-second-with-video
muse-video-720p
seedance2-1080p-second-1
seedance2-1080p-second-2
seedance2-480p-second-1
seedance2-480p-second-2
seedance2-480p-second-3
seedance2-4k-second
seedance2-720p-second-1
seedance2-720p-second-2
seedance2-fast-480p-second-1
seedance2-fast-480p-second-2
seedance2-fast-720p-second-1
seedance2-fast-720p-second-2
seedance2-mini-480p-second
seedance2-mini-720p-second
seedance2.5-1080p-second-1
seedance2.5-480p-second-1
seedance2.5-480p-second-2
seedance2.5-480p-second-3
seedance2.5-480p-second-4
seedance2.5-720p-second-1
seedance2.5-720p-second-2
seedance2.5-720p-second-3
seedance2.5-flash-480p-second-1
seedance2.5-flash-720p-second-1
seedance2.5-flash-720p-second-2
wan3-1080p-second
wan3-480p-second-1
wan3-480p-second-2
wan3-720p-second-1
wan3-720p-second-2
wan3.0-video-1080p
wan3.0-video-480p
wan3.0-video-720p
wan3.0-video-prime-1080p
wan3.0-video-prime-480p
wan3.0-video-prime-720p
```

调用必须传 `X-Route-Group: video`。该清单表示上游目录和抽样调用支持这些模型，不等于 49 个模型已在同一窗口全部完成生成验收。

## 5. Plan（17，初步核实可调）

```text
gemini-omni-1.1-flash-1080p-10s
gemini-omni-1.1-flash-1080p-10s-first-last-frame
gemini-omni-1.1-flash-720p-10s
gemini-omni-1.1-flash-720p-10s-first-last-frame
gemini-omni-1.2-flash-720p-10s
gemini-omni-1.2-flash-720p-15s
seedance2-720p-once-1
seedance2-720p-once-2
seedance2-720p-once-3
seedance2-fast-720p-once-1
seedance2-fast-720p-once-3
seedance2-fast-720p-once-4
seedance2.5-720p-once
seedance2.5-flash-480p-once-2
seedance2.5-flash-720p-once-2
wan3-1080p-once
wan3-720p-once
```

> **2026-11-26 更新**：上游同步已将 `seedance2.5-flash-480p-once-1` / `seedance2.5-flash-720p-once-1` 替换为 `-once-2`；当前生产渠道 69 与 A3 `plan` 返回 `-once-2`。`-once-1` 仅保留在历史记录中，不得作为当前调用清单。

调用必须传 `X-Route-Group: plan`。`plan` 的池级语义为按次视频；具体价格、时长和素材限制仍以每个精确模型的目录价格和接口说明为准。

## 6. Chat 复测备注（2026-11-23）

| 模型 | 11-23 观测 | 处理建议 |
| --- | --- | --- |
| `gpt-6-astra` | 首次 524；随后直连上游 3 次均 200 | 按瞬时超时处理，允许有限重试 |
| `gpt-6.1-sol` | 1 次 524 | 按瞬时超时处理，允许有限重试 |
| `gemini-4-argon` | `Gemini官方直连` 线路稳定复现 502；`Gemini号池1` 有成功记录 | 复测时记录 `X-Route-Channel-ID`；持续失败时切换备用线路或暂时停用故障线路 |
| 其余 23 个 | 11-23 有成功记录 | 正常复测 |

## 7. 已下架/停用模型（禁止作为当前调用清单）

以下 29 个模型在供应商当前目录中已下架或停用，部分仍可能出现在旧 A3 快照或历史文档中：

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

## 8. 复测要求

1. 每次模型调用携带 `X-Harness-Request-Id` 和 `X-Harness-Run-Id`，均为 1–64 字符。
2. `chat` 默认池；`image`、`video`、`plan` 必须显式传对应 `X-Route-Group`。
3. 不要用响应体 `model` 反查账本；使用请求 ID 关联 B1/B3。
4. 视频提交成功后只轮询原 task id，不因轮询超时盲目重提。
5. 图片长请求超时后，先查 B3/产物，再决定是否重试。
6. 目录条目、能力标签和本清单不一致时，以本清单和最新现场确认为准，并反馈具体模型、池和请求 ID。
