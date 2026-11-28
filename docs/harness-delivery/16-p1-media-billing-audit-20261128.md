# P1-0 媒体计量只读审计（2026-11-28）

**性质**：只读审计，不改代码、不改数据库、不创建 P1 开发分支。\
**生产基线**：`harness-api-20261127-7868b83`，A3 `catalog_version=v12`。\
**审计范围**：media 池 76 个条目（video 49 + plan 17 + image 10）；按唯一模型 ID 去重后同为 76 个。\
**数据来源**：生产只读 `billing_setting.billing_expr` / `ModelPrice` / `ModelRatio` / `abilities` / `channels` / `harness_settlements` / `harness_ledger_events`，本地代码与生产日志；未对 76 个模型逐个付费调用。

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
