# 图片生成调用样例（GPT Image 同步 / Nano Banana Gemini 同步）

**版本**：V0.15（2026-11-25 修订）
**日期**：2026-11-14（2026-11-17、2026-11-23、2026-11-25 修订）
**对应网关版本**：`harness-api` 分支，82 实例 `harness-api-20261117-b040241`（Nano Banana 实测证据来自 `harness-api-20261116-df87870`，图片接口未变）
**适用对象**：Harness 后端与图片生成联调

本文只说明图片生成调用。图片池 `image` 当前有 10 个模型，按协议分为两类：
GPT Image 2.5/2.6 使用同步 `POST /v1/images/generations`；Nano Banana 的正式协议是
同步 Gemini `POST /v1beta/models/{URL编码模型}:generateContent`，直接读取
`candidates[].content.parts[].inlineData.data`，**不返回 task id，也不再使用
`/v1/videos` 异步任务**。所有图片生成提交请求都要带 `X-Route-Group: image`，
使用 Harness JWT，scope 为 `gateway.chat`；提交接口还需带
`X-Harness-Request-Id`（1–64 字符）/ `X-Harness-Run-Id`（1–80 字符）。

> **2026-11-25 实测**：`testapi.soloai.chat` 与 `papi.soloai.chat` 的 OpenResty 代理读超时已放宽到 600 秒。
> Nano Banana 2.1 1K 公网 44.7 秒返回 200，4K 公网 109.5 秒返回 200；4K 响应约 26.9MB。
> `inlineData.mimeType` 仍可能标为 `image/png` 而实际文件头是 JPEG（`ffd8ffe0`），接收端必须按文件头识别。

---

## 1. 图片池与模型

2026-11-23 校准时，A3 `image` 池 `catalog_version` 为 `"10"`，返回 10 个模型。
下表价格沿用 2026-11-17 配置口径，实际金额以 B3 最新 `settled` 记录为准。

| 示例顺序 | 模型 id | 协议 | 价格（USD/张） |
| --- | --- | --- | --- |
| 1 | `Nano Banana 2.1 1K` | Nano Banana 同步（Gemini） | 0.009 |
| 2 | `Nano Banana 2.1 2K` | Nano Banana 同步（Gemini） | 0.009 |
| 3 | `Nano Banana 2.1 4K` | Nano Banana 同步（Gemini） | 0.011 |
| 4 | `Nano Banana Pro 1K` | Nano Banana 同步（Gemini） | 0.016 |
| 5 | `Nano Banana Pro 2K` | Nano Banana 同步（Gemini） | 0.016 |
| 6 | `Nano Banana Pro 4K` | Nano Banana 同步（Gemini） | 0.016 |
| 7 | `gpt-image-2.5-flare` | GPT Image 同步 | 0.018 |
| 8 | `gpt-image-2.5-sunburst` | GPT Image 同步 | 0.018 |
| 9 | `gpt-image-2.6-flare` | GPT Image 同步 | 0.018 |
| 10 | `gpt-image-2.6-sunburst` | GPT Image 同步 | 0.018 |

模型 id 必须按 A3 返回值原样传给 `model`（Nano Banana 的 id 含空格）。
`X-Route-Group: image` 是图片池的池选择头，所有图片生成提交调用都要携带，不能省略。
2026-11-23 反馈处理记录为 GPT Image 4/4、Nano Banana 4/6；Nano Banana 余下 2 款
与超时/MIME 整改相关，不应把 10 个目录条目理解为 10/10 生成验收通过。

---

## 2. 两种协议对比

| 项 | GPT Image 2.5/2.6 | Nano Banana |
| --- | --- | --- |
| 模型数量 | 4 | 6 |
| 调用方式 | 同步 | 同步（Gemini `generateContent`） |
| 提交接口 | `POST /v1/images/generations` | `POST /v1beta/models/{URL编码模型}:generateContent` |
| 查询接口 | 无（提交即返回结果） | 无（同步返回） |
| task id | 无 | **无**；不要构造或轮询视频 task id |
| 长请求 | 使用足够长的客户端超时 | 1K/4K 均可能触发 524/500；使用足够长的客户端超时 |
| 关键请求字段 | `model` / `prompt` / `n=1` / `size` | `contents` / `generationConfig.imageConfig.imageSize` / `responseModalities` / `candidateCount` |
| 结果位置 | `data[0].b64_json`（可能） | `candidates[].content.parts[].inlineData.data`（Base64） |
| 下载方式 | 解码 `b64_json` | 解码 `inlineData.data` |
| 价格 | 0.018 USD/张 | 0.009 / 0.011 / 0.016 USD/张 |
| 完成判定 | HTTP 200 且返回图片数据；结算以 B3 `settled` 为准 | HTTP 200 且拿到 `inlineData`；结算以 B3 `settled` 为准 |

---

## 3. GPT Image 同步样例

以 `gpt-image-2.6-flare` 为例，1K 尺寸为 `1024x1024`，样例使用 `n: 1`。

```bash
curl -sS -X POST "http://<gateway>/v1/images/generations" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_image_gpt_001" \
  -H "X-Harness-Run-Id: run_image_gpt_001" \
  -H "X-Route-Group: image" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2.6-flare",
    "prompt": "A red paper lantern on a wooden desk, soft studio light",
    "n": 1,
    "size": "1024x1024"
  }'
```

成功时 HTTP 200。响应图片数据在 `data[0]`，可能是 `b64_json`：

```json
{
  "data": [
    {
      "b64_json": "<BASE64_IMAGE>"
    }
  ]
}
```

- 不要依赖 `response_format=url` 取回图片；当前按 `data[0].b64_json` 解码保存。
- 样例使用 `n: 1`、`size: "1024x1024"`（1K）。
- 4 个 GPT Image 模型（2.5/2.6 的 flare/sunburst）均为 0.018 USD/张。

---

## 4. Nano Banana 同步样例（Gemini generateContent）

以 `Nano Banana 2.1 1K` 为例。模型名含空格，路径必须 URL 编码。

```bash
curl -sS -X POST "http://<gateway>/v1beta/models/Nano%20Banana%202.1%201K:generateContent" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_image_banana_001" \
  -H "X-Harness-Run-Id: run_image_banana_001" \
  -H "X-Route-Group: image" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {"role": "user", "parts": [{"text": "A banana-shaped balloon floating above a quiet beach, soft morning light"}]}
    ],
    "generationConfig": {
      "candidateCount": 1,
      "responseModalities": ["IMAGE"],
      "imageConfig": {"aspectRatio": "1:1", "imageSize": "1K"}
    }
  }'
```

成功时 HTTP 200，响应关键结构：

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "inlineData": {
              "mimeType": "image/png",
              "data": "<BASE64_IMAGE>"
            }
          }
        ]
      }
    }
  ]
}
```

- **这是正式同步图片协议：同步返回，不需要轮询，也没有 task id。** 旧异步
  `/v1/videos` 说明不代表当前 Nano Banana 合同；不要用视频 task 状态判断图片完成。
- `imageSize` 必须按模型填写：1K / 2K / 4K；`aspectRatio` 供应网关已补测 10 种
  比例（1:1、16:9、9:16、3:2、2:3、4:3、3:4、5:4、4:5、21:9，见 4.1）；请求仍用
  `generationConfig.imageConfig.aspectRatio` + `imageSize`，`candidateCount=1`；
  下游走 Gemini 原生透传，比例不影响按张价格。
- 实际字节可能是 JPEG 或 PNG；`inlineData.mimeType` 可能误标 `image/png`。
  保存时必须按文件头识别真实格式（PNG 或 JPEG），不要只信 MIME 或扩展名。
- 客户端必须允许长超时和大响应；4K 响应体可能很大（历史实测约 8.7 MB Base64）。
  本文不规定未经统一复测的超时秒数，接入方应按实际网络与代理链路配置，不能沿用
  小 JSON 请求的读取上限。
- 1K/4K 长请求可能触发 524/500。**超时或网络中断不等于确定失败，也不要盲目
  重提**：先按 `X-Harness-Request-Id` 查 B3，并核对可取得的响应/产物，确认是否
  已生成或计费后再决定是否重试。同步协议没有 task id，不能靠轮询任务替代这一步。
- 价格：2.1 1K/2K 为 0.009 USD/张，2.1 4K 为 0.011 USD/张，Pro 1K/2K/4K 为
  0.016 USD/张；实际金额以 B3 为准。

### 4.1 比例与实测像素（2026-11-16 供应网关补测）

供应网关 2026-11-16 补测：2 个系列（Nano Banana 2.1 / Pro）× 3 个档位
（1K/2K/4K）× 10 种比例，共 60 组合；复用 6 个历史 1:1 样本，新增 54 次非方图
请求，每组合最多一次，失败不自动重试。本轮 60 组合均有符合样本，但仅为有限
样本，不代表稳定性或全功能验收。下表为解码后的实际宽×高；2.1 与 Pro 同档位
像素相同，按比例合并列出。非方图带“≈”表示正常像素取整或轻微比例偏差，保留
真实宽高。

| 比例 | 1K | 2K | 4K |
| --- | --- | --- | --- |
| 1:1 | 1024×1024 | 2048×2048 | 4096×4096 |
| 16:9 | 1376×768 ≈ | 2752×1536 ≈ | 5504×3072 ≈ |
| 9:16 | 768×1376 ≈ | 1536×2752 ≈ | 3072×5504 ≈ |
| 3:2 | 1264×848 ≈ | 2528×1696 ≈ | 5056×3392 ≈ |
| 2:3 | 848×1264 ≈ | 1696×2528 ≈ | 3392×5056 ≈ |
| 4:3 | 1200×896 ≈ | 2400×1792 ≈ | 4800×3584 ≈ |
| 3:4 | 896×1200 ≈ | 1792×2400 ≈ | 3584×4800 ≈ |
| 5:4 | 1152×928 ≈ | 2304×1856 ≈ | 4608×3712 ≈ |
| 4:5 | 928×1152 ≈ | 1856×2304 ≈ | 3712×4608 ≈ |
| 21:9 | 1584×672 ≈ | 3168×1344 ≈ | 6336×2688 ≈ |

---

## 5. B1/B3 账本口径

| 流 | 图片任务行为 | 用途 |
| --- | --- | --- |
| B1 用量 | **没有 `status` 字段**；Nano Banana 同步请求走 Gemini 路径，B1 为 `usage_source=estimated`，按请求估算 prompt Token（实测 `total_tokens=10`、`prompt_tokens=10`、`completion_tokens=0`、`split_known=false`）；GPT Image 同步为 `usage_source=upstream`，带上游实际 input/output Token | 不要用于判断完成 |
| B3 成本 | 有 `status`；成功任务最终为 `status_change`，`status=settled`，`pricing_group=图片自营号池`，`group_ratio=1`，`billing.total_amount` 为按张价格（Nano Banana 0.009/0.011/0.016，GPT Image 0.018） | 完成与成本以 B3 为准 |

- B1 的 Token 数不代表费用；图片按张计费，费用以 B3 为准。
- 当前 B1 的 `usage_unit=token`、B3 的 `billing_basis=token` 是已知字段限制，
  **不表示图片按 token 计费**。Nano Banana 的 `usage_source=estimated` 和估算
  Token 也不等同于上游官方 usage；按张金额以 B3 最新版本的 `total_amount`、
  `status=settled` 和提交时价格版本为准。
- Nano Banana 已改为 Gemini 同步请求，B1 的 Token 是估算值；旧异步合同的
  `unavailable` / `null` 口径不再适用于 Nano Banana。无 Token 用量的异步视频任务
  （PAI/Seedance）仍按 `03-data-contract.md` 的 `unavailable` 语义处理。
- B1 只在用量或用量元数据变化时追加事件，完成与成本仍以 B3 为准。
- `image_count` 为本次生成张数，当前只验证 `n=1`；它用于网关按张计费，
  不在 B1 中暴露。
- Harness 应以 B3 的最新 `record_version` 作为图片任务的完成和成本依据，不要用
  B1 推断任务状态。

> 以上为 2026-11-17 同步图片合同口径；82 实例已部署
> `harness-api-20261116-df87870`。Nano Banana Pro 1K 实测：B1 `estimated` 10 Token，
> B3 `图片自营号池` 0.016 USD；GPT Image 4 款 0.018 USD。

---

## 6. 注意事项与已知限制

1. Nano Banana 同步已补测 10 种比例（见 4.1），`candidateCount: 1`；参考图、
   编辑、GIF、并发、退款均未验证，异步接口仍停用，原始响应披露尚未验收。
2. GPT Image 样例只覆盖 `gpt-image-2.6-flare` + `1024x1024` + `n=1`；4 个
   GPT Image 模型共用同一同步接口和价格。
3. 同步请求超时或网络中断时不要盲目重试；Nano Banana 没有 task id，先按请求 ID
   查 B3，并核对可取得的响应/产物，确认是否已生成或计费后再决定是否重新提交。
   上游 1K/4K 长请求可能返回 524/500，客户端应使用足够长的超时。
4. 网关不暴露供应商 URL 和密钥；下游只使用网关接口和 Harness JWT。
5. Nano Banana 同步响应体可能很大（4K 历史实测约 8.7 MB Base64）；客户端要允许
   大响应和较长超时，不要按小 JSON 设置读取上限。实际 PNG/JPEG 格式按文件头识别，
   不要依赖 `inlineData.mimeType` 或文件扩展名。
6. 2026-11-23 反馈处理记录为 GPT Image 4/4、Nano Banana 4/6；Nano Banana 余下
   2 款与超时/MIME 整改相关。历史供应网关补测覆盖 1K/2K/4K 与 10 种比例，但不等于
   6 款模型已全部完成当前窗口的生成验收。
7. 2026-11-25 公网复测：1K/4K 均返回 200（44.7s / 109.5s），但 MIME 仍把 JPEG 标成 PNG；
   GPT Image 2.6 flare 遇供应商侧 524。网关侧 quota 0，未向用户计费；超时后仍须先查
   B3/产物再决定是否重试。

---

## 7. 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| V0.13 | 2026-11-17 | Nano Banana 章节补充供应网关 2026-11-16 的 10 种比例 × 1K/2K/4K 实测像素表（2.1 与 Pro 同档位像素相同）；更新 `aspectRatio` 支持范围与已知限制。Nano Banana 实测证据来自 `harness-api-20261116-df87870`；当前实例已升级到 `harness-api-20261117-b040241`（Flash 修复），图片接口未变。 |
| V0.14 | 2026-11-23 | 按第 11、12 号文档校准 A3 `catalog_version=10` 与 Nano Banana 4/6 记录；强化正式同步协议、无 task id、`X-Route-Group: image`、长超时、524/500 后先查 B3/产物、PNG/JPEG 文件头识别，以及 B3 `billing_basis=token` 字段限制。 |
| V0.15 | 2026-11-25 | 记录公网 1K/4K 实测、OpenResty 600 秒超时修复、MIME 仍标错与 GPT Image 供应商 524；4K 响应约 26.9MB。 |
