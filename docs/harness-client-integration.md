# Harness 调用方联调指南

本文仅说明 Harness 如何作为调用方接入 New API；不包含 New API 管理员的渠道、路由、价格或令牌配置步骤。管理端配置由 New API 团队负责。

## 接入前向 New API 团队获取

- New API 的 HTTPS Base URL
- 已启用、未限制模型范围，并已获智能路由授权的服务令牌
- 本令牌可用的路由组 ID（也可从目录接口返回的 `groups[].id` 获取）
- 实例的展示币种（也可从目录接口返回的 `groups[].models[].pricing.currency` 获取）

请将服务令牌保存在 Harness 服务端的密钥配置中，不要写入代码、提交记录或普通日志。Harness 无需调用 `/api/intelligent-routing` 等管理端接口。

## 1. 获取可调用模型目录

```bash
export NEW_API_BASE_URL="https://<new-api-host>"
export NEW_API_TOKEN="sk-<service-token>"

curl --fail-with-body --silent --show-error \
  -H "Authorization: Bearer ${NEW_API_TOKEN}" \
  "${NEW_API_BASE_URL}/api/harness/catalog"
```

成功响应的核心结构如下（具体模型和数值由当前实例返回）：

```json
{
  "success": true,
  "data": {
    "quota_per_unit": 500000,
    "price_unit": "display_currency",
    "groups": [
      {
        "id": "global",
        "name": "Global priority",
        "models": [
          {
            "name": "<model-name>",
            "upstream_models": ["<upstream-model>"],
            "capabilities": ["openai"],
            "pricing": {
              "mode": "token",
              "currency": "CNY",
              "unit": "1M_tokens",
              "input": 2.0,
              "output": 8.0,
              "cache_read": 0.2
            }
          }
        ],
        "unpriced_models": []
      }
    ]
  }
}
```

### 1.1 目录返回的是价格，不是倍率

这是本次契约最重要的变化：**catalog 直接返回采购价格，Harness 不需要、也不应该再做倍率换算。**

已被移除的旧字段（收到任意一个都说明对接的是旧版构建，应升级而不是兼容）：

```
model_ratio, completion_ratio, cache_ratio, create_cache_ratio, image_ratio,
audio_ratio, audio_completion_ratio, quota_type, model_price,
billing_mode, billing_expr, billing_usage_schema, billing_usage_examples
```

这些信息全部由 `pricing` 对象表达。

> **`model_price` 的单位修正**：旧文档曾把 `model_price` 描述成"固定单价（quota 数）"，这是错的。`model_price` 的真实语义是**美元/次**——每次请求的美元价格；网关内部按 `quota = model_price × quota_per_unit × 分组倍率` 换算。现在它被 `pricing.mode = "fixed"` 下的 `pricing.price` 取代，且 `price` 已经是展示币种下的金额。

### 1.2 `pricing` 对象

| 字段 | 类型 | 出现条件 | 说明 |
| --- | --- | --- | --- |
| `mode` | string | 恒有 | `token`（按 token）、`fixed`（按次）、`tiered_expr`（表达式） |
| `currency` | string | 恒有 | 本实例展示币种；`tiered_expr` 恒为 `USD` |
| `unit` | string | `token`/`fixed` | `token` → `1M_tokens`；`fixed` → `request` |
| `input` | number? | `token` | 每百万输入 token 的价格 |
| `output` | number? | `token` | 每百万输出 token 的价格 |
| `cache_read` | number? | `token` 且已配置 | 每百万缓存读取 token 的价格 |
| `cache_write` | number? | `token` 且已配置 | 每百万缓存写入 token 的价格（5 分钟 TTL / 通用） |
| `cache_write_1h` | number? | `token` 且为 Claude 模型且已配置 | 每百万 1 小时 TTL 缓存写入 token 的价格 |
| `image_input` | number? | `token` 且已配置 | 每百万图片输入 token 的价格 |
| `price` | number? | `fixed` | 每次请求的价格 |
| `expression` | string | `tiered_expr` | 计费表达式 |
| `usage_schema` | object? | `tiered_expr` | 表达式可读取的用量事实定义；**非空表示任务用量型结算**（为空时需按 3.4 进一步判断） |
| `usage_examples` | object[]? | `tiered_expr` | 用量示例，仅展示，不参与计费 |

顶层 `price_unit` 现在是 `display_currency`（此前是 `new_api_quota`）。`quota_per_unit` 仍然返回，但**仅作诊断**，不再用于把价格换算成金额。

### 1.3 价格字段缺失 = 未配置，绝不是免费

| 你看到的 | 正确理解 | 错误理解 |
| --- | --- | --- |
| `pricing` 中没有 `cache_read` | 该模型没有暴露缓存读取价格 | 缓存读取免费 / 缓存读取按 0 计 |
| `pricing` 中没有 `image_input` | 该模型没有暴露图片输入价格 | 图片输入免费 |
| `pricing` 中没有 `price`（`mode=fixed` 时） | 数据异常，需上报 | —— |
| 字段存在且为 `0` | 管理员配置的零价格，该项免费 | —— |

**接入侧必须做的实现约束：** 用"字段是否存在"判断，不要用"值是否为 0"判断。例如在 Go 中用 `*float64`，在 TypeScript 中用 `number | undefined` 配合 `"cache_read" in pricing`，不要用 `pricing.cache_read ?? 0`。

产生某车道用量但该车道价格缺失时，按 1.5「本轮已知缺口」处理，不要凭空估算。

### 1.4 币种与汇率

- `pricing.currency` 是**本实例的展示币种**，价格数值已经按该币种折算完成，Harness 不需要再做币种换算。
- 实例配置为 `CNY` 且汇率是 `1:1` 时，数字与美元数值完全相同——这是常见配置，不代表价格以美元结算。
- **不要硬编码币种，也不要硬编码汇率。** 币种从 `pricing.currency` 读，金额按该币种原样记账。
- `mode = "tiered_expr"` 例外：**该模式恒为美元**，`currency` 也返回 `USD`，不做任何缩放。

### 1.5 本轮已知缺口

以下维度 catalog **不暴露**，Harness 不能从目录推导，必须显式处理：

1. **音频价格未暴露。** `pricing` 中没有音频输入/输出价格。**Harness 不得用 catalog 对音频模型计价**，需与 New API 团队单独确认价格，或本轮不接入音频模型。
2. **内置工具按次附加费未暴露。** 搜索、代码执行等内置工具的每次调用附加费不在 `pricing` 中，涉及工具的请求成本可能被低估。需要精确核算时按实际账单核对，或在对接侧显式标注"不含工具费"。
3. **固定价图片不含按尺寸/张数的倍率。** `mode = "fixed"` 的图片价格是单次基础价，不含按尺寸、质量的倍率，也不含 `n` 张数的乘数。不能直接用 `price` 计算多图或大图请求。

这些是**已知且已确认**的缺口，不是字段遗漏。遇到上述场景应显式报错或降级，不能按缺省值静默估算。

### 1.6 其他接入要点

- `groups` 仅包含当前服务令牌可访问的路由组；运行时应使用返回的 `id`，不要假设一定存在 `global`。
- 使用 `groups[].models[].name` 作为请求模型名；`upstream_models` 是网关侧的上游映射信息，不是 Harness 应优先请求的模型名。
- 只向用户展示或调用 `models` 中的模型。`unpriced_models` 仅用于诊断；这些模型不在可计价目录中，Harness 不应展示或请求。显式价格 `0` 与未配置价格不同，仍可能是有效的免费模型。
- 按 `capabilities` 选择兼容的请求协议，例如 `openai`、`anthropic` 或 `gemini`。
- 若模型带 `billing_plugin_variants`，不要假定该模型只有一个价格：每个变体的 `pricing` 与模型级 `pricing` 同构，**直接按该变体的 `pricing` 计费**。`tiered_expr` 变体必须用变体自己的 `expression` / `usage_schema`，不能用模型级价格代替。

## 2. 发起带路由的模型请求

每次 Harness 发起模型/任务提交请求时，都要选择一个目录中可用的路由组，并发送 `X-Route-Group`。已获智能路由授权的令牌漏传此头会得到 HTTP 403，不会静默回退到普通路由。模型列表和只读任务结果查询不要求该头。

```bash
curl --include --fail-with-body \
  -H "Authorization: Bearer ${NEW_API_TOKEN}" \
  -H "X-Route-Group: global" \
  -H "Content-Type: application/json" \
  --data '{
    "model": "<model-name-from-catalog>",
    "messages": [{"role": "user", "content": "联调测试"}]
  }' \
  "${NEW_API_BASE_URL}/v1/chat/completions"
```

将 `global` 替换为本次要使用的 `groups[].id`。其他兼容协议沿用其原有请求路径与请求体，只需附加相同的路由头。

### WebSocket 客户端

原生 WebSocket 浏览器接口不能设置自定义 HTTP 请求头。需要从浏览器发起 WebSocket 时，可将路由组作为 `Sec-WebSocket-Protocol` 子协议传入：

```text
openai-insecure-route-group.<group-id>
```

例如 `openai-insecure-route-group.global`。已有的 WebSocket 协议项和认证方式仍需保留。由于浏览器子协议中可能携带令牌，不要将长期服务令牌暴露给不受信任的客户端；优先由 Harness 服务端代发请求。

## 3. 根据响应头记录实际路由与价格

成功的普通 HTTP 模型调用会返回实际命中的路由元数据：

| 响应头 | Harness 用途 |
| --- | --- |
| `X-Route-Group` | 本次实际使用的路由组 |
| `X-Route-Channel-ID` | 实际命中的上游渠道标识，用于追踪与核对 |
| `X-Route-Pricing-Group-ID` | 实际命中的 New API 计价分组 |
| `X-Route-Group-Ratio` | 本次路由适用的倍率/折扣系数 |
| `X-Route-Tax-Rate-Percent` | 实际渠道的供应商税率（百分数） |
| `X-Oneapi-Request-Id` | 与 New API 团队排障时使用的请求 ID |

**分工边界**：基础价格来自 catalog，`X-Route-Group-Ratio` 和 `X-Route-Tax-Rate-Percent` **只从本次请求的响应头读取**。catalog 不返回这两项，Harness 也不应从其他来源缓存或推断。

### 3.1 计价公式

```
最终成本 final_cost = base_cost × X-Route-Group-Ratio × (1 + X-Route-Tax-Rate-Percent / 100)
```

其中 `base_cost` 按 `pricing.mode` 计算（单位 = `pricing.currency`；`tiered_expr` 恒为 USD）：

| `mode` | `base_cost` |
| --- | --- |
| `token` | `Σ(各车道 tokens × 对应每百万 token 价格) / 1_000_000`（单位 = `currency`） |
| `fixed` | `pricing.price`（单位 = `currency`） |
| `tiered_expr` | 求值 `pricing.expression` 后按表达式类型换算，结果恒为 USD（见 3.4） |

**`final_cost` 就是金额，不要再除以 `quota_per_unit`。** 旧文档中的 `amount_usd = final_cost / quota_per_unit` 已失效。

### 3.2 完整算例（token 模式）

假设目录返回：

```json
"pricing": {
  "mode": "token",
  "currency": "CNY",
  "unit": "1M_tokens",
  "input": 2.0,
  "output": 8.0,
  "cache_read": 0.2
}
```

本次请求的实际用量：输入侧总计 1,200,000 token，其中缓存命中 200,000 token；输出 300,000 token。响应头：`X-Route-Group-Ratio: 0.8`、`X-Route-Tax-Rate-Percent: 6.5`。

**第一步：拆分车道。** 缓存命中的 200,000 已单独计价，不能重复算进 `input` 车道。

```
input 车道       = 1,200,000 - 200,000 = 1,000,000
cache_read 车道  = 200,000
output 车道      = 300,000
```

**第二步：算基础成本。**

```
input 部分      = 1,000,000 × 2.0 / 1,000,000 = 2.00
cache_read 部分 =   200,000 × 0.2 / 1,000,000 = 0.04
output 部分     =   300,000 × 8.0 / 1,000,000 = 2.40
base_cost       = 2.00 + 0.04 + 2.40 = 4.44 CNY
```

**第三步：应用路由折扣。**

```
routed_cost = 4.44 × 0.8 = 3.552 CNY
```

**第四步：应用税费。**

```
tax        = 3.552 × 6.5 / 100 = 0.23088 CNY
final_cost = 3.552 + 0.23088 = 3.78288 CNY
```

即本次请求的采购成本为 **3.78288 CNY**。

### 3.3 固定价算例

假设目录返回 `"pricing": { "mode": "fixed", "currency": "CNY", "unit": "request", "price": 0.04 }`，响应头 `X-Route-Group-Ratio: 0.8`、`X-Route-Tax-Rate-Percent: 6.5`：

```
base_cost   = 0.04 CNY
routed_cost = 0.04 × 0.8 = 0.032
tax         = 0.032 × 6.5 / 100 = 0.00208
final_cost  = 0.032 + 0.00208 = 0.03408 CNY
```

### 3.4 表达式计费算例

表达式计费有**两种内部结算语义**，求值结果换算成美元的规则不同，必须先判断类型：

| 表达式类型 | 判别依据 | `base_cost` |
| --- | --- | --- |
| token 型 | `usage_schema` 缺失或为空，表达式只用 `p`/`c`/`cr`/`cc`/`img`/`ai`/`ao` 等 token 变量 | `eval(expression) / 1_000_000` |
| 任务用量型 | `usage_schema` 非空，表达式用 `u("key")` 读取用量事实 | `eval(expression)` |

> ⚠️ **判别依据不是完美充分条件。** `usage_schema` 非空基本可判定为任务用量型；但 `usage_schema` 为空时仍可能是任务用量型（某任务模型的 schema 恰好为空）。契约目前没有显式的判别字段（见第 5 章第 6 条）。实现上建议：`usage_schema` 非空 → 不除 1,000,000；为空 → 先检查表达式是否引用了 `u(...)`，引用了则按任务用量型处理，都没有才按 token 型，并在无法判断时报错。

**token 型算例。** 假设目录返回：

```json
"pricing": {
  "mode": "tiered_expr",
  "currency": "USD",
  "expression": "tier(\"base\", p * 2 + c * 8)"
}
```

本次实际用量 `p = 500,000`、`c = 100,000`：

```
eval(expression) = 500,000 × 2 + 100,000 × 8
                 = 1,000,000 + 800,000 = 1,800,000
base_cost        = 1,800,000 / 1,000,000 = 1.80 USD
```

**任务用量型算例。** 假设目录返回：

```json
"pricing": {
  "mode": "tiered_expr",
  "currency": "USD",
  "expression": "tier(\"base\", u(\"seconds\") * 0.4)",
  "usage_schema": { "seconds": { "type": "number", "unit": "second" } }
}
```

本次实际用量 `seconds = 5`：

```
eval(expression) = 5 × 0.4 = 2.0
base_cost        = 2.0 USD        // 不再除以 1,000,000
```

**应用倍率与税点**（两个算例通用）：若响应头 `X-Route-Group-Ratio: 0.8`、`X-Route-Tax-Rate-Percent: 6.5`：

```
final_cost = base_cost × 0.8 × (1 + 0.065)
```

token 算例：`final_cost = 1.80 × 0.8 × 1.065 = 1.5336 USD`
任务算例：`final_cost = 2.0 × 0.8 × 1.065 = 1.704 USD`

注意表达式模式**恒为 USD**，即使实例展示币种是 CNY，结果也是 USD，不做缩放。

### 3.5 实现注意事项

1. **按最终响应头计价。** 发生渠道重试时，必须以最终成功响应的响应头计价，不能保留第一次选中渠道的数据。
2. **流式响应先取头。** 流式 HTTP 响应应在读取响应体之前保存响应头。
3. **价格缺失不按 0 算。** 用车道的价格字段是否存在来判断，不要用 `?? 0`。
4. **币种原样记账。** 不要硬编码币种或汇率；将 `pricing.currency` 与金额一起落库，便于后续审计。
5. **`tiered_expr` 单独处理。** 不要用 token 模式的公式去近似表达式模式。

## 4. 联调与错误处理

| HTTP 状态 | 常见原因 | Harness 处理 |
| --- | --- | --- |
| `401` | 服务令牌无效或未发送 | 检查 Base URL 与 `Authorization: Bearer ...`；不要记录令牌值 |
| `403` | 令牌未获路由授权、使用了模型受限令牌、缺少 `X-Route-Group`，或组 ID 不在授权范围内 | 检查目录中的 `groups[].id` 和令牌授权；不要自动移除路由头重试 |
| `503` | 所选组当前没有可用渠道，或没有满足价格配置的路由 | 返回可诊断错误并联系 New API 团队；不要静默切换到普通路由 |

建议联调顺序：

1. 用服务令牌请求 catalog，确认返回 `200` 且至少有一个可用路由组和模型。
2. 检查 `pricing.mode` 是否在 `token`/`fixed`/`tiered_expr` 之内，确认 `pricing.currency` 与 Harness 账本币种是否一致。
3. 用 catalog 返回的模型名与组 ID 发起一次非流式调用。
4. 保存并核对 `X-Route-*` 响应头及 `X-Oneapi-Request-Id`。
5. 按 3.2 / 3.3 / 3.4 的算例手工复核一次 `final_cost`。
6. 挑一个缺少 `cache_read` 的模型，确认实现按"未配置"处理而不是按 0。
7. 验证流式调用也使用最终响应头；随后再测试重试和计价模式。

排障时向 New API 团队提供请求时间、路由组 ID、模型名、HTTP 状态和 `X-Oneapi-Request-Id`；不要发送令牌值。

## 5. 待确认

以下几点本文按较保守的方式表述，实现前建议与 New API 团队确认：

1. **输出 token 价格的缺失语义。** 本文要求 `output` 正常一定存在；若某实例确实未返回 `output`，本文未定义回退规则，需确认后补充。
2. **缓存/图片车道价格缺失时的回退。** 本文要求"不按 0、不自行折算、标记为价格未暴露"。当前 New API 结算侧对未配置的倍率存在内置默认回退（缓存读取、图片输入通常按输入价计，缓存写入按更高倍率计），但这些默认值**没有出现在 catalog 契约里**，Harness 不应自行假设。若希望 Harness 精确对齐账单，需要 New API 团队确认：是把这些默认回退显式补进 `pricing`，还是书面给出回退规则。
3. **`tiered_expr` 下 `usage_examples` 的 `facts` 键名。** 本文示例使用 `usage_schema` 中声明的键；若实际返回的键集合与之不同，需以 `usage_schema` 为准并在文档中固定。
4. **自定义展示币种的 `currency` 取值。** 实例可能配置 `CUSTOM` 展示类型，此时 `currency` 是自定义符号而非 ISO 代码；Harness 若需要 ISO 代码，需确认映射方式。
5. **工具附加费与音频价格的补齐时间。** 见 1.5；本轮不接入，后续版本是否补齐需确认。
6. **`tiered_expr` 的两种换算语义没有显式字段。** 本文按"`usage_schema` 是否为空 + 表达式是否引用 `u(...)`"判断 token 型 / 任务用量型，这与 New API 当前内部实现一致，但契约里没有专门的判别字段。建议 New API 后续在 `pricing` 中补一个显式标识（例如 `billing_unit: token | request` 或 `expr_kind`），避免 Harness 反推。在此之前，Harness 应按 3.4 的判别表实现，并在无法判断时报错而不是猜。
