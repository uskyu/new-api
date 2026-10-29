# Harness 联调验收清单

**版本**：V0.9.1
**对应网关版本**：`harness-api` 分支 `fd00dae`

本文档供双方逐项核验。每项包含：执行方、请求、预期响应、通过标准。

---

## 一、前置条件

| 项 | 提供方 | 状态 |
| --- | --- | --- |
| JWKS URL | Harness | ✅ `https://alphapro-test.xyz/.well-known/jwks.json` |
| `iss` 值 | 双方商定 | ✅ `harness` |
| `aud` 值 | 双方商定 | ✅ `new-api` |
| 服务主体 `sub` | Harness | ✅ `harness-gateway` |
| 网关地址 | New API | ✅ `https://testapi.soloai.chat` |
| 资源池 id（`economy` / `enterprise`）与池内模型 id | New API | ✅ A3 动态返回 |

---

## 二、鉴权链路（P0）

### 2.1 健康检查连通性

**执行方**：Harness

```bash
curl -sS "http://<gateway>/internal/v1/gateway/health" \
  -H "Authorization: Bearer $HARNESS_JWT"
```

**预期**：`HTTP 200`

```json
{
  "status": "ok",
  "interface_version": "v2",
  "upstream_baseline": "45fbb149",
  "time": "<当前时间>"
}
```

**通过标准**：
- [ ] HTTP 200
- [ ] `interface_version` 为 `v2`
- [ ] `upstream_baseline` 为 `45fbb149`

---

### 2.2 无效 JWT 被拒绝

**执行方**：Harness

```bash
curl -sS "http://<gateway>/internal/v1/gateway/health" \
  -H "Authorization: Bearer invalid.jwt.token"
```

**预期**：`HTTP 401`

**通过标准**：
- [ ] HTTP 401
- [ ] `code` 为 `GATEWAY_UNAUTHORIZED`

---

### 2.3 Scope 隔离

**执行方**：Harness

用只含 `gateway.chat` 的 JWT 访问 B1：

```bash
curl -sS "http://<gateway>/internal/v1/gateway/usage-events?limit=1" \
  -H "Authorization: Bearer $CHAT_ONLY_JWT"
```

**预期**：`HTTP 403`

```json
{
  "error": {
    "code": "GATEWAY_UNAUTHORIZED",
    "details": { "required_scope": "gateway.usage.read" }
  }
}
```

**通过标准**：
- [ ] HTTP 403
- [ ] `details.required_scope` 为 `gateway.usage.read`

---

### 2.4 关联头与资源池校验

**执行方**：Harness

```bash
# 缺失 X-Harness-Run-Id
curl -sS -X POST "http://<gateway>/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_test_norun" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"hi"}]}'
```

**预期**：`HTTP 400`，`code` 为 `REQUEST_INVALID`，消息含 `X-Harness-Run-Id`

```bash
# 不存在的资源池
curl -sS -X POST "http://<gateway>/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_test_nopool" \
  -H "X-Harness-Run-Id: run_test_nopool" \
  -H "X-Route-Group: no-such-pool" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"hi"}]}'
```

**预期**：`HTTP 403`，`code` 为 `access_denied`，消息
`requested resource pool is not configured`

**通过标准**：
- [ ] 缺失 `X-Harness-Run-Id` → 400 `REQUEST_INVALID`
- [ ] `X-Harness-Run-Id` 超过 64 字符 → 400
- [ ] 未知资源池 → 403
- [ ] 以上拒绝均不产生费用，也不产生 B1/B3 事件

---

## 三、目录接口 A3（P0）

### 3.1 首次拉取

**执行方**：Harness

```bash
curl -sS -D- "http://<gateway>/v1/models" \
  -H "Authorization: Bearer $CATALOG_JWT"
```

**预期**：`HTTP 200`，响应头含 `ETag`

```json
{
  "object": "list",
  "data": [
    {
      "id": "economy",
      "object": "model",
      "display_name": "经济档",
      "kind": "text",
      "models": ["deepseek-v4.1-flash"],
      "capabilities": ["chat", "stream", "tools"]
    }
  ],
  "catalog_version": "2",
  "etag": "W/\"...\""
}
```

**通过标准**：
- [ ] HTTP 200
- [ ] `ETag` 响应头非空
- [ ] `data[].id` 为**资源池 id**（用作 `X-Route-Group`）
- [ ] `data[].models` 为**真实模型 id**（用作请求的 `model`）
- [ ] 响应体 `etag` 与响应头一致
- [ ] 不含渠道信息、采购价

---

### 3.2 条件查询

**执行方**：Harness

```bash
curl -sS -D- "http://<gateway>/v1/models" \
  -H "Authorization: Bearer $CATALOG_JWT" \
  -H 'If-None-Match: W/"<上一步的ETag>"'
```

**预期**：`HTTP 304`，**无响应体**

**通过标准**：
- [ ] HTTP 304
- [ ] 响应体为空

---

## 四、模型调用 A1/A2/A4/A9（P0）

### 4.1 A1 OpenAI Chat

**执行方**：Harness

```bash
curl -sS -X POST "http://<gateway>/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_test_a1" \
  -H "X-Harness-Run-Id: run_test_a1" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"hi"}],"max_tokens":10}'
```

**预期**：`HTTP 200`，OpenAI Chat 格式响应，含 `usage`

**通过标准**：
- [ ] HTTP 200
- [ ] 响应含 `choices[].message.content`
- [ ] 响应含 `usage.prompt_tokens` / `usage.completion_tokens`
- [ ] 响应头含 `X-NewAPI-Request-Id`
- [ ] 记录该 `X-NewAPI-Request-Id` 供后续账本核验

---

### 4.2 A2 OpenAI Responses

```bash
curl -sS -X POST "http://<gateway>/v1/responses" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_test_a2" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4.1-flash","input":"hi"}'
```

**通过标准**：
- [ ] HTTP 200
- [ ] 响应 `object` 为 `response`
- [ ] 响应含 usage 信息

---

### 4.3 A4 Anthropic Messages

```bash
curl -sS -X POST "http://<gateway>/v1/messages" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_test_a4" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4.1-flash","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'
```

**通过标准**：
- [ ] HTTP 200
- [ ] 响应 `type` 为 `message`
- [ ] 响应含 `usage.input_tokens` / `usage.output_tokens`

---

### 4.4 A9 Gemini 原生

```bash
curl -sS -X POST "http://<gateway>/v1beta/models/deepseek-v4.1-flash:generateContent" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_test_a9" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"hi"}]}]}'
```

**通过标准**：
- [ ] HTTP 200
- [ ] 响应含 `candidates[].content.parts[].text`
- [ ] 响应含 `usageMetadata.promptTokenCount`

---

### 4.5 流式调用

```bash
curl -sS -N -X POST "http://<gateway>/v1/chat/completions" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_test_stream" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4.1-flash","messages":[{"role":"user","content":"count 1 to 3"}],"stream":true,"stream_options":{"include_usage":true}}'
```

**通过标准**：
- [ ] 收到 `data: ` 前缀的 SSE 事件流
- [ ] 以 `data: [DONE]` 结束
- [ ] 末尾事件含 `usage`

---

## 五、账本接口 B1/B3（P0）

### 5.1 B1 增量拉取

**执行方**：Harness

```bash
curl -sS "http://<gateway>/internal/v1/gateway/usage-events?limit=10" \
  -H "Authorization: Bearer $USAGE_JWT"
```

**通过标准**：
- [ ] HTTP 200
- [ ] `event_id` 以 `ue_` 开头
- [ ] 含 `items` / `next_cursor` / `has_more`
- [ ] 第 4 步产生的调用出现在 `items` 中
- [ ] `request_id` 与第 4 步的 `X-Harness-Request-Id` 一致
- [ ] `gateway_request_id` 与第 4 步的 `X-NewAPI-Request-Id` 一致

---

### 5.2 B3 增量拉取

```bash
curl -sS "http://<gateway>/internal/v1/gateway/cost-records?limit=10" \
  -H "Authorization: Bearer $COST_JWT"
```

**通过标准**：
- [ ] HTTP 200
- [ ] `event_id` 以 `ce_` 开头
- [ ] `billing` 含四个字段
- [ ] `currency` 为 `USD`
- [ ] 金额为**字符串**（非数字）
- [ ] `status` 为 `settled` 或 `pending`

---

### 5.3 B1-batchGet 命中

```bash
curl -sS -X POST "http://<gateway>/internal/v1/gateway/usage-events:batchGet" \
  -H "Authorization: Bearer $USAGE_JWT" \
  -H "Content-Type: application/json" \
  -d '{"request_ids":["hrq_test_a1"],"include_corrections":true}'
```

**通过标准**：
- [ ] HTTP 200
- [ ] `items[0].found` 为 `true`
- [ ] `items[0].events` 非空
- [ ] `missing_request_ids` 为空数组
- [ ] `truncated` 为 `false`

---

### 5.4 B1-batchGet 缺失 ID

```bash
curl -sS -X POST "http://<gateway>/internal/v1/gateway/usage-events:batchGet" \
  -H "Authorization: Bearer $USAGE_JWT" \
  -H "Content-Type: application/json" \
  -d '{"request_ids":["hrq_test_a1","hrq_does_not_exist"]}'
```

**通过标准**：
- [ ] HTTP 200（**不是 404**）
- [ ] `hrq_test_a1` 的 `found` 为 `true`
- [ ] `hrq_does_not_exist` 出现在 `missing_request_ids` 中
- [ ] 整体请求未失败

---

### 5.5 B1-batchGet 超过 100 个 ID

```bash
# 构造 101 个 ID
curl -sS -X POST "http://<gateway>/internal/v1/gateway/usage-events:batchGet" \
  -H "Authorization: Bearer $USAGE_JWT" \
  -H "Content-Type: application/json" \
  -d '{"request_ids":["hrq_1","hrq_2",...]}'
```

**通过标准**：
- [ ] HTTP 400
- [ ] `code` 为 `REQUEST_INVALID`
- [ ] `details.max` 为 `100`

---

### 5.6 B3-batchGet

```bash
curl -sS -X POST "http://<gateway>/internal/v1/gateway/cost-records:batchGet" \
  -H "Authorization: Bearer $COST_JWT" \
  -H "Content-Type: application/json" \
  -d '{"request_ids":["hrq_test_a1"],"include_status_events":true}'
```

**通过标准**：
- [ ] HTTP 200
- [ ] `items[0].found` 为 `true`
- [ ] `items[0].records` 非空
- [ ] 记录含 `billing` 与 `status`

---

## 六、账本事件语义（P1）

### 6.1 首次事件

**通过标准**（检查上述拉取到的第一条事件）：
- [ ] `record_version` 为 `1`
- [ ] `supersedes_event_id` 为 `null`

### 6.2 用量更正

**执行方**：双方协同（需要触发一次更正）

**通过标准**：
- [ ] 更正后 B1 出现新事件
- [ ] 新事件 `event_type` 为 `usage_correction`
- [ ] `record_version` 递增
- [ ] `supersedes_event_id` 指向前一事件
- [ ] 前序 ID 以 `ue_` 开头

### 6.3 成本状态变化

**通过标准**：
- [ ] `pending → settled` 且金额不变时，`event_type` 为 `status_change`
- [ ] 金额未变
- [ ] `record_version` 递增

### 6.4 两条流独立

**通过标准**：
- [ ] B1 的 `supersedes_event_id` 只指向 `ue_` 前缀
- [ ] B3 的 `supersedes_event_id` 只指向 `ce_` 前缀
- [ ] 任一流的更正不影响另一流的版本号

---

## 七、分页与时间窗（P1）

### 7.1 游标翻页

```bash
# 第一页
curl -sS "http://<gateway>/internal/v1/gateway/usage-events?limit=2" \
  -H "Authorization: Bearer $USAGE_JWT"

# 用返回的 next_cursor 取第二页
curl -sS "http://<gateway>/internal/v1/gateway/usage-events?limit=2&cursor=<next_cursor>" \
  -H "Authorization: Bearer $USAGE_JWT"
```

**通过标准**：
- [ ] 两页数据不重复
- [ ] `has_more` 正确反映是否还有数据
- [ ] 末页 `next_cursor` 为 `null`

### 7.2 游标跨流混用

```bash
curl -sS "http://<gateway>/internal/v1/gateway/cost-records?cursor=<B1的cursor>" \
  -H "Authorization: Bearer $COST_JWT"
```

**通过标准**：
- [ ] HTTP 400
- [ ] `code` 为 `CURSOR_INVALID`

### 7.3 时间窗超限

```bash
curl -sS "http://<gateway>/internal/v1/gateway/usage-events?from=2026-10-01T00:00:00Z&to=2026-10-25T00:00:00Z" \
  -H "Authorization: Bearer $USAGE_JWT"
```

**通过标准**：
- [ ] HTTP 400
- [ ] `code` 为 `WINDOW_TOO_LARGE`

---

## 八、验收汇总表

| # | 项目 | 执行方 | 优先级 | 状态 |
| --- | --- | --- | --- | --- |
| 2.1 | 健康检查连通 | Harness | P0 | ⏳ |
| 2.2 | 无效 JWT 拒绝 | Harness | P0 | ⏳ |
| 2.3 | Scope 隔离 | Harness | P0 | ⏳ |
| 3.1 | 目录首次拉取 | Harness | P0 | ⏳ |
| 3.2 | 目录条件查询 304 | Harness | P0 | ⏳ |
| 4.1 | A1 模型调用 | 双方 | P0 | ⏳ |
| 4.2 | A2 模型调用 | 双方 | P0 | ⏳ |
| 4.3 | A4 模型调用 | 双方 | P0 | ⏳ |
| 4.4 | A9 模型调用 | 双方 | P0 | ⏳ |
| 4.5 | 流式调用 | 双方 | P0 | ⏳ |
| 5.1 | B1 增量拉取 | Harness | P0 | ⏳ |
| 5.2 | B3 增量拉取 | Harness | P0 | ⏳ |
| 5.3 | B1-batchGet 命中 | Harness | P0 | ⏳ |
| 5.4 | B1-batchGet 缺失 ID | Harness | P0 | ⏳ |
| 5.5 | BatchGet 超限拒绝 | Harness | P0 | ⏳ |
| 5.6 | B3-batchGet | Harness | P0 | ⏳ |
| 6.1 | 首次事件版本 | 双方 | P1 | ⏳ |
| 6.2 | 用量更正类型 | 双方 | P1 | ⏳ |
| 6.3 | 状态变化类型 | 双方 | P1 | ⏳ |
| 6.4 | 两流独立 | 双方 | P1 | ⏳ |
| 7.1 | 游标翻页 | Harness | P1 | ⏳ |
| 7.2 | 游标跨流拒绝 | Harness | P1 | ⏳ |
| 7.3 | 时间窗超限 | Harness | P1 | ⏳ |

---

## 九、变更记录

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版 |
| 2026-10-29 | V0.9.1 | A3 增加池字段与真实模型 id；调用示例改用真实模型 id；新增 2.4 关联头与资源池校验 |
