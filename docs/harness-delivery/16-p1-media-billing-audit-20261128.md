# P1-0 媒体计量只读审计（2026-11-28）

**性质**：只读审计，不改代码、不改数据库、不创建 P1 开发分支。\
**生产基线**：`harness-api-20261127-7868b83`，A3 `catalog_version=v12`。\
**审计范围**：media 池 76 个条目（video 49 + plan 17 + image 10）；按唯一模型 ID 去重后同为 76 个。\
**数据来源**：生产只读 `billing_setting.billing_expr` / `ModelPrice` / `ModelRatio` / `abilities` / `channels` / `harness_settlements` / `harness_ledger_events`，本地代码与生产日志；未对 76 个模型逐个付费调用。\
**P1-A 状态**：2026-11-28 冻结；A1 已合并 `c6ae2a125`（MySQL 生产口径验证通过）；A2 第一切片已合并 `c5544b5c7`，Package/GHCR 成功，A2 整体未验收；PostgreSQL 待补。冻结基线、wire 契约与验收矩阵见 §10。

## 1. 结论

1. New API 的**内部计价能力已经存在**：图片张数、视频秒数、按次固定价、阶梯表达式、结算金额都能算。
2. 缺口在**计量事实到 B1/B3 的传递**：当前 49 个 video + 17 个 plan 的 B1 都是 `usage_unit=token`、token 字段 `null`、`split_known=false`、`usage_source=unavailable`；B3 只有 `billing_basis=token` + 金额，没有 quantity/components。图片 B1 不同：GPT Image 有上游 token 数据、Nano Banana 有估算/上游 token 数据，但都没有 `image_count`；B3 同样没有 image quantity。总述不能把图片说成 token 全空。
3. 不是所有模型都能直接复用：视频 49 个条目中，14 个 A 型（实际秒可用，只缺 B1/B3），19 个 B 型（实际输出 + 估算参考混用），8 个 C 型（完成实测被结算跳过），8 个 D 型（上游完成秒缺失/不确定）。Plan 17 个是 per_call 固定表达式，主要缺 B3 的 `per_call` + quantity。
4. 图片 10 个：GPT Image 4 个内部已按实际 `data[]` 数量调整 ModelPrice（A+C）；Nano Banana 6 个当前固定 1 张/请求，`candidateCount` 未进入计价（C+D，若未来上游支持多图则升级 B+C）。
5. **P0.1 已热修**：Plan 池的 `seedance2.5-flash-480p-once-2` / `-720p-once-2` 在 A3 和渠道 69 中存在，但 `supplyvideo` 插件原先只声明 `-once-1`；生产真实调用返回 400 `invalid_api_platform`。已补插件声明（提交 `ae5b00b6d`，镜像 `harness-api-20261128-ae5b00b` 已部署），生产零费用校验已通过；尚未跑 $0.85/$1.20 的真实生成完成结算。
6. 媒体账本扩展必须 additive：历史行保持 token；B1 新增 `media_usage`，B3 新增 `billing_quantity`/`billing_components`；顶层 `billing_basis` 的新枚举必须提前通知 Harness，`mixed` 不能单方面启用。

## 2. 分级口径

本报告使用多标签，不互斥：

- **A 直接复用**：内部已有完整事实/金额，只缺 B1/B3 暴露。
- **B 补充适配**：上游有数据或已有 completion facts，但内部没有正确使用/结算。
- **C 契约扩展**：内部金额已算出，B1/B3 缺对应商业表达。
- **D 能力受限**：上游本身不提供所需数据，或当前不支持多图/多秒。
- **E 计价口径待确认**：请求估算、实测、按次、参考秒等口径未冻结。

## 3. 图片模型能力表（10）

| 模型 | 协议 | 请求数量 | 实际数量来源 | 内部最终计费用实际数 | 当前 B1 | 当前 B3 | 标签 | 证据 |
|---|---|---|---|---|---|---|---|---|
| `gpt-image-2.5-flare` / `-2.5-sunburst` / `-2.6-flare` / `-2.6-sunburst` | OpenAI Images 同步 | `n`，转换/覆盖后的出站 n | 非流式 `data[]` 中带非空 `url`/`b64_json` 的条目数；SSE 按 completed 事件数 | 正常完成时是：`UpdateImageCount` 写回 `otherRatios["n"]`，ModelPrice 按实际张数结算；空/解析失败/中断回退请求 n | `usage_unit=token`，token 字段为上游 token/估算；无 `image_count` | `billing_basis=token` + 金额；无 `image_count`/quantity | **A+C**；`n>1` 当前上游 400（多图能力 D，不影响 n=1） | `relay/channel/openai/relay_image.go`、`relay/image_handler.go`、`relay/common/relay_info.go`、`service/text_quota.go` |
| `Nano Banana 2.1 1K/2K/4K`、`Nano Banana Pro 1K/2K/4K` | Gemini 原生同步 `generateContent` | `candidateCount` 透传；当前计费未读取 | `candidates[].parts[].inlineData` 计数；实际调用 `candidateCount=2` 只返回 1 个 candidate/1 张图 | 否：固定 ModelPrice 每次请求 1 份；parts 数量不影响 quota | `usage_unit=token`；Nano Banana B1 常为估算 token；无 `image_count` | `billing_basis=token` + 固定金额；无 `image_count`/quantity | **C+D**（当前多图能力未证实；若上游支持多图则升级 B+C） | `relay/channel/gemini/*`、`relaykit/dto/gemini.go`、`service/text_quota.go` |

### 3.1 图片实测结论

- **GPT Image**：生产历史 B1/B3 只有 token + 金额，例如 `gpt-image-2.6-flare` B1 `89/7028/7117` tokens、B3 `0.018`；consume log 有 “Image count 1”，但 `other` 无结构化 `image_count`。真实 `n=2` 调用返回 **400 `n must be 1; multiple images per request are not supported by the upstream image_generation tool yet`**，quota 0，无 settlement。当前上游 GPT Image 只能按单张调用。
- **Nano Banana**：真实 `candidateCount=2` 调用返回 HTTP 200，但 `candidates=1`、`inlineData` 1 张，结算 `0.009`、B1 token `13/1024/1037`；没有 `image_count`。当前应视为单图模型，不能对外承诺一次多张。

## 4. 视频池 49 模型计费事实表

所有 49 个当前 B1 都是 `usage_unit=token` / token 字段 null / `split_known=false` / `usage_source=unavailable`；B3 都是 `billing_basis=token` + 金额。下表省略重复的 B1/B3 形状。

| 组 | 模型 | 表达式/价格 | 计价单位 | 请求事实 | 完成事实/来源 | 内部用实际 | 标签 |
|---|---|---|---|---|---|---|---|
| A | `grok-video1.5-720p-second`；`happyhorse1.1-720p/1080p/720p-edit-second`；`wan3.0-video-480p/720p/1080p`、`wan3.0-video-prime-480p/720p/1080p`；`kling4-720p/1080p-second-no-video`、`kling4-720p/1080p-second-with-video`（共 14） | `output_seconds * rate`；kling with-video 使用 input/output 三元式 | video_second | 提交 facts 为 0/0 | 上游 `usage.output_seconds`/`input_seconds`；实测量 | 是 | **A+C**；kling input 缺失时保持 0，可能少计 input_longer 档 |
| B | `seedance2-1080p-second-1/-2`、`seedance2-480p-second-1/-3`、`seedance2-4k-second`、`seedance2-720p-second-1`、`seedance2-fast-480p-second-1`、`seedance2-fast-720p-second-1`、`seedance2-mini-480p-second`、`seedance2-mini-720p-second`、`seedance2.5-1080p-second-1`、`seedance2.5-480p-second-1/-3`、`seedance2.5-720p-second-1`（14）；`wan3-1080p-second`、`wan3-480p-second-1/-2`、`wan3-720p-second-1/-2`（5） | `(reference_seconds>0 ? ceil(seconds+reference_seconds) : seconds) * rate` | video_second | `seconds`、`reference_seconds`（参考视频时请求秒数）、`output_seconds`、`input_seconds` 均为请求估算 | 完成 `output_seconds` 实测；`reference_seconds` 无实测时保留请求估算 | 输出实测，参考混用 | **B+C+E**：需把实测输出/估算参考拆开标源 |
| C | `seedance2-480p-second-2`、`seedance2-720p-second-2`、`seedance2-fast-480p-second-2`、`seedance2-fast-720p-second-2`、`seedance2.5-480p-second-2`、`seedance2.5-480p-second-4`、`seedance2.5-720p-second-2`、`seedance2.5-720p-second-3`（8） | 无 expr；`ModelPrice p × seconds`（隐式倍率） | video_second（隐式） | 只返回请求 `seconds` | 完成 facts 会提取，但 `PerCallBilling` 跳过结算 | 否：冻结请求秒数，忽略完成实测 | **B+C+E**：内部对实际输出不重算；带参考视频时可能少收参考秒 |
| D1 | `MiniMax-H3-Max-480p`、`-480p-first-last-frame`（0.012/s）；`-768p`、`-768p-first-last-frame`（0.024/s）；`muse-video-720p`（0.012/s） | `u("seconds") * rate` | video_second | `{seconds:请求值}` | 完成 `usage.seconds` 有则实测，无则 state/请求估算；真实 MiniMax 5s 完成响应无 usage | 有 usage 时实际，否则估算 | **C+D+E**：上游常无 usage；真实任务按请求 5s 结算，B1 应标 unavailable，B3 应 quantity 5 estimated |
| D2 | `seedance2.5-flash-480p-second-1`、`-720p-second-1`、`-720p-second-2`（3） | 分别 `0.09/0.05`、`0.13/0.065`、`0.06` | video_second | 四事实估算 | 上游通常不报 output/input 秒；有 `usage.seconds` 才更新 | 参考/输出可能估算 | **C+D+E**：Flash 上游无实测秒，只能估算 |
| 合计 | 49 | | | | | | A=14，B=19，C=8，D=8 |

### 4.1 生产实测补充

- `MiniMax-H3-Max-480p` 真实 5s 任务：提交 200，任务 `SUCCESS`，quota 30000，settlement `0.06`；内部 `billing_context.tiered_snapshot.usage_facts={"seconds":5}`，日志 `usage_facts={"seconds":5}`；上游 `data` 无 `usage`。B1 仍是 token/unavailable。→ **内部按请求 5s 结算，B1 无法说明这是估算，B3 无法说明 5s 来源**。
- `seedance2-mini-480p-second` 生产事件：B1 token/0/unavailable，B3 token 0.16；表达式 4s×0.04。→ **内部金额对，账本数量缺**。
- `seedance2-fast-720p-once-1` 生产事件：B1 token/0/unavailable，B3 token 0.52；Plan per_call。→ **B3 应 per_call + quantity 1**。
- **TA 14 个 A 型高风险**：插件提交事实故意写 `{output_seconds:0,input_seconds:0}`；如果完成响应没有 `usage`，完成 facts 为空，表达式按 0 求值，内部可能结算 0/退款，而不是按请求秒估算。这是比“只缺 B1/B3”更严重的无数据兜底缺口，P1-A 必须先冻结该族合同口径（请求估算 vs 0/退款）。
- **Token fallback 风险**：`extractUsageOnComplete` 在秒事实缺失但响应带 `usage.total_tokens` 时会回退返回 `{tokens,totalTokens}`；此时 B1 可能变成 `usage_source=upstream` 并带 Token 总量，而表达式仍按秒估算/按零结算，B1 与 B3 语义错位。

## 5. Plan 17 模型

17 个全部是 `tier("per_video", 常量)` 固定表达式，内部按次金额正确；当前 B1 仍 token/unavailable，B3 token + 固定金额。

| 组 | 模型 | 表达式 | 内部 | 标签 |
|---|---|---|---|---|
| PAI 按次 | `seedance2-720p-once-1/-2/-3`、`seedance2-fast-720p-once-1/-3/-4`、`seedance2.5-720p-once`、`seedance2.5-flash-480p-once-2`、`seedance2.5-flash-720p-once-2`（9）；`wan3-1080p-once`、`wan3-720p-once`（2） | `tier("per_video", 0.40–1.20)` | 固定金额正确 | **A+C**；`-once-2` 两个模型存在插件断链（见 5.1） |
| video-line 按次 | `gemini-omni-1.1-flash-1080p-10s`、`-1080p-10s-first-last-frame`、`-720p-10s`、`-720p-10s-first-last-frame`、`gemini-omni-1.2-flash-720p-10s`、`gemini-omni-1.2-flash-720p-15s` | `tier("per_video", 0.102–0.24)` | 固定金额正确 | **A+C** |

### 5.1 生产阻断：2 个 Plan 模型在插件中不存在

- 生产 A3/plan 与渠道 69 有 `seedance2.5-flash-480p-once-2`、`seedance2.5-flash-720p-once-2`。
- `plugins/tasks/supplyvideo/plugin.js` 只声明 `-once-1`；`isKnownModel`/`modelProfile` 对 `-once-2` 不支持。
- 生产真实调用 `seedance2.5-flash-480p-once-2` 返回 **400 `invalid_api_platform`**，未创建任务、无费用。
- 这不是 P1 账本问题，而是 **当前 A3/插件目录漂移的 P0.1 阻断项**。已补 `-once-2` 两个模型（提交 `ae5b00b6d`，镜像 `harness-api-20261128-ae5b00b`）；生产用缺 prompt 的零费用请求验证，两个模型均返回插件校验 `field prompt is required`，不再是 `invalid_api_platform`。实际付费生成/完成结算未跑，避免 $0.85/$1.20 成本；本地 `TestSupplyVideoDecodeRequest` 已覆盖 `-once-2` 解码与 per-call facts。
- 旧 `-once-1` 与其他插件历史模型未批量删除：插件支持范围与当前生产启用范围不必完全一致，避免影响历史任务和已部署实例。后续可加“A3 目录 ⊆ 插件支持”的部署前检查，而不是靠删模型对齐。
- 同批目录漂移还包括：`billing_expr` 里仍有 4 个未进入任何 video 池的 `SECOND_MODELS`（`grok-video1.5-480p-second`、`minimax-h3-max-480p/768p/1080p-second`）；插件 `models` 声明也多于生产 abilities。P0.1 应一并清理或明确标记历史。

## 6. B1/B3 账本缺口

### 6.1 当前真实 B1

```json
{
  "event_id": "ue_...",
  "event_type": "usage",
  "request_id": "9e71f659-...",
  "gateway_request_id": "202611270807...",
  "run_id": "9e71f659-...",
  "usage_unit": "token",
  "usage": {
    "input_tokens": null,
    "output_tokens": null,
    "cache_read_tokens": null,
    "cache_write_tokens": null,
    "split_known": false
  },
  "usage_source": "unavailable",
  "is_estimated": false,
  "observed_at": "2026-11-27T08:07:10.046Z",
  "record_version": 1,
  "supersedes_event_id": null
}
```

### 6.2 当前真实 B3

```json
{
  "event_id": "ce_...",
  "event_type": "status_change",
  "cost_id": "cost_202611270807...",
  "request_id": "9e71f659-...",
  "gateway_request_id": "202611270807...",
  "status": "settled",
  "currency": "USD",
  "billing": {
    "base_amount": "0.16",
    "group_ratio": 1,
    "tax_rate_percent": 0,
    "total_amount": "0.16"
  },
  "billing_basis": "token",
  "procurement_price_version": "pw-...",
  "record_version": 1,
  "supersedes_event_id": null,
  "observed_at": "2026-11-27T08:07:10.125Z"
}
```

### 6.3 缺口表

| 代表 | 内部真实事实 | 当前 B1 | 当前 B3 | 应补 |
|---|---|---|---|---|
| `seedance2-mini-480p-second` | 表达式 4s×0.04，settlement 0.16 | token/0/unavailable | token/0.16 | B1 `media_usage.video.output_seconds=4`（upstream）或 unavailable；B3 `billing_basis=video_second`、quantity 4、source |
| `MiniMax-H3-Max-480p-first-last-frame` | 内部 usage_facts seconds=5，settle 0.06；上游 data 无 usage | token/0/unavailable | token/0.06 | B1 video unavailable/null；B3 video_second quantity 5 source estimated |
| `wan3-480p-second-1` | 表达式 4s×0.04，settle 0.16 | token/0/unavailable | token/0.16 | 同 A 型 |
| `seedance2-fast-720p-once-1` | per_video 0.52 | token/0/unavailable | token/0.52 | B3 per_call quantity 1 source request |
| `gpt-image-2.6-flare` | ModelPrice 0.018×1，B1 token 89/7028/7117 | token/token | token/0.018 | B1 media_usage.images count 1 source observed；B3 image quantity 1 |
| `Nano Banana 2.1 1K` | ModelPrice 0.009×1，B1 token 13/1024/1037 | token/token | token/0.009 | B1 image count 1 observed；B3 image quantity 1 |

### 6.4 根因

- `harnessSettlementCorrelationFromContext` 默认 `UsageUnit: token`；`recordTaskHarnessSettlement` 直接构造 `harnessSettlementCorrelation{UsageUnit: harnessUsageUnitToken}`。
- 任务结算只接收 token total，不接收 seconds/image_count；tiered 完成路径 `RecalculateTaskQuota` 不带 usage 参数。
- B1 事件只复制 token 字段；`harnessUsageDiffers` 不比较 media 字段。
- B3 事件只复制金额；没有 quantity/components。
- DB 表 `HarnessLedgerEvent` 没有 media quantity/components 列，wire DTO 也没有字段。

## 7. 建议的 additive 契约

**B1**（保留原 `usage` 不动）：

```json
{
  "usage": {"input_tokens": 10, "output_tokens": 0, "total_tokens": 10, "split_known": true},
  "media_usage": {
    "images": {"count": 1, "unit": "image", "source": "observed"},
    "video": {"input_seconds": null, "output_seconds": null, "unit": "video_second", "source": "unavailable"}
  }
}
```

- 原 `usage_unit` 保持 `token` 历史语义；媒体维度各自带 `source`。
- `source` 枚举：`upstream`（上游 usage）、`observed`（网关从响应/产物计数或直接观测）、`estimated`（请求参数/规则估算）、`unavailable`（无数据）。
- 实际视频时长归 B1；最终计费时长归 B3，不混用。

**B3**（保留金额权威）：

```json
{
  "billing_basis": "video_second",
  "billing_quantity": "5",
  "billing_quantity_source": "estimated",
  "billing_components": [
    {"basis": "video_second", "quantity": "5", "source": "estimated"},
    {"basis": "reference_video_second", "quantity": "0", "source": "request"}
  ],
  "billing": {"base_amount": "0.06", "total_amount": "0.06"}
}
```

- `billing_basis` 新枚举必须提前给 Harness allowlist；`mixed` 只在双方确认后使用。
- `billing_components` 只承诺 basis/quantity/source 可追溯，不承诺每个组件的金额拆分；金额仍以 `billing.base_amount/total_amount` 为权威。
- Plan：`billing_basis=per_call`、`billing_quantity="1"`、`billing_quantity_source="request"`。
- 历史行保持 `billing_basis=token`，不回填、不改写。

## 8. 实施边界建议

- **P0.1（已完成热修，独立于 P1）**：`supplyvideo` 插件已补 `-once-2` 两个模型（`ae5b00b6d`），镜像 `harness-api-20261128-ae5b00b` 已部署；生产零费用路由校验通过。旧模型不批量删除；建议后续增加 A3 目录与插件支持范围的一致性检查。
- **P1-A（媒体用量与账本扩展）**：B1 `media_usage`、B3 `billing_quantity/components`；图片/视频事实贯通；先做图片和 Plan per_call，再做视频秒。
- **P1-B（异步结算可靠性）**：Pending、not_dispatched、dispatched_unknown、idempotency、未知结果、退款、修正；高风险，单独交付。
- P1-A 与 P1-B 不捆绑；P1 单独报价、单独确认，不并入原 ¥4,000/20 天 P0 范围。

## 9. 本次审计未覆盖与付费调用

- 76 个模型的逐个付费调用（只做了 Nano Banana 单图/双图尝试、GPT Image n=2 拒绝、MiniMax 5s 真实任务；其余用代码/生产历史/表达式矩阵）。
- 本次付费调用成本：MiniMax 5s 结算 `0.06 USD`；Nano Banana 2.1 1K 两次各 `0.009 USD`；GPT Image `n=2` 上游 400，quota 0。合计约 `0.078 USD`。
- PNG/WebP 真实上游样本（P0 已有定向测试；本审计未再调用）。
- 上游合同层面的“按请求秒还是实际输出秒”最终口径；表中 E 类需双方确认。

---

## 10. P1-A 冻结基线（2026-11-28）

> 本节是 P1-A 的最终开发基线。§7 的早期示例（媒体对象共用 `source`、B3 只有 `quantity`）由本节“每条事实独立携带 source”的 facts 数组取代；历史审计结论不回改，以本节为执行口径。B3 字段属于 A2 实现，本节只是 wire 草案；A1 只交付 B1。

### 10.1 内部 Facts Schema

内部事实只描述现实，不包含计费结果。`billing_quantity` / `billing_components` 是结算派生层，不能写回 facts。

```json
{
  "version": 1,
  "facts": [
    {
      "name": "requested_seconds",
      "dimension": "video",
      "quantity": "10",
      "unit": "video_second",
      "source": "request"
    },
    {
      "name": "output_seconds",
      "dimension": "video",
      "quantity": "8.5",
      "unit": "video_second",
      "source": "upstream"
    }
  ]
}
```

来源枚举（每条事实独立）：

| source | 含义 |
|---|---|
| `upstream` | 上游明确返回 |
| `observed` | 网关从响应/产物实际观测 |
| `request` | 请求参数明确提供 |
| `estimated` | 按规则估算 |
| `unavailable` | 无法获取 |

数量规则：

- 内部统一十进制字符串，`decimal` 规范化（`10.0` → `10`，`8.50` → `8.5`）。
- 未知用 `null`，禁止写成 `0`；`source=unavailable` 时 `quantity` 必须为 `null`。
- facts 按 `(dimension, name)` 排序，保证同一事实集合的序列化稳定，供 diff 比较。
- 图片事实：`name=count`、`dimension=image`、`unit=image`、`source=observed`。
- 视频事实：`requested_seconds` / `input_seconds` / `output_seconds` / `reference_seconds`，`unit=video_second`。

### 10.2 单一开关与单笔生命周期

- 开关 key：`HarnessMediaLedgerEnabled`（`common.OptionMap`），默认不存在即关闭。
- 采样点：`genBaseRelayInfo` 创建 `RelayInfo` 时采样一次，覆盖整个请求（含重试与流式）。
- 异步任务：提交时把开关与 facts 写入 `TaskBillingContext.MediaLedger` / `MediaFacts`；完成路径从 task 行读取，跨进程、重启可恢复。
- 同一开关同时控制**事实捕获、B1 diff、wire 输出**。只关 wire 会改变事件条数与 `record_version`，不满足兼容要求。
- 关闭时：B1/B3 JSON、事件条数、`record_version` 与旧版完全一致；成本行不带 `media_usage`；历史行不回填。
- 打开只影响新请求；在途任务按提交时状态执行；A1 与 A2 共用一个生产开关，测试环境同一机制打开。
- Facts 是只增的观测记录：后续 settle 不带 facts 时不得清空已发布的事实；只有非空且变化的事实列表才触发新的 B1 版本。
- A1 的任务事实只覆盖 `/v1/videos`（video/plan）；异步图片任务与音频任务不写入视频事实，避免维度错标。
- 生产默认关闭，A1/A2 验收完成、Harness 契约确认后才统一启用。

### 10.3 B1 wire 契约（A1）

`media_usage` 是 B1 的 additive 字段，仅在 facts 非空时出现；为空时省略，保持旧版 JSON 不变。顶层 `usage_source` 继续只描述 token 维度，媒体来源由每条 fact 自己的 `source` 表达。

字段说明：

- `version`：facts schema 版本，当前为 `1`。
- `facts[]`：每条包含 `name`、`dimension`、`quantity`、`unit`、`source`。
- `quantity`：图片张数为 JSON 整数（如 `1`），视频秒数为十进制字符串（如 `"8.5"`），未知为 `null`。
- `source`：五枚举之一（见 §10.1）。

**示例 1：GPT Image（实际 1 张，token 已有）**

```json
{
  "usage_unit": "token",
  "usage": {"input_tokens": 89, "output_tokens": 7028, "split_known": true},
  "usage_source": "upstream",
  "media_usage": {
    "version": 1,
    "facts": [
      {"name": "count", "dimension": "image", "quantity": 1, "unit": "image", "source": "observed"}
    ]
  }
}
```

**示例 2：视频正常完成（输出实测 8.5s，参考 2s 来自请求）**

```json
{
  "usage_unit": "token",
  "usage": {"input_tokens": null, "output_tokens": null, "split_known": false},
  "usage_source": "unavailable",
  "media_usage": {
    "version": 1,
    "facts": [
      {"name": "output_seconds", "dimension": "video", "quantity": "8.5", "unit": "video_second", "source": "upstream"},
      {"name": "reference_seconds", "dimension": "video", "quantity": "2", "unit": "video_second", "source": "request"},
      {"name": "requested_seconds", "dimension": "video", "quantity": "10", "unit": "video_second", "source": "request"}
    ]
  }
}
```

**示例 3：Plan per_call（请求 5s，完成秒数未知）**

```json
{
  "usage_unit": "token",
  "usage": {"input_tokens": null, "output_tokens": null, "split_known": false},
  "usage_source": "unavailable",
  "media_usage": {
    "version": 1,
    "facts": [
      {"name": "output_seconds", "dimension": "video", "quantity": null, "unit": "video_second", "source": "unavailable"},
      {"name": "requested_seconds", "dimension": "video", "quantity": "5", "unit": "video_second", "source": "request"}
    ]
  }
}
```

**示例 4：TA 无 usage（请求 10s，输出未知；零结算仅识别，不修复）**

```json
{
  "usage_unit": "token",
  "usage": {"input_tokens": null, "output_tokens": null, "split_known": false},
  "usage_source": "unavailable",
  "media_usage": {
    "version": 1,
    "facts": [
      {"name": "output_seconds", "dimension": "video", "quantity": null, "unit": "video_second", "source": "unavailable"},
      {"name": "requested_seconds", "dimension": "video", "quantity": "10", "unit": "video_second", "source": "request"}
    ]
  }
}
```

B3 对应：`status=settled`、`total_amount=0`、不输出 `billing_quantity`。测试固定断言该组合，交付报告标记“已识别、未修复”；不得宣称 TA 零结算风险已解决。

**示例 5：失败退款**

B1 照实记录提交时已知事实（如 `requested_seconds` / `output_seconds=null`）；B3 `status=refunded`、金额 0、不输出 `billing_quantity`，不得用 `per_call` 或估算秒数伪造计费数量。

### 10.4 B3 wire 草案（A2，不随 A1 交付）

保留金额权威：`billing.base_amount` / `billing.total_amount` 不变。新增：

- `billing_basis`：A2 allowlist 草案为 `token`、`image`、`video_second`、`reference_video_second`、`per_call`；`mixed` 仅在 Harness 明确同意后使用。
- `billing_quantity`：十进制字符串，必须能解释金额。
- `billing_quantity_source`：五枚举之一。
- `billing_components[]`：`basis` / `quantity` / `source`；复杂计价拆组件，金额仍以 `billing` 为准。
- `cost_calculation_version`：内部算法版本（DB 已有）。
- `procurement_price_version`：沿用现有字段名，不重命名。
- `billing_expr_hash` / `billing_expr_version`：规则标识；完整表达式只存内部账本，不上 wire。

**示例：PAI C 组（请求 10s、实测 8s、按请求 10s 计费，金额不动）**

```json
{
  "billing_basis": "video_second",
  "billing_quantity": "10",
  "billing_quantity_source": "request",
  "billing_components": [
    {"basis": "video_second", "quantity": "10", "source": "request"}
  ],
  "billing": {"base_amount": "0.40", "total_amount": "0.40"},
  "procurement_price_version": "pw-...",
  "cost_calculation_version": "task_frozen_v1",
  "billing_expr_hash": "...",
  "billing_expr_version": 1
}
```

**示例：Plan per_call**

```json
{
  "billing_basis": "per_call",
  "billing_quantity": "1",
  "billing_quantity_source": "request",
  "billing_components": [
    {"basis": "per_call", "quantity": "1", "source": "request"}
  ]
}
```

**示例：TA 无 usage 的 settled 零金额**

`billing_quantity` 省略或 `null`；不得出现“估算 5 秒 + 金额 0”的矛盾组合。该记录是零结算异常的审计证据，不是修复。

**能力边界**：即使向 Harness 提供完整表达式，离线复算仍需要价格表与求值器。对外只承诺“数量、金额、版本、规则标识可核对”，不承诺“完全独立复算”。完整表达式保留在内部账本（A2 增加 `billing_expr` 列），必要时走受控审计导出，不进 B3 wire。

**历史兼容**：历史行保持 `billing_basis=token`，不回填、不改写。

### 10.5 A1 / A2 范围

| 阶段 | 交付 |
|---|---|
| A1 | 内部 facts 模型、单开关、持久化、GPT Image / Nano Banana 张数、Plan 提交事实、B1 `media_usage`、diff/版本链 |
| A2 | 视频 49 模型事实（TA 14、PAI B 19、PAI C 8、D 组 8）、完成轮询观测、B3 `billing_basis`/quantity/components、表达式快照列、三库完整回归 |

D 组规则：有上游 `usage` 记 `upstream`，没有才回退 `request`/`estimated` + `output_seconds=unavailable`；不得一律标记 unavailable。C 组完成秒数需要轮询路径独立捕获，但金额继续按现有请求秒规则，不改变扣费。

### 10.6 验收矩阵

| # | 门槛 | 验证方式 |
|---:|---|---|
| 1 | 49 个视频模型用量与来源分类正确 | 代码路径/夹具覆盖 A/B/C/D 全部组；每类代表模型真实上游调用；不做 49 次付费生成 |
| 2 | B1/B3 数量与计费依据一致 | B3 quantity 能由 B1 facts 或请求事实推导；推不出时省略 quantity，不强行相等 |
| 3 | 金额、扣费、退款不变 | 同输入/同配置前后对比；GPT Image、Nano Banana、Plan、TA、PAI B/C/D 各代表；含 TVC 退款路径 |
| 4 | 开关关闭完全兼容 | B1/B3 JSON、事件条数、`record_version` golden；历史行不回填 |
| 5 | 三库与回滚 | SQLite/MySQL/PostgreSQL 全新建库、升级、二次启动；新列可空/默认空，旧镜像可跑新 schema |
| 6 | TA 零结算留痕 | 固定断言 `status=settled`、`total_amount=0`、无 `billing_quantity`；报告标记“已识别、未修复” |

### 10.7 P1-B 边界与不做项

P1-A 包含：pending→settled 的正常版本递增、facts 变化触发的正常 diff、`supersedes_event_id` 版本链。

P1-B 仍独立冻结：未知受理、重复提交/幂等、异常退款、人工对账、复杂修正编排。

本期不做：媒体计费口径变更（TA 零结算修复、PAI C 组实际秒改价）、完整表达式对 Harness 开放、P1-B 状态机。

### 10.8 A1 实现与合并状态（2026-11-28）

> **状态更新（2026-11-28）**：A1 已合并到 `harness-api`（`c6ae2a125`），`Package harness-api` 与 GHCR 镜像发布成功（`harness-api-20261128-c6ae2a1` / `harness-latest`）；MySQL 8.0.43 账本迁移/升级/读写通过；生产未部署，开关默认关闭；PostgreSQL 未验证。以下开发过程记录保留，不覆盖历史。

已实现（分支 `feat/harness-p1a-media-ledger`）：

- `relay/common/media_facts.go`：facts schema、五来源校验、十进制规范化、canonical JSON。
- `setting/harness_media.go`：`HarnessMediaLedgerEnabled` 单一开关。
- `relay/common/relay_info.go`：请求级采样、`MediaFacts`、`RecordObservedImageCount`（GPT Image 走 `UpdateImageCount`）。
- `relay/channel/gemini/*`：native/chat/responses 同步、流式与 Imagen 的 inlineData 张数捕获；只记录事实，不进入计价。
- `controller/relay.go` + `model/task.go`：Plan/任务提交事实持久化（仅 `/v1/videos`；`requested_seconds` 有则记、`output_seconds=unavailable`）。
- `model/harness_ledger.go` + `service/harness_ledger_query.go` + `service/quota.go`：B1 `media_usage`、diff、版本链、开关关闭零变化。
- 评审修正：B1 因 token/usage 变化产生的新版本继承已发布 facts（不会被空事实列表擦除）；`image` 维度数量强制整数；Gemini 非图片 `inlineData` 不计入 image count。

已运行：

- `go build ./...`。
- `go test ./relay/common ./relay/channel/openai ./relay/channel/gemini`：facts 校验、GPT Image 同步/SSE/JSON-SSE/中断、Gemini 同步/流式/Imagen 张数捕获。
- `go test ./service`（全量）：B1/B3、版本链、开关关闭零变化、金额不变。
- `go test ./controller -run TestTaskMediaFactsCapturesRequestFacts`：Plan/任务提交事实。
- `go test ./model -run 'TestHarnessLedgerMigrateIsIdempotent|TestHarnessLedgerMediaFactsRoundTripAcrossDialects|TestHarnessLedgerMediaFactsColumnUpgradesLegacyTable|TestTaskBillingContextMediaFactsJSONRoundTrip'`：SQLite + **MySQL 8.0.43** 的 AutoMigrate 两次、旧表升级加列、新列写入/读取、任务 private_data 重启恢复均通过；PostgreSQL 在本机跳过（无服务/psql/WSL/Docker）。
- `go test ./router -run TestHarness`：Harness 路由端到端回归；`TestHarnessGatewayEndToEnd` 通过真实 HTTP 断言 B1 `media_usage` 的图片张数为 JSON 整数。
- `go test ./relay`：任务/图片 relay 回归。
- 全量 `go test ./...` 在本机运行中内存耗尽中断（runtime fatal，不是测试断言失败）；受影响包已改为 `-p 1` 顺序执行并通过。全量回归仍需在 CI 或高内存环境重跑。

生产环境使用 MySQL；本次 MySQL 8.0.43 实库的 AutoMigrate、旧表升级、写入/读取已通过。PostgreSQL 仍是项目三库规范的一部分，补齐前不能宣称三库兼容。

未完成：PostgreSQL 实库验证（本机 `TEST_POSTGRES_DSN` 未配置、无 PostgreSQL 服务/WSL/Docker）、全量回归、Harness 联调。按生产 MySQL 口径 A1 已闭环；按项目三库规范，当前不得宣称三库兼容。A2 未开始。

### 10.9 A2 实现清单（2026-11-28 冻结，未开始）

A2 覆盖 49 个视频模型：TA A 组 14、PAI B 组 19、PAI C 组 8、D 组 8。不要求 49 个模型全部付费生成；代码路径/夹具全覆盖，每类代表模型做有预算的真实调用。

**1. 视频 Facts 采集**

- TA 14：上游 `output_seconds` / `input_seconds` 有则记 `upstream`，缺失记 `unavailable`；不改金额。
- PAI B 19：`output_seconds` 实测记 `upstream`，`reference_seconds` 记 `request`，两者 source 分开；表达式不变。
- PAI C 8：轮询完成观测通道独立捕获实际输出秒；B1 记录 `upstream`，金额继续按现有请求秒规则。
- D 8：有 `usage.seconds` 记 `upstream`，没有则保留 `request`/`estimated` 事实并把 `output_seconds` 记 `unavailable`；不得一律标 unavailable。

**2. 完成事实合并**

完成轮询把实际用量合并进 `TaskBillingContext.MediaFacts`，`recordTaskHarnessSettlement` 读取并进入 settlement correlation；B1 diff 正常产生 `record_version + 1` / `supersedes_event_id`，空事实不得擦除已发布 facts。

**3. B3 商业账本字段**

- `billing_basis`：allowlist 草案 `token` / `image` / `video_second` / `reference_video_second` / `per_call`；`mixed` 仅在 Harness 明确同意后使用。
- `billing_quantity` + `billing_quantity_source`。
- `billing_components[]`：每项 `basis` / `quantity` / `source`；金额仍以 `billing.base_amount` / `total_amount` 为权威。
- B3 diff：`harnessCostDiffers` 比较 quantity、source、components、basis；金额未变不得产生 cost 事件，数量变化产生正常版本链。
- 历史行保持 `billing_basis=token`，不回填、不改写。

**4. 表达式快照与版本暴露**

- `HarnessLedgerEvent` 新增 `BillingExpr`、`BillingExprHash`、`BillingExprVersion`（A2 迁移列）。
- B3 wire 新增 `cost_calculation_version`、`billing_expr_hash`、`billing_expr_version`；`procurement_price_version` 已存在，沿用字段名。
- `CostCalculationVersion`、`PricingVersion` 内部已有，不从零开发；A2 只补 B3 暴露与表达式快照持久化。

**5. 验收与测试**

- 49 模型夹具覆盖 A/B/C/D 组；代表模型真实调用有预算上限。
- 金额/扣费/退款不变，含 TVC 退款路径与 PAI C 组。
- TA ZeroSettlement：`status=settled`、`total_amount=0`、无 `billing_quantity`；标记“已识别、未修复”。
- 开关关闭 golden：B1/B3 JSON、事件条数、`record_version` 与旧版一致；历史行不回填。
- MySQL 8.0.43 + SQLite 迁移/升级/重启；PostgreSQL 待补；全量测试在 CI/高内存环境重跑。

**6. 契约沟通**

A2 开工时把 §10.3/§10.4 与最终 JSON 示例作为“实现基线通知”发给 Harness，不等审批、不停止开发；给对方一个异议窗口。`billing_basis` 新枚举、数量来源和金额解释仍需保持可对账，若对方提出差异，按具体差异协商。

**7. 不做项**

TA 零结算修复、PAI C 组实际秒改价、完整表达式对 Harness 开放、P1-B Pending/幂等/异常退款状态机，均不纳入 A2。

### 10.10 A2 第一切片进度（2026-11-28）

> 合并状态（2026-11-29）：A2 第一切片已提交 `c5544b5c7` 并推送 `harness-api`；`Package harness-api` 与 GHCR 镜像发布成功（workflow tag 规则：`harness-api-20261128-c5544b5`）；生产未部署，开关默认关闭。以下为代码与测试状态，A2 整体仍未验收。

已落地（待整体验收）：

- B3 账本列与 wire：`billing_quantity`、`billing_quantity_source`、`billing_components`、`billing_basis`、`cost_calculation_version`、`billing_expr_hash`、`billing_expr_version`；`procurement_price_version` 沿用。
- B3 diff：`harnessCostDiffers` 比较数量/来源/组件/basis/表达式 hash/version；数量变化生成 `cost_correction` 版本链，金额不变不重复计费。
- 完成事实合并：`MergeTaskCompletionMediaFacts` 在 `finalizeTerminalTask` 结算前合并 `TaskInfo.UsageFacts`；immediate terminal 在 `LogTaskConsumption` 前合并；缺失键不覆盖已发布事实。
- 基础派生：Plan `per_call`、C 组请求秒、A 组 output（缺失不伪造）、B 组 requested+reference、D 组 seconds、同步图片 observed/requested。
- 退款抑制：`refunded` B3 不输出 quantity/components/basis 媒体字段。
- TA ZeroSettlement 识别测试：`status=settled`、`total_amount=0`、无 `billing_quantity`，标记“已识别、未修复”。

已验证：

- `go test ./service`（全量）通过。
- `go test ./model`（SQLite + MySQL 8.0.43）账本迁移、旧表升级、B3 字段 roundtrip 通过；PostgreSQL 跳过。
- `go test -p 1 ./relay/common ./relay/channel/openai ./relay/channel/gemini ./router`、`./controller` 定向、`./relay` 通过。

未完成：49 个视频模型的完整夹具矩阵与每类代表模型真实上游调用（需预算）、全量 `go test ./...`（本机 OOM）、PostgreSQL 实库、Harness 联调、生产部署。A2 整体仍未验收。
