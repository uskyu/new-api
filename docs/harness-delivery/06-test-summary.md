# Harness 网关对外测试摘要

**版本**：V0.9
**对应网关版本**：`harness-api` 分支 `bfec3fe2e`
**测试时间**：2026-10-25

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
- 无效 JWT 拒绝、Scope 隔离、主体映射均已验证

### 未完成

**未与 Harness 真实鉴权体系联调。**

当前所有 JWT 测试的签发方是网关测试代码，不是 Harness 的真实签发服务。
真实的 JWKS URL、`iss`、`aud`、服务主体 `sub` 均**尚未提供**。

因此：

- ✅ 可以说「网关的 JWT 校验实现已通过集成测试」
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
| B4 `GET /health` | 集成测试 | ✅ HTTP 200，格式与文档一致 |
| B1 `GET /usage-events` | 集成测试 | ✅ HTTP 200，游标翻页正常 |
| B3 `GET /cost-records` | 集成测试 | ✅ HTTP 200，金额为字符串 |
| B1-batchGet 命中 | 集成测试 | ✅ `found: true` + `events` |
| B1-batchGet 缺失 ID | 集成测试 | ✅ 进入 `missing_request_ids`，整体不失败 |
| B1-batchGet 101 个 ID | 集成测试 | ✅ HTTP 400 `REQUEST_INVALID`，`details.max: 100` |
| B3-batchGet | 集成测试 | ✅ 结构对称，`records` 字段 |
| 游标跨流混用 | 集成测试 | ✅ HTTP 400 `CURSOR_INVALID` |

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

已验证：修改采购倍率或税率导致 `total_amount` 变化而 `base_amount` 不变时，
事件类型为 `cost_correction` 而非 `status_change`。

这一点在本轮开发中修正过一次——原实现只比较 `base_amount`，会错误输出
`status_change`。现已修复并有回归测试覆盖。

---

## 五、数据库方言

| 方言 | 版本 | 结果 |
| --- | --- | --- |
| SQLite | 内置 | ✅ 全部测试通过 |
| MySQL | 8.0.43（真实实例） | ✅ `model` 包全量通过，迁移幂等 |
| **PostgreSQL** | — | ❌ **未验证**（无可用实例） |

### PostgreSQL 说明

`excluded.` 语法是 PostgreSQL 原生形式，当前实现修复后仍生成该形式，
**理论兼容但未经实测**。

**部署到 PostgreSQL 前必须补验。** 本文档不声称 PostgreSQL 已验证。

### MySQL 相关说明

开发过程中发现一个真实缺陷：结算 upsert 原先手写 `excluded.col`（PostgreSQL
专属语法），MySQL 报 `Unknown column`，导致所有结算写入失败。已修复为 GORM
方言渲染，并在真实 MySQL 8.0.43 上验证通过。

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
| **Harness 真实 JWKS 联调** | ❌ 未开始 | 等待对方提供 JWKS URL、`iss`、`aud`、`sub` |
| **PostgreSQL 实测** | ❌ 未开始 | 无可用实例 |
| Docker 镜像构建 | ❌ 暂缓 | 部署准备，非接口交付阻塞项 |
| 500 事件上限触发 | ⚠️ 未触发 | 测试数据量不足，逻辑已实现 |
| A5 count_tokens | — | 本期不做 |
| B2 单次详情 | — | 本期取消 |

---

## 八、测试环境

| 项 | 说明 |
| --- | --- |
| 网关版本 | `bfec3fe2e` |
| 上游 | 真实模型供应商（地址与凭证不对外提供） |
| 数据库 | SQLite（默认）/ MySQL 8.0.43（方言验证） |
| JWT | **网关侧自行签发**（非 Harness 真实签发） |

---

## 九、结论

### 已具备对接条件

- 全部 10 个接口路径、方法、Scope 已实现并验证
- 请求/响应格式与文档一致
- 错误码与拒绝行为明确
- 账本事件语义完整

### 尚需完成才能进入正式验收

1. Harness 提供 JWKS 与主体配置，完成真实鉴权联调
2. 双方确认四项待确认契约（见 `05-pending-confirmations.md`）
3. PostgreSQL 验证（若部署使用 PG）
4. 按 `04-acceptance-checklist.md` 逐项执行验收

---

## 十、变更记录

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版 |

---

## 附：内部报告说明

内部完整验收报告包含：

- 每个测试的完整命令行与实际输出
- 缺陷发现与修复过程的详细记录
- 已知基线测试失败的复现证据
- 测试数据构造方法

这些内容涉及内部实现细节，不随对外交付材料提供。如需某项具体证据，可按需单独提供。
