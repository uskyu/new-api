# Harness ⇄ New API 联调对接文档

> 面向 Harness 团队的接口配置说明。本文的字段、错误码与响应头取自当前部署实例的实测结果；catalog 价格契约以第 4 章的说明为准。
>
> 文档版本：对应 New API 构建 `harness-api-20261018-a25f1f5`；**2026-10-19 起 catalog 改为返回 `pricing` 价格对象**（详见第十章变更记录）。

---

## 一、环境信息

### 1.1 服务地址

| 项目 | 值 |
| --- | --- |
| 基础地址（Base URL） | `http://82.158.231.22:3000` |
| 当前构建版本 | `harness-api-20261018-a25f1f5` |
| 部署形态 | Docker 容器 `new-api`，由 `/root/new-api/docker-compose.yml` 管理 |
| 已启用渠道数 | 4 个（全部在 `default` 分组） |

> **注意**：当前为 HTTP 明文访问。生产接入前请确认是否需要加 TLS 反向代理。

### 1.2 实例能力自检

部署后已实测通过：

| 检查项 | 结果 |
| --- | --- |
| 首页静态资源 | `GET /` → 200 |
| Harness 目录接口鉴权 | 未带 Token 访问 `GET /api/harness/catalog` → **401**（符合预期） |
| 智能路由管理接口鉴权 | 未登录访问 `GET /api/intelligent-routing` → **401**（符合预期） |

---

## 二、整体分工

```
┌─────────────┐   1. 拉取模型与采购价格      ┌──────────────┐
│             │ ───────────────────────────> │              │
│   Harness   │      GET /api/harness/catalog│              │
│   （采购侧） │      返回 pricing 价格对象     │   New API    │
│             │   2. 发起真实模型调用         │  （网关侧）   │
│             │ ───────────────────────────> │              │
│             │      POST /v1/chat/completions│             │
│             │      Header: X-Route-Group   │              │
│             │                              │              │
│             │   3. 响应头回传路由定价信息    │              │
│             │ <─────────────────────────── │              │
│             │      X-Route-* 系列响应头     │              │
└─────────────┘                              └──────────────┘
```

**职责划分**：

- **New API** 负责：维护渠道、模型标准价格（`pricing`，含 token 单价 / 按次价 / 计费表达式）、采购折扣规则（计价分组倍率）、渠道税率；根据优先级选择上游渠道
- **Harness** 负责：用 catalog 的基础价格 × 本次响应头返回的实际倍率与税点，计算采购成本与税费

New API **不参与税费核算**，仅把税点作为元数据通过响应头传出。catalog **不返回**计价分组倍率与税点，这两项只随每次调用的响应头下发。

---

## 三、前置准备（New API 管理端配置）

Harness 接入前，New API 管理员必须完成以下配置。**顺序不能颠倒**。

> ### ⚠️ 当前实例配置状态（截至 2026-10-18）
>
> 与代码部署无关，以下**管理端配置尚未完成**，Harness 直接调用会失败：
>
> | 配置项 | 状态 | 未完成时的表现 |
> | --- | --- | --- |
> | 全渠道优先级 | ❌ 未配置 | 带 `X-Route-Group` 的请求返回 403 `requested resource pool is not configured` |
> | 计价分组优先级 | ❌ 未配置 | 即使配置了渠道顺序，也会因无可用计价组而返回 503 |
> | 渠道税率 | ❌ 全部渠道均未填写 | 响应头 `X-Route-Tax-Rate-Percent` 返回 `0` |
> | 服务令牌授权 | ❌ 未授权任何令牌 | catalog 返回 403 `Token has no authorized resource pools` |
>
> **联调前必须先在 New API 管理界面完成 3.1 – 3.5 全部步骤。**

### 3.1 创建服务令牌

在「控制台 → 令牌」创建令牌，要求：

| 要求 | 说明 |
| --- | --- |
| **不能**限制模型范围 | 开启「模型限制」的令牌会被拒绝 |
| 分组字段为单个有效分组名 | 例如 `default`，**不能**写成 `default,vip` |

> 错误示例：令牌分组填 `default,vip,group_1` 会导致所有调用返回 403，且报错与智能路由无关，排查时容易误判。

### 3.2 配置全渠道优先级

在「系统设置 → 智能路由 → 全渠道优先级设置」中：

1. 勾选允许参与路由的渠道
2. 拖动调整顺序（越靠前优先级越高）
3. 保存

### 3.3 配置计价分组优先级

在「系统设置 → 智能路由 → 计价分组优先级」中：

1. 勾选允许参与路由计价的 New API 分组
2. 拖动调整顺序（越靠前优先级越高）
3. 保存

> **重要**：如果这里没有勾选任何分组，智能路由将**没有可用渠道**，所有带 `X-Route-Group` 的请求都会返回 503。

### 3.4 配置渠道税率

在「渠道 → 编辑渠道 → 连接与模型 → 模型与分组」卡片底部，填写「供应商税点（%）」：

- 取值范围：`0 ~ 100`，支持两位小数（如 `6.5`）
- 未填写时默认按 `0%` 处理
- 该值会通过 `X-Route-Tax-Rate-Percent` 响应头传出

### 3.5 授权令牌

在「系统设置 → 智能路由 → 全渠道优先级设置」页面底部「Harness 路由令牌」区块：

1. 在令牌列表中勾选刚创建的服务令牌
2. 点击「授权 N 个令牌」
3. 授权后该令牌自动获得**全部**路由分组（`global` + 所有资源池）的访问权限

授权后可在此处看到已授权令牌列表，支持**复制密钥**（自动带 `sk-` 前缀）和**撤销**。

---

## 四、接口一：获取模型与标准价格目录

### 4.1 请求

```http
GET /api/harness/catalog
Authorization: Bearer sk-<token-key>
```

### 4.2 鉴权失败响应

| 场景 | HTTP | 响应体 `message` |
| --- | --- | --- |
| Token 无效或不存在 | 401 | `Invalid service token` |
| 令牌限制了模型范围 | 403 | `Harness catalog requires a full-model service token` |
| 令牌未被授权任何路由分组 | 403 | `Token has no authorized resource pools` |

### 4.3 成功响应结构

以下为结构示例（模型名与数值由部署实例决定）。catalog 返回的是**采购价格**，不再返回倍率：

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
            "name": "deepseek-v4.1-flash",
            "upstream_models": ["deepseek-chat"],
            "capabilities": ["openai"],
            "pricing": {
              "mode": "token",
              "currency": "CNY",
              "unit": "1M_tokens",
              "input": 2.0,
              "output": 8.0,
              "cache_read": 0.2,
              "cache_write": 2.5,
              "image_input": 2.0
            }
          },
          {
            "name": "deepseek-v4-flash",
            "upstream_models": ["deepseek-v4-flash"],
            "capabilities": ["openai"],
            "pricing": {
              "mode": "fixed",
              "currency": "CNY",
              "unit": "request",
              "price": 0.04
            }
          }
        ],
        "unpriced_models": []
      }
    ]
  }
}
```

Claude 模型会额外给出 1 小时缓存写入价格（`cache_write_1h` 只对名称包含 `claude` 的模型返回）：

```json
"pricing": {
  "mode": "token",
  "currency": "CNY",
  "unit": "1M_tokens",
  "input": 3.0,
  "output": 15.0,
  "cache_read": 0.3,
  "cache_write": 3.75,
  "cache_write_1h": 6.0
}
```

任务类（插件计费）模型使用表达式计费，金额恒为美元：

```json
"pricing": {
  "mode": "tiered_expr",
  "currency": "USD",
  "expression": "tier(\"base\", p * 2 + c * 8)",
  "usage_schema": { "seconds": { "type": "number", "unit": "second" } },
  "usage_examples": [ { "label": "5 秒", "facts": { "seconds": 5 } } ]
}
```

`usage_examples` 只是界面预览用的样例，`facts` 不参与真实计费，Harness 不应据此推导价格。

### 4.4 字段说明

#### 顶层字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `quota_per_unit` | number | 仅用于诊断的额度换算基数，当前实例为 `500000`（`1 美元 = 500000 quota`）。新增价格接口**不再需要**用它把价格换算成金额 |
| `price_unit` | string | 固定为 `display_currency`，表示 `pricing` 中的金额已经是展示币种下的实际价格 |
| `groups` | array | 分组列表。**每个分组独立计算自己的模型列表** |

> **旧字段变化**：顶层 `price_unit` 过去为 `new_api_quota`（表示价格是内部额度），现在为 `display_currency`（表示价格是货币金额）。若仍收到 `new_api_quota`，说明对接的是旧版构建。
>
> `quota_per_unit` 仍然返回，但只作为诊断信息保留，**不再参与价格换算**。不要再用 `价格 / quota_per_unit` 之类的公式推导金额。

> **关键**：`groups` 不是"全量模型列表"，而是**按分组过滤后的模型视图**。`global` 分组和各资源池分组的模型列表可以不同——某模型只被某个池的渠道支持时，它只出现在那个池下面。

#### 分组字段 `groups[]`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 分组 ID。`global` 为全局路由，其他为管理员创建的资源池 ID |
| `name` | string | 分组显示名称 |
| `models` | array | 该分组下可调用且已配置有效价格的模型列表 |
| `unpriced_models` | array | 渠道支持但未配置有效价格、因此从 `models` 中排除的诊断列表；不可调用 |

#### 未定价诊断字段 `groups[].unpriced_models[]`

该列表仅供管理和诊断，模型不会出现在 `groups[].models`，Harness 不应向 New API 请求这些模型。缺少价格和显式配置为 `0` 不同：显式 `0` 表示免费，仍会作为有效价格模型出现在 `models` 中。New API 的自用模式回退倍率（例如 `37.5`）不视为已配置价格，不会让模型进入可选列表。

计价按 Relay 实际解析后的模型名计算，避免带推理参数的别名被误报为已定价却返回回退倍率。普通渠道必须有模型级价格；任务插件路由可使用模型级价格，也可使用该插件专属的非空、兼容计费表达式。只有插件变体定价时，当前资源池内所有可选插件路由都必须有有效表达式，模型才会进入 `models`。`billing_plugin_variants` 只列出该资源池可达的插件变体。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `name` | string | 有渠道支持但尚未配置有效价格的逻辑模型名 |
| `upstream_models` | string[] | 对应渠道映射的上游模型名，去重排序 |
| `reason` | string | 排除原因；当前为 `pricing_not_configured` |

#### 模型字段 `groups[].models[]`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `name` | string | 逻辑模型名（客户端请求时使用的模型名） |
| `upstream_models` | string[] | 该模型可能映射到的上游模型名，去重排序 |
| `capabilities` | string[] | 支持的端点类型，如 `openai`、`anthropic`、`gemini` |
| `pricing` | object | 计价配置，见下方「`pricing` 计价对象」 |
| `billing_plugin_variants` | array? | 插件变体计价配置；每个变体的 `pricing` 与模型级 `pricing` 同构 |

> **catalog 返回的是价格，不是倍率。** 旧版字段 `model_ratio`、`completion_ratio`、`cache_ratio`、`create_cache_ratio`、`image_ratio`、`audio_ratio`、`audio_completion_ratio`、`quota_type`、`model_price`、`billing_mode`、`billing_expr`、`billing_usage_schema`、`billing_usage_examples` 已全部移除，其信息统一由 `pricing` 表达。收到旧字段说明对接的是旧版构建，应升级而非兼容。
>
> 顺带修正旧文档对 `model_price` 的一处错误描述：**`model_price` 的单位是美元/次，不是 quota 次数。** 它表示一次请求的美元价格，内部换算为 `quota = model_price × quota_per_unit × 分组倍率`。旧文档把它写成"固定单价（quota 数）"是错的。现在它被 `pricing.mode = "fixed"` 的 `pricing.price` 取代，且 `price` 已经是展示币种下的金额，不需要再乘 `quota_per_unit`。

#### `pricing` 计价对象

| 字段 | 类型 | 出现条件 | 说明 |
| --- | --- | --- | --- |
| `mode` | string | 恒有 | 计价模式：`token`（按 token）、`fixed`（按次）、`tiered_expr`（表达式） |
| `currency` | string | 恒有 | 本实例的展示币种。`token`/`fixed` 的价格都以该币种计；`tiered_expr` 恒为 `USD` |
| `unit` | string | `token`/`fixed` | `token` 为 `1M_tokens`（每百万 token 的价格），`fixed` 为 `request`（每次请求的价格）；`tiered_expr` 不返回该字段 |
| `input` | number? | `token` | 每百万输入 token 的价格 |
| `output` | number? | `token` | 每百万输出 token 的价格 |
| `cache_read` | number? | `token` 且管理员已配置 | 每百万缓存命中（读取）token 的价格 |
| `cache_write` | number? | `token` 且管理员已配置 | 每百万缓存写入 token 的价格（5 分钟 TTL / 通用） |
| `cache_write_1h` | number? | `token` 且为 Claude 模型且已配置 | 每百万 1 小时 TTL 缓存写入 token 的价格 |
| `image_input` | number? | `token` 且管理员已配置 | 每百万图片输入 token 的价格 |
| `price` | number? | `fixed` | 每次请求的价格 |
| `expression` | string | `tiered_expr` | 计费表达式，见「表达式计费」 |
| `usage_schema` | object? | `tiered_expr` | 表达式可用的用量事实定义；非空表示任务用量型结算 |
| `usage_examples` | object[]? | `tiered_expr` | 用量示例，仅用于展示，不参与计费 |

#### 价格缺失的含义（重要）

**价格字段缺失表示「该档价格未配置 / 不适用」，绝不等于 0，也绝不等于免费。**

- `token` 模式下的模型一定有 `input`；`output` 正常情况下也一定存在（New API 对输出有模型族默认倍率）。
- `cache_read`、`cache_write`、`cache_write_1h`、`image_input` 只在管理员为该模型显式配置后才出现。
- 缺失某档价格时，**不要按 0 计价，也不要自行用 `input` 折算**。该车道在网关侧仍可能按内置默认倍率计费（例如缓存读取、图片输入未配置时通常等同输入价，缓存写入的默认回退则高于输入价），因此「缺失」绝不等于「免费」；但这些默认倍率**不在本契约内**，Harness 不能自行假设。正确做法是把该车道的费用标记为「价格未暴露」，向 New API 团队确认后再结算。若本次请求没有该车道用量（例如没有缓存命中），缺失与否不影响本次成本。
- 显式 `0` 与缺失不同：显式 `0` 是管理员配置的零价格，表示该项免费。Harness 必须区分「字段不存在」和「字段值为 `0`」，不能用同一个默认值吞掉差异。
- `cache_write_1h` 只在模型名包含 `claude`（不区分大小写）且管理员配置了缓存写入价格时出现；其他模型的 1 小时缓存写入不存在独立价格。
- 上表未列出的价格维度（音频输入/输出、内置工具按次附加费、图片按尺寸/张数的倍率）本轮**不提供**，见 4.5「本轮已知缺口」。

#### 币种与汇率

`pricing.currency` 是**本实例的展示币种**，由 New API 管理端的「额度展示类型」决定：

| 管理端展示类型 | 目录返回的 `currency` |
| --- | --- |
| `USD` | `USD`，汇率按 `1` |
| `CNY` | `CNY`，汇率按管理端配置的美元兑人民币汇率 |
| `CUSTOM` | 管理员配置的自定义币种符号，汇率按管理端配置 |
| `TOKENS` / 未识别 | `USD`，汇率按 `1`（无货币展示时回退美元） |

- 价格数值已经按该币种折算完成，Harness **不需要**再做一次币种换算。
- 当实例配置为 `CNY` 且美元兑人民币汇率为 `1:1` 时，返回的数字与美元数值完全相同。这是常见的本地/联调配置，不代表价格以美元结算。
- Harness **不得**硬编码币种，也**不得**硬编码汇率。币种必须从 `pricing.currency` 读取，金额必须按该币种原样记账。
- `CUSTOM` 模式下 `currency` 是符号（例如 `¤`），不是 ISO 代码；若 Harness 需要 ISO 代码，需与 New API 团队确认映射方式。
- 若 Harness 内部只支持美元账本，请与 New API 团队确认实例币种，不能自动乘以某个汇率来"还原"美元。

#### `tiered_expr` 表达式计费

`mode = "tiered_expr"` 表示价格由 `expression` 求值得到：

- **该模式恒为美元（USD），不受 `currency` 影响，也不做任何币种缩放。** 即使实例展示币种是 `CNY`，`currency` 字段也会返回 `USD`。
- 表达式系数是真实的美元单价：token 车道上是**每百万 token 的美元价**（`p * 2` 表示每百万输入 token 2 美元），任务用量车道上是**每单位用量的美元价**（`u("seconds") * 0.4` 表示每秒 0.4 美元）。
- **求值结果换算成美元的方式取决于表达式类型**（这是 New API 内部两种不同的结算语义）：
  - **token 型表达式**（只使用 `p` / `c` / `cr` / `cc` / `cc1h` / `img` / `ai` / `ao` 等 token 变量）：求值结果是「美元/百万 token 单价 × token 数」的放大值，**需要除以 `1_000_000`** 才是本次请求的美元成本。
  - **任务用量型表达式**（使用 `u("key")` 读取 `usage_schema` 声明的事实）：求值结果**已经是本次请求的美元成本**，**不需要再除以 `1_000_000`**。
  - 公式：
    ```
    token 型：  base_cost = eval(expression) / 1_000_000   // USD
    任务用量型：base_cost = eval(expression)               // USD
    ```
  - `usage_schema` 非空时基本可以判定为任务用量型。但注意**反向不成立**：`usage_schema` 为空（或某任务模型的 schema 恰好为空）也可能仍是任务用量型。当前契约没有显式的判别字段，最终以 New API 团队的书面说明为准；Harness 在无法判断时应报错而不是猜。
- `usage_examples` 只是展示样例，不参与计费。

表达式语法示例：

```
len <= 128000 ? tier("base", p * 3 + c * 9) : tier("long", p * 5 + c * 15)
```

变量含义：

| 变量 | 含义 |
| --- | --- |
| `p` | 输入 token 数（已自动排除被单独定价的子类别） |
| `c` | 输出 token 数（已自动排除被单独定价的子类别） |
| `cr` | 缓存读取 token 数 |
| `cc` | 缓存写入 token 数 |
| `cc1h` | 1 小时 TTL 缓存写入 token 数（Claude） |
| `img` | 图片输入 token 数 |
| `ai` / `ao` | 音频输入 / 输出 token 数 |
| `len` | 输入上下文总长度（用于档位条件判断，不会被缓存扣减） |
| `u("key")` | `usage_schema` 中声明的任务用量事实 |

#### 插件变体 `groups[].models[].billing_plugin_variants[]`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `plugin_key` | string | 插件标识 |
| `plugin_name` | string | 插件显示名 |
| `pricing` | object | 与模型级 `pricing` **同构**；`mode` 可能为 `token` 或 `tiered_expr` |

同一模型可能由多个任务插件承接，每个插件的计费方式可不同。catalog 只列出当前资源池**可达**的插件变体；Harness 不应假定模型只有一个价格。**计费时直接使用变体自己的 `pricing` 对象**：`token` 变体的数值与模型级 token 单价相同（复用同一套 token 车道价），`tiered_expr` 变体必须使用它自己的 `expression` / `usage_schema`，不能用模型级价格代替。

### 4.5 本轮已知缺口

以下价格维度在本次契约中**不暴露**，Harness 不能从 catalog 推导：

| 缺口 | 影响 | 本轮处理建议 |
| --- | --- | --- |
| 音频 token 价格 | `pricing` 中没有 `audio_input` / `audio_output`。仅靠 `input`/`output` 无法还原音频价格 | **不要用 catalog 对音频模型计价。** 音频类模型需与 New API 团队单独确认价格，或暂不接入 |
| 内置工具按次附加费 | 搜索、代码执行等内置工具的每次调用附加费不在 `pricing` 中 | 涉及工具的请求成本可能被低估；需要精确核算时按实际账单核对，或在对接侧显式标注为"不含工具费" |
| 图片按尺寸/张数的倍率 | `fixed` 模式下的图片价格是**单次基础价**，不含按尺寸、质量的倍率，也不含 `n` 张数的乘数 | 不能仅用 `price × 1` 计算多图或大图请求；需与 New API 团队确认后再计费 |

这些缺口是**已知且已确认**的，不是字段遗漏。遇到上述场景时应显式报错或降级，不能按缺省值静默估算。

---

## 五、接口二：发起模型调用并获取路由定价

### 5.1 请求

在普通模型请求基础上，**额外携带一个请求头**：

```http
POST /v1/chat/completions
Authorization: Bearer sk-<token-key>
X-Route-Group: global
Content-Type: application/json

{
  "model": "deepseek-v4-flash",
  "messages": [{"role": "user", "content": "hello"}]
}
```

### 5.2 `X-Route-Group` 的作用

| 情况 | 行为 |
| --- | --- |
| **不携带** | 未授权令牌保持原有普通路由；已授权令牌对需要选择模型/上游渠道的请求返回 403。模型列表和只读任务结果查询无需此头 |
| 携带 `global` | 使用「全渠道优先级」中配置的渠道顺序 |
| 携带资源池 ID（如 `pool-fast`） | 使用该资源池中配置的渠道顺序 |

浏览器 WebSocket API 无法设置自定义 HTTP 请求头，可在 `Sec-WebSocket-Protocol` 中增加 `openai-insecure-route-group.<group-id>`（例如 `openai-insecure-route-group.global`）。服务端会按普通 `X-Route-Group` 规则验证令牌授权。

> **这是最关键的一点**：已授权令牌漏传路由分组会被拒绝，不会静默回退到普通路由；普通未授权令牌仍保持原有调用行为。

### 5.3 鉴权失败响应

| 场景 | HTTP | 响应体 `message` |
| --- | --- | --- |
| 授权令牌未携带必需的路由分组 | 403 | `this token requires X-Route-Group for routed requests; choose an authorized resource pool` |
| 令牌限制了模型范围 | 403 | `request-level resource pool selection requires a full-model service token` |
| 令牌未授权任何路由分组 | 403 | `token is not configured for intelligent routing` |
| 请求的分组未配置 | 403 | `requested resource pool is not configured` |
| 令牌未被授权该分组 | 403 | `token is not authorized for the requested resource pool` |

### 5.4 成功响应头

调用成功（HTTP 2xx）时，响应头会包含以下字段：

| 响应头 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `X-Route-Group` | string | 本次使用的路由分组 ID | `global` |
| `X-Route-Channel-ID` | integer | 实际选中的上游渠道 ID | `9` |
| `X-Route-Pricing-Group-ID` | string | 选中渠道命中的 New API 计价分组 | `default` |
| `X-Route-Group-Ratio` | number | 该计价分组的倍率（折扣系数） | `1` |
| `X-Route-Tax-Rate-Percent` | number | 该渠道的供应商税点（百分比） | `6.5` |

此外还会返回 `X-Oneapi-Request-Id` 用于问题追踪。

> **分工再确认**：`X-Route-Group-Ratio`（采购折扣）和 `X-Route-Tax-Rate-Percent`（税点）**只从本次响应的响应头读取**，catalog 中不返回这两项。catalog 给的是基础价格，响应头给的是本次实际生效的折扣与税率。

以上响应头**均已加入 CORS 暴露列表**（`Access-Control-Expose-Headers`），浏览器端可直接读取。

### 5.5 实测样例

以下为本地验证环境的真实调用返回：

```
HTTP 200

X-Route-Group: global
X-Route-Channel-ID: 1
X-Route-Pricing-Group-ID: vip
X-Route-Group-Ratio: 1
X-Route-Tax-Rate-Percent: 6.5
```

该次调用中：
- 渠道 1 同时属于 `default` 和 `vip` 两个分组
- 管理员配置的计价分组优先级为 `group_1 > vip > default`
- 因此选中 `vip`（比 `default` 优先级更高）
- 税点 `6.5` 来自渠道 1 的配置

### 5.6 重试时的行为

如果第一次选中的渠道调用失败，系统会按优先级自动切换到下一个渠道重试。此时：

- **响应头会被清空并重新写入**，最终返回的是**实际成功那个渠道**的信息
- 例如渠道 A（税点 6.5%）失败、渠道 B（税点 12%）成功，则 `X-Route-Tax-Rate-Percent` 返回 `12`

Harness 必须按**最终返回的响应头**计价，不能缓存首次选择结果。

### 5.7 流式请求

`stream: true` 时，响应头在流开始前写入，行为与非流式一致。

### 5.8 新增问题排查：`pricing` 字段缺失

如果 catalog 返回的模型里 `pricing` 缺少 `cache_read`、`cache_write`、`image_input` 等字段，这是**正常**的：表示管理员没有为该模型配置这一档价格，不代表这些 token 免费。处理方式见 4.4「价格缺失的含义」与 4.5「本轮已知缺口」。

---

## 六、计价规则速查

Harness 拿到响应头后，按如下方式计算采购成本。**基础价格来自 catalog，折扣与税点来自本次响应的响应头。**

### 6.1 统一公式

```
基础成本 base_cost（见 6.2，单位 = pricing.currency）
路由后成本 routed_cost = base_cost × X-Route-Group-Ratio
税费        tax         = routed_cost × (X-Route-Tax-Rate-Percent / 100)
最终成本    final_cost  = routed_cost + tax
```

等价写法：

```
final_cost = base_cost × X-Route-Group-Ratio × (1 + X-Route-Tax-Rate-Percent / 100)
```

`final_cost` 的币种与 `pricing.currency` 一致（`tiered_expr` 模式下恒为 USD）。

### 6.2 按模式计算基础成本 `base_cost`

**`mode = "token"`**：各条车道分别计价后求和，再除以 1,000,000。

```
base_cost =
  ( input_tokens        × pricing.input
  + output_tokens       × pricing.output
  + cache_read_tokens   × pricing.cache_read
  + cache_write_tokens  × pricing.cache_write
  + cache_1h_tokens     × pricing.cache_write_1h
  + image_input_tokens  × pricing.image_input
  ) / 1_000_000
```

注意：

- `pricing` 中的价格是**每百万 token** 的价格，所以最后一定要除以 `1_000_000`。
- `input_tokens` / `output_tokens` 是**已按车道拆分后**的数量：被缓存、图片等单独定价的 token 不应再计入 `input` 车道，否则会重复计费。若某车道价格字段缺失而本次请求又确实产生了该车道用量，说明该车道的价格未暴露，见 4.4「价格缺失的含义」——**不能按 0 计，也不能自行折算**。
- 只对实际发生的车道求和。某次请求没有缓存命中，就不加 `cache_read` 项。
- `output` 一般总是存在（New API 对输出有模型族默认价格）；若确实缺失，同样按"价格未暴露"处理。

**`mode = "fixed"`**：

```
base_cost = pricing.price
```

按次固定价，与 token 数量无关。

**`mode = "tiered_expr"`**：

```
token 型表达式（无 usage_schema）：
  base_cost = eval(pricing.expression) / 1_000_000      // 结果已是 USD

任务用量型表达式（usage_schema 非空）：
  base_cost = eval(pricing.expression)                  // 结果已是 USD
```

两种情况都不做币种换算。判别方式与内部两种结算语义的差异见第四章「`tiered_expr` 表达式计费」。

### 6.3 换算为金额

`final_cost` 已经是金额，单位为 `pricing.currency`（`tiered_expr` 为 USD）。

```
amount = final_cost
```

**不再需要除以 `quota_per_unit`。** `quota_per_unit` 仅作为诊断字段保留，旧文档中的 `amount_usd = final_cost / quota_per_unit` 已失效。

### 6.4 计算顺序小结

1. 从 catalog 取该模型的 `pricing`
2. 按 `mode` 与本次实际用量算出 `base_cost`
3. 乘以本次响应头的 `X-Route-Group-Ratio`
4. 乘以 `(1 + X-Route-Tax-Rate-Percent / 100)`
5. 得到最终采购成本，币种按 `pricing.currency`（`tiered_expr` 为 USD）

### 6.5 计算示例（token 模式）

设某模型 `pricing` 为 `{"mode":"token","currency":"CNY","unit":"1M_tokens","input":2.0,"output":8.0,"cache_read":0.2}`，本次用量为输入侧总 token 1,200,000（其中缓存命中 200,000）、输出 300,000，响应头为 `X-Route-Group-Ratio: 0.8`、`X-Route-Tax-Rate-Percent: 6.5`。

```
# 未命中缓存的输入 = 1,200,000 - 200,000 = 1,000,000
base_cost = (1,000,000 × 2.0 + 300,000 × 8.0 + 200,000 × 0.2) / 1,000,000
          = (2,000,000 + 2,400,000 + 40,000) / 1,000,000
          = 4.44 CNY
routed_cost = 4.44 × 0.8 = 3.552 CNY
tax         = 3.552 × 6.5 / 100 = 0.23088 CNY
final_cost  = 3.552 + 0.23088 = 3.78288 CNY
```

---

## 七、接口一览

| 接口 | 方法 | 鉴权 | 用途 |
| --- | --- | --- | --- |
| `/api/harness/catalog` | GET | 服务令牌 | 获取分组与模型标准价格 |
| `/v1/chat/completions` | POST | 服务令牌 + `X-Route-Group` | 发起调用并获取路由定价响应头 |

管理端接口（Harness **不需要**调用）：

| 接口 | 方法 | 用途 |
| --- | --- | --- |
| `/api/intelligent-routing` | GET / PUT | 读写路由配置 |
| `/api/intelligent-routing/token-access` | GET | 读令牌授权 |
| `/api/intelligent-routing/token-access/:id` | PUT | 写令牌授权 |

---

## 八、常见问题

### Q1：调用返回 503，提示无可用渠道

**原因**：`pricing_group_order` 为空，或请求的分组里没有任何渠道同时满足「已勾选计价分组」条件。

**处理**：检查「计价分组优先级」页是否已勾选分组并保存。

### Q2：catalog 返回 403 `Token has no authorized resource pools`

**原因**：令牌还没被授权。

**处理**：到「智能路由 → 全渠道优先级」页面底部的令牌区块，勾选该令牌并点击授权。

### Q3：所有调用返回 403，报错提到分组

**原因**：令牌的 `group` 字段配置错误（例如填了逗号分隔的多值）。

**处理**：把令牌分组改为**单个**有效分组名。

### Q4：响应头里没有 `X-Route-*`

**原因**：请求没有带 `X-Route-Group` 头，或者带了空值。

**处理**：确认请求头中存在 `X-Route-Group: global` 或具体的资源池 ID。

### Q5：`X-Route-Tax-Rate-Percent` 返回 0，但渠道配了税点

**原因**：该渠道未在界面保存过税点，或保存后未生效。

**处理**：编辑该渠道，确认「供应商税点（%）」已填写并保存。

---

## 九、联调建议步骤

1. **确认令牌可用**：用服务令牌调用 `GET /api/harness/catalog`，应返回 200 和分组列表
2. **确认分组存在**：检查 `groups[].id` 中是否包含 `global` 或目标资源池
3. **确认价格形态**：检查 `models[].pricing.mode` 是否在 `token`/`fixed`/`tiered_expr` 之内，并确认 `pricing.currency` 与 Harness 内部账本币种一致
4. **发起一次非流式调用**：带 `X-Route-Group` 调用 `/v1/chat/completions`，检查五个响应头是否齐全
5. **对照计价**：用响应头中的倍率和税点，按第六章公式与 catalog 中的价格核对计算结果
6. **验证缺失字段**：挑一个没有缓存价格的模型，确认缺字段时按「价格未暴露」处理（不按 0、也不自行折算），并能走通与 New API 团队的确认流程
7. **验证重试**：故意让一个渠道失败，确认切换后响应头反映的是新渠道
8. **接入流式**：确认 `stream: true` 时响应头仍可正常读取

---

## 十、变更记录

| 日期 | 变更 |
| --- | --- |
| 2026-10-19 | catalog 改为返回价格（`pricing`），移除全部倍率字段；`price_unit` 改为 `display_currency`；补充价格缺失语义、币种规则、`tiered_expr` 恒为 USD 及其两种换算语义、新的成本公式与三个已知缺口；修正旧文档对 `model_price` 单位的错误描述 |
| 2026-10-18 | catalog 仅公开有效定价模型，按分组增加未定价模型诊断字段 |
| 2026-10-18 | 初版。对应构建 `harness-api-20261018-a25f1f5` |
