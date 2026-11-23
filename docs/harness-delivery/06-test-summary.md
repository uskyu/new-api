# Harness 网关对外测试摘要

**版本**：V1.2（2026-11-23 交付包版本；正文为截至 2026-11-17 的历史测试摘要，保留过程记录）\
**对应网关版本**：早期记录对应 `harness-api` `725ec9d6d`；历史末次记录为 `harness-api` `df8787085`（`harness-api-20261116-df87870`）；**当前网关为 `harness-api-20261117-b040241`（revision `b04024190`）**\
**测试时间**：2026-10-25 至 2026-10-26；图片模型验证 2026-11-14；用量来源修复复测 2026-11-15；全模型验收 2026-11-16 至 2026-11-17

> 本文是**历史证据摘要**：记录该轮使用**网关自签 JWT** 的本地联调结果，以及后续
> 图片、用量来源和全模型验收过程，按原样保留。当前反馈处理结论见
> [11-feedback-resolution-20261123.md](./11-feedback-resolution-20261123.md)，当前模型
> 范围与复测清单见
> [12-retest-model-list-20261123.md](./12-retest-model-list-20261123.md)。真实 JWT 验收
> 记录见 `07-acceptance-evidence.md`（`ec3cdc0`）；图片模型验证见第十二节
> （本地临时 JWT，非 Harness 正式 JWT）。正文历史事实不改写，当前口径以 11/12 为准。

本文档是对外测试摘要。内部完整验收报告（含详细命令、失败排查记录）保留在
New API 内部，不随交付材料提供。

---

## 一、测试分类说明

本文档严格区分四类测试状态：

| 类别 | 定义 |
| --- | --- |
| **真实上游调用** | 经真实 HTTP 请求打到真实模型供应商，验证完整链路 |
| **集成测试** | 使用**网关侧自签 JWT + 自建 JWKS**，真实数据库，真实 HTTP |
| **协议测试** | 只验证路由注册与鉴权边界，不发起上游调用 |
| **未完成** | 尚未验证 |

---

## 二、鉴权状态（重要）

### 已完成

- JWT 校验逻辑实现完整（RS256、`iss`、`aud`、`exp`、`scope`、`sub` 映射）
- 使用**网关侧自行生成**的 RSA 密钥对和**自建 JWKS 服务**做集成测试
- **已在真实部署实例上跑通完整链路**：自签 JWT 经网关校验、映射到内部令牌、
  调用真实模型、结算落库、账本事件写出，再由 B1/B3 读回
- 无效 JWT 拒绝、Scope 隔离、主体映射均已验证

### 本轮在真实部署上发现并修复的问题

此前只在本机测试中验证过账本链路，部署到真实实例后才暴露三个问题：

1. **账本事件始终为空**（MySQL 专有缺陷）。`pricing_version` 列宽 64，
   而实际值是 `pw-` 加一个带引号的 64 位摘要，共 71 字符，任何写入都失败。
   账本写入失败只记日志、不返回错误（记账失败不能影响调用，这是正确设计），
   所以外部只表现为"没有数据"，无法从接口看出报错。已加宽至 96 并加回归测试。
2. **档位名无法路由**。档位名不是上游模型名，必须同时加入渠道模型列表
   （才会生成 ability 行）并配置 `model_mapping`，缺一不可。
3. **档位名缺少定价**。定价按请求的模型名查询，因此档位名本身也需要价格行。

> 第 2、3 项记录的是当时“用档位名当模型名”的方案。该方案已废弃：现在的契约是
> `model` 填 A3 返回的真实模型 id，池由 `X-Route-Group` 选择。保留此段仅为记录
> 问题发现过程。

### 未完成（本轮回看）

本轮尚未与 Harness 真实鉴权体系联调。该联调已于 2026-10-26 与 2026-10-29
两轮完成，见 `07-acceptance-evidence.md`。

因此：

- ✅ 可以说「网关的 JWT 校验实现已通过集成测试，并在部署实例上端到端跑通」
- ❌ **不能**说「Harness 真实鉴权联调已完成」

真实联调需在拿到四项配置后进行。

---

## 三、接口测试结果

### 3.1 模型调用类

| 接口 | 测试类型 | 结果 |
| --- | --- | --- |
| A1 `POST /v1/chat/completions` | **真实上游调用** | ✅ HTTP 200，返回内容与 usage |
| A2 `POST /v1/responses` | **真实上游调用** | ✅ HTTP 200，Responses 格式 |
| A4 `POST /v1/messages` | **真实上游调用** | ✅ HTTP 200，Anthropic 格式，含 `usage.input_tokens` |
| A9 `POST /v1beta/models/{tier}:generateContent` | **真实上游调用** | ✅ HTTP 200，Gemini 格式，含 `usageMetadata` |
| 流式 `stream: true` | **真实上游调用** | ✅ 收到 32 个 SSE 事件，末尾含 usage |
| 四协议路由鉴权 | 协议测试 | ✅ 未认证时均返回 401（非 404，证明路由已注册） |

**说明**：A2/A4/A9 通过上游的 OpenAI 兼容层转换。响应格式符合各自原生协议。

### 3.2 目录类

| 接口 | 测试类型 | 结果 |
| --- | --- | --- |
| A3 `GET /v1/models` 首次 | 集成测试 | ✅ HTTP 200，返回 ETag |
| A3 条件查询 `If-None-Match` | 集成测试 | ✅ HTTP 304，无响应体 |
| A3 缺 `gateway.catalog.read` scope | 集成测试 | ✅ HTTP 403 |
| A3 未签名调用 | 集成测试 | ✅ HTTP 401 |

### 3.3 账本类

| 接口 | 测试类型 | 结果 |
| --- | --- | --- |
| B4 `GET /health` | **部署实例验证** | ✅ HTTP 200，格式与文档一致 |
| B1 `GET /usage-events` | **部署实例验证** | ✅ HTTP 200，返回真实用量事件 |
| B3 `GET /cost-records` | **部署实例验证** | ✅ HTTP 200，金额为字符串 |
| B1 通过 JWT 调用写入账本 | **部署实例验证** | ✅ `usage` + `cost` 两条事件均写出，`record_version=1` |
| B1-batchGet 缺失 ID | **部署实例验证** | ✅ 进入 `missing_request_ids`，整体不失败 |
| B1-batchGet 命中 | 集成测试 | ✅ `found: true` + `events` |
| B1-batchGet 101 个 ID | 集成测试 | ✅ HTTP 400 `REQUEST_INVALID`，`details.max: 100` |
| B3-batchGet | **部署实例验证** | ✅ 结构对称，`records` 字段 |
| 游标跨流混用 | 集成测试 | ✅ HTTP 400 `CURSOR_INVALID` |

**说明**：「部署实例验证」指在真实运行的网关上，用自签 JWT 发起真实模型调用，
结算落库后经 B1/B3 读回同一笔请求，整条链路闭合。

---

## 四、账本语义验证

### 4.1 已通过

| 项 | 结果 |
| --- | --- |
| 首次事件 `record_version=1`、`supersedes_event_id=null` | ✅ |
| 用量更正 → `usage_correction` | ✅ |
| 金额更正 → `cost_correction` | ✅ |
| 纯状态变化 → `status_change` | ✅ |
| B1/B3 独立版本链 | ✅ |
| 完全重复结算不产生事件 | ✅ |
| 退款作首事件仍为 `cost` | ✅ |
| 金额为完整快照（非增量） | ✅ |

### 4.2 关于「amount 变化」的定义

已验证：修改采购倍率导致 `total_amount` 变化而 `base_amount` 不变时，
事件类型为 `cost_correction` 而非 `status_change`。

这一点在本轮开发中修正过一次——原实现只比较 `base_amount`，会错误输出
`status_change`。现已修复并有回归测试覆盖。

> **V0.10 变更提示**：供应商税率已取消，金额改为
> `total_amount = base_amount × group_ratio`，`tax_rate_percent` 恒 0 并废弃。
> 本节记录的是当时含税率的验证结果，未重跑；不要据此认为税额仍参与计算。

---

## 五、数据库方言

| 方言 | 版本 | 结果 |
| --- | --- | --- |
| SQLite | 内置 | ✅ 全部测试通过 |
| MySQL | 8.0.43（真实实例） | ✅ `model` 包全量通过，迁移幂等 |
| PostgreSQL | — | 不在本期范围（部署目标为 MySQL） |

### MySQL 相关说明

开发过程中在 MySQL 上发现两个真实缺陷，均已修复：

1. **结算 upsert 语法错误**。原先手写 `excluded.col`（PostgreSQL 专属语法），
   MySQL 报 `Unknown column`，导致所有结算写入失败。已改为 GORM 方言渲染。
2. **`pricing_version` 列宽不足**。列宽 64，实际值 71 字符，MySQL 报
   `Data too long for column`，导致所有账本事件写入失败。已加宽至 96。

这两个缺陷都只在 MySQL 上暴露，SQLite 不会报错（不检查 varchar 宽度），
因此本机测试无法发现。第二个还在部署后才暴露，因为账本写入失败只记日志。

已补充回归测试：对比生成值的长度与从 schema 读出的列宽，防止再次回归。

---

## 六、权限与拒绝

| 场景 | 响应 | 结果 |
| --- | --- | --- |
| 无 Authorization 头 | 403 `GATEWAY_UNAUTHORIZED` | ✅ |
| 缺指定 scope | 403 + `details.required_scope` | ✅ |
| 无效 JWT | 401 | ✅ |
| 无效模型令牌 | 401 | ✅ |
| 不存在的模型 | 4xx/5xx | ✅ |

---

## 七、未完成事项

| 项 | 状态 | 说明 |
| --- | --- | --- |
| **Harness 真实 JWKS 联调** | ✅ 已完成 | 2026-10-26 起使用真实 JWT，见 `07-acceptance-evidence.md` |
| Docker 镜像 | ✅ 已完成 | `ghcr.io/uskyu/new-api:harness-latest` 已构建并在部署实例上运行 |
| 500 事件上限触发 | ⚠️ 未触发 | 测试数据量不足，逻辑已实现 |
| A5 count_tokens | — | 本期不做 |
| B2 单次详情 | — | 本期取消 |

---

## 八、测试环境

| 项 | 说明 |
| --- | --- |
| 网关版本 | `725ec9d6d`（镜像 `harness-api-20261026-725ec9d`） |
| 上游 | 真实模型供应商（地址与凭证不对外提供） |
| 数据库 | MySQL 8.0.43（部署实例）/ SQLite（单元测试） |
| JWT | **网关侧自行签发**（非 Harness 真实签发） |

---

## 九、结论

### 已具备对接条件

- 全部 10 个接口路径、方法、Scope 已实现并验证
- 请求/响应格式与文档一致
- 错误码与拒绝行为明确
- 账本事件语义完整
- **在真实部署实例上端到端跑通**：JWT 鉴权 → 模型调用 → 结算落库 → 账本事件 → B1/B3 读回
- Docker 镜像可直接部署

### 尚需完成才能进入正式验收

1. 双方确认剩余三项契约（见 `05-pending-confirmations.md`）
2. 按 `04-acceptance-checklist.md` 逐项执行验收
3. 使用 Harness 正式 JWT 重跑图片 E2E（当前仅本地临时 JWT，见第十二节）

真实 JWKS 联调已完成（2026-10-26、2026-10-29 两轮）；
数据库部署目标为 MySQL，PostgreSQL 不在本期范围。

---

## 十、变更记录

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版 |
| 2026-10-26 | V0.9 | 补入部署实例端到端验证结果；记录 `pricing_version` 列宽缺陷及修复 |
| 2026-11-05 | V0.10 | 版本对齐交付包；标注税率已取消（历史结果未重跑） |
| 2026-11-14 | V0.11 | 补入图片模型验证：`supplyvideo` 1.3.0 单测、A3 图片池 10 模型、Nano Banana 2.1 1K 与 GPT Image 2.6-flare E2E；说明 B1 无 `status`、Nano Banana 0 Token（修复前口径）、GPT Image 可带 Token，完成以 B3 `settled` 为准；本地临时 JWT，真实 Harness JWT 待验 |
| 2026-11-15 | V0.11（修订） | 修复异步任务用量来源：无 Token 用量时 B1 `unavailable`/`null`，仅有 `total_tokens` 时 `upstream`；撤销 B1 结算收尾事件。部署 `harness-api-20261115-de7fce8`，Nano Banana / GPT Image / PAI 三模型复测通过（第十三节） |
| 2026-11-23 | V1.2 | 交付包版本对齐；页首补充当前网关 `harness-api-20261117-b040241`，链接第 11、12 号文档；本文保留为历史证据摘要，历史事实不改写 |

---

## 十一、RC48 合入与部署验证（2026-11-07 / 11-08）

本节记录 RC48 候选的本地代码验证与合入部署结果；部署健康检查已完成，不代表真实供应商联调验收。

| 检查 | 结果 |
| --- | --- |
| Go 版本 | `go1.26.10`，与 RC48 基线一致 |
| `go vet ./...` | ✅ 根模块、`relaykit`、`tokenkit` 全部通过 |
| `make test` | ✅ 根模块、`relaykit`、`tokenkit` 全部通过 |
| Web 类型检查 | ✅ 1,422 个文件通过 |
| Web 测试 | ✅ 203 个测试文件、2,470 个测试通过 |
| 审计日志排序索引迁移 | ✅ SQLite、MySQL 8.0.43、PostgreSQL 17.5；新建和升级路径均验证幂等、保留数据与原索引、排序和唯一约束 |
| Harness 账本与结算 | ✅ SQLite、MySQL 8.0.43、PostgreSQL 17.5；包含更正链、分页、批量读取与结算写入 |
| 路由 scope | ✅ A1/A2/A4/A9、图片/视频生成、Responses/视频任务读取与内容读取均拒绝缺少 `gateway.chat` 的 Harness JWT |
| MySQL 本地复核 | ✅ 本机 MySQL 8.0.43：Harness 账本/结算、任务结算、价格/模型/审计矩阵通过；本地完整启动/迁移通过 |
| 部署 | ✅ 82 实例 `new-api` 容器 healthy，版本 `harness-api-20261108-bb26602`，`/api/status` 成功 |

验证中发现并修复了 PostgreSQL 结算 upsert 对 Harness 关联 ID 的歧义引用，以及流式 keepalive 已发送后错误事件被提前丢弃的问题。测试使用本地模拟服务和隔离测试数据库；没有生产 JWT 或真实供应商请求，部署后也未执行线上 E2E。

GitHub 的 `Package harness-api` 工作流生成本轮多平台下载包；`Publish harness image` 发布并部署了 `harness-api-20261108-bb26602`。

---

## 十二、图片模型（2026-11-14）

本轮在 82 实例 `harness-api-20261114-c29f1a6` 上验证图片生成链路。镜像 digest
`sha256:20104c9253c254fc90c5d36ac37f738a7f50596c8845615a8ce1dd37dbf4b7b0`，
容器 healthy，`/api/status` 返回版本 `harness-api-20261114-c29f1a6`。

| 检查 | 结果 |
| --- | --- |
| `supplyvideo` 1.3.0 单元测试 | ✅ 通过 |
| A3 图片池 | ✅ `catalog_version="7"`，10 个模型 |
| Nano Banana 2.1 1K E2E | ✅ 创建 200、轮询 completed、下载 468,870 bytes（实际 JPEG）；B3 `0.009` USD `settled` |
| GPT Image 2.6-flare E2E | ✅ HTTP 200；B3 `0.018` USD `settled`；B1 99 input / 7,028 output tokens |
| B1 语义（修复前记录） | ✅ 无 `status` 字段；Nano Banana 异步任务 Token 为 0，结算后可能追加 `usage_correction`；GPT Image 同步可带上游 Token（本例 99/7,028） |
| B3 语义 | ✅ 成功图片任务最终为 `status_change`，`status=settled` |

> **当前口径（2026-11-15 修复后）**：异步任务没有 Token 用量时，B1 为
> `usage_source=unavailable`，`input_tokens` / `output_tokens` /
> `cache_read_tokens` / `cache_write_tokens` 均为 `null`，`split_known=false`，
> `total_tokens` 省略；只拿到 `total_tokens` 时为 `usage_source=upstream`、
> `total_tokens=N`、四个分项为 `null`、`split_known=false`。B1 只在用量或用量
> 元数据变化时追加事件，结算不再追加同值 `usage_correction`。GPT Image 同步保持
> `usage_source=upstream` 与上游实际 input/output Token。本节 2026-11-14 的数字
> 与结果是修复前事实，未改写。

A3 图片池 10 个模型（按返回顺序）：`Nano Banana 2.1 1K`、`Nano Banana 2.1 2K`、
`Nano Banana 2.1 4K`、`Nano Banana Pro 1K`、`Nano Banana Pro 2K`、
`Nano Banana Pro 4K`、`gpt-image-2.5-flare`、`gpt-image-2.5-sunburst`、
`gpt-image-2.6-flare`、`gpt-image-2.6-sunburst`。协议与样例见
[`09-image-api-examples.md`](./09-image-api-examples.md)。

2026-11-14 修复前的图片任务 B1/B3 口径：B1 只有用量事件、没有 `status`。Nano Banana
异步任务 Token 为 0，可能先 `usage` 后 `usage_correction`；GPT Image 同步可能带上游
Token（本例 99/7,028）。两者都不能用 B1 判断完成；B3 的 `status_change` /
`status=settled` 才是完成信号，`billing.total_amount` 为按张价格（`0.009` /
`0.011` / `0.016` / `0.018` USD）。`image_count` 为 `n=1` 并用于按张计费，
不在 B1 暴露。

**JWT 限制**：本轮使用本地临时铸造的测试 JWT 和临时 JWKS，**不是 Harness 正式
签发的 JWT**。本节不构成正式 JWT 验收结论；真实 Harness JWT 四模型复测见第十三节。

---

## 十三、异步任务用量来源修复（2026-11-15）

本轮修复异步任务在无 Token 用量时被误标为 `usage_source=upstream`、0 Token 的问题，
并撤销结算时的 B1 强制收尾事件。

| 检查 | 结果 |
| --- | --- |
| 任务路径单测 | ✅ Nano Banana / PAI 无 Token：B1 1 条 `unavailable`，四个 Token 字段 `null`，`split_known=false`；B3 仍为 pending → settled |
| total-only 单测 | ✅ settle 传 `5000`：B1 第 2 条 `usage_correction`，`upstream` + `total_tokens=5000` + 分项 `null`；重复无 usage settle 不覆盖总量 |
| B1/B3 独立性 | ✅ 无用量变化的 settle 不再追加 B1；B3 `status_change` 仍为完成信号 |
| 部署 | ✅ `harness-api-20261115-de7fce8`，digest `sha256:b1764aa141089f738919839e908e9289643a6d74ed9d2f32234644fce0fe0e5f`，82 healthy |
| Nano Banana 2.1 1K E2E | ✅ B1 1 条 `unavailable`/null；B3 `0.009` pending → settled |
| GPT Image 2.6-flare E2E | ✅ B1 `upstream` 98/7,028；B3 `0.018` settled |
| PAI 按次 E2E | ✅ B1 1 条 `unavailable`/null；B3 `0.52` pending → settled |
| PAI 按秒 E2E | ✅ B1 1 条 `unavailable`/null；B3 `0.16` pending → settled |
| 真实 Harness JWT E2E | ✅ 四模型 B1/B3 4/4 命中；GPT Image PNG 1024×1024、Nano Banana JPEG 1024×1024 完整下载与 PIL 解码通过 |
| JWKS 鉴权行为 | ✅ 正式 JWT 200；临时 JWT 401；chat-only 对 A3/B1/B3 分别 403，B1/B3 带 `required_scope` |
| JWT 限制 | 2026-11-15 已用 Harness 正式签发 JWT 复测通过；本地临时 JWT 仅用于本地调试 |

请求 ID 与完整金额核对见 `07-acceptance-evidence.md` 第十六节。

---

## 十四、全模型验收（2026-11-17）

本轮在 `harness-api-20261116-df87870` 上完成语言、图片、视频全模型实测，完整接口样例和模型清单见 [10-full-model-acceptance-20261117.md](./10-full-model-acceptance-20261117.md)。

| 类别 | 结果 |
| --- | --- |
| 语言 | 35/38 条调用记录通过；3 条为 `国模号池1` 上游 token 不服务的模型（预期 503） |
| 图片 | 10/10 通过（GPT Image 4、Nano Banana 6） |
| 视频 | 55/61 个模型通过；6 个为上游阻塞：MiniMax 4、`wan3-1080p-once`、`seedance2-720p-second-1` |
| 参考素材 | Seedance 参考图/音频、Kling 参考视频、Omni 首尾帧、HappyHorse `video-edit` 通过；MiniMax 首尾帧上游 401 |
| `invalid api platform: 60` | ✅ 已修复；18 个新增视频模型已声明并通过对应渠道提交 |
| Nano Banana 同步 | ✅ 1K/2K/4K 通过；B1 `estimated` 10 Token，B3 `0.016` settled；旧异步合同停用 |
| 部署 | ✅ `harness-api-20261116-df87870`（revision `df8787085`），82 healthy |

---

## 附：内部报告说明

内部完整验收报告包含：

- 每个测试的完整命令行与实际输出
- 缺陷发现与修复过程的详细记录
- 已知基线测试失败的复现证据
- 测试数据构造方法

这些内容涉及内部实现细节，不随对外交付材料提供。如需某项具体证据，可按需单独提供。
