# Harness 联调反馈差异与处理指引

**文档版本**：V1.1（2026-11-25 更新）\
**对应网关版本**：`harness-api-20261117-b040241`\
**测试 Base URL**：`https://testapi.soloai.chat`\
**对照基线**：《New API 网关 ↔ 平台 接口对接方案 V2.3》、Harness V1.1（2026-11-17）\
**用途**：只说明差异、处理动作和复测标准；完整接口手册作为参考，不要求本次逐页重读。

> **2026-11-25 更新**：渠道目录已与上游对齐，A3 变为 chat 26 / video 49 / image 10 / plan 17；本地 7 个视频代表模型实测均最终 SUCCESS。完整证据见 [14-alignment-smoke-test-20261125.md](./14-alignment-smoke-test-20261125.md)。

## 1. 结论速览

| 领域 | 当前结论 | 本次动作 |
| --- | --- | --- |
| 鉴权与 A3 基础链路 | 已通过 | 保持 |
| 文字模型 | Harness 11-21 实测 26/26；11-23 有 3 条上游失败记录 | 按模型备注复测，不改成自用模式 |
| 图片 | Nano Banana 1K/4K 公网通过；MIME 仍标错；GPT Image 遇供应商 524 | 继续修 MIME；超时先查 B3/产物 |
| 视频 | 7 个上游分组代表模型实测均 SUCCESS | 按新 A3 与第 12 号清单复测；95% 不等于失败 |
| A3 目录 | **已对齐为 video 49 / plan 17**；A3 直接返回真实可调集合 | 已关闭，见第 14 号文档 |
| B1/B3 | 基础流程通过；媒体单位字段仍为 token | 明确字段限制与金额口径 |
| 文档 | V2.3 与当前实现存在多处差异 | 以本指引 + 11/12 号文档为当前口径 |

**2026-11-25 已变更渠道 `models` 声明与 OpenResty 代理超时（均已备份）。** 本次仍无生产代码变更；需要修复的项已明确标注责任方和验收标准。

## 2. 逐项差异与处理

### 01 · Run ID 上限与必填规则

> **2026-11-27 更新**：Run ID 已放宽到 80，Request ID 仍为 64；以下为 2026-11-23 历史差异与复测口径，旧数字保留不改。

| 项目 | 内容 |
| --- | --- |
| 差异 | V2.3 写上限 80，现网实际为 64 |
| 当前实现 | `X-Harness-Request-Id`、`X-Harness-Run-Id` 均必填，1–64 字符，仅允许 `A-Z a-z 0-9 - _`；不强制前缀或 ULID |
| 拒绝行为 | 缺失、空值、非法字符、超过 64 字符均在转发上游前返回 `400 REQUEST_INVALID`，不产生费用和 B1/B3 |
| Harness 动作 | 将客户端校验改为 1–64；不要截断超长 ID；不要用响应体 model 反查账本 |
| 网关动作 | 无需改代码；修订共同文档，将 80 改为 64 |
| 复测标准 | 64 字符通过；65 字符、缺失、非法字符均 400，且查不到 B1/B3 |

### 02 · 模型与路由契约

| 项目 | 内容 |
| --- | --- |
| 差异 | V2.3 未定义图片、视频和 plan 媒体接口，也未说明 `X-Route-Group` |
| 当前实现 | A3 返回 `chat/video/image/audio/plan` 五个池；`chat` 默认，`image/video/plan` 必须显式传对应池；未知池返回 `403 access_denied` |
| 模型路由 | 模型 ID 不会被池路由重写；池只限制可选渠道集合 |
| Harness 动作 | SDK 按协议拼接路径；媒体请求必须带 `X-Route-Group`；不要假设 `/v1` 一定需要或一定不需要，按 SDK 要求处理 |
| 网关动作 | 在共同文档补充媒体提交、查询、下载、错误和计费字段 |
| 复测标准 | 同一模型在指定池内可路由；未知池 403；不传媒体池头时按默认 chat 处理并可能失败 |

### 03 · 目录与渠道

| 项目 | 内容 |
| --- | --- |
| 差异 | V1.1 写 video 53 / plan 42；2026-11-23 A3 曾返回 video 83 / plan 80，存在跨池重复 ID |
| 重要澄清 | **跨池重复本身合法**，两个池可以引用同一供应商或同一模型；不能以“交集必须为 0”作为验收标准 |
| 真实问题 | 部分渠道 `models` 曾声明上游不提供或已下架的模型；**2026-11-25 已清理完成** |
| 当前快照 | **2026-11-25 已对齐**：A3 = chat 26 / video 49 / image 10 / audio 0 / plan 17；渠道 `models` 与上游 `/v1/models` 一致 |
| 能力标签 | A3 `capabilities` 是池级端点类型汇总，可能含 `chat/stream/tools`，不能据此判断图片/视频真实能力 |
| Harness 动作 | 重新拉取 A3；按第 12 号文档复测；A3 已与可调清单一致 |
| 网关动作 | **已完成（2026-11-25）**：21 个渠道 extra=0；渠道 66 置空、84 只留同名 4 个 |
| 复测标准 | 每个上线目录条目都能在指定池找到有效渠道；已下架模型不再作为当前候选 |

### 04 · 视频任务失败

| 项目 | 内容 |
| --- | --- |
| 差异 | TA token 抽样在 video/plan 合计 5 次提交返回 500；另 1 个 plan 按次模型返回 400 `MODEL_DISABLED` |
| 根因 | TA Seedance Token 22 档已在上游当前目录中不可用；另有 7 个视频规格已下架/停用 |
| 当前行为 | 2026-11-25 已从渠道声明移除；当前 A3 不再返回这些 ID，旧 ID 调用属于预期拒绝 |
| Harness 动作 | 从当前模型配置中移除 TA token 和已下架 ID；改用第 12 号清单 |
| 网关动作 | **已完成（2026-11-25）**：渠道 66 置空；67/69/78/80/81/82/83 按上游目录收敛 |
| 复测标准 | 可复测清单中的模型不再出现“可用渠道不存在”；`seedance2-fast-720p-once-1` 已实测 SUCCESS（task_Us2pCcqrXnCxd7OudXURpyCySFelHeaE）；旧模型不作为上线候选 |

### 05 · Nano Banana 接口与超时

| 项目 | 内容 |
| --- | --- |
| 差异 | 同步 1K 返回 524，4K 返回 500；旧说明中的异步 `/v1/videos` 代表请求返回 503；成功样本有 JPEG 被标为 PNG |
| 当前正式协议 | Nano Banana 使用同步图片接口，不返回 task id；旧异步 `/v1/videos` 合同不代表当前正式协议 |
| Harness 动作 | 使用同步图片协议；客户端设置足够长的超时；超时后先按请求 ID 查 B3/产物，再决定是否重试；保存文件时按 PNG/JPEG 文件头判断真实格式 |
| 网关动作 | **超时已完成（2026-11-25）**：OpenResty 代理超时放宽到 600 秒；MIME 仍待修正，当前要求接收端按文件头识别 |
| 复测标准 | 1K/4K 在约定超时内返回或给出明确终态；2026-11-25 公网 1K 44.7s、4K 109.5s 均 200；文件 MIME 仍可能标错，按文件头解码 |

### 06 · 异步视频账本

| 项目 | 内容 |
| --- | --- |
| 差异 | 历史 TA 完成记录报告 40,594 tokens；B1 复查仍只有版本 1 且为零 |
| 当前实现 | 上游不返回 usage 的异步视频，B1 使用 `usage_source=unavailable`，token 字段为 `null`，`split_known=false`，`total_tokens` 省略 |
| 重要澄清 | `null/unavailable` 不是“实测为 0”；不得伪造 token 用量 |
| Harness 动作 | 不要用 B1 token 判断视频任务是否完成；完成与费用以 B3 `settled` 和任务状态为准 |
| 网关动作 | 维持 unavailable/null；如需历史回补，必须由上游提供可信 usage 和请求映射 |
| 复测标准 | 无 usage 的视频任务 B1 为 unavailable/null；B3 仍正常结算 |

### 07 · 非 token 计费与用量

| 项目 | 内容 |
| --- | --- |
| 差异 | 视频按次/按秒、图片按张，但 B1/B3 单位字段仍标 token；Nano Banana 可能回传估算 token |
| 当前实现 | B1 `usage_unit=token`；B3 `billing_basis=token`。这是当前字段限制，不代表媒体按 token 计费 |
| 金额口径 | 以 B3 最新版本 `total_amount`、`status=settled` 和提交时价格版本为准 |
| 估算口径 | `usage_source=estimated` 表示估算；Gemini thinking tokens 按当前 B1 归一化结果读取，不另行相加 |
| Harness 动作 | 不依赖 `usage_unit/billing_basis` 判断媒体实际单位；按模型价格/表达式和 B3 金额对账 |
| 网关动作 | 后续将单位字段修正为 second/per_call/image，或双方确认保留 token 限制 |
| 复测标准 | 媒体金额与模型价格一致；字段限制有明确文档，不产生重复计费 |

### 08 · 响应字段及文档

| 项目 | 内容 |
| --- | --- |
| 差异 | 部分 401/403 缺 `retryable` 或 `request_id`；MiniMax MP4 返回通用 MIME；旧统计与当前样本不同 |
| 当前错误体 | JWT 401、模型 scope 403、A3/B1/B3 403 使用不同错误体；`retryable/details/request_id/type` 不能全局强依赖 |
| 下载 MIME | 视频下载当前透传上游 `Content-Type`；接收端应结合文件头校验 |
| A5 | `count_tokens` 当前未交付：Claude 路由未注册，Gemini `:countTokens` 返回 404 |
| 分页 | B1/B3 `limit` 默认 50、最大 200；超过 200 静默 clamp 到 200，并返回 `next_cursor` |
| BatchGet | 最多 100 个 request ID、合计 500 条事件；跨请求超 500 返回前 500 条并标 `truncated=true`；单请求自身超 500 返回 `400 EVENT_LIMIT_EXCEEDED` |
| `/api/pricing` | 通用定价展示接口，默认公开但可配置关闭/鉴权；不是 A3，也不是 B3 对账依据 |
| Harness 动作 | 错误解析保持宽容；分页按 `next_cursor` 继续；下载校验文件头；A5 暂按 404 处理 |
| 网关动作 | 统一错误体和 MIME；维护唯一当前版本说明 |
| 复测标准 | 401/403/404/400 行为与文档一致；分页和 BatchGet 边界可复现 |

## 3. 模型目录变更摘要

| 池 | 2026-11-23 A3（历史） | 2026-11-25 A3（当前） | 当前处理 |
| --- | ---: | ---: | --- |
| chat | 26 | 26 | 正常复测；关注 3 条上游失败备注 |
| image | 10 | 10 | GPT Image 4 款；Nano Banana 4/6；MIME 仍待整改 |
| video | 83 | **49** | 渠道声明已对齐 |
| plan | 80 | **17** | TA token 22 档已移除 |
| audio | 0 | 0 | 预留池，本期不可调用 |

**可复测模型完整清单见 `12-retest-model-list-20261123.md`。本指引不重复罗列 66 个当前候选，避免信息过载。**

## 4. Harness 侧修改清单

> **2026-11-27 更新**：Request ID 仍为 1–64 字符；Run ID 已放宽为 1–80 字符。以下为 2026-11-23 历史清单。

- [ ] Request ID / Run ID 校验改为必填、1–64 字符。
- [ ] `chat` 默认池；`image/video/plan` 显式传 `X-Route-Group`。
- [ ] 从当前模型配置移除 TA token 和已下架/停用 ID。
- [ ] 不要用 A3 `capabilities` 判断媒体真实能力。
- [ ] 不要用响应体 `model` 反查账本，统一用请求 ID 关联 B1/B3。
- [ ] Nano Banana 使用同步图片协议；长超时后先查 B3/产物再重试。
- [ ] 图片/视频保存时按文件头校验真实格式。
- [ ] 视频完成与费用以任务终态和 B3 `settled` 为准，不把 B1 `null` 当 0。
- [ ] 错误解析兼容不同错误体；分页按 `next_cursor` 继续。
- [ ] A5 `count_tokens` 当前按 404 处理。

## 5. 网关侧修复清单

- [ ] 按上游实时目录逐渠道校正 `models`，移除已下架/跨组声明。
- [ ] 拉长 OpenResty/图片长请求超时，避免 60 秒默认超时。
- [ ] 修正图片/视频 MIME，或明确接收端文件头校验规则。
- [ ] 评估 B1/B3 媒体单位字段修正为 second/per_call/image。
- [ ] 统一 401/403 错误体、request_id 和 retryable 口径。
- [ ] 将 V2.3 修订为双方共同版本，纳入本指引的 8 项差异。

## 6. 联合复测用例

| # | 用例 | 预期结果 |
| --- | --- | --- |
| 1 | Request ID = 64 字符；Run ID = 64/65/80 字符 | 正常进入模型调用 |
| 2 | Request ID = 65 字符或缺失；Run ID = 81 字符或缺失 | 400 `REQUEST_INVALID`，无 B1/B3 |
| 3 | chat / image / video / plan 各选 1 个当前清单模型 | 指定池路由成功，返回有效内容或 task id |
| 4 | TA token 或已下架模型 | 不再作为当前候选；如调用则明确拒绝 |
| 5 | Nano Banana 1K/4K | 约定超时内返回或明确终态；超时后可查 B3/产物 |
| 6 | 无 usage 的异步视频 | B1 `unavailable/null`，B3 正常结算 |
| 7 | B1/B3 `limit=201` | clamp 到 200，返回 `next_cursor` |
| 8 | BatchGet 100 个 ID / 101 个 ID | 100 正常；101 拒绝；跨请求超 500 条标 `truncated=true` |
| 9 | A5 `count_tokens` | 当前 404，文档标注未交付 |
| 10 | 请求 ID 关联 B1/B3 | 同一请求可关联 usage 与 cost，金额一致 |

> **2026-11-27 更新**：Run ID 当前上限为 80（Request ID 仍 64）；本表已按新上限修订，下方历史差异说明保留旧数字。

> **视频轮询补充（2026-11-25）**：任务可能停在 95% 十余分钟。`wan3-720p-once` 实测 17 分 46 秒才完成；停在 95% 不是失败，继续轮询原 task id，不要重提。

## 7. 验收标准

本次反馈关闭需同时满足：

1. 8 项差异均有双方确认的处理结论和责任方。
2. Harness 侧完成第 4 节修改。
3. 网关侧完成第 5 节修复，或明确列为双方接受的已知限制。
4. 第 6 节 10 个复测用例通过。
5. 当前可复测清单中的模型不再出现“目录有、渠道无”的假候选（2026-11-25 已满足，A3 = video 49 / plan 17）。
6. 最终 Base URL、DNS、证书续期和回退方式由双方运维确认。

## 8. 待双方确认

| 事项 | 需要确认 |
| --- | --- |
| 共同文档版本 | V2.3 是否升级为 V2.4，并引用本指引与 11/12 号文档 |
| 目录校正 | 是否以 video 49 / plan 17 作为切换前临时复测基线 |
| 媒体单位字段 | 立即修正，还是阶段性保留 `usage_unit/billing_basis=token` |
| A5 | 本期明确 404，还是纳入后续实现 |
| 最终入口 | 公网域名、Harness 内网域名、DNS 拆分解析和证书续期方式 |
| 回退 | 渠道配置备份、回切公网入口和旧版本镜像的触发条件 |

## 附录：已下架/停用模型（29）

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

以上模型不得作为当前上线或复测清单；完整在架候选见第 12 号文档。
