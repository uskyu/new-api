# Harness 调用方联调指南

本文仅说明 Harness 如何作为调用方接入 New API；不包含 New API 管理员的渠道、路由、价格或令牌配置步骤。管理端配置由 New API 团队负责。

## 接入前向 New API 团队获取

- New API 的 HTTPS Base URL
- 已启用、未限制模型范围，并已获智能路由授权的服务令牌
- 本令牌可用的路由组 ID（也可从目录接口返回的 `groups[].id` 获取）

请将服务令牌保存在 Harness 服务端的密钥配置中，不要写入代码、提交记录或普通日志。Harness 无需调用 `/api/intelligent-routing` 等管理端接口。

## 1. 获取可调用模型目录

```bash
export NEW_API_BASE_URL="https://<new-api-host>"
export NEW_API_TOKEN="sk-<service-token>"

curl --fail-with-body --silent --show-error \
  -H "Authorization: Bearer ${NEW_API_TOKEN}" \
  "${NEW_API_BASE_URL}/api/harness/catalog"
```

成功响应的核心结构如下（具体模型和数值由当前实例返回）：

```json
{
  "success": true,
  "data": {
    "quota_per_unit": 500000,
    "price_unit": "new_api_quota",
    "groups": [
      {
        "id": "global",
        "name": "Global priority",
        "models": [
          {
            "name": "<model-name>",
            "capabilities": ["openai"],
            "quota_type": 0,
            "model_ratio": 1,
            "completion_ratio": 1,
            "billing_mode": "ratio"
          }
        ],
        "unpriced_models": []
      }
    ]
  }
}
```

接入要点：

- `groups` 仅包含当前服务令牌可访问的路由组；运行时应使用返回的 `id`，不要假设一定存在 `global`。
- 使用 `groups[].models[].name` 作为请求模型名；`upstream_models` 是网关侧的上游映射信息，不是 Harness 应优先请求的模型名。
- 只向用户展示或调用 `models` 中的模型。`unpriced_models` 仅用于诊断；这些模型不在可计价目录中，Harness 不应展示或请求。显式价格 `0` 与未配置价格不同，仍可能是有效的免费模型。
- 按 `capabilities` 选择兼容的请求协议，例如 `openai`、`anthropic` 或 `gemini`。
- 不要把 `quota_per_unit`、价格倍率或税率写死；以目录和每次调用的响应头为准。

## 2. 发起带路由的模型请求

每次 Harness 发起模型/任务提交请求时，都要选择一个目录中可用的路由组，并发送 `X-Route-Group`。已获智能路由授权的令牌漏传此头会得到 HTTP 403，不会静默回退到普通路由。模型列表和只读任务结果查询不要求该头。

```bash
curl --include --fail-with-body \
  -H "Authorization: Bearer ${NEW_API_TOKEN}" \
  -H "X-Route-Group: global" \
  -H "Content-Type: application/json" \
  --data '{
    "model": "<model-name-from-catalog>",
    "messages": [{"role": "user", "content": "联调测试"}]
  }' \
  "${NEW_API_BASE_URL}/v1/chat/completions"
```

将 `global` 替换为本次要使用的 `groups[].id`。其他兼容协议沿用其原有请求路径与请求体，只需附加相同的路由头。

### WebSocket 客户端

原生 WebSocket 浏览器接口不能设置自定义 HTTP 请求头。需要从浏览器发起 WebSocket 时，可将路由组作为 `Sec-WebSocket-Protocol` 子协议传入：

```text
openai-insecure-route-group.<group-id>
```

例如 `openai-insecure-route-group.global`。已有的 WebSocket 协议项和认证方式仍需保留。由于浏览器子协议中可能携带令牌，不要将长期服务令牌暴露给不受信任的客户端；优先由 Harness 服务端代发请求。

## 3. 根据响应头记录实际路由与价格

成功的普通 HTTP 模型调用会返回实际命中的路由元数据：

| 响应头 | Harness 用途 |
| --- | --- |
| `X-Route-Group` | 本次实际使用的路由组 |
| `X-Route-Channel-ID` | 实际命中的上游渠道标识，用于追踪与核对 |
| `X-Route-Pricing-Group-ID` | 实际命中的 New API 计价分组 |
| `X-Route-Group-Ratio` | 本次路由适用的倍率/折扣系数 |
| `X-Route-Tax-Rate-Percent` | 实际渠道的供应商税率（百分数） |
| `X-Oneapi-Request-Id` | 与 New API 团队排障时使用的请求 ID |

计价规则：

1. `billing_mode = "ratio"` 且 `quota_type = 0` 时，以实际用量、目录中的 token 倍率和相关缓存/图像/音频倍率计算基础额度。
2. `billing_mode = "ratio"` 且 `quota_type = 1` 时，基础额度使用 `model_price`。
3. 将基础额度乘以本次响应的 `X-Route-Group-Ratio`；税费按 `X-Route-Tax-Rate-Percent / 100` 计算。
4. 如需换算为 Harness 的金额单位，使用目录返回的 `quota_per_unit`；不要依赖默认值。
5. 若 `billing_mode = "tiered_expr"`，按 `billing_expr`、`billing_usage_schema` 和 `billing_usage_examples` 实现对应计费；若目录含 `billing_plugin_variants`，不要假定模型只有一个插件价格。暂不支持这些模式时，应明确禁用相应模型或与 New API 团队确认，不能用 `model_ratio` 代替。

发生渠道重试时，必须以最终成功响应的响应头计价，不能保留第一次选中渠道的数据。流式 HTTP 响应也应在读取响应体前保存响应头。

## 4. 联调与错误处理

| HTTP 状态 | 常见原因 | Harness 处理 |
| --- | --- | --- |
| `401` | 服务令牌无效或未发送 | 检查 Base URL 与 `Authorization: Bearer ...`；不要记录令牌值 |
| `403` | 令牌未获路由授权、使用了模型受限令牌、缺少 `X-Route-Group`，或组 ID 不在授权范围内 | 检查目录中的 `groups[].id` 和令牌授权；不要自动移除路由头重试 |
| `503` | 所选组当前没有可用渠道，或没有满足价格配置的路由 | 返回可诊断错误并联系 New API 团队；不要静默切换到普通路由 |

建议联调顺序：

1. 用服务令牌请求 catalog，确认返回 `200` 且至少有一个可用路由组和模型。
2. 用 catalog 返回的模型名与组 ID 发起一次非流式调用。
3. 保存并核对 `X-Route-*` 响应头及 `X-Oneapi-Request-Id`。
4. 验证流式调用也使用最终响应头；随后再测试重试和计价模式。

排障时向 New API 团队提供请求时间、路由组 ID、模型名、HTTP 状态和 `X-Oneapi-Request-Id`；不要发送令牌值。
