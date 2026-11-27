# Harness 网关接口交付包

**版本**：V1.4（2026-11-27 修订）\
**测试 Base URL**：`https://testapi.soloai.chat`\
**对应网关版本**：`harness-api-20261126-882ffc5`（2026-11-27 生产；P0 修复后以新镜像版本为准）\
**A3 目录版本**：截至 2026-11-27 实测 `catalog_version=v12`；以实际接口响应为准\
**B4 接口版本**：`interface_version=v2`（与本文档 V1.4 不是同一版本号）\
**本次主交付**：[联调反馈差异与处理指引 V1.0（DOCX）](./releases/Harness_联调反馈差异与处理指引_V1.0_2026-11-23.docx) —— 只讲差异、处理动作和复测标准；完整手册为参考\
**2026-11-27 增量**：[V2.3 复测问题修复与证据报告](./15-v23-retest-remediation-20261127.md) —— TVC 8 次请求的预扣/退款凭证、22+7 停用清单、P0 修复范围\
**首次交付日期**：2026-10-25

---

## 一、文件清单

| # | 文件 | 用途 | 阅读顺序 |
| --- | --- | --- | --- |
| 13 | [联调反馈差异与处理指引](./13-feedback-diff-and-action-guide-20261123.md) | 8 项反馈的差异、Harness/网关动作、联合复测与验收标准 | **本次主交付** |
| 14 | [目录对齐与本地复测报告](./14-alignment-smoke-test-20261125.md) | 2026-11-25 渠道目录对齐、A3 新数量、7 个视频任务与图片实测结果 | **本次增量更新** |
| 15 | [V2.3 复测问题修复与证据报告](./15-v23-retest-remediation-20261127.md) | TVC 8 次请求预扣/退款凭证、22+7 停用清单、P0 修复与验收边界 | **当前口径优先看** |
| 01 | [接入指南](./01-integration-guide.md) | Base URL、接口清单、请求头、资源池、分页、错误体与调用示例 | 完整参考 |
| 11 | [联调反馈处理说明](./11-feedback-resolution-20261123.md) | 2026-11-21 反馈的逐项结论、当前环境、已知限制与后续待办 | **当前口径优先看** |
| 12 | [可复测模型清单](./12-retest-model-list-20261123.md) | A3 目录声明与初步核实可调模型的区别、分池模型清单与复测要求 | **模型联调前必读** |
| A3 | [模型列表结构更新](./A3-model-catalog-update.md) | A3 响应结构与展示方式；当前可调集合以第 12 号文档为准 | 目录结构参考 |
| 08 | [视频调用样例](./08-video-api-examples.md) | 视频池 / plan 池请求差异、任务轮询与内容下载示例 | 视频联调前读 |
| 09 | [图片调用样例](./09-image-api-examples.md) | GPT Image 同步 / Nano Banana Gemini 同步调用示例 | 图片联调前读 |
| 10 | [全模型验收报告](./10-full-model-acceptance-20261117.md) | 2026-11-17 历史验收结果与上游阻塞清单 | 历史验收参考 |
| 02 | [鉴权配置](./02-auth-setup.md) | JWKS / iss / aud / sub 配置与 JWT 校验规则 | 第 2 篇 |
| 03 | [数据契约](./03-data-contract.md) | B1/B3 字段语义、事件类型、版本链、金额与单位字段限制 | 第 3 篇 |
| 04 | [联调验收清单](./04-acceptance-checklist.md) | 测试用例与预期响应；需结合第 11、12 号文档校准 | 第 4 篇 |
| 05 | [待确认事项](./05-pending-confirmations.md) | 历史契约差异记录；当前结论以第 11 号文档和 03 数据契约为准 | 历史参考 |
| 06 | [对外测试摘要](./06-test-summary.md) | 各接口历史测试状态 | 参考 |
| 07 | [验收证据](./07-acceptance-evidence.md) | 历史 JWT、图片、用量来源与金额核对记录 | 参考 |

### 1.1 交付文件（主交付 + 参考）

**本次主交付：**

- [Harness 联调反馈差异与处理指引 V1.0（DOCX）](./releases/Harness_联调反馈差异与处理指引_V1.0_2026-11-23.docx)
- [Harness 联调反馈差异与处理指引 V1.0（PDF）](./releases/Harness_联调反馈差异与处理指引_V1.0_2026-11-23.pdf)

**完整参考手册（按需查阅，不要求本次逐页重读）：**

- [Harness 最终模型接口联调手册 V1.2（DOCX）](./releases/Harness_最终模型接口联调手册_V1.2_2026-11-23.docx)
- [Harness 最终模型接口联调手册 V1.2（PDF）](./releases/Harness_最终模型接口联调手册_V1.2_2026-11-23.pdf)

> **注意**：`releases/` 下的 DOCX/PDF 仍是 2026-11-23 快照，尚未重新渲染；2026-11-25 的目录对齐与实测结果以第 14 号 Markdown 为准。

---

## 二、建议阅读顺序

**给 Harness 后端开发：**

1. 先读 **13 联调反馈差异与处理指引** —— 只处理 8 项差异、动作和复测标准
2. 需要模型范围时再看 **12 可复测模型清单** —— 不要仅凭 A3 条目选择付费模型
3. 需要完整接口细节时查阅 **01 接入指南** —— Base URL、请求头、资源池和错误体
4. 阅读 **03 数据契约** —— 理解 B1/B3、分页、批查和金额口径
5. 联调时对照 **04 验收清单**，并结合 11/12/13 的修订逐项验证

**给 Harness 技术负责人：**

1. 先读 **13** —— 只了解 8 项差异、双方动作和验收标准
2. 需要模型范围时再看 **12** —— 确认当前可复测范围与已下架模型
3. 最后按需查看 **02 鉴权配置** 和 **06 测试摘要**，确认配置与验证边界

---

## 三、当前状态

### 3.1 当前环境

| 项 | 当前值 |
| --- | --- |
| HTTPS Base URL | `https://testapi.soloai.chat` |
| 网关版本 | `harness-api-20261117-b040241` |
| A3 | `GET /v1/models`；2026-11-25 对齐后为 chat 26 / video 49 / image 10 / plan 17，`catalog_version` 自动递增 |
| B4 | `GET /internal/v1/gateway/health`，`interface_version=v2` |
| Harness 内网访问 | 同 VPC 内当前域名解析到 `10.245.222.6`，HTTPS 443 正常 |
| 公网访问 | 受当前 DNS / 入口配置影响；公网复测前需由运维确认解析与安全组 |
| 健康检查 | `GET /api/status` 返回 200 |
| 当前反馈结论 | [11-feedback-resolution-20261123.md](./11-feedback-resolution-20261123.md) |
| 当前可复测清单 | [12-retest-model-list-20261123.md](./12-retest-model-list-20261123.md) |
| 2026-11-25 复测结果 | [14-alignment-smoke-test-20261125.md](./14-alignment-smoke-test-20261125.md) |

> 本文档不包含上游供应 Key、Harness JWT、证书私钥或内部转接组件凭据。

### 3.2 目录声明与实测可调

A3 返回的是资源池和渠道声明的真实模型 ID。2026-11-25 已将 21 个渠道的 `models` 与上游 `/v1/models` 全量对齐，A3 现在直接返回真实可调集合；跨池重复仍然合法。

| 池 | 2026-11-23 A3（历史） | 2026-11-25 A3（当前） | 当前说明 |
| --- | ---: | ---: | --- |
| `chat` | 26 | 26 | Harness 11-21 实测 26/26；11-23 有 3 条上游瞬时或线路失败，详见 12 |
| `image` | 10 | 10 | GPT Image 4/4；Nano Banana 4/6，MIME 仍待整改 |
| `video` | 83 | **49** | 渠道声明已与上游对齐，A3 不再返回虚条目 |
| `plan` | 80 | **17** | 渠道声明已与上游对齐；TA token 22 档已移除 |
| `audio` | 0 | 0 | 预留池，本期不可调用 |

- `chat` 为默认池；`image`、`video`、`plan` 调用必须显式传对应的 `X-Route-Group`。
- 资源池只限制可选渠道集合，不会重写模型 ID；同一模型可以合法出现在多个池。
- A3 的 `capabilities` 是池级端点类型汇总，可能包含 `chat` / `stream` / `tools` 等通用标签，不能据此判断图片或视频的真实能力。
- 目录已对齐，但第 12 号文档仍是本轮复测清单；该清单是现场快照，不是永久合同清单。
- 2026-11-25 已按 7 个上游分组各抽测 1 个视频模型，7 个任务最终均 SUCCESS；不代表 49/17 个模型已全量完成生成验收，详见第 14 号文档。

### 3.3 已通过或已确认

| 项 | 状态 |
| --- | --- |
| RS256 JWT / JWKS 与 scope 隔离 | 已通过 |
| A3 200、ETag 304、无 JWT 401、错误 scope 403 | 已通过 |
| 文字模型 | Harness 11-21 实测 26/26；11-23 有 3 条上游失败记录，详见 12 |
| GPT Image | 4/4（历史）；2026-11-25 抽测 `gpt-image-2.6-flare` 遇供应商侧 524 |
| Nano Banana | 4/6（历史）；2026-11-25 公网 1K 44.7s、4K 109.5s 均 200；MIME 仍标错 |
| 视频抽样 | 2026-11-25 按 7 个上游分组各 1 个模型实测，最终均 SUCCESS；`wan3-720p-once` 耗时 17m46s |
| B1 / B3 | 列表、分页、24 小时窗口、批查、权限隔离和 B4 字段已核对 |
| 请求 ID 关联 | Request ID 与 B1/B3 关联已确认 |

### 3.4 已知限制与后续待办

| 事项 | 责任方 | 当前口径 / 完成标准 |
| --- | --- | --- |
| 渠道 `models` 按上游真实供货校正 | 网关运维 | **已完成（2026-11-25）**：21 个渠道 extra=0，A3 为 video 49 / plan 17 |
| OpenResty / 图片长请求超时 | 网关运维 | **已完成（2026-11-25）**：testapi/papi 代理超时 600 秒；4K 公网 109.5s 返回 200 |
| 图片 / 视频 MIME 归一 | 网关研发 | **仍待处理**：Nano Banana 实际 JPEG 仍标 `image/png`；当前要求按文件头识别 |
| B1 / B3 媒体单位字段 | 网关研发 + Harness | `usage_unit` / `billing_basis` 能表达 `second` / `per_call` / `image`，或双方确认保留 `token` 限制 |
| 统一错误体与 A5 | 网关研发 + Harness | JWT、scope、任务插件错误体口径一致；A5 `count_tokens` 交付范围明确 |
| 共同文档 V2.3 修订 | 双方 | Run ID、池路由、媒体接口、错误体、分页和 A5 口径一致 |
| 最终公网 / 内网入口 | 双方运维 | Base URL、DNS、安全组、证书续期和回退方式确认 |

### 3.5 已废止的旧口径

- Run ID 上限曾按 64 校准；**2026-11-27 起放宽为必填 1–80 字符**，Request ID 仍为 1–64 字符；两个关联 ID 均只允许 `A-Z`、`a-z`、`0-9`、`-`、`_`，不强制前缀或 ULID。
- Nano Banana 当前使用同步 Gemini 图片协议，不返回 task id；旧异步 `/v1/videos` 说明不代表当前正式协议。
- A3 条目不等于当前均可直接调用；不得把历史 TA Token 模型或已下架 ID 当作当前在售模型。
- 异步视频 B1 没有真实 usage 时为 `usage_source=unavailable`、Token 字段 `null`、`split_known=false`，`total_tokens` 省略；这不是 0 Token。
- B1 `usage_unit=token`、B3 `billing_basis=token` 是当前字段限制，不表示图片或视频按 Token 计费，也不存在 B3 `usage_unit` 字段。
- `/api/pricing` 是通用定价展示接口，不是 A3，也不是 B3 对账依据。

### 3.6 历史基线

官方基线为 `QuantumNous/new-api` `v1.0.0-rc.48`，commit `706144769e5bed5a47b78721e3bb11da3cf4c0b9`；RC48 候选保留 Harness 的 A3 目录分流、内部账本、智能路由池与任务插件协议。历史构建和验收记录见第 06、07、10 号文档，不代表当前目录可调范围。

---

## 四、快速开始

**第 1 步：确认测试入口与鉴权配置**

使用 `https://testapi.soloai.chat`，参照 `02-auth-setup.md` 确认 JWKS URL、`iss`、`aud`、`sub`。不要在文档、日志或工单中粘贴真实 JWT。

**第 2 步：确认接口清单**

参照 `01-integration-guide.md` 第一节，确认接口路径、Scope 和当前错误体。A5 `count_tokens` 当前未交付。

**第 3 步：选择可复测模型**

参照 `12-retest-model-list-20261123.md`，不要仅凭 A3 条目或 `capabilities` 选择付费模型。

**第 4 步：准备请求头和资源池**

- 每次模型调用必须携带 `X-Harness-Request-Id`（1–64 字符）和 `X-Harness-Run-Id`（1–80 字符）。
- `chat` 使用默认池，也可显式传 `X-Route-Group: chat`。
- `image`、`video`、`plan` 必须显式传对应池；不存在的池返回 `403 access_denied`。
- 对账使用 Request ID 关联 B1/B3，不要用响应体 `model` 反查账本。

**第 5 步：联调与复核**

按 `04-acceptance-checklist.md` 从 P0 项开始验证。图片长请求超时后先查 B3 / 产物，再决定是否重试；视频任务只轮询原 task id，不因轮询超时盲目重提。

---

## 五、接口速查

| 编号 | 方法 | 路径 | Scope |
| --- | --- | --- | --- |
| A1 | POST | `/v1/chat/completions` | `gateway.chat` |
| A2 | POST | `/v1/responses` | `gateway.chat` |
| A3 | GET | `/v1/models` | `gateway.catalog.read` |
| A4 | POST | `/v1/messages` | `gateway.chat` |
| A9 | POST | `/v1beta/models/{model}:{action}` | `gateway.chat` |
| B1 | GET | `/internal/v1/gateway/usage-events` | `gateway.usage.read` |
| B1-batchGet | POST | `/internal/v1/gateway/usage-events:batchGet` | `gateway.usage.read` |
| B3 | GET | `/internal/v1/gateway/cost-records` | `gateway.cost.read` |
| B3-batchGet | POST | `/internal/v1/gateway/cost-records:batchGet` | `gateway.cost.read` |
| B4 | GET | `/internal/v1/gateway/health` | 无（仅需有效 JWT） |

- **B2 单次详情本期取消，未实现。**
- **A5 `count_tokens` 当前未交付**：Claude 路由未注册，Gemini `:countTokens` 返回 404。
- **`/api/pricing` 不是合同接口**：默认公开但可配置关闭或鉴权，不用于 A3 目录判断或 B3 对账。

---

## 六、联系与反馈

| 类型 | 说明 |
| --- | --- |
| 契约疑问 | 字段语义、事件规则或错误体不清晰 |
| 模型可用性 | 反馈具体模型、资源池、请求 ID 和观测时间，并对照第 12 号文档 |
| 缺陷报告 | 实际调用与第 11 号文档或 01 接入指南不符 |
| 配置交付 | JWKS URL 等非秘密配置；秘密凭据通过约定安全渠道交付 |

---

## 七、版本说明

本文档为 **V1.4（2026-11-27 修订）**。2026-11-27 增量：`X-Harness-Run-Id` 上限由 64 放宽到 **80 字符**（`X-Harness-Request-Id` 仍为 64）；账本列 `run_id` / `harness_run_id` 扩到 `varchar(96)`，升级顺序为先扩列再部署应用。本文档此前为 **V1.3（2026-11-25 修订）**，对应网关 `harness-api-20261117-b040241`。2026-11-25 增量：21 个渠道与上游目录对齐，A3 变为 chat 26 / video 49 / image 10 / plan 17；7 个上游分组代表视频模型实测均 SUCCESS；OpenResty 长请求超时放宽到 600 秒；Nano Banana 1K/4K 公网通过，MIME 仍待修正。V1.2 的历史变化如下：

1. 测试 Base URL 统一为 `https://testapi.soloai.chat`。
2. Run ID 校准为必填、1–64 字符（2026-11-27 已放宽到 1–80），并明确合法字符与拒绝行为。
3. 补充 `X-Route-Group`：`chat` 为默认池，`image` / `video` / `plan` 必须显式传对应池。
4. 区分 A3 v10 目录声明与 2026-11-23 初步核实可调集合，不再承诺 A3 模型均可直接调用。
5. 明确 Nano Banana 同步协议、长请求超时、MIME 校验与超时后先查账本 / 产物的重试规则。
6. 明确异步视频 B1 `usage_source=unavailable`、Token 字段 `null`、`split_known=false`、`total_tokens` 省略。
7. 披露 B1 `usage_unit=token` 与 B3 `billing_basis=token` 的已知字段限制；媒体实际计费以 B3 `settled` 金额和提交时价格版本为准。
8. 按实际实现拆分 JWT / scope / 任务插件错误体，明确分页 clamp 200、BatchGet 100 ID / 500 事件、A5 404 和 `/api/pricing` 的非合同性质。

历史版本结论仅用于追溯；与本版不一致时，以本版及后续修订为准。
