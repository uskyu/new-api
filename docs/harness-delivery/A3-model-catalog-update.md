# Harness × New API｜A3 模型目录结构更新

**日期**：2026-11-08  
**适用对象**：Harness 后端与模型列表页面  
**变更范围**：仅模型目录 A3 及模型选择方式；B1/B3 账本接口沿用现有对接规范。

## 1. 本次调整

双方已沟通取消原有的 **economy / enterprise 业务档位**及模型列表的**二级分组**。Harness 直接读取模型 ID，不再根据供应商、价格、渠道或能力二次分组；上游渠道及路由由 New API 负责。

- **模型列表接口**：`GET /v1/models`（Harness JWT，需 `gateway.catalog.read`）。
- **前台四个展示分区**：文字模型、图片生成模型、视频模型、Plan 计划。
- **API 当前仍返回五个资源池**：`chat`、`video`、`image`、`audio`、`plan`。其中 `audio` 为预留，不进入本期四个展示分区。
- **实际调用**：使用对应 `models[]` 中的真实模型 ID 作为请求 `model`；普通文字调用不传 `X-Route-Group`，Plan 计划调用可传 `X-Route-Group: plan`。
- **`capabilities`**：目前是**资源池级汇总**，不能代表池内每个模型均支持这些能力；本期列表展示可忽略，不作为逐模型能力判断依据。

> 这里取消的是旧档位和列表二级分组，并不是删除网关内部按产品线配置的资源池。视频、图片等资源池是否可调用，以对应 `models[]` 是否存在可用模型及实际接口为准。

## 2. 请求示例

```http
GET /v1/models
Authorization: Bearer <HARNESS_JWT>
```

首次读取返回 HTTP 200；可保存响应头 `ETag`，下次携带 `If-None-Match`，目录未变时返回 HTTP 304（无响应体）。

## 3. 返回样例

以下为 **2026-11-08 的 82 实例真实样例**：`chat` 15 个模型，`video` 8 个 PAI 按次模型，`plan` 22 个 TA token 模型；`image` / `audio` 为空。`models` 按模型 id 字典序返回；模型 ID、版本及 ETag 以实际联调环境响应为准，模型集合会随渠道与价格变化。

```json
{
  "object": "list",
  "data": [
    {
      "id": "chat",
      "object": "model",
      "display_name": "通用对话",
      "kind": "text",
      "models": ["claude-fable-5", "claude-fable-5-1", "claude-haiku-5-5", "claude-opus-4-8", "claude-opus-5", "claude-opus-5-5", "claude-sonnet-5", "claude-sonnet-5-5", "deepseek-flash", "deepseek-v4-pro", "glm-5.3", "glm-5.3-flash", "kimi-k3", "kimi-k3.1-pro", "kimi-k3.1-pro-highspeed"],
      "capabilities": ["chat", "stream", "tools"]
    },
    {
      "id": "video",
      "object": "model",
      "display_name": "视频",
      "kind": "video",
      "models": ["seedance2-1080p-once", "seedance2-480p-once", "seedance2-720p-once-1", "seedance2-720p-once-2", "seedance2-720p-once-3", "seedance2-fast-720p-once-1", "seedance2-fast-720p-once-2", "seedance2.5-720p-once"],
      "capabilities": []
    },
    {
      "id": "image",
      "object": "model",
      "display_name": "图片生成",
      "kind": "image",
      "models": [],
      "capabilities": []
    },
    {
      "id": "audio",
      "object": "model",
      "display_name": "语音",
      "kind": "audio",
      "models": [],
      "capabilities": []
    },
    {
      "id": "plan",
      "object": "model",
      "display_name": "畅用计划",
      "kind": "video",
      "billing": "per_call",
      "models": ["seedance2-1080p-token-no-video", "seedance2-1080p-token-with-video", "seedance2-480p-token-no-video", "seedance2-480p-token-with-video", "seedance2-4k-token-no-video", "seedance2-4k-token-with-video", "seedance2-720p-token-no-video", "seedance2-720p-token-with-video", "seedance2-fast-480p-token-no-video", "seedance2-fast-480p-token-with-video", "seedance2-fast-720p-token-no-video", "seedance2-fast-720p-token-with-video", "seedance2-mini-480p-token-no-video", "seedance2-mini-480p-token-with-video", "seedance2-mini-720p-token-no-video", "seedance2-mini-720p-token-with-video", "seedance2.5-1080p-token-no-video", "seedance2.5-1080p-token-with-video", "seedance2.5-480p-token-no-video", "seedance2.5-480p-token-with-video", "seedance2.5-720p-token-no-video", "seedance2.5-720p-token-with-video"],
      "capabilities": []
    }
  ],
  "catalog_version": "4",
  "etag": "W/\"example-catalog-etag\""
}
```

## 4. Harness 接入要点

| 展示分区 | 使用的 `data[].id` | 识别规则 |
| --- | --- | --- |
| 文字模型 | `chat` | `kind=text` |
| 图片生成模型 | `image` | `kind=image` |
| 视频模型 | `video` | `kind=video`，无 `per_call` |
| Plan 计划 | `plan` | `billing=per_call`，独立于视频列表 |

- 每个分区直接展开 `models[]`，无需再建立第二层分组；`models: []` 表示暂无可用模型。
- `audio` 目前仅保留接口结构，不进入展示。未来如果接入语音，优先根据实际业务引入 TTS；**本次不要求定义 TTS/ASR 二级分类，也不承诺语音接口可用**。
- 旧 `/api/harness/catalog` 的 `groups[]` 结构**不再作为 Harness 的模型列表对接依据**。
- B1/B3 账本、关联 ID 和 JWT 的详细规则请继续参考既有联调文档；本说明只替换模型列表部分。

**完整接口指南**：[`01-integration-guide.md`](./01-integration-guide.md)
