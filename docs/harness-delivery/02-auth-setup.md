# Harness 鉴权配置指南

**版本**：V0.9.1
**对应网关版本**：`harness-api` 分支 `fd00dae`

本文档说明网关侧的 JWT 校验实现，以及需要 Harness 提供的信息。

---

## 一、配置总览

网关需要四项配置，写入管理端 option 表。前三项由**双方共同确定**，第四项由
**New API 运营方**在管理端配置。

| 配置项 | Option Key | 由谁提供 | 当前状态 |
| --- | --- | --- | --- |
| JWKS 地址 | `HarnessJWKSURL` | **Harness 提供** | ⏳ 待提供 |
| 签发方 | `HarnessJWTIssuer` | **双方商定** | ⏳ 待提供 |
| 受众 | `HarnessJWTAudience` | **双方商定** | ⏳ 待提供 |
| 服务主体映射 | `HarnessSubjectTokenMap` | New API 运营方配置 | 运营方按下方格式填 |

---

## 二、需要 Harness 提供的信息

### 2.1 JWKS URL

**要求**：可公开访问的 HTTPS 地址，返回标准 JWKS 文档。

```json
{
  "keys": [
    {
      "kty": "RSA",
      "kid": "harness-2026-10",
      "use": "sig",
      "alg": "RS256",
      "n": "0vx7agoebGcQ...",
      "e": "AQAB"
    }
  ]
}
```

**网关行为**：

- 网关按 JWT header 中的 `kid` 在 JWKS 中查找对应公钥
- JWKS 结果在内存缓存；遇到未知 `kid` 会强制刷新一次
- 若 `kid` 仍不存在，拒绝该请求

**需要提供**：

- [ ] JWKS URL（完整地址）
- [ ] 密钥轮换策略（新 `kid` 何时上线、旧 `kid` 保留多久）

### 2.2 Issuer（`iss`）

JWT 的 `iss` claim 期望值。

**需要提供**：

- [ ] 期望的 `iss` 字符串

### 2.3 Audience（`aud`）

JWT 的 `aud` claim 期望值。

**需要提供**：

- [ ] 期望的 `aud` 字符串

### 2.4 服务主体名称（`sub`）

JWT 的 `sub` claim，网关用它映射到内部模型令牌。

**需要提供**：

- [ ] `sub` 字符串（例如 `harness-gateway`）

**New API 侧配置格式**：

```json
{
  "harness-gateway": 12
}
```

值可以是令牌 ID（整数）或令牌名称（字符串）。建议用令牌 ID，避免名称歧义。

---

## 三、网关已实现的校验规则

以下规则**已实现并测试**，Harness 签发的 JWT 必须满足：

### 3.1 签名算法

**只接受 `RS256`**。其他算法（HS256、ES256、none）一律拒绝。

### 3.2 必需 claim

| Claim | 要求 | 缺失后果 |
| --- | --- | --- |
| `iss` | 必须等于配置的 Issuer | 拒绝 |
| `aud` | 必须包含配置的 Audience | 拒绝 |
| `exp` | **必须存在** | 拒绝 |
| `sub` | 必须存在于服务主体映射中 | 拒绝 |
| `scope` | 见 3.4 | 无 scope 则所有接口 403 |

**`exp` 是强制的**——网关不会接受无过期时间的令牌。

### 3.3 时钟偏移容忍

网关允许 **60 秒**偏移。若 Harness 与网关时钟差异超过 60 秒，`exp`/`nbf` 校验可能失败。

**建议**：双方启用 NTP 同步。

### 3.4 Scope 格式

`scope` claim 必须是**空格分隔的字符串**，不是数组：

```json
{
  "scope": "gateway.chat gateway.usage.read gateway.cost.read"
}
```

**错误写法**（会被忽略，导致 403）：

```json
{
  "scopes": ["gateway.chat"]
}
```

### 3.5 各接口所需 Scope

| Scope | 授权接口 |
| --- | --- |
| `gateway.chat` | A1、A2、A4、A9（模型调用） |
| `gateway.catalog.read` | A3（目录） |
| `gateway.usage.read` | B1、B1-batchGet |
| `gateway.cost.read` | B3、B3-batchGet |
| （无） | B4：只需有效 JWT，不校验额外 scope |

**Scope 是独立校验的**，不隐含传递。持有 `gateway.chat` 不自动获得
`gateway.usage.read`。

### 3.6 映射令牌的要求

`sub` 映射到的内部令牌必须：

- 状态为启用
- 未被限制模型范围（`model_limits_enabled = false`）

否则访问账本接口会被拒绝。

---

## 四、Gateway 侧配置示例

运营方在管理端「系统设置」写入：

| Key | 示例值 | 说明 |
| --- | --- | --- |
| `HarnessJWKSURL` | `https://harness.example.com/.well-known/jwks.json` | 填 Harness 提供值 |
| `HarnessJWTIssuer` | `harness` | 填双方商定值 |
| `HarnessJWTAudience` | `new-api` | 填双方商定值 |
| `HarnessSubjectTokenMap` | `{"harness-gateway":12}` | 映射到实际令牌 ID |

配置为空时，JWT 鉴权**不启用**，Harness 凭证会被当作普通令牌处理并失败。

---

## 五、自检方法

Harness 拿到配置后，可用健康接口验证连通性：

```bash
curl -sS "http://<gateway>/internal/v1/gateway/health" \
  -H "Authorization: Bearer $HARNESS_JWT"
```

**预期**：

```json
{
  "status": "ok",
  "interface_version": "v2",
  "upstream_baseline": "45fbb149",
  "time": "2026-10-25T14:07:29.745Z"
}
```

| 现象 | 原因 |
| --- | --- |
| `401 GATEWAY_UNAUTHORIZED` | JWT 签名/iss/aud/exp 任一不通过 |
| `403` + `required_scope` | JWT 有效，但缺该接口所需 scope |
| `200` | 鉴权链路已通 |

---

## 六、当前状态

| 项 | 状态 |
| --- | --- |
| 网关 JWT 校验实现 | ✅ 已完成 |
| 自签 JWT + 自建 JWKS 集成测试 | ✅ 已通过 |
| **Harness 真实 JWKS 联调** | ❌ **未开始**（等待对方提供配置） |

**重要说明**：网关侧自签 JWT 的本地联调已完成；使用 Harness 真实签发 JWT 的
端到端验收已于 2026-10-26 与 2026-10-29 两轮通过，详见
`07-acceptance-evidence.md`。测试 JWT 有效期至 2026-11-18，到期需换发。

---

## 七、变更记录

| 日期 | 版本 | 内容 |
| --- | --- | --- |
| 2026-10-25 | V0.9 | 初版 |
| 2026-10-29 | V0.9.1 | 更新网关版本；记录真实 JWT 联调已完成及 Token 有效期 |
