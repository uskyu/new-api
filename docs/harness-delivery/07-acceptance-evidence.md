# Harness 联调验收结果（真实 JWT）

**交付包版本**：V0.10（下列证据对应 V0.9.1 构建；V0.10 变更见第十二节，未验证）
**日期**：2026-10-26（2026-10-29 复测）
**网关版本**：复测记录为 `ec3cdc0`（版本头 `harness-api-20261029-ec3cdc0`）；
V0.9.1 交付构建为 `fd00dae`（含 run id 校验补丁）；V0.10 为本轮构建（待发布）
**JWT 来源**：Harness 正式签发（JWKS `https://alphapro-test.xyz/.well-known/jwks.json`）
**Token 有效期**：至 2026-11-18

> 第一至十一节是 V0.9.1 构建的历史实测记录。其中出现的 `economy` / `enterprise`
> 池 id、按池变化的税率（税 3% / 10%）以及含税额的 `total_amount` 都属于旧口径；
> V0.10 已取消经济/企业档与供应商税率（见第十二节）。这些段落按原样保留，
> 不再代表当前行为。

---

## 一、鉴权

| 项 | 结果 |
| --- | --- |
| RS256 签名验证 | ✅ 通过 Harness JWKS |
| `iss=harness` | ✅ |
| `aud=new-api` | ✅ |
| `sub=harness-gateway` → 内部令牌映射 | ✅ |
| 无效 JWT | ✅ 401 `GATEWAY_UNAUTHORIZED` |
| 无 Authorization | ✅ 401 |

---

## 二、模型调用（全权限 Token）

| 接口 | 状态 | 用量 | 账本入账 |
| --- | --- | --- | --- |
| A1 `POST /v1/chat/completions` | ✅ 200 | 37/16 | ✅ |
| A2 `POST /v1/responses` | ✅ 200 | 32/12 | ✅ |
| A4 `POST /v1/messages` | ✅ 200 | 32/11 | ✅ |
| A9 `POST /v1beta/models/{tier}:generateContent` | ✅ 200 | 32/13 | ✅ |
| 流式 `stream:true` | ✅ 200，38 个 SSE 事件，含 `[DONE]` | 36/36 | ✅ |

**流式说明**：流式与非流式共用同一条结算路径（`PostTextConsumeQuota`），
实测确认流式请求同样产生完整的 B1/B3 事件。

---

## 三、目录 A3

| 项 | 结果 |
| --- | --- |
| 首次读取 | ✅ 200，`object=list`，4 个资源池（2 文字 + 2 视频） |
| 池内模型 | ✅ `economy` / `enterprise` 各返回 `deepseek-v4-pro`、`deepseek-v4.1-flash` |
| 视频池 | ✅ `models: []`，按最终结构返回，暂无可调模型 |
| ETag | ✅ `W/"e10bdade..."` |
| 条件查询 `If-None-Match` | ✅ 304，响应体为空 |
| 不暴露 | ✅ 无渠道、上游厂商、采购价 |

---

## 四、账本读取

| 接口 | 结果 |
| --- | --- |
| B4 `GET /health` | ✅ 200，`interface_version=v2` |
| B1 `GET /usage-events` | ✅ 200，分页正常 |
| B3 `GET /cost-records` | ✅ 200，金额为字符串 |
| B1-batchGet | ✅ 全部 `found=true`，`missing_request_ids=[]`，`truncated=false` |
| B3-batchGet | ✅ 同上 |

---

## 五、账本关联

四个协议 + 流式的请求 ID 全部可查回，每条恰好 1 条 `ue_` + 1 条 `ce_` 事件，
均为 `record_version=1`、`supersedes_event_id=null`。

```
a1     hrq_a13007207942z97yjkw3x5
a2     hrq_a230072099023w4y1xr1ao
a4     hrq_a430072118276qvkzd10xb
a9     hrq_a93007213686yseh82qh5y
stream hrq_stream_acceptance_01
```

---

## 六、金额核对

公式：`base_amount = (p + 2c) / 2 / 500000`

| 请求 | tokens | 计算值 | 账本值 | 结果 |
| --- | --- | --- | --- | --- |
| a1 | 37/16 | 0.000069 | 0.000069 | ✅ |
| a2 | 32/12 | 0.000056 | 0.000056 | ✅ |
| a4 | 32/11 | 0.000054 | 0.000054 | ✅ |
| a9 | 32/13 | 0.000058 | 0.000058 | ✅ |
| stream | 36/36 | 0.000108 | 0.000108 | ✅ |

`group_ratio=1`、`tax_rate_percent=0`，因此 `total_amount = base_amount`。

**采购成本与客户售价解耦已验证**：账本金额基于 `PreGroupQuota`（客户分组倍率
生效前），因此调整客户分组倍率不会影响采购成本。

---

## 七、权限隔离（仅 `gateway.chat`）

| 接口 | 状态 | code | required_scope |
| --- | --- | --- | --- |
| A3 `/v1/models` | ✅ 403 | `GATEWAY_UNAUTHORIZED` | 无 `details`（契约定义） |
| B1 `/usage-events` | ✅ 403 | `GATEWAY_UNAUTHORIZED` | `gateway.usage.read` |
| B3 `/cost-records` | ✅ 403 | `GATEWAY_UNAUTHORIZED` | `gateway.cost.read` |
| B1-batchGet | ✅ 403 | `GATEWAY_UNAUTHORIZED` | `gateway.usage.read` |
| B3-batchGet | ✅ 403 | `GATEWAY_UNAUTHORIZED` | `gateway.cost.read` |
| B4 `/health` | ✅ 200 | — | 不校验 scope |
| A1 chat | ✅ 200 | — | 有 `gateway.chat` |

**A3 与 B1/B3 的错误体结构不同**：A3 只返回 `code` + `message` + `retryable`，
B1/B3 另含 `details.required_scope`。这是当前契约定义，不是缺陷。

---

## 八、分页与错误处理

| 场景 | 结果 |
| --- | --- |
| 游标翻页 | ✅ 两页不重复 |
| 游标跨流混用 | ✅ 400 `CURSOR_INVALID` |
| BatchGet 101 个 ID | ✅ 400 `REQUEST_INVALID` |
| 时间窗 > 24h | ✅ 400 `WINDOW_TOO_LARGE` |

---

## 九、本轮未覆盖

| 项 | 说明 |
| --- | --- |
| 事件更正链（`usage_correction` / `cost_correction` / `status_change`） | 需构造重复结算或退款场景，本轮未触发 |
| BatchGet 500 事件上限 | 数据量不足 |
| `gateway.health.read` 语义 | 待对方确认 |
| 视频池模型调用 | 两个视频池当前 `models: []` |

---

## 十、2026-10-29 复测（`ec3cdc0`）

使用 Harness 续发的 20 天 JWT，在 `https://testapi.soloai.chat` 上复测：

| 项 | 结果 |
| --- | --- |
| 版本确认 | ✅ 响应头 `harness-api-20261029-ec3cdc0` |
| A3 目录 | ✅ 200，4 个池，`catalog_version=2`，304 条件请求正常 |
| A1 非流式 | ✅ 200，默认池 → 渠道 61，税 3% |
| A1 流式 | ✅ 200，`X-Route-Group: enterprise` → 渠道 62，税 10% |
| A2 / A4 / A9 | ✅ 200，账本入账 |
| B1/B3 batchGet | ✅ `ue_` / `ce_` 成对返回 |
| 金额复算 | ✅ 与 `(p + 2c) / 2 / 500000` 一致 |
| 缺失 `X-Harness-Run-Id` | ✅ 400 `REQUEST_INVALID` |
| 未知资源池 | ✅ 403 `access_denied` |

### 本轮发现并修复的问题

**run id 长度上限与账本列宽不一致。** 校验允许 80 字符，账本列是
`varchar(64)`。用 70 字符的 run id 实测：调用返回 200，但 B1/B3 都查不到该
请求——MySQL 严格模式写入报 `data too long`，而账本写入失败只记服务端日志、
不返回给调用方，结果是一笔已计费却无法对账的请求。

修复：校验上限对齐列宽为 64，并加回归测试防止再次漂移。补丁已包含在构建
`fd00dae` 中（`ghcr.io/uskyu/new-api:harness-api-20261029-fd00dae`）。
无论是否已切换到该构建，调用方都应保证 run id ≤64 字符（`run_<ULID>` 为 30 字符）。

### 延迟现象

同样的 16 token 小请求，实测耗时在 2.7 秒到 100 秒以上波动。非流式请求在
上游返回前不发送任何响应头，客户端超时后网关可能仍完成结算。对账应以 B1/B3
账本为准，不能以“客户端超时”判定未计费。

---

## 十一、部署后复验（2026-10-30，`fd00dae`）

| 项 | 结果 |
| --- | --- |
| 运行构建 | ✅ 版本头 `harness-api-20261029-fd00dae` |
| 真实 JWT 拉 A3 | ✅ 200，JWKS 配置未改动 |
| 70 字符 run id | ✅ 400 `REQUEST_INVALID`，B1/B3 均无记录 |
| 正常调用 | ✅ 200，7.5 秒，32/8 tokens |
| B1 用量事件 | ✅ `ue_01M647QGJFPY04C5KEA1RM12K0`，`run_id=run_postdeploy_01` |
| B3 成本记录 | ✅ `ce_01M647QGJFZK7AV61JBNH5Z1P9`，`base_amount=0.000048`，税 3%，`total_amount=0.00004944` |

超长 run id 现在在转发前被拒绝，不再出现“调用成功、账本缺失”的情况。

---

## 十二、V0.10 口径变更（2026-11-05，未部署、未验证）

本节只记录口径变更，**不构成验收证据**。前面的验收记录全部属于 V0.9.1 构建
（`fd00dae` 及之前的 `ec3cdc0`），未覆盖下列变更。

| 变更 | 内容 | 状态 |
| --- | --- | --- |
| 档位取消 | 经济/企业档划分取消，路由完全由网关决定；Harness 只调用 A3 返回的模型 id | 分支已改 |
| 池分类 | 资源池改为产品线：`chat`（默认，text）、`video`（video，无池级计费）、`audio`（audio，占位）、`plan`（video，`per_call`，畅用计划） | 分支已改 |
| 路由头 | `X-Route-Group` 改为可选，仅 `plan` 有业务含义；普通调用不传，落默认 `chat` 池 | 分支已改 |
| 税额 | 取消供应商税率，`total_amount = base_amount × group_ratio`；`tax_rate_percent` 恒 0 并废弃，后续版本移除 | 分支已改 |
| 构建与部署 | 本轮构建（待发布）；尚未部署 | ⏳ |
| 端到端复验 | 未执行：A3 五池返回、`plan` 池识别、B3 无税额金额核对均未实测 | ⏳ 待验证 |

> 因此本文档**不能**用于声称 V0.10 已通过验收。旧记录中的 `economy` /
> `enterprise`、税 3%/10%、`total_amount` 含税额等描述，均为历史构建的实测事实，
> 保留原样不作改写。

---

## 十三、变更记录

| 日期 | 内容 |
| --- | --- |
| 2026-10-26 | 首次使用 Harness 正式签发 JWT 完成端到端验收 |
| 2026-10-29 | 使用续发 JWT 复测 `ec3cdc0`；A3 改为资源池 + 真实模型 id；发现并修复 run id 列宽缺陷 |
| 2026-10-30 | 部署 `fd00dae` 并复验：真实 JWT、超长 run id 拒绝、正常调用与 B1/B3 入账 |
| 2026-11-05 | 记录 V0.10 口径变更（五池产品线、`X-Route-Group` 仅 `plan`、取消税额）为分支已改、未部署未验证；旧证据保持原样 |

---

## 十四、RC48 候选代码验证（2026-11-07，不是线上验收）

代码候选位于 `harness-api-codex`，基于官方 `v1.0.0-rc.48`
（`706144769e5bed5a47b78721e3bb11da3cf4c0b9`）。本地 Go 全量测试、Web 类型检查、
2,470 个 Web 测试，以及 SQLite/MySQL/PostgreSQL 审计索引迁移与 Harness 账本/结算
测试通过。验证还补齐了 task-plugin 协议路由的 scope 检查。

这些结果只证明候选代码通过自动检查。没有在部署实例上使用 Harness 正式 JWT，
没有调用真实模型供应商，也没有验证 A3 目录、真实路由池或线上 B3 金额。因此本节
不改变第一至十三节的历史性质，不能作为 V0.10 线上验收结论。
