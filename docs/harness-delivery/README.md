# Harness 网关接口交付包

**版本**：V0.12（2026-11-17 修订）
**对应网关版本**：`harness-api` 分支；已部署 82 实例 `harness-api-20261116-df87870`
**交付日期**：2026-10-25（2026-10-29、2026-11-05、2026-11-06、2026-11-07、2026-11-14、2026-11-15、2026-11-17 更新）

---

## 一、文件清单

| # | 文件 | 用途 | 阅读顺序 |
| --- | --- | --- | --- |
| 01 | [接入指南](./01-integration-guide.md) | 接口清单、路径、Scope、请求/响应样例、分页、错误码、调用示例 | 第 1 篇 |
| A3 | [模型列表结构更新](./A3-model-catalog-update.md) | **最新模型列表**：取消经济/企业档、四个展示分区、JSON 示例与模型调用方式 | **模型列表对接优先看** |
| 08 | [视频调用样例](./08-video-api-examples.md) | 视频池 / plan 池请求差异、真实跑通模型与轮询、内容下载示例 | **视频联调前读** |
| 09 | [图片调用样例](./09-image-api-examples.md) | 图片生成调用样例（GPT Image 同步 / Nano Banana Gemini 同步） | **图片联调前读** |
| 10 | [全模型验收报告](./10-full-model-acceptance-20261117.md) | 全模型实测结果、接口样例、上游阻塞清单 | **全模型联调优先看** |
| 02 | [鉴权配置](./02-auth-setup.md) | JWKS/iss/aud/sub 配置、JWT 校验规则、需对方提供的信息 | 第 2 篇 |
| 03 | [数据契约](./03-data-contract.md) | B1/B3 字段语义、事件类型、版本链、金额口径 | 第 3 篇 |
| 04 | [联调验收清单](./04-acceptance-checklist.md) | 逐项测试用例、预期响应、通过标准 | 第 4 篇 |
| 05 | [待确认事项](./05-pending-confirmations.md) | 三项尚未冻结的契约差异（第 4、5 项已定案） | **必读** |
| 06 | [对外测试摘要](./06-test-summary.md) | 各接口测试状态，区分真实/集成/协议/未完成 | 参考 |
| 07 | [验收证据](./07-acceptance-evidence.md) | 真实 JWT 历史记录 + 2026-11-14 本地临时 JWT 图片验收 + 2026-11-15 用量来源修复复测（含真实 Harness JWT 四模型）、请求 ID 与金额核对 | 参考 |

---

## 二、建议阅读顺序

**给 Harness 后端开发：**

1. 先读 **01 接入指南** —— 了解能调哪些接口、怎么调
2. 再读 **03 数据契约** —— 理解账本字段语义
3. 联调时对照 **04 验收清单** 逐项打勾

**给 Harness 技术负责人：**

1. 先读 **05 待确认事项** —— 三项需要你们决策的契约问题（第 4、5 项已定案）
2. 再看 **02 鉴权配置** —— 确认能提供哪些配置
3. 最后看 **06 测试摘要** —— 了解当前验证到什么程度

---

## 三、当前状态

### 已完成

| 项 | 状态 |
| --- | --- |
| 10 个接口路径实现 | ✅ |
| JWT 校验逻辑 | ✅ |
| 账本写入与事件语义 | ✅ |
| SQLite / MySQL 8.0 验证 | ✅ |
| 真实 JWT 端到端联调 | ✅ 2026-10-26、2026-10-29 两轮 |
| A3 返回资源池 + 真实模型 id | ✅ 2026-10-29 |
| A1/A2/A4/A9 文字模型调用（含流式） | ✅ 2026-10-29 |
| 图片池 10 个模型与两种图片协议 | ✅ 2026-11-14 |
| 异步任务用量来源修复（无 Token → `unavailable`，不再强制 B1 收尾） | ✅ 2026-11-15 |
| 真实 Harness JWT 四模型 E2E（含完整图片解码与 Scope 校验） | ✅ 2026-11-15 |

### V0.11 变更（2026-11-14，已构建并部署）

| 项 | 状态 |
| --- | --- |
| `image` 池配置 10 个图片模型：4 个 GPT Image（同步） + 6 个 Nano Banana（异步），可调用 | ✅ 已构建并部署 |
| 图片按张计价：GPT Image 0.018 USD/张；Nano Banana 2.1 1K/2K 0.009、2.1 4K 0.011、Pro 0.016 USD/张 | ✅ 已部署 |
| `X-Route-Group: image` 绑定图片池 | ✅ 已部署 |
| 图片任务账本语义：B1 仅 usage、无状态；B3 `status_change` → `settled` 为完成与成本依据 | ✅ 已部署 |
| 本地临时 JWT（临时 JWKS）图片 E2E：Banana 2.1 1K 0.009 USD、GPT Image 2.6-flare 0.018 USD，B3 `settled` | ✅ 2026-11-14 |
| 部署 `harness-api-20261114-c29f1a6`（digest `sha256:20104c9253c254fc90c5d36ac37f738a7f50596c8845615a8ce1dd37dbf4b7b0`），容器 healthy，`/api/status` 一致 | ✅ 2026-11-14 |
| 真实 Harness JWT 图片/视频 E2E | ✅ 2026-11-15（四模型，见 07 第十六节） |

### V0.11 修订（2026-11-15，已构建并部署）

| 项 | 状态 |
| --- | --- |
| 异步任务无 Token 用量时 B1 改为 `usage_source=unavailable`、Token 字段 `null`、`split_known=false`；已有 `total_tokens` 保持 `upstream` | ✅ 已构建并部署 |
| 撤销 B1 结算强制收尾事件；B1 只在用量或用量元数据变化时追加，B3 `status_change` 为完成信号 | ✅ 已构建并部署 |
| 任务路径回归测试：Nano Banana / PAI 无 Token、total-only、重复结算 | ✅ 通过 |
| 部署 `harness-api-20261115-de7fce8`（digest `sha256:b1764aa141089f738919839e908e9289643a6d74ed9d2f32234644fce0fe0e5f`），容器 healthy，`/api/status` 一致 | ✅ 2026-11-15 |
| 本地临时 JWT 复测：Banana/PAI B1 `unavailable`，GPT Image `upstream` 98/7,028；B3 0.009/0.018/0.52 `settled` | ✅ 2026-11-15 |
| 真实 Harness JWT 四模型复测：GPT Image / Nano Banana / PAI 按次 / PAI 按秒，B1/B3 4/4 命中；PNG/JPEG 完整下载与解码通过；正式 JWKS 下临时 JWT 401、chat-only 403 | ✅ 2026-11-15 |

### V0.12 变更（2026-11-17，已构建并部署）

| 项 | 状态 |
| --- | --- |
| Nano Banana 改为 Gemini 同步 `generateContent`；旧异步 `/v1/videos` 合同停用 | ✅ 已部署 |
| Nano Banana 6 款改为固定 `ModelPrice`：2.1 1K/2K 0.009、2.1 4K 0.011、Pro 1K/2K/4K 0.016 USD/张 | ✅ 已部署 |
| `supplyvideo` 插件 1.4.0：补 18 个视频模型 + 4 个 Seedance 变体；PAI 参考素材、HappyHorse `video-edit`、Kling 参考视频 | ✅ 已部署 |
| 全模型验收：语言 35/38、图片 10/10、视频 55/61；6 个视频模型为上游阻塞（MiniMax 4、`wan3-1080p-once`、`seedance2-720p-second-1`） | ✅ 2026-11-17 |
| 部署 `harness-api-20261116-df87870`（revision `df8787085`），容器 healthy | ✅ 2026-11-17 |
| 全模型验收报告 [10-full-model-acceptance-20261117.md](./10-full-model-acceptance-20261117.md) | ✅ |

### V0.10 变更（2026-11-05 / 11-06，已构建并部署）

| 项 | 状态 |
| --- | --- |
| 取消经济/企业档，路由完全归网关 | ✅ 分支已改 |
| 资源池改为产品线 `chat` / `video` / `image` / `audio` / `plan` | ✅ 分支已改 |
| 模型列表展示改为四个对外分区：文字模型（`chat`）/ 图片生成模型（`image`）/ 视频模型（`video`）/ plan 计划（`plan`）；分区内 `models` 平铺，不再二次分组；`audio` 为预留池，不进入这四个分区 | ✅ 已构建并部署 |
| 取消 `global_channel_order` / 全渠道优先级全局池，资源池成为唯一路由顺序 | ✅ 已构建并部署 |
| 取消 `token_group_access` / 令牌授权 | ✅ 已构建并部署 |
| `X-Route-Group` 改为按池绑定：`chat` 不传，`video` 传 `video`，`image` 传 `image`（V0.11 起），`plan` 传 `plan`；仅经验证的 Harness 调用方可使用 | ✅ 已构建并部署 |
| `pricing_group_order` 保留为采购分组资格与倍率来源，不是渠道路由顺序 | ✅ 保留；当前仅 `["default"]`，倍率 1 |
| 取消供应商税率，`total_amount = base_amount × group_ratio` | ✅ 分支已改 |
| `tax_rate_percent` 保留为恒 0 的废弃字段 | ✅ 分支已改 |
| RC48 本地验证 | ✅ Go、Web 与 SQLite/MySQL 8.0.43 真库检查通过；MySQL 启动、迁移、账本与结算通过 |
| 下载包构建 | ✅ GitHub `Package harness-api`（run `42448454071`）生成多平台产物 |
| 部署与真实端到端复验 | ✅ 已部署 `harness-api-20261108-bb26602`，容器 healthy；真实 Harness JWT / 供应商 E2E 待执行 |

> 上述状态是 2026-11-06 时 `harness-api` 分支的记录。RC48 候选已合入
> `harness-api`，并于 2026-11-08 部署到 82 实例（`harness-api-20261108-bb26602`）；
> 真实 Harness JWT 与模型供应商的端到端复验仍未执行。`07-acceptance-evidence.md`
> 中的历史验收记录都属于 V0.9.1 构建（`fd00dae`），不能用来证明 V0.10 的线上行为。

### 已由 Harness 提供

| 项 | 状态 |
| --- | --- |
| JWKS URL | ✅ 测试环境已提供 |
| `iss` / `aud` / `sub` | ✅ `harness` / `new-api` / `harness-gateway` |
| 测试 JWT | ✅ 20 天有效期（至 2026-11-18） |

### 待双方确认

见 **05 待确认事项**，当前剩三项：

1. `usage` 字段是否允许 `null`
2. 是否接受新增 `total_tokens` / `split_known`
3. `refunded` 时 `total_amount` 可否非零

第 4 项（缺失关联头）已定案为 400 拒绝，网关已实施；第 5 项（供应商税率）已定案
为取消，分支按此调整（待验证）。两项均不再等待答复。

### 未完成

| 项 | 影响 |
| --- | --- |
| 更正链（`usage_correction` / `cost_correction` / `refund`） | 需构造用量变化、金额更正或退款场景 |
| BatchGet 500 事件上限 | 数据量不足 |
| `audio` 池渠道 | 渠道未接入，当前返回空模型列表，**不可调用** |
| `video` / `plan` 池 | 已配置渠道与模型：`video` 8 个 PAI 按次模型，`plan` 22 个 TA token 模型；可调用性以实际接口为准 |

图片池已配置并完成本地 JWT 端到端验证：Banana 2.1 1K（`hrq_banana_e2e_20261114_3`，
0.009 USD，下载 468,870 字节 JPEG）与 GPT Image 2.6-flare（`hrq_img26_e2e_20261114_1`，
0.018 USD）均在 B3 结算为 `settled`。该验证使用本地临时签发的测试 JWT 与临时 JWKS，
**不是 Harness 正式签发的 JWT**；2026-11-15 已使用 Harness 正式签发的 JWT 完成四模型
复测，见 `07-acceptance-evidence.md` 第十六节。

**Docker 镜像**：82 实例当前运行版本为
`harness-api-20261116-df87870`（revision `df8787085`），并通过容器健康检查与
`/api/status` 探测。该构建包含 V0.12 的 Nano Banana Gemini 同步、`supplyvideo`
1.4.0 全模型声明、PAI 参考素材、HappyHorse `video-edit` 与 Kling 参考视频修复；
全模型验收见 [10-full-model-acceptance-20261117.md](./10-full-model-acceptance-20261117.md)。
数据库部署目标为 MySQL，PostgreSQL 不在生产范围。

### RC48 合入与部署（2026-11-07 / 11-08）

| 项 | 结果 |
| --- | --- |
| 官方基线 | `QuantumNous/new-api` `v1.0.0-rc.48`，commit `706144769e5bed5a47b78721e3bb11da3cf4c0b9`；对比 RC44 共 122 个上游提交 |
| 工作分支 | `harness-api-codex`，从 `harness-api` 创建，已 fast-forward 合入 `harness-api`；保留 Harness 的 A3 目录分流、内部账本、智能路由池与 `supplyvideo` 插件 |
| Go 检查 | 根模块、`relaykit`、`tokenkit` 的 `go vet` 与全量 `make test` 通过 |
| Web 检查 | 类型检查通过；203 个测试文件中的 2,470 个测试通过 |
| 数据库检查 | 审计排序索引与 Harness 账本/结算用例在 SQLite、MySQL 8.0.43、PostgreSQL 17.5 通过（生产仅使用 MySQL）；本地 MySQL 8.0.43 完整启动/迁移与 82 实例 MySQL 8.2 启动迁移复核通过 |
| GitHub 打包 | ✅ `Package harness-api`（run `42448454071`）生成多平台下载包；`Publish harness image` 发布 `harness-api-20261108-bb26602` |
| 部署 | ✅ 已部署 82 实例，容器 healthy，版本 `harness-api-20261108-bb26602`，`/api/status` 成功 |
| 真实联调 | ⏳ 未执行；本轮不使用生产凭据，也不调用真实上游模型 |

---

## 四、快速开始

拿到材料后最小验证路径：

**第 1 步**：确认配置能提供

参照 `02-auth-setup.md` 第二节，确认能否提供 JWKS URL、`iss`、`aud`、`sub`。

**第 2 步**：确认接口清单

参照 `01-integration-guide.md` 第一节，确认接口路径与 Scope 与预期一致。

**第 3 步**：提出确认意见

参照 `05-pending-confirmations.md`，对剩余三项待确认事项给出明确结论。

**第 4 步**：准备联调

配置到位后，按 `04-acceptance-checklist.md` 从 P0 项开始逐项验证。

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

**B2 本期取消，未实现。**

---

## 六、联系与反馈

对本交付包的任何疑问，请按以下分类反馈：

| 类型 | 说明 |
| --- | --- |
| 契约疑问 | 字段语义、事件规则不清晰 |
| 待确认项答复 | 对 `05-pending-confirmations.md` 剩余三项的结论 |
| 缺陷报告 | 实际调用与文档不符 |
| 配置交付 | JWKS URL 等四项配置 |

---

## 七、版本说明

本文档为 **V0.11（2026-11-15 修订）**。V0.9.1 的结论仍然成立：接口已可对接，真实 JWT
端到端联调通过，文字模型的 A1/A2/A4/A9 与流式调用已验证，A3 可直接驱动模型选择。
V0.10 在此之上调整了产品口径，已完成构建并部署到 82 实例，但尚未完成真实 Harness 调用验收。
V0.11 补齐图片池：`image` 池配置 10 个图片模型（4 个 GPT Image 同步 + 6 个 Nano Banana
异步），按张计价，图片调用样例见 `09-image-api-examples.md`。2026-11-15 修订修复了
异步任务的 B1 用量来源：无 Token 用量时输出 `unavailable`/`null`，已有 `total_tokens`
时保持 `upstream`，并撤销 B1 结算收尾事件；B3 金额与按张计费不变。真实 Harness JWT
四模型复测已于 2026-11-15 通过。

**尚有三项契约待确认**（见 05），另有两类场景未覆盖（更正链、500 事件上限）。
三项定稿后发布 V1.0 正式版。

> 2026-10-29 更新要点：`model` 改为 A3 返回的真实模型 id；A3 增加
> `display_name`/`kind`/`billing`/`models`；`X-Harness-Run-Id` 改为必填且两个
> 关联头均 ≤64 字符；缺失关联头由“接受但不记账”改为 400 拒绝。

> 2026-11-05 更新要点（V0.10）：取消经济/企业档，路由完全由网关决定，Harness
> 只调用一个模型 id；资源池改为产品线 `chat`（默认，text）/ `video`（video，
> 无池级计费）/ `audio`（audio，占位）/ `plan`（video，`per_call`，畅用计划）；
> `X-Route-Group` 改为可选且仅 `plan` 有意义；取消供应商税率，
> `total_amount = base_amount × group_ratio`，`tax_rate_percent` 保留为恒 0 的
> 废弃字段，后续版本移除；2026-11-05 时 `video` / `audio` 渠道未接入，其
> `models` 为空属预期，不可调用。本轮构建已发布并部署（2026-11-08），真实 Harness 调用验收待执行。
> 2026-11-08 起 `video` 池配置 8 个 PAI 按次模型，`plan` 池配置 22 个 TA token 模型；
> 当日 `image` / `audio` 仍为空（2026-11-14 起 `image` 池已配置 10 个图片模型）。
>
> 2026-11-06 更新要点（仍为 V0.10）：路由进一步简化。取消
> `global_channel_order` / 「全渠道优先级」全局池，资源池是唯一路由顺序；取消
> `token_group_access` / 「令牌授权」，Harness 不再需要按池授权；`X-Route-Group`
> 仅对通过 Harness JWT 校验的调用方生效，且当时只有 `plan` 有业务含义；V0.11 起
> `image` 也使用该头。普通 `chat`/`video`/`audio` 调用不传该头，落默认 `chat` 池。`pricing_group_order`
> 保留，但它属于采购分组资格与 `group_ratio` 来源，**不是**渠道路由顺序；当前仅
> `default` 一个分组，配置为 `["default"]`，倍率为 1。上述两项移除已构建并部署
> （2026-11-08），真实调用验收待执行。
>
> 2026-11-14 更新要点（V0.11）：`image` 池配置 10 个图片模型（4 个 GPT Image 同步
> `POST /v1/images/generations`，0.018 USD/张；6 个 Nano Banana 异步
> `POST /v1/videos`，2.1 1K/2K 0.009、2.1 4K 0.011、Pro 0.016 USD/张），统一传
> `X-Route-Group: image`，样例见 `09-image-api-examples.md`。图片任务 B1 仅 usage、
> 无状态字段，完成与成本以 B3 `status_change` → `settled` 为准。已部署
> `harness-api-20261114-c29f1a6`，容器 healthy，`/api/status` 版本一致；图片 E2E
> 使用本地临时 JWT；真实 Harness JWT 四模型复测已于 2026-11-15 完成。
>
> 2026-11-15 更新要点（V0.11 修订）：异步任务没有 Token 用量时，B1 改为
> `usage_source=unavailable`、四个 Token 字段 `null`、`split_known=false`，
> `total_tokens` 省略；已有 `total_tokens` 的任务保持 `upstream` + total-only。
> 撤销结算时强制追加同值 `usage_correction` 的逻辑，B1 只在用量或用量元数据变化时
> 追加，B3 `status_change` 仍是完成信号。已部署 `harness-api-20261115-de7fce8`，
> Nano Banana / PAI 按次 / PAI 按秒 / GPT Image 四模型复测通过；真实 Harness JWT
> 在正式 JWKS 下通过，本地临时 JWT 被拒绝；B3 金额与按张计费不变。
