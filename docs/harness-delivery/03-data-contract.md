# Harness 账本数据契约说明

**版本**：V0.11（2026-11-14）
**对应网关版本**：`harness-api` 分支（已合入 RC48 候选提交；已部署 82 实例 `harness-api-20261114-c29f1a6`）

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
  "group_ratio": 1,
  "tax_rate_percent": 0,
  "total_amount": "0.00003304"
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `base_amount` | **string** | 采购基础金额（客户分组倍率影响之前） |
| `group_ratio` | float | 采购计价分组倍率（来自 `pricing_group_order` 命中的分组，与资源池、渠道路由顺序无关；Harness 不参与选择） |
| `tax_rate_percent` | float | **恒为 `0`，已废弃（legacy）**。供应商税率已取消，保留字段仅为兼容旧解析器，后续版本移除 |
| `total_amount` | **string** | 最终采购成本 |

**金额必须是字符串十进制**，不可当 float 解析——十进制精度会在转换中丢失。

样例中 `group_ratio = 1`（当前只有 `default` 一个采购分组，无采购折扣），因此
`total_amount` 等于 `base_amount`；`tax_rate_percent` 恒为 `0`，是废弃字段。

**计价分组优先级不是渠道路由顺序。** `pricing_group_order` 决定哪些
渠道+模型能力具备采购分组资格，并提供该能力的 `group_ratio`；它不决定请求落到
哪个资源池，也不决定池内渠道的先后。资源池的渠道顺序才是唯一的路由顺序。当前
配置为单分组 `["default"]`，倍率 `1`，因此正常情况下 `group_ratio` 恒为 `1`。

### 3.2 金额计算口径

```
base_amount  = PreGroupQuota / QuotaPerUnit
total_amount = base_amount × group_ratio
```

**税额乘数已取消**：V0.10 起不存在供应商税率，`total_amount` 不再乘以
`(1 + tax_rate_percent / 100)`，`tax_rate_percent` 在报文中恒为 `0`。Harness 若已
按旧公式计算，需要去掉税额项，否则会与账本不一致。

**关键**：`base_amount` 在**客户分组倍率生效之前**取得，因此采购成本与客户售价
完全解耦。修改客户分组倍率不影响采购成本。

### 3.3 金额口径的重要限制

`total_amount` 是**网关按自身配置的模型价格和采购倍率计算出的金额**，
**不等于**供应商最终出具的实际账单。

Harness 应将其视为「网关计价结果」，而非「供应商已确认的实付金额」。

### 3.4 图片任务的 B1/B3 口径

图片生成任务与文本任务共用 B1/B3，但完成判定不同：

- **B1 只有用量，没有 `status` 字段。** 图片任务不按 Token 计费；Nano Banana
  异步任务的 B1 `usage` 中 Token 为 0，可能先出现一条 `usage`（0 Token），结算后
  再出现一条 `usage_correction`（0 Token）。GPT Image 同步请求可能携带上游返回的
  Token 用量（实测 99 input / 7,028 output），但同样不能据此判断完成。
  **不要用 B1 判断任务是否完成。**
- **B3 才带 `status`。** 成功的图片任务最终会产生 `status_change`，`status` 为
  `settled`，`billing.total_amount` 为按张价格（如 Nano Banana 2.1 1K 为
  `0.009`，GPT Image 2.6 为 `0.018`）。
- **完成与成本以 B3 为准。** `image_count` 为本次生成张数（当前只验证 `n=1`），
  用于网关按张计费，不通过 B1 暴露。

---

## 四、事件类型规则

### 4.1 B1 用量事件类型

| 场景 | event_type |
| --- | --- |
| 首次用量记录 | `usage` |
| 已有记录，用量被更正 | `usage_correction` |
| 零 token 任务结算收尾（Nano Banana 等） | `usage_correction` |

**判定逻辑**：有前序事件即为 `usage_correction`。通常只有用量实际变化时才追加；
零 token 的异步任务在结算为 `settled` 时会追加一条同值的 `usage_correction` 作为
B1 收尾。B1 没有 `status` 字段，完成与成本仍以 B3 为准。

### 4.2 B3 成本事件类型

| 场景 | event_type |
| --- | --- |
| 首次成本记录（无论初始状态） | `cost` |
| 金额变化（含倍率导致的 total 变化） | `cost_correction` |
| 金额不变，仅状态变化 | `status_change` |
| 已有记录 → `refunded` | `refund` |

**判定优先级**：

1. 无前序事件 → `cost`
2. 状态为 `refunded` → `refund`
3. 金额任一字段变化 → `cost_correction`
4. 其余 → `status_change`

**「金额变化」包含**：`base_amount`、`total_amount`、`group_ratio`、`currency` 任一变化。
`tax_rate_percent` 恒为 `0`，不会再产生变化，保留在字段表中仅为兼容。

这很重要：修改采购倍率时 `base_amount` 不变，但 `total_amount` 会变，
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
| `usage_unit` | **只产生 `token`**；Nano Banana 异步图片任务 Token 为 0，GPT Image 同步可能带上游 Token |
| `currency` | **固定 `USD`** |
| 计费对象 | 文本按 Token；图片按张（`image_count`，当前只验证 `n=1`）；`plan` 池按次计费渠道尚未接入 |
| 独立工具计费 | 不做 |
| 图像计费 | **已接入**：B3 按张计价，`0.009` / `0.011` / `0.016` / `0.018` USD/张 |
| 音频计费 | 不做 |
| `status = failed` | 内部状态，**不会出现在 B3** |

### 6.1 关于 usage_unit

第四轮文档列出 `token` / `image` / `audio_second` / `character` / `request` / `other`
是为未来定义可选值。**本期实际只会产生 `token`**，其他值不会出现。Nano Banana
异步图片任务同样产生 `token` 单位的用量事件（Token 为 0）；GPT Image 同步请求
可能带上游 Token。按张计费只体现在 B3 金额，不引入 `image` 单位。

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
| 2026-11-05 | V0.10 | 金额口径改为 `total_amount = base_amount × group_ratio`，取消供应商税率；`tax_rate_percent` 恒 0 并标记废弃；事件类型判定去掉税率项；补充 `group_ratio` 由网关按资源池决定 |
| 2026-11-06 | V0.10（修订） | 更正 `group_ratio` 来源：来自采购计价分组 `pricing_group_order`，不由资源池决定；明确计价分组优先级是采购资格/倍率分组，不是渠道路由顺序；当前单 `default` 分组、倍率 1 |
| 2026-11-14 | V0.11 | 新增「图片任务的 B1/B3 口径」：B1 无 `status`；Nano Banana 异步任务 Token 为 0，GPT Image 同步可携带 Token；完成与成本以 B3 `status_change`/`settled` 为准；`image_count` 按张计费且不在 B1 暴露；更新本期范围中的图像计费状态 |
