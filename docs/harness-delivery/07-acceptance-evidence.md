# Harness 联调验收结果（真实 JWT）

**日期**：2026-10-26
**网关版本**：`31be79010`（镜像 `harness-latest`）
**JWT 来源**：Harness 正式签发（JWKS `https://alphapro-test.xyz/.well-known/jwks.json`）
**Token 有效期**：至 2026-10-28 16:58 +08:00

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
| 首次读取 | ✅ 200，`object=list`，2 个业务档位 |
| ETag | ✅ `W/"513047bc..."` |
| 条件查询 `If-None-Match` | ✅ 304，响应体为空 |
| 只返回业务档位 | ✅ 不暴露真实模型名 |

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
| PostgreSQL | 无可用实例 |
| `gateway.health.read` 语义 | 待对方确认 |

---

## 十、变更记录

| 日期 | 内容 |
| --- | --- |
| 2026-10-26 | 首次使用 Harness 正式签发 JWT 完成端到端验收 |
