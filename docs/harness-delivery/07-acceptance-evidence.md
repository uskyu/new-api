# Harness 联调验收结果（真实 JWT）

**日期**：2026-10-26（2026-10-29 复测）
**网关版本**：复测记录为 `ec3cdc0`（版本头 `harness-api-20261029-ec3cdc0`）；
当前交付构建为 `fd00dae`（含 run id 校验补丁）
**JWT 来源**：Harness 正式签发（JWKS `https://alphapro-test.xyz/.well-known/jwks.json`）
**Token 有效期**：至 2026-11-18

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

## 十一、变更记录

| 日期 | 内容 |
| --- | --- |
| 2026-10-26 | 首次使用 Harness 正式签发 JWT 完成端到端验收 |
| 2026-10-29 | 使用续发 JWT 复测 `ec3cdc0`；A3 改为资源池 + 真实模型 id；发现并修复 run id 列宽缺陷 |
