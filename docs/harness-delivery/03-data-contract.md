# Harness 账本数据契约说明

**版本**：V0.9.1
**对应网关版本**：`harness-api` 分支 `ec3cdc0`

本文档说明 B1 用量事件与 B3 成本记录的字段语义、事件模型和金额口径。

---

## 一、两条流的分工

网关把一次请求的记账拆成两条**互相独立**的事件流：

| 流 | 回答的问题 | 前缀 | 对应接口 |
| --- | --- | --- | --- |
| **B1 用量** | 这次请求消耗了多少 Token？ | `ue_` | usage-events |
| **B3 成本** | 这次采购花了多少钱？当前什么状态？ | `ce_` | cost-records |

**独立性的含义**：

- 用量变化只产生 B1 事件，不影响 B3
- 成本变化只产生 B3 事件，不影响 B1
- 两条流各有自己的版本链

**重要推论**：`pending → settled` 时若用量未变，**只产生 B3 事件，不产生 B1 事件**。
Harness 不能假设"每条 B3 事件都对应一条 B1 事件"。

---

## 二、B1 用量事件字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `event_id` | string | ✓ | 前缀 `ue_`，不可变 |
| `event_type` | string | ✓ | `usage` / `usage_correction` |
| `request_id` | string | ✓ | Harness Request ID（`X-Harness-Request-Id`） |
| `gateway_request_id` | string | ✓ | New API 请求 ID |
| `run_id` | string | ✓ | Harness Run ID（`X-Harness-Run-Id`），≤64 字符，恒非空 |
| `usage_unit` | string | ✓ | 本期恒为 `token`（见第五节） |
| `usage` | object | ✓ | 见 2.1 |
| `usage_source` | string | ✓ | `upstream` / `estimated` / `unavailable` |
| `is_estimated` | bool | ✓ | 是否估算 |
| `observed_at` | string | ✓ | RFC3339 UTC 毫秒 |
| `record_version` | int | ✓ | 从 1 开始 |
| `supersedes_event_id` | string\|null | ✓ | 首次为 `null` |

`run_id` 来自请求头 `X-Harness-Run-Id`。该头是模型调用的必填头，缺失或超过 64 字符时
网关在转发上游之前返回 400，因此账本里不会出现 `run_id` 为空的事件。一个 run 可以
包含多次模型调用，B1 事件按 `request_id` 唯一、按 `run_id` 可聚合。

B3 成本记录按第四轮冻结的字段表**不返回** `run_id`，但账本行仍存储该值；需要时可由
网关侧补充输出。

### 2.1 usage 对象

```json
{
  "input_tokens": 39,
  "output_tokens": 20,
  "cache_read_tokens": 0,
  "cache_write_tokens": 0,
  "total_tokens": 59,
  "split_known": true
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `input_tokens` | int \| null | 输入 Token；`null` 表示未测量 |
| `output_tokens` | int \| null | 输出 Token；`null` 表示未测量 |
| `cache_read_tokens` | int \| null | 缓存读取 |
| `cache_write_tokens` | int \| null | 缓存写入 |
| `total_tokens` | int \| null | 上游报告的总量（**待确认字段**） |
| `split_known` | bool | 输入/输出是否分别观测（**待确认字段**） |

### 2.2 `null` 与 `0` 的区别

这是本契约最重要的语义之一：

| 值 | 含义 |
| --- | --- |
| `0` | **确认**没有该消耗 |
| `null` | **未取得**该数据 |

三种典型场景：

**场景 A：完整用量**

```json
{"input_tokens": 128, "output_tokens": 512, "cache_read_tokens": 0, "cache_write_tokens": 0, "total_tokens": 640, "split_known": true}
```

**场景 B：上游只给总量，无法拆分**

```json
{"input_tokens": null, "output_tokens": null, "cache_read_tokens": null, "cache_write_tokens": null, "total_tokens": 1000, "split_known": false}
```

**场景 C：完全无用量数据**

```json
{"input_tokens": null, "output_tokens": null, "cache_read_tokens": null, "cache_write_tokens": null, "split_known": false}
```

区分 B 与 C 靠 `usage_source`：B 为 `upstream`，C 为 `unavailable`。

> **待确认**：`null`、`total_tokens`、`split_known` 三项尚未与 Harness 冻结，
> 见 `05-pending-confirmations.md`。

### 2.3 usage_source 三态

| 值 | 含义 | `is_estimated` |
| --- | --- | --- |
| `upstream` | 上游返回的实测值 | `false` |
| `estimated` | 网关本地计数或估算 | `true` |
| `unavailable` | 完全无可用数据 | `false` |

---

## 三、B3 成本记录字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `event_id` | string | ✓ | 前缀 `ce_`，不可变 |
| `event_type` | string | ✓ | `cost` / `status_change` / `cost_correction` / `refund` |
| `cost_id` | string | ✓ | 成本记录 ID，关联网关请求，更正时不变 |
| `request_id` | string | ✓ | Harness Request ID |
| `gateway_request_id` | string | ✓ | New API 请求 ID |
| `status` | string | ✓ | `pending` / `settled` / `refunded` / `adjusted` |
| `currency` | string | ✓ | 本期恒为 `USD` |
| `billing` | object | ✓ | 见 3.1 |
| `billing_basis` | string | ✓ | 与 `usage_unit` 一致 |
| `procurement_price_version` | string | ✓ | 采购价配置版本 |
| `record_version` | int | ✓ | 从 1 开始 |
| `supersedes_event_id` | string\|null | ✓ | 首次为 `null` |
| `observed_at` | string | ✓ | RFC3339 UTC 毫秒 |

### 3.1 billing 对象

```json
{
  "base_amount": "0.00003304",
  "group_ratio": 0,
  "tax_rate_percent": 0,
  "total_amount": "0.00003304"
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `base_amount` | **string** | 采购基础金额（客户分组倍率影响之前） |
| `group_ratio` | float | 采购分组倍率 |
| `tax_rate_percent` | float | 供应商税率（百分比） |
| `total_amount` | **string** | 最终采购成本 |

**金额必须是字符串十进制**，不可当 float 解析——十进制精度会在转换中丢失。

### 3.2 金额计算口径

```
base_amount = PreGroupQuota / QuotaPerUnit
total_amount = base_amount × group_ratio × (1 + tax_rate_percent / 100)
```

**关键**：`base_amount` 在**客户分组倍率生效之前**取得，因此采购成本与客户售价
完全解耦。修改客户分组倍率不影响采购成本。

### 3.3 金额口径的重要限制

`total_amount` 是**网关按自身配置的模型价格、采购倍率和税率计算出的金额**，
**不等于**供应商最终出具的实际账单。

Harness 应将其视为「网关计价结果」，而非「供应商已确认的实付金额」。

---

## 四、事件类型规则

### 4.1 B1 用量事件类型

| 场景 | event_type |
| --- | --- |
| 首次用量记录 | `usage` |
| 已有记录，用量被更正 | `usage_correction` |

**判定逻辑**：有前序事件即为 `usage_correction`。因为只有用量**实际变化**时才会
追加新事件，所以有前序的追加必然是更正。

### 4.2 B3 成本事件类型

| 场景 | event_type |
| --- | --- |
| 首次成本记录（无论初始状态） | `cost` |
| 金额变化（含倍率/税率导致的 total 变化） | `cost_correction` |
| 金额不变，仅状态变化 | `status_change` |
| 已有记录 → `refunded` | `refund` |

**判定优先级**：

1. 无前序事件 → `cost`
2. 状态为 `refunded` → `refund`
3. 金额任一字段变化 → `cost_correction`
4. 其余 → `status_change`

**「金额变化」包含**：`base_amount`、`total_amount`、`group_ratio`、
`tax_rate_percent`、`currency` 任一变化。

这很重要：修改采购倍率或税率时 `base_amount` 不变，但 `total_amount` 会变，
此时必须是 `cost_correction` 而非 `status_change`。

---

## 五、版本链与事件关联

### 5.1 版本递增

- 两条流**各自独立**维护 `record_version`
- 首次事件 `record_version = 1`
- 每次追加 `record_version = 上一版本 + 1`
- 更正事件的 `supersedes_event_id` 指向**同一流**的上一条事件

### 5.2 版本链示例

一次请求经历「结算 → 更正用量 → 退款」：

**B1 用量流**

```
v1  ue_AAA  usage              tokens=100/20
v2  ue_BBB  usage_correction   tokens=100/25   supersedes=ue_AAA
```

**B3 成本流**

```
v1  ce_XXX  cost               status=settled   total=0.0100
v2  ce_YYY  refund             status=refunded  total=0.0100   supersedes=ce_XXX
```

**注意**：B1 到 v2、B3 到 v2，但两者**互不引用**。

### 5.3 金额是完整快照，不是增量

每个版本携带该记录的**完整最新金额**，不是差额。

Harness 更新自己的账本时应：

- 用 `request_id` / `cost_id` 定位记录
- 用 `record_version` 判断新旧
- **直接采用**最新版本的金额
- **不要**把历史版本金额相加

### 5.4 重复结算不产生事件

同一请求、同一状态、同一金额的重复结算**不产生新事件**。

网关会正常更新快照，但事件流不追加——否则版本链会膨胀而又不表达新信息。

---

## 六、本期范围限制

| 项 | 本期实现 |
| --- | --- |
| `usage_unit` | **只产生 `token`** |
| `currency` | **固定 `USD`** |
| 计费对象 | 仅模型 Token |
| 独立工具计费 | 不做 |
| 图像/音频计费 | 不做 |
| `status = failed` | 内部状态，**不会出现在 B3** |

### 6.1 关于 usage_unit

第四轮文档列出 `token` / `image` / `audio_second` / `character` / `request` / `other`
是为未来定义可选值。**本期实际只会产生 `token`**，其他值不会出现。

### 6.2 关于 status = failed

网关内部结算表有 `failed` 状态，用于异步任务失败。经代码审查，生产路径
**不会**将其写入 B3 事件。B3 的 `status` 只会是 `pending` / `settled` /
`refunded` / `adjusted` 四者之一。

---

## 七、变更记录

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版 |
| 2026-10-29 | V0.9.1 | 明确 `run_id` 必填且 ≤64、缺失时调用被拒绝；说明 B3 不返回 `run_id` 但账本仍存储 |
