# 目录对齐与本地复测报告（2026-11-25）

**文档版本**：V1.0（2026-11-25）\
**对应网关版本**：`harness-api-20261117-b040241`\
**测试 Base URL**：`https://testapi.soloai.chat`\
**性质**：现场复测快照，不是永久合同清单；完整模型清单见 `12-retest-model-list-20261123.md`。

## 1. 结论

| 领域 | 2026-11-25 状态 |
| --- | --- |
| 渠道声明与上游目录 | **已对齐**。21 个渠道逐一与上游 `/v1/models` 比对，声明了上游不存在模型的渠道数为 0 |
| A3 目录 | `chat 26` / `video 49` / `image 10` / `plan 17` / `audio 0`；不再返回 83/80 的虚条目；`catalog_version` 会在下一次拉取时自动递增 |
| 11-23 反馈 03（目录与渠道） | **已关闭** |
| 11-23 反馈 04（视频任务失败） | **根因已关闭**。TA token 22 档与 7 个已下架型号已移出渠道声明；`seedance2-fast-720p-once-1` 已实测成功 |
| 11-23 反馈 05（图片超时/MIME） | **超时已修复，MIME 仍待处理**。OpenResty 60 秒代理超时已放宽到 600 秒；4K 公网 109.5 秒返回 200；实际 JPEG 仍可能被标为 `image/png` |
| 11-23 反馈 01/02/06/07/08 | **仍按第 13 号文档的文档修订与已知限制处理**，不因目录对齐而消失 |

## 2. 11-24 20:13 八次 HTTP 500 定位

TVC 的 4 个 Harness 请求 ID、8 次提交均在网关侧有记录；每次按 plan 池渠道顺序尝试 3 个渠道：

1. 渠道 69（pai-seedance-按次）：上游返回 Cloudflare `503 Service Temporarily Unavailable`，属于供应商瞬时故障。
2. 渠道 80（pai-wan3.0-按次）：上游返回 `model_not_found`；该分组上游实际只有 2 个 `wan3` 按次模型。
3. 渠道 82（video-line-v1-once）：上游返回 `model_not_found`；该分组上游实际只有 6 个 `gemini-omni` 模型。

三条渠道全部失败后，网关返回 500。核实结果：**未创建任务、B1/B3 均无记录、未计费**。根因是“渠道声明过宽”叠加渠道 69 的瞬时 503，导致重试落到不可能服务的渠道。

## 3. 2026-11-25 目录对齐

| 渠道 | 分组 | 对齐前声明 | 对齐后声明 |
| --- | --- | ---: | ---: |
| 66 | TA-seedance分组（token 系列） | 22 | **0**（上游该密钥已无模型） |
| 67 | TA-其他视频模型分组 | 18 | 14 |
| 69 | pai-seedance-按次 | 12 | 9 |
| 78 | TA-其他视频模型分组 | 68 | 14 |
| 80 | pai-wan3.0-按次 | 70 | 2 |
| 81 | pai-wan3.0-按秒 | 73 | 5 |
| 82 | video-line-v1-once | 74 | 6 |
| 83 | video-line-v1-second | 73 | 5 |
| 84 | 国模号池1 | 7 | 4 |

- 其余 12 个渠道原本就与上游完全一致，未改动。
- 渠道 84 只保留双方同名的 4 个（`kimi-k3`、`kimi-k3.1-pro`、`glm-5.3`、`glm-5.3-flash`）；上游独有的 `deepseek-v4.1-flash-1017`、`deepseek-v4-pro-0813` 未暴露，避免改变 Harness 现有 chat 模型名。
- `/api/pricing` 模型数由 131 降为 102，减少的 29 个均为已下架型号。

对齐后各池的 A3 可路由且可计价模型数：

| 池 | 对齐前 | 对齐后 |
| --- | ---: | ---: |
| `chat` | 26 | 26 |
| `video` | 83 | **49** |
| `image` | 10 | 10 |
| `plan` | 80 | **17** |
| `audio` | 0 | 0 |

## 4. 本地实测

**方法**：为每个上游分组创建临时测试 token，按渠道分组直连；测试完成后已删除全部临时 token。由于对齐后每个模型只剩一个渠道分组有 ability，直连命中的渠道与 plan/video 池路由一致。

### 4.1 视频（7 个上游分组各 1 个代表模型）

| 池/分组 | 模型 | 结果 | task id | 耗时 | quota |
| --- | --- | --- | --- | ---: | ---: |
| plan / pai-seedance-按次 | `seedance2-fast-720p-once-1` | SUCCESS | `task_Us2pCcqrXnCxd7OudXURpyCySFelHeaE` | 126s | 260000 |
| plan / pai-wan3.0-按次 | `wan3-720p-once` | SUCCESS | `task_vLh1z0CkKD90bUmVwLbp45qo96ai9Bgd` | 1066s | 445000 |
| plan / video-line-v1-once | `gemini-omni-1.2-flash-720p-10s` | SUCCESS | `task_QphI4sY42PJHGFgNKvh1raGUv7PSMB63` | 119s | 60000 |
| video / pai-seedance-按秒 | `seedance2-720p-second-1` | SUCCESS | `task_j96B11ERuag3c1dakqvnQD4tS0xBmpCn` | 208s | 150000 |
| video / TA-其他视频模型分组 | `kling4-720p-second-no-video` | SUCCESS | `task_wd3ch6tJIg6yemSbg8XkGnBOXp8ZDBXi` | 236s | 475000 |
| video / pai-wan3.0-按秒 | `wan3-720p-second-1` | SUCCESS | `task_BhhMqQDnugrfxjrCwwxqWMnvIIAk4jhQ` | 264s | 175000 |
| video / video-line-v1-second | `MiniMax-H3-Max-480p` | SUCCESS | `task_MOLgzFAGo10aBU4AZrTd0CHWcIDF7P4v` | 322s | 30000 |

说明：

- `wan3-720p-once` 在 95% 停留约 12 分钟，提交后 17 分 46 秒才完成 100%。**95% 不是失败，不能因此重提。**
- 7 个任务最终均为 SUCCESS，合计 quota 1,595,000（约 USD 3.19）。

### 4.2 Chat 与图片

| 模型 | 入口 | 结果 | 说明 |
| --- | --- | --- | --- |
| `deepseek-flash` | 本机直连 | HTTP 200 | 模型正常返回 |
| `Nano Banana 2.1 1K` | 公网 `testapi.soloai.chat` | HTTP 200，44.7s | 响应约 1.98MB；`inlineData.mimeType=image/png`，实际文件头 `ffd8ffe0`（JPEG） |
| `Nano Banana 2.1 4K` | 公网 `testapi.soloai.chat` | HTTP 200，109.5s | 放宽代理超时后返回，响应约 26.9MB；仍为 PNG 标注、JPEG 实际内容 |
| `gpt-image-2.6-flare` | 本机直连 | HTTP 524 | 供应商侧超时；网关侧 quota 0，未向用户计费 |

**4K 超时验证**：修复前客户端在 60.1 秒收到 OpenResty `504 Gateway Time-out`，但网关后端在 05:35:45 实际完成并扣 5500 quota。说明“客户端超时 ≠ 任务失败”，Harness 超时后必须先查 B3/产物，不能盲目重提。

### 4.3 边缘超时修复

`testapi.soloai.chat` 与 `papi.soloai.chat` 的 OpenResty 配置已增加：

```nginx
proxy_connect_timeout 60s;
proxy_send_timeout 600s;
proxy_read_timeout 600s;
```

`nginx -t` 通过并已 reload。修复后 Nano Banana 4K 公网返回 200（109.5s）。

## 5. 给 Harness 的复测要求

1. 重新拉取 A3，确认模型数变为 `chat 26` / `video 49` / `image 10` / `plan 17`，`catalog_version` 自动递增。
2. 按 `12-retest-model-list-20261123.md` 的当前清单复测；已下架 29 个型号不再作为候选。
3. 视频任务停在 95% 时继续轮询原 task id，不要重提；本次 `wan3-720p-once` 最终成功。
4. 图片请求超时或收到 504/524 时，先按请求 ID 查 B3 和产物；同步图片没有 task id，不能靠轮询替代。
5. Nano Banana 保存文件时按文件头识别 PNG/JPEG，不要只信 `mimeType` 或扩展名。
6. 异步视频 B1 仍可能是 `usage_source=unavailable`、Token 字段 `null`；费用以 B3 `settled` 金额为准。
7. 继续按第 13 号文档的 10 个联合复测用例执行。

## 6. 仍未关闭/未覆盖

- **MIME 归一**：Nano Banana 返回的实际 JPEG 仍标注 `image/png`，需要网关后续修正或双方确认由接收端按文件头识别。
- **GPT Image 供应商 524**：`gpt-image-2.6-flare` 实测遇到供应商侧 524；本网关未向用户计费，但该线路仍需供应商侧处理或改用重试策略。
- **B1/B3 单位字段**：`usage_unit` / `billing_basis` 仍为 `token`，是字段限制，不代表媒体按 token 计费。
- **A5 `count_tokens`**：仍未交付，按 404 处理。
- **全量覆盖**：本次为 7 个上游分组代表模型冒烟，不是 49 个 `video` + 17 个 `plan` 的全量逐一生成验收。

## 7. 证据与备份

- 7 个视频任务记录保存在网关 `tasks` 表，task id 见 4.1。
- 渠道与 abilities 对齐前备份：`/opt/new-api/backups/models-align-20261125-024736.sql.gz`。
- 边缘配置备份：`/opt/new-api/backups/edge-conf-20261125-053651`。
- 本文档不包含 Harness JWT、上游供应 Key、数据库密码或证书私钥。
