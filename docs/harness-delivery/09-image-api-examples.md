# 图片生成调用样例（GPT Image 同步 / Nano Banana 异步）

**版本**：V0.11
**日期**：2026-11-14
**对应网关版本**：`harness-api` 分支，82 实例 `harness-api-20261114-c29f1a6`
**适用对象**：Harness 后端与图片生成联调

本文只说明图片生成调用。图片池 `image` 当前有 10 个模型，按协议分为两类：
GPT Image 2.5/2.6 使用同步 `POST /v1/images/generations`；Nano Banana 使用异步
`POST /v1/videos` 提交任务，再轮询、下载。所有图片生成提交请求都要带
`X-Route-Group: image`，使用 Harness JWT，scope 为 `gateway.chat`；提交接口还需
带 `X-Harness-Request-Id` / `X-Harness-Run-Id`（均 ≤64 字符），轮询和下载只需要
JWT。

---

## 1. 图片池与模型

2026-11-14 的 82 实例中，A3 `image` 池 `catalog_version` 为 `"7"`，按以下顺序
返回 10 个模型：

| 顺序 | 模型 id | 协议 | 价格（USD/张） |
| --- | --- | --- | --- |
| 1 | `Nano Banana 2.1 1K` | Nano Banana 异步 | 0.009 |
| 2 | `Nano Banana 2.1 2K` | Nano Banana 异步 | 0.009 |
| 3 | `Nano Banana 2.1 4K` | Nano Banana 异步 | 0.011 |
| 4 | `Nano Banana Pro 1K` | Nano Banana 异步 | 0.016 |
| 5 | `Nano Banana Pro 2K` | Nano Banana 异步 | 0.016 |
| 6 | `Nano Banana Pro 4K` | Nano Banana 异步 | 0.016 |
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
| 调用方式 | 同步 | 异步 |
| 提交接口 | `POST /v1/images/generations` | `POST /v1/videos` |
| 查询接口 | 无（提交即返回结果） | `GET /v1/videos/{task_id}` |
| 关键请求字段 | `model` / `prompt` / `n=1` / `size` | `model` / `prompt` / `aspect_ratio` / `n=1` |
| 结果位置 | `data[0].b64_json`（可能） | 完成后 `media_type=image`，`image_url` / `url` |
| 下载方式 | 解码 `b64_json` | `GET` / `HEAD` `/v1/tasks/{task_id}/artifacts/image/content` |
| 价格 | 0.018 USD/张 | 0.009 / 0.011 / 0.016 USD/张 |
| 完成判定 | HTTP 200 且返回图片数据；结算以 B3 `settled` 为准 | 轮询 `completed` 后下载；结算以 B3 `settled` 为准 |

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

## 4. Nano Banana 异步样例

以 `Nano Banana 2.1 1K` 为例。先提交任务：

```bash
curl -sS -X POST "http://<gateway>/v1/videos" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -H "X-Harness-Request-Id: hrq_image_banana_001" \
  -H "X-Harness-Run-Id: run_image_banana_001" \
  -H "X-Route-Group: image" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Nano Banana 2.1 1K",
    "prompt": "A banana-shaped balloon floating above a quiet beach, soft morning light",
    "aspect_ratio": "1:1",
    "n": 1
  }'
```

创建返回 HTTP 200，响应含任务 `id`（例如 `task_xxxxxxxx`）。**保存该 task id**，
轮询和下载都要使用；任务完成后仍需保留，用于重试下载和对账。

轮询：

```bash
curl -sS "http://<gateway>/v1/videos/task_xxxxxxxx" \
  -H "Authorization: Bearer $HARNESS_JWT"
```

完成时响应包含以下关键字段：

```json
{
  "id": "task_xxxxxxxx",
  "status": "completed",
  "media_type": "image",
  "image_url": "/v1/tasks/task_xxxxxxxx/artifacts/image/content",
  "url": "/v1/tasks/task_xxxxxxxx/artifacts/image/content"
}
```

> 以上只列关键字段；实际响应还包含模型、进度等字段，以实例返回为准。

下载：

```bash
curl -sS "http://<gateway>/v1/tasks/task_xxxxxxxx/artifacts/image/content" \
  -H "Authorization: Bearer $HARNESS_JWT" \
  -o output.jpg
```

也可以用 `HEAD` 探测大小/类型。下载行为：

- 下载路径使用 `image_url` / `url` 返回的
  `/v1/tasks/{task_id}/artifacts/image/content`，并携带 Harness JWT。
- 下载响应返回 `Content-Type: application/octet-stream`，实际字节可能是 JPEG 或
  PNG；按文件头判断格式，不要只看 `Content-Type`。
- `Range` 会被忽略，服务端返回 200 和完整响应体；不要依赖 206。
- 不要使用 `content_url` 或 `/v1/videos/{task_id}/content` 下载 Nano Banana；
  上游对 `/v1/videos/{task_id}/content` 返回 410。

价格：2.1 1K/2K 为 0.009 USD/张，2.1 4K 为 0.011 USD/张，Pro 1K/2K/4K 为
0.016 USD/张。

---

## 5. B1/B3 账本口径

| 流 | 图片任务行为 | 用途 |
| --- | --- | --- |
| B1 用量 | **没有 `status` 字段**；Nano Banana 异步任务 Token 为 0，可能先出现 `usage`（0 Token），结算后出现 `usage_correction`（0 Token）；GPT Image 同步可能带上游 Token（实测 99/7,028） | 不要用于判断完成 |
| B3 成本 | 有 `status`；成功任务最终为 `status_change`，`status=settled`，`billing.total_amount` 为按张价格 | 完成与成本以 B3 为准 |

- B1 的 Token 数不代表费用；图片任务按张计费，费用只体现在 B3。
- Nano Banana 异步任务的 B1 为 0 Token；GPT Image 同步即使带上游 Token，
  也不改变按张计费。
- `image_count` 为本次生成张数，当前只验证 `n=1`；它用于网关按张计费，
  不在 B1 中暴露。
- Harness 应以 B3 的最新 `record_version` 作为图片任务的完成和成本依据，不要用
  B1 推断任务状态。

---

## 6. 注意事项

1. Nano Banana 当前只验证了 `aspect_ratio: "1:1"`、`n: 1`；其他比例、参考图、
   编辑和 GIF 均未验证。
2. GPT Image 样例只覆盖 `gpt-image-2.6-flare` + `1024x1024` + `n=1`；4 个
   GPT Image 模型共用同一同步接口和价格。
3. 提交超时或网络中断时不要自动重试；任务可能已创建并计费，应先按 Request ID
   查 B3 或按 task id 查任务状态，再决定是否重新提交。
4. 网关不暴露供应商 URL 和密钥；下载只能使用网关路径，并携带 Harness JWT。
5. Nano Banana 的 `Range` 不生效；下载总是返回完整响应体。
6. 本节样例基于 2026-11-14 本地临时 JWT 联调，**不是 Harness 正式签发的 JWT**；
   真实 Harness JWT 的图片 E2E 仍待执行。
