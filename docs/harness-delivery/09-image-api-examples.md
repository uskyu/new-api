# 图片生成调用样例（GPT Image 同步 / Nano Banana Gemini 同步）

**版本**：V0.12（2026-11-17 修订）
**日期**：2026-11-14（2026-11-17 修订）
**对应网关版本**：`harness-api` 分支，82 实例 `harness-api-20261116-df87870`
**适用对象**：Harness 后端与图片生成联调

本文只说明图片生成调用。图片池 `image` 当前有 10 个模型，按协议分为两类：
GPT Image 2.5/2.6 使用同步 `POST /v1/images/generations`；Nano Banana 使用同步
Gemini `POST /v1beta/models/{URL编码模型}:generateContent`，直接读取
`candidates[].content.parts[].inlineData.data`，**不再使用 `/v1/videos` 异步任务**。
所有图片生成提交请求都要带 `X-Route-Group: image`，使用 Harness JWT，scope 为
`gateway.chat`；提交接口还需带 `X-Harness-Request-Id` / `X-Harness-Run-Id`
（均 ≤64 字符）。

---

## 1. 图片池与模型

2026-11-14 的 82 实例中，A3 `image` 池 `catalog_version` 为 `"7"`，按以下顺序
返回 10 个模型：

| 顺序 | 模型 id | 协议 | 价格（USD/张） |
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
`X-Route-Group: image` 是图片池的池选择头，所有图片生成提交调用都要携带。

---

## 2. 两种协议对比

| 项 | GPT Image 2.5/2.6 | Nano Banana |
| --- | --- | --- |
| 模型数量 | 4 | 6 |
| 调用方式 | 同步 | 同步（Gemini `generateContent`） |
| 提交接口 | `POST /v1/images/generations` | `POST /v1beta/models/{URL编码模型}:generateContent` |
| 查询接口 | 无（提交即返回结果） | 无（同步返回） |
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

- **同步返回，不需要轮询，也没有 task id。**
- `imageSize` 必须按模型填写：1K / 2K / 4K；`aspectRatio` 当前只验证 `1:1`。
- 实际字节可能是 JPEG 或 PNG；`inlineData.mimeType` 可能误标 `image/png`，
  必须按文件头判断格式。
- 4K 响应体可能很大（实测约 8.7 MB Base64），客户端要允许大响应和较长超时。
- 上游偶发 524 超时；重试前先确认是否已经生成，避免重复扣费。
- 价格：2.1 1K/2K 为 0.009 USD/张，2.1 4K 为 0.011 USD/张，Pro 1K/2K/4K 为
  0.016 USD/张。

---

## 5. B1/B3 账本口径

| 流 | 图片任务行为 | 用途 |
| --- | --- | --- |
| B1 用量 | **没有 `status` 字段**；Nano Banana 同步请求走 Gemini 路径，B1 为 `usage_source=estimated`，按请求估算 prompt Token（实测 `total_tokens=10`、`prompt_tokens=10`、`completion_tokens=0`、`split_known=false`）；GPT Image 同步为 `usage_source=upstream`，带上游实际 input/output Token | 不要用于判断完成 |
| B3 成本 | 有 `status`；成功任务最终为 `status_change`，`status=settled`，`pricing_group=图片自营号池`，`group_ratio=1`，`billing.total_amount` 为按张价格（Nano Banana 0.009/0.011/0.016，GPT Image 0.018） | 完成与成本以 B3 为准 |

- B1 的 Token 数不代表费用；图片按张计费，费用只体现在 B3。
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

## 6. 注意事项

1. Nano Banana 同步当前只验证了 `aspectRatio: "1:1"`、`candidateCount: 1`；
   其他比例、参考图、编辑和 GIF 均未验证。
2. GPT Image 样例只覆盖 `gpt-image-2.6-flare` + `1024x1024` + `n=1`；4 个
   GPT Image 模型共用同一同步接口和价格。
3. 同步请求超时或网络中断时不要盲目重试；先确认是否已经生成并计费，再决定是否
   重新提交。上游偶发 524，重试前先查 B3 或实际产物。
4. 网关不暴露供应商 URL 和密钥；下游只使用网关接口和 Harness JWT。
5. Nano Banana 同步响应体可能很大（4K 约 8.7 MB Base64）；客户端要允许大响应和
   较长超时，不要按小 JSON 设置读取上限。
6. 本节样例基于 2026-11-17 的 `harness-api-20261116-df87870` 实测；Nano Banana
   已使用 Harness JWT 完成同步调用（1K/2K/4K 均通过，实际字节按文件头识别）。
