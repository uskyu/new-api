# Harness 网关接口交付包

**版本**：V0.9.1
**对应网关版本**：`harness-api` 分支 `ec3cdc0`（镜像 `harness-latest`）
**交付日期**：2026-10-25（2026-10-29 更新）

---

## 一、文件清单

| # | 文件 | 用途 | 阅读顺序 |
| --- | --- | --- | --- |
| 01 | [接入指南](./01-integration-guide.md) | 接口清单、路径、Scope、请求/响应样例、分页、错误码、调用示例 | 第 1 篇 |
| 02 | [鉴权配置](./02-auth-setup.md) | JWKS/iss/aud/sub 配置、JWT 校验规则、需对方提供的信息 | 第 2 篇 |
| 03 | [数据契约](./03-data-contract.md) | B1/B3 字段语义、事件类型、版本链、金额口径 | 第 3 篇 |
| 04 | [联调验收清单](./04-acceptance-checklist.md) | 逐项测试用例、预期响应、通过标准 | 第 4 篇 |
| 05 | [待确认事项](./05-pending-confirmations.md) | 三项尚未冻结的契约差异（第 4 项已定案） | **必读** |
| 06 | [对外测试摘要](./06-test-summary.md) | 各接口测试状态，区分真实/集成/协议/未完成 | 参考 |
| 07 | [验收证据](./07-acceptance-evidence.md) | 真实 JWT 下的端到端验收记录、请求 ID 与金额核对 | 参考 |

---

## 二、建议阅读顺序

**给 Harness 后端开发：**

1. 先读 **01 接入指南** —— 了解能调哪些接口、怎么调
2. 再读 **03 数据契约** —— 理解账本字段语义
3. 联调时对照 **04 验收清单** 逐项打勾

**给 Harness 技术负责人：**

1. 先读 **05 待确认事项** —— 三项需要你们决策的契约问题（第 4 项已定案）
2. 再看 **02 鉴权配置** —— 确认能提供哪些配置
3. 最后看 **06 测试摘要** —— 了解当前验证到什么程度

---

## 三、当前状态

### 已完成

| 项 | 状态 |
| --- | --- |
| 10 个接口路径实现 | ✅ |
| JWT 校验逻辑 | ✅ |
| 账本写入与事件语义 | ✅ |
| SQLite / MySQL 8.0 验证 | ✅ |
| 真实 JWT 端到端联调 | ✅ 2026-10-26、2026-10-29 两轮 |
| A3 返回资源池 + 真实模型 id | ✅ 2026-10-29 |
| A1/A2/A4/A9 文字模型调用（含流式） | ✅ 2026-10-29 |

### 已由 Harness 提供

| 项 | 状态 |
| --- | --- |
| JWKS URL | ✅ 测试环境已提供 |
| `iss` / `aud` / `sub` | ✅ `harness` / `new-api` / `harness-gateway` |
| 测试 JWT | ✅ 20 天有效期（至 2026-11-18） |

### 待双方确认

见 **05 待确认事项**，当前剩三项：

1. `usage` 字段是否允许 `null`
2. 是否接受新增 `total_tokens` / `split_known`
3. `refunded` 时 `total_amount` 可否非零

第 4 项（缺失关联头）已定案为 400 拒绝，网关已实施。

### 未完成

| 项 | 影响 |
| --- | --- |
| 更正链（`usage_correction` / `cost_correction` / `refund`） | 需构造重复结算或退款场景 |
| BatchGet 500 事件上限 | 数据量不足 |
| 视频池模型 | 两个视频池当前 `models: []`，待渠道接入 |

**Docker 镜像已完成**：`ghcr.io/uskyu/new-api:harness-latest`，已在部署实例上运行
并通过端到端验证（JWT 鉴权 → 模型调用 → 结算落库 → 账本事件 → B1/B3 读回）。
数据库部署目标为 MySQL，PostgreSQL 不在本期范围。

---

## 四、快速开始

拿到材料后最小验证路径：

**第 1 步**：确认配置能提供

参照 `02-auth-setup.md` 第二节，确认能否提供 JWKS URL、`iss`、`aud`、`sub`。

**第 2 步**：确认接口清单

参照 `01-integration-guide.md` 第一节，确认接口路径与 Scope 与预期一致。

**第 3 步**：提出确认意见

参照 `05-pending-confirmations.md`，对剩余三项待确认事项给出明确结论。

**第 4 步**：准备联调

配置到位后，按 `04-acceptance-checklist.md` 从 P0 项开始逐项验证。

---

## 五、接口速查

| 编号 | 方法 | 路径 | Scope |
| --- | --- | --- | --- |
| A1 | POST | `/v1/chat/completions` | `gateway.chat` |
| A2 | POST | `/v1/responses` | `gateway.chat` |
| A3 | GET | `/v1/models` | `gateway.catalog.read` |
| A4 | POST | `/v1/messages` | `gateway.chat` |
| A9 | POST | `/v1beta/models/{model}:{action}` | `gateway.chat` |
| B1 | GET | `/internal/v1/gateway/usage-events` | `gateway.usage.read` |
| B1-batchGet | POST | `/internal/v1/gateway/usage-events:batchGet` | `gateway.usage.read` |
| B3 | GET | `/internal/v1/gateway/cost-records` | `gateway.cost.read` |
| B3-batchGet | POST | `/internal/v1/gateway/cost-records:batchGet` | `gateway.cost.read` |
| B4 | GET | `/internal/v1/gateway/health` | 无（仅需有效 JWT） |

**B2 本期取消，未实现。**

---

## 六、联系与反馈

对本交付包的任何疑问，请按以下分类反馈：

| 类型 | 说明 |
| --- | --- |
| 契约疑问 | 字段语义、事件规则不清晰 |
| 待确认项答复 | 对 `05-pending-confirmations.md` 剩余三项的结论 |
| 缺陷报告 | 实际调用与文档不符 |
| 配置交付 | JWKS URL 等四项配置 |

---

## 七、版本说明

本文档为 **V0.9.1**，表示接口已具备对接条件：真实 JWT 端到端联调已通过，
文字模型的 A1/A2/A4/A9 与流式调用已验证，A3 可直接驱动模型选择。

**尚有三项契约待确认**（见 05），另有两类场景未覆盖（更正链、500 事件上限）。
三项定稿后发布 V1.0 正式版。

> 2026-10-29 更新要点：`model` 改为 A3 返回的真实模型 id；A3 增加
> `display_name`/`kind`/`billing`/`models`；`X-Harness-Run-Id` 改为必填且两个
> 关联头均 ≤64 字符；缺失关联头由“接受但不记账”改为 400 拒绝。
