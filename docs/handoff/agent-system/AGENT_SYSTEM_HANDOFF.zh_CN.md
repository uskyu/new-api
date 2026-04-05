# 代理与提现系统交接文档

## 1. 文档目的

本文档用于交接当前已开发完成的代理返利系统、二级代理体系以及提现系统第一版能力，帮助后续开发者快速理解：

- 业务目标与规则
- 当前已实现内容
- 数据库结构变化
- 前后端页面与接口分布
- 已知限制与待办事项
- 本地调试、验证与上线注意事项

本文档以当前分支 `feat/agent-rebate-system` 为准。

## 2. 当前业务背景

本次需求从最初的“邀请人数奖励”逐步演进为“代理返利与提现体系”，最终目标包括：

- 用户通过代理推广链接进入站点后自动记录来源
- 注册后自动绑定直属邀请关系
- 被邀请用户充值后，直属代理按配置比例获得返利
- 支持代理发展直属下级代理
- 下级代理的有效比例不得高于直属上级代理
- 一级代理只吃直属普通用户与直属下级代理本人的充值
- 一级代理不吃二级代理名下普通用户的充值
- 返利余额独立于原钱包余额
- 支持代理提现、管理员导出打款、导入回执并标记已打款

## 3. 当前已经确认的核心业务规则

### 3.1 邀请与归因

- 推广链接通过 `?aff=CODE` 传递来源
- 用户可以先进入主页或任意页面，不必直达注册页
- 前端会全局捕获 `aff` 参数并保存到浏览器本地
- 注册时自动携带 `aff_code`
- OAuth 登录会通过后端 session 传递 `aff`

### 3.2 返利触发入口

当前只纳入以下充值成功场景：

- 易支付成功回调
- 管理员补单成功

当前不纳入：

- 订阅
- 其他第三方支付渠道（如 Stripe、Creem、Waffo）

### 3.3 返利口径

- 返利按支付金额计算
- 返利金额保存为金额最小单位（分）
- 返利余额不进入 `user.quota`
- 返利余额保存在独立代理账本中

### 3.4 代理层级与返利规则

- 当前支持两级代理关系（实际是“直属关系”，不是无限层级）
- 一级代理和二级代理只是管理上方便理解的层级称呼
- 真正返利按“直属上级代理”判断

具体规则：

- 普通用户充值：直属代理吃返利
- 下级代理本人充值：直属上级代理吃返利
- 下级代理名下普通用户充值：只归下级代理，不再向上穿透

### 3.5 比例限制规则

- 每个代理都有一个“有效比例”
- 有效比例来源优先级：
  - 自定义覆盖
  - 否则跟随分组比例
- 下级代理有效比例不能高于直属上级代理有效比例

当管理员修改分组比例或代理资料时：

- 系统会做超限校验
- 如果某些下级代理超出直属上级上限，则拦截保存
- 接口返回冲突列表，要求管理员手工调整后再保存

### 3.6 提现规则（当前第一版）

- 不设置最低提现金额
- 不做人工驳回流程
- 用户提现时填写：
  - 姓名
  - 支付宝账号
  - 提现金额
- 提现申请后先冻结金额，不直接扣减到账
- 管理员导出 CSV 后线下打款
- 导入 CSV 回执时，系统按申请单 ID 匹配
- 当导入文件中最后一列“打款订单号”有值时，标记该申请为已打款

## 4. 当前已实现功能总览

### 4.1 第一阶段已实现

- 代理返利独立账本基础模型
- 代理初始化流程
- 三库兼容迁移：SQLite / MySQL / PostgreSQL
- 用户注册归因
- 易支付返利结算
- 管理员补单返利结算
- 推广链接管理
- 管理端代理基础页面
- 用户端合作代理页面
- 管理员调账

### 4.2 第二阶段已实现

- 二级代理直属关系模型
- 直属下级直接开通代理
- 代理比例上限校验
- 代理资料列表增加：
  - 代理级别
  - 直属上级
  - 比例来源
  - 上级比例上限
- 用户端直属下级列表
- 开通代理后自动创建默认推广链接

### 4.3 提现第一版已实现

- 提现账户信息保存（支付宝账号、姓名）
- 提现申请创建
- 冻结金额
- 提现记录查询
- 管理端提现申请列表
- 状态筛选
- 日期筛选
- 导出提现 CSV
- 导入回执 CSV
- 导入后按申请单 ID + 外部打款订单号标记已打款

## 5. 前端页面结构

### 5.1 管理端

页面路径：`/console/agent`

主要模块：

- 代理总览
- 代理分组
- 代理资料
- 最近调账记录
- 推广链接管理
- 提现申请列表
- 代理详情弹窗
  - 渠道统计
  - 下级用户概况

关键文件：

- `web/src/pages/Agent/index.jsx`
- `web/src/components/layout/SiderBar.jsx`
- `web/src/App.jsx`

### 5.2 用户端

页面路径：`/console/agent-center`

主要模块：

- 返利余额
- 冻结金额
- 累计返利
- 当前比例
- 返利笔数
- 我的推广链接
- 渠道统计
- 我的直属下级
- 返利流水
- 人工调账记录
- 提现记录
- 提现弹窗

关键文件：

- `web/src/pages/AgentCenter/index.jsx`

## 6. 后端主要接口

### 6.1 管理端接口

- `GET /api/agent/status`
- `GET /api/agent/overview`
- `POST /api/agent/init`
- `GET /api/agent/groups`
- `POST /api/agent/group`
- `DELETE /api/agent/group/:id`
- `GET /api/agent/profiles`
- `POST /api/agent/profile`
- `POST /api/agent/adjust`
- `GET /api/agent/adjustments`
- `GET /api/agent/promo-links`
- `POST /api/agent/promo-link`
- `DELETE /api/agent/promo-link/:id`
- `GET /api/agent/promo-link-stats`
- `GET /api/agent/downlines`
- `GET /api/agent/withdraw-requests`
- `GET /api/agent/withdraw-requests/export`
- `POST /api/agent/withdraw-requests/import`

### 6.2 用户端接口

- `GET /api/agent/self`
- `GET /api/agent/self/downlines`
- `POST /api/agent/self/upgrade-request`
- `GET /api/agent/self/promo-links`
- `POST /api/agent/self/promo-link`
- `DELETE /api/agent/self/promo-link/:id`
- `GET /api/agent/self/promo-link-stats`
- `GET /api/agent/self/rebates`
- `GET /api/agent/self/adjustments`
- `POST /api/agent/self/withdraw-request`
- `GET /api/agent/self/withdraw-requests`

## 7. 数据模型与数据库改动

### 7.1 已新增 / 已扩展的代理相关结构

核心定义集中在：`model/agent_rebate.go`

已存在或已新增的重要结构：

- `AgentRebateGroup`
- `AgentProfile`
- `AgentPromoLink`
- `AgentRebateRecord`
- `AgentRebateAdjustment`
- `AgentRelationship`
- `AgentUpgradeRequest`
- `AgentWithdrawAccount`
- `AgentWithdrawRequest`
- `AgentBalanceLedger`

### 7.2 现有表字段新增

用户表：

- `user.promo_link_id`

代理资料表：

- `agent_profiles.rebate_frozen_amount`

### 7.3 提现相关状态说明

提现申请状态：

- `pending`
- `exported`
- `paid`

账本变化类型：

- `rebate_income`
- `admin_adjust`
- `withdraw_freeze`
- `withdraw_paid`

## 8. 关键代码入口

### 8.1 返利结算

主要文件：`model/agent_rebate.go`

关键点：

- `SettleAgentRebateTx`
- 按直属关系识别返利归属代理
- 入账时同步写代理余额流水

### 8.2 易支付回调

主要文件：`controller/topup.go`

当前逻辑：

- 易支付成功回调
- 增加用户额度
- 调用代理返利结算

### 8.3 管理员补单

主要文件：`model/topup.go`

当前逻辑：

- 管理员补单成功后
- 增加用户额度
- 调用代理返利结算

### 8.4 注册归因

主要文件：

- `controller/user.go`
- `controller/oauth.go`
- `model/agent_rebate.go`
- `web/src/helpers/api.js`
- `web/src/App.jsx`

当前逻辑：

- 全局捕获 `aff`
- 注册自动带入
- OAuth 通过 session 保留来源

## 9. 当前已经修复过的重要问题

### 9.1 用户端代理身份误判

问题：

- 一级代理登录后可能显示“你当前还不是代理”

原因：

- 之前通过模糊搜索列表判断自己是否为代理
- 分页和模糊匹配导致拿不到自己

修复：

- 改为按 `user_id` 精确查询自己的代理资料

### 9.2 管理端提现列表位置错误

问题：

- 提现申请列表曾被误放进“代理详情弹窗”中

修复：

- 已移动回管理页主页面

### 9.3 提现导入结果不直观

问题：

- 导入回执后用户无法确认是否处理成功

修复：

- 导入后显示处理条数
- 列表状态改为标签显示

### 9.4 提现筛选交互过于原始

问题：

- 日期筛选依赖手工输入文本

修复：

- 改为 `input type=date`

## 10. 当前已知限制

### 10.1 提现导入仍是简化版

- 仅通过申请单 ID 匹配
- 需要在 CSV 最后一列手工填“打款订单号”
- 不做打款失败/驳回回退流程

### 10.2 提现筛选第一版交互仍可继续优化

- 当前采用简化日期控件
- 后续可以替换为 Semi 的 RangePicker 或更高级筛选区

### 10.3 代理升级申请接口保留但当前前端已改为直开

- 申请审核流相关结构还保留在后端
- 目前前端已经改成直属下级直接开通代理
- 如果以后要恢复审核制，可以复用现有结构

## 11. 本地启动与验证

### 11.1 后端启动

```powershell
go run main.go
```

### 11.2 前端启动

```powershell
cd web
bun run dev
```

### 11.3 前端构建验证

```powershell
cd web
bun run build
```

### 11.4 主要测试命令

```powershell
go test ./model -run "Test(ResolveRegistrationAttribution|SettleAgentRebateTx|AdjustAgentRebateBalance|UpsertAndDeleteAgentPromoLink|UpsertAgentProfileAutoCreatesDefaultPromoLink|UpsertAndDeleteAgentRebateGroup|AgentPromoLinkStatsAndDownlines|AgentUpgradeRequestAndRateConflict|AgentWithdrawWorkflow)$" -count=1
go test ./controller -run TestDoesNotExist -count=1
go test ./router -run TestDoesNotExist -count=1
go test ./service -run "Test(GetAgentBootstrapStatusReady|InitializeAgentModule)$" -count=1
```

## 12. 当前分支与提交信息

当前工作主要在：

- 分支：`feat/agent-rebate-system`

最近关键提交：

- `b9c72cae` `feat(agent): add referral rebate workflow and agent center`
- `e37cd9cd` `feat(agent): support direct agent upgrades and hierarchy caps`
- `fbd502a4` `feat(agent): add withdraw workflow and fix self agent lookup`
- `08004867` `fix(agent): surface withdraw lists in admin and agent views`
- `e0758a9d` `fix(agent): improve withdraw list filtering and guidance`

## 13. 建议的后续工作

如果继续完善，建议优先做：

1. 提现回执导入模板进一步规范化
- 明确列头
- 支持导入结果预览

2. 提现列表更接近业务结算页风格
- 顶部筛选区强化
- 状态汇总
- 导出批次展示

3. 驳回 / 回退流程
- 增加提现失败或驳回后的余额退回

4. 手机号支持
- 当前导出里主要使用用户名、邮箱
- 如果业务必须手机号，需要确认用户表或额外资料表来源

5. 审核日志与操作留痕强化
- 尤其提现导出和导入人审计信息

## 14. 交接注意事项

- 本次代理系统改动较大，但整体以“新增模型和新增流程”为主
- 正常升级不应清空已有代理资料
- 如果出现“管理员能看到、用户端看不到”的情况，优先检查：
  - 后端是否重启到最新代码
  - 前端页面是否热更新到最新组件
  - 用户端是否误走了旧查询逻辑
- 本地曾出现的多次问题，大部分都不是数据库丢失，而是：
  - 旧进程未重启
  - 页面组件放错位置
  - 查询口径错误

---

如果后续继续接手开发，建议先从本文件第 13 节的后续工作继续推进。
