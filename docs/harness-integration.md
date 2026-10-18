# Harness ⇄ New API 联调对接文档

> 面向 Harness 团队的接口配置说明。本文所有字段、错误码和响应头均取自当前部署实例的实测结果。
>
> 文档版本：对应 New API 构建 `harness-api-20261018-a25f1f5`

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
┌─────────────┐   1. 拉取模型与标准价格      ┌──────────────┐
│             │ ───────────────────────────> │              │
│   Harness   │      GET /api/harness/catalog│              │
│   （采购侧） │                              │   New API    │
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

- **New API** 负责：维护渠道、模型标准价格、采购折扣规则（计价分组倍率）、渠道税率；根据优先级选择上游渠道
- **Harness** 负责：根据 New API 返回的实际渠道、倍率、税点，计算采购成本与税费

New API **不参与税费核算**，仅把税点作为元数据通过响应头传出。

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

以下为本地验证环境的实测响应（生产实例配置完成后结构相同）：

```json
{
  "success": true,
  "data": {
    "quota_per_unit": 500000,
    "price_unit": "new_api_quota",
    "groups": [
      {
        "id": "global",
        "name": "Global priority",
        "models": [
          {
            "name": "deepseek-v4-flash",
            "upstream_models": ["deepseek-v4-flash"],
            "capabilities": ["openai"],
            "quota_type": 1,
            "model_ratio": 0,
            "model_price": 5,
            "completion_ratio": 1,
            "billing_mode": "ratio",
            "billing_expr": "",
            "cache_ratio": 0.1,
            "create_cache_ratio": 1.25,
            "image_ratio": 2,
            "audio_ratio": 16,
            "audio_completion_ratio": 2,
            "billing_usage_schema": {},
            "billing_usage_examples": [],
            "billing_plugin_variants": []
          }
        ]
      }
    ]
  }
}
```

### 4.4 字段说明

#### 顶层字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `quota_per_unit` | number | 额度换算基数，当前实例为 `500000`。即 `1 美元 = 500000 quota` |
| `price_unit` | string | 固定为 `new_api_quota`，表示价格单位为 New API 内部额度 |
| `groups` | array | 分组列表。**每个分组独立计算自己的模型列表** |

> **关键**：`groups` 不是"全量模型列表"，而是**按分组过滤后的模型视图**。`global` 分组和各资源池分组的模型列表可以不同——某模型只被某个池的渠道支持时，它只出现在那个池下面。

#### 分组字段 `groups[]`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 分组 ID。`global` 为全局路由，其他为管理员创建的资源池 ID |
| `name` | string | 分组显示名称 |
| `models` | array | 该分组下可调用的模型列表 |

#### 模型字段 `groups[].models[]`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `name` | string | 逻辑模型名（客户端请求时使用的模型名） |
| `upstream_models` | string[] | 该模型可能映射到的上游模型名，去重排序 |
| `capabilities` | string[] | 支持的端点类型，如 `openai`、`anthropic`、`gemini` |
| `quota_type` | number | 计价类型，见下方说明 |
| `model_ratio` | number | 模型倍率（仅 `quota_type=0` 有效） |
| `model_price` | number | 固定单价（仅 `quota_type=1` 有效） |
| `completion_ratio` | number | 输出 token 相对于输入 token 的倍率 |
| `billing_mode` | string | `ratio` 或 `tiered_expr` |
| `billing_expr` | string | 阶梯计费表达式（仅 `billing_mode=tiered_expr` 时有值） |
| `cache_ratio` | number? | 缓存命中 token 倍率，无则为 `null` |
| `create_cache_ratio` | number? | 缓存写入 token 倍率 |
| `image_ratio` | number? | 图片 token 倍率 |
| `audio_ratio` | number? | 音频输入 token 倍率 |
| `audio_completion_ratio` | number? | 音频输出 token 倍率 |
| `billing_usage_schema` | object? | 用量字段定义（插件型计费模型使用） |
| `billing_usage_examples` | array? | 用量示例 |
| `billing_plugin_variants` | array? | 插件变体计费配置 |

#### `quota_type` 与计价方式

| `quota_type` | 含义 | 计价公式 |
| --- | --- | --- |
| `0` | 按 token 计费 | `(输入tokens + 输出tokens × completion_ratio + 缓存tokens × cache_ratio + ...) × model_ratio × 分组倍率` |
| `1` | 按次固定价 | `model_price × 分组倍率` |

> 实测：当前实例的 `deepseek-v4-flash` 为 `quota_type=1, model_price=5`（按次计费）；`deepseek-v4.1-flash` 为 `quota_type=0, model_ratio=37.5`（按 token 计费）。

#### 阶梯计费表达式

当 `billing_mode = "tiered_expr"` 时，需按 `billing_expr` 计算。表达式语法示例：

```
len <= 128000 ? tier("base", p * 3 + c * 9) : tier("long", p * 5 + c * 15)
```

变量含义：

| 变量 | 含义 |
| --- | --- |
| `p` | 输入 token 数 |
| `c` | 输出 token 数 |
| `cr` | 缓存读取 token 数 |
| `cc` | 缓存写入 token 数 |
| `img` | 图片 token 数 |
| `len` | 输入长度 |

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
| **不携带** | 走原有随机选渠道逻辑。**路由优先级、计价分组、税率全部不生效**，也不返回任何 `X-Route-*` 响应头 |
| 携带 `global` | 使用「全渠道优先级」中配置的渠道顺序 |
| 携带资源池 ID（如 `pool-fast`） | 使用该资源池中配置的渠道顺序 |

> **这是最关键的一点**：不传 `X-Route-Group`，整套智能路由功能完全不触发。

### 5.3 鉴权失败响应

| 场景 | HTTP | 响应体 `message` |
| --- | --- | --- |
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

---

## 六、计价规则速查

Harness 拿到响应头后，按如下方式计算采购成本：

### 6.1 标准成本（来自 catalog）

```
按 token 计费（quota_type = 0）：
  cost_quota = (prompt_tokens + completion_tokens × completion_ratio) × model_ratio

按次计费（quota_type = 1）：
  cost_quota = model_price
```

### 6.2 应用路由折扣

```
routed_cost_quota = cost_quota × X-Route-Group-Ratio
```

### 6.3 应用税费

```
tax = routed_cost_quota × (X-Route-Tax-Rate-Percent / 100)
final_cost = routed_cost_quota + tax
```

### 6.4 换算为货币

```
amount_usd = final_cost / quota_per_unit
```

当前实例 `quota_per_unit = 500000`。

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
3. **发起一次非流式调用**：带 `X-Route-Group` 调用 `/v1/chat/completions`，检查五个响应头是否齐全
4. **对照计价**：用响应头中的倍率和税点，与 catalog 中的标准价格核对计算结果
5. **验证重试**：故意让一个渠道失败，确认切换后响应头反映的是新渠道
6. **接入流式**：确认 `stream: true` 时响应头仍可正常读取

---

## 十、变更记录

| 日期 | 变更 |
| --- | --- |
| 2026-10-18 | 初版。对应构建 `harness-api-20261018-a25f1f5` |
