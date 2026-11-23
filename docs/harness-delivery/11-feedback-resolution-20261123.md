# Harness 联调反馈处理说明（2026-11-23）

**文档版本**：V1.2\
**对应网关版本**：`harness-api-20261117-b040241`\
**测试 Base URL**：`https://testapi.soloai.chat`\
**A3 目录版本**：`10`\
**B4 接口版本**：`interface_version=v2`（与本文档 V1.2 不是同一版本号）\
**范围**：回应 Harness 2026-11-21 反馈的 8 项问题；只更新文档和可复测清单，不改变现有鉴权、计费实现和生产渠道配置。\
**对外主交付**：第 13 号《联调反馈差异与处理指引》为精简版；本文作为详细底稿保留。

## 1. 当前环境

| 项 | 当前值 |
| --- | --- |
| HTTPS Base URL | `https://testapi.soloai.chat` |
| 网关版本 | `harness-api-20261117-b040241` |
| A3 | `GET /v1/models`，`catalog_version=10` |
| Harness 内网访问 | 同 VPC 内当前域名解析到 `10.245.222.6`，HTTPS 443 正常 |
| 公网访问 | 受当前 DNS/入口配置影响；公网复测前需由运维确认解析与安全组 |
| 健康检查 | `GET /api/status` 返回 200；B4 为 `/internal/v1/gateway/health` |
| 可复测清单 | 见 `12-retest-model-list-20261123.md` |

> 本文档不包含上游供应 Key、Harness JWT、证书私钥或内部转接组件凭据。

## 2. 逐项处理结果

### 01 · Run ID

**结论：文档与实现不一致，当前实现为必填、最大 64 字符。**

- `X-Harness-Request-Id`：必填，1–64 字符。
- `X-Harness-Run-Id`：必填，1–64 字符。
- 合法字符：`A-Z`、`a-z`、`0-9`、`-`、`_`。
- 不强制 `hrq_` / `run_` 前缀，不校验 ULID 格式。
- 缺失、空值、非法字符或超过 64 字符：在转发上游前返回 `400 REQUEST_INVALID`，不产生费用和 B1/B3 事件。
- V2.3 中的“上限 80”应修正为 64；§6.1 的必填规则与实现一致。

**状态**：实现不变，文档已校准。

### 02 · 模型与路由契约

**结论：媒体接口和 `X-Route-Group` 需要进入双方共同版本。**

- A3 返回的是池和真实模型 ID，不是价格档位。
- `chat` 为默认池；`image`、`video`、`plan` 调用必须显式传对应 `X-Route-Group`。
- 不存在的池返回 `403 access_denied`。
- 模型 ID 不会被池路由重写；池只限制可选渠道集合。
- 图片、视频的提交、查询、下载、错误和计费字段在本版文档中单独定义。
- 同一模型可以合法出现在多个池；是否可调取决于该池内是否存在能服务该模型的启用渠道。

**状态**：文档已补充；V2.3 需由双方共同修订或引用本版手册。

### 03 · 目录与渠道

**结论：跨池重复不是错误；真实问题是目录候选与当前可调集合不一致。**

- 跨池重复本身合法，不应以“交集必须为 0”作为验收标准。
- 当前 A3 v10：`chat=26`、`video=83`、`image=10`、`audio=0`、`plan=80`。
- 2026-11-23 按上游实时目录与抽样调用初步核实：`video=49`、`plan=17` 可调。
- 当前 68 个跨池重复主要来自渠道 `models` 中的过期/跨组声明，不代表 68 个模型都在两个池真实可用。
- A3 的 `capabilities` 是池级端点类型汇总，会包含 `chat/stream/tools` 等通用标签，**不能据此判断图片/视频的真实能力**。
- Harness 在目录校正完成前，以 `12-retest-model-list-20261123.md` 为复测清单。

**状态**：文档已区分“目录声明”和“实测可调”；渠道声明校正属于后续配置工作，本版未修改生产渠道。

### 04 · 视频任务失败

**结论：失败样本主要是已下架模型或当前池内无有效供货渠道。**

- TA Seedance Token 22 档在上游当前目录中已不可用，旧渠道声明和旧文档仍可能展示这些 ID。
- 另有 7 个视频规格已下架/停用：`grok-video1.5-480p-second`、3 个旧小写 MiniMax second、`seedance2-480p-once`、`seedance2-1080p-once`、`seedance2-fast-720p-once-2`。
- 这些模型调用出现 500/503/400 属于预期拒绝，不应作为当前可上线模型。
- 可复测的 `video=49`、`plan=17` 清单见第 12 号文档。

**状态**：可复测清单已给出；旧模型需从渠道声明和 A3 中移除或禁用后，目录才能与真实供货一致。

### 05 · Nano Banana 接口与超时

**结论：正式协议为同步图片接口；长超时和 MIME 需要按已知限制处理。**

- Nano Banana 使用同步图片协议，不返回 task id；旧异步 `/v1/videos` 说明不代表当前正式协议。
- 1K/4K 长请求可能触发 524/500；客户端应使用足够长的超时，并在超时后先查 B3/产物，再决定是否重试。
- 成功样本可能出现实际 JPEG 内容被标为 PNG 的情况；保存文件时应按文件头判断真实格式，不要只信 MIME 或扩展名。
- 幂等：同步图片请求没有统一 task id，超时后不能盲目重提；先核对 B3 的 `request_id` 和产物。

**状态**：调用与重试规则已写入图片文档；服务端超时和 MIME 归一仍属于待修项。

### 06 · 异步视频账本

**结论：上游不返回 usage 的异步视频，B1 无法提供真实 token 用量。**

- 当前异步视频 B1 使用 `usage_source=unavailable`。
- token 字段为 `null`，`split_known=false`，`total_tokens` 省略；这不是“实测为 0”。
- 历史 TA Token 完成记录中的 token 数不能直接回补到当前 B1，尤其是模型已下架的情况下。
- B3 的 `settled` 和金额仍是费用依据；B1 缺失 usage 不影响 B3 结算。

**状态**：文档已明确不可伪造用量；如需历史回补，必须由上游提供可信 usage 和对应请求映射。

### 07 · 非 token 计费与用量

**结论：当前 B1/B3 单位字段仍固定为 token，是已知字段限制。**

- B1：`usage_unit=token`。
- B3：`billing_basis=token`。
- 该字段**不表示**图片按 token、视频按 token 计费；实际计费仍按模型价格/表达式执行。
- 图片按张、视频按秒/按次的金额以 B3 最新版本 `total_amount`、`status=settled` 和提交时价格版本为准。
- Nano Banana 可能回传估算 token；`usage_source=estimated` 表示估算，不等同于上游官方 usage。
- Gemini thinking tokens 是否计入 output，以当前归一化实现和 B1 实际返回为准，不另行相加。
- `pending` 或零额退款记录的价格版本可能为空，是否为有效终态需结合事件类型和后续版本链判断。

**状态**：文档已披露限制；`usage_unit` / `billing_basis` 的媒体单位修正属于后续代码工作。

### 08 · 响应字段及文档

**结论：错误体和 MIME 尚未完全统一，本版按实际实现分别描述。**

- JWT 401：OpenAI 风格 `error.message/type/code`，不保证包含 `retryable` 或 `request_id`。
- Scope 403：模型调用通常为 `access_denied`；A3/B1/B3 可能使用 `GATEWAY_UNAUTHORIZED` 并带 `retryable=false` 或 `details.required_scope`。
- 任务插件部分错误可能使用顶层 `{code,message,data}` 或插件原生错误体。
- 视频下载当前透传上游 `Content-Type`，MiniMax 等样本可能出现通用 MIME；接收端应结合文件头校验。
- A5 `count_tokens` 当前未交付：Claude 路由未注册，Gemini `:countTokens` 返回 404。
- B1/B3 增量列表 `limit` 默认 50、最大 200；超过 200 会静默 clamp 到 200，并返回 `next_cursor`。
- BatchGet 最多 100 个 request ID、合计 500 条事件；跨请求合计超过 500 返回前 500 条并标 `truncated=true`；单个请求自身超过 500 条返回 `400 EVENT_LIMIT_EXCEEDED`。
- `/api/pricing` 是通用定价展示接口，默认公开但可配置关闭/鉴权；不是 A3，也不是 B3 对账依据。

**状态**：实际行为已写入文档；统一错误体、MIME 和长期接口承诺仍需双方确认。

## 3. 已通过项（保留）

- RS256 JWT/JWKS、scope 隔离。
- A3 200、ETag 304、无 JWT 401、错误 scope 403。
- 文字 26/26（Harness 11-21）；11-23 有 3 条上游失败记录，详见可复测清单备注。
- GPT Image 4/4；Nano Banana 4/6。
- PAI 按秒两款、MiniMax 一款的提交/查询/完成/下载抽样。
- B1/B3 列表、分页、24 小时窗口、批查、权限隔离和 B4 字段。
- 请求 ID 与 B1/B3 关联。

## 4. 后续待办

| 事项 | 责任方 | 完成标准 |
| --- | --- | --- |
| 渠道 `models` 按上游真实供货校正 | 网关运维 | A3 条目与可调清单一致，已下架模型不再作为当前候选 |
| OpenResty/图片长请求超时 | 网关运维 | Nano Banana 长请求不再因代理 60 秒默认超时提前断开 |
| 图片/视频 MIME 归一 | 网关研发 | 下载响应 MIME 与真实文件类型一致，或文档明确接收端校验规则 |
| B1/B3 媒体单位字段 | 网关研发 + Harness | `usage_unit` / `billing_basis` 能表达 second/per_call/image，或双方确认保留 token 的限制 |
| 共同文档 V2.3 修订 | 双方 | Run ID、池路由、媒体接口、错误体、分页和 A5 口径一致 |
| 最终公网/内网入口 | 双方运维 | Base URL、DNS、安全组、证书续期和回退方式确认 |

## 5. 变更摘要

V1.2 相对 V1.1 的主要变化：

1. 纠正 Run ID 上限为 64。
2. 补充 `X-Route-Group` 与媒体接口契约。
3. 区分 A3 当前条目和实测可调清单。
4. 明确跨池重复合法，已下架模型禁止调用。
5. 补充 Nano Banana 同步协议、超时和 MIME 注意事项。
6. 明确异步视频 B1 `unavailable/null` 口径。
7. 披露 B1/B3 当前 `token` 单位字段限制。
8. 按实际实现拆分错误体、分页、BatchGet、A5 和 `/api/pricing` 说明。
