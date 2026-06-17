# 重构：将接收者配置从 AlertChannel 移动到 AlertTemplate

## 问题背景

当前接收者配置（如飞书的 `receive_id_type` / `receive_id`）存储在 `AlertChannel.Config` JSON 中。对于 SDK 类渠道（飞书 App、飞书 Bot），同一个应用要发送给不同的人/群就必须创建多个 Channel，即使 AppID/Secret 完全相同。这导致 Channel 冗余、管理成本高，且 Channel 与 Template 职责不清。

## 渠道分类

不同渠道类型的"接收者"性质不同，设计上需要统一处理：

| 渠道类型               | 传输凭证            | 接收者                       | 可分离？   |
| ---------------------- | ------------------- | ---------------------------- | ---------- |
| 飞书 App (feishuApp)   | AppID + AppSecret   | receive_id_type + receive_id | ✅ 可分离   |
| 飞书 Bot (feishuBoot)  | AppID + AppSecret   | receive_id_type + receive_id | ✅ 可分离   |
| 钉钉机器人 (dingtalk)  | WebhookURL + Secret | 隐含在 URL 中                | ❌ 不可分离 |
| 通用 Webhook (webhook) | URL + Secret        | 隐含在 URL 中                | ❌ 不可分离 |
| 邮件 (email)           | SMTP 配置           | To 列表                      | ✅ 可分离   |

结论：**接收者应统一放在 AlertTemplate 层，用渠道无关的通用字段承载，各渠道类型的发送逻辑自行解读。对于 Webhook 类渠道，如果目标已经固定在 URL 中，Template 中该字段可为空，发送时回退使用 Channel 中的 URL。但这也意味着 Webhook 类渠道要多个接收者仍需多个 Channel——这是 Webhook 本身的限制，不是设计缺陷。**

## 目标

- `AlertChannel.Config` 只保留**传输层凭证**（AppID/Secret、Webhook URL 等）
- `AlertTemplate` 新增**渠道无关的接收者配置**，各渠道 Notify 实现自行消费
- 一个 SDK 类 Channel（如同一飞书 App）可通过换绑 Template 切换接收者，无需重建 Channel
- 架构对未来新增渠道（钉钉、邮件等）友好

## 影响范围总览

| 层级    | 文件                           | 变更类型                                                   |
| ------- | ------------------------------ | ---------------------------------------------------------- |
| Model   | `model/alert_channel.go`       | FeishuAppConfig 移除 ReceiveIdType / ReceiveId             |
| Model   | `model/alert_template.go`      | 新增 ReceiveIdType / ReceiveId 字段（渠道通用）            |
| Types   | `base/types/alert_template.go` | Create/Update 请求新增接收者字段                           |
| Helper  | `base/helper/alert.go`         | VerificationAlertConfig 不再校验接收者；新增模板接收者校验 |
| Service | `service/v1/alert_template.go` | Create/Update 持久化接收者                                 |
| Service | `service/v1/alert_channel.go`  | 逻辑不变                                                   |
| Service | `pkg/feishu/feishu.go`         | renderAndSend / Notify 从模板读接收者                      |
| App     | `base/app/app.go`              | 不受影响                                                   |
| DB      | Migration                      | alert_templates 加两列 + 数据迁移                          |

## 详细变更步骤

### Step 1: Model — AlertTemplate 新增字段

**文件**: `model/alert_template.go`

```go
type AlertTemplate struct {
    // ... 现有字段保持不变 ...
    // 接收者配置（渠道通用，SDK 类渠道发送时使用）
    ReceiveIdType string `gorm:"column:receive_id_type;type:varchar(50);not null;default:'';comment:接收者类型(open_id/user_id/email/chat_id/空-Webhook类无需指定)" json:"receiveIdType"`
    ReceiveId     string `gorm:"column:receive_id;type:varchar(255);not null;default:'';comment:接收者ID" json:"receiveId"`
}
```

字段含义：
- 飞书 App/Bot：`receive_id_type` = `open_id|user_id|email|chat_id`，`receive_id` = 具体 ID
- 钉钉机器人：留空（Webhook URL 已决定目标群）
- 邮件：`receive_id_type` = `email`，`receive_id` = 收件地址（或后续扩展为 JSON 数组）
- 未来其他 SDK 渠道：各自按需使用

### Step 2: Model — FeishuAppConfig 移除接收者字段

**文件**: `model/alert_channel.go`

```go
// 变更前
type FeishuAppConfig struct {
    AppID         string `json:"app_id"`
    AppSecret     string `json:"app_secret"`
    ReceiveIdType string `json:"receive_id_type"`  // 移除
    ReceiveId     string `json:"receive_id"`        // 移除
}

// 变更后
type FeishuAppConfig struct {
    AppID     string `json:"app_id"`
    AppSecret string `json:"app_secret"`
}
```

### Step 3: Types — AlertTemplate 请求类型新增接收者字段

**文件**: `base/types/alert_template.go`

```go
type AlertTemplateCreateRequest struct {
    // ... 现有字段 ...
    ReceiveIdType string `json:"receiveIdType" binding:"required,oneof=open_id user_id email chat_id"`
    ReceiveId     string `json:"receiveId" binding:"required"`
}

type AlertTemplateUpdateRequest struct {
    *IDRequest
    // ... 现有字段 ...
    ReceiveIdType string `json:"receiveIdType" binding:"required,oneof=open_id user_id email chat_id"`
    ReceiveId     string `json:"receiveId" binding:"required"`
}
```

> **注意**：当前 binding 的 `oneof` 枚举是针对 SDK 类渠道的。后续如果 Webhook 类渠道也需要通过 Template 指定接收者（例如同一个 Webhook Channel 绑不同 Template 发不同群），可以把 `ReceiveIdType` 的 `required` 去掉，并在 service 层根据渠道类型做条件校验。当前阶段先按 SDK 类必须传来处理。

### Step 4: Helper — 调整校验逻辑

**文件**: `base/helper/alert.go`

- `VerificationAlertConfig` 中移除对 `receive_id` 和 `receive_id_type` 的必填校验
- 新增 `ValidateTemplateRecipient(receiveIdType, receiveId string) error` 用于 Template 层的接收者校验

### Step 5: Service — AlertTemplate Create/Update

**文件**: `service/v1/alert_template.go`

- `CreateAlerTemplate`: 写入 `ReceiveIdType` / `ReceiveId`
- `UpdateTemplate`: 更新接收者字段

### Step 6: Service — Feishu Notify 从模板读取接收者

**文件**: `pkg/feishu/feishu.go`

`renderAndSend` 签名变更：

```go
// 变更前
func (receiver *FeiShu) renderAndSend(ctx context.Context, larkCli *lark.Client, conf *model.FeishuAppConfig, ...) error

// 变更后
func (receiver *FeiShu) renderAndSend(ctx context.Context, larkCli *lark.Client, receiveIdType, receiveId string, ...) error
```

`Notify` 方法中：
- `GetFeishuAppConfig()` 仅用于获取 AppID/AppSecret → 建连
- 接收者从 `alertChannel.AlertTemplate.ReceiveIdType` / `ReceiveId` 获取

### Step 7: DB Migration

```sql
ALTER TABLE alert_templates
    ADD COLUMN receive_id_type VARCHAR(50) NOT NULL DEFAULT '' COMMENT '接收者类型',
    ADD COLUMN receive_id VARCHAR(255) NOT NULL DEFAULT '' COMMENT '接收者ID';
```

数据迁移脚本：
1. 遍历 `alert_channels`（`type = 'feishuApp'` 且有 `alert_template_id`）
2. 从 `config` JSON 中提取 `receive_id_type` / `receive_id`
3. 写入对应 `alert_templates` 行
4. 从 `alert_channels.config` JSON 中移除 `receive_id_type` / `receive_id`

### Step 8: GORM Gen 重新生成 Store

修改 Model 后重新运行 `gormgen/main.go`。

## 未来新增渠道的扩展模式

以新增**钉钉机器人**为例：

1. `AlertChannel.Config` 存入 `{"webhook_url": "...", "secret": "..."}`（传输凭证）
2. `AlertTemplate.ReceiveIdType` / `ReceiveId` 留空（Webhook URL 已决定目标）
3. 如需同一机器人发多个群：仍需多个 Channel（Webhook 自身限制），但 Template 可复用
4. 发送时 `pkg/dingtalk/dingtalk.go` 的 `Notify` 实现只从 Channel.Config 读 URL，忽略 Template 接收者字段

以新增**邮件**为例：

1. `AlertChannel.Config` 存入 `{"smtp_host": "...", "smtp_port": 465, "username": "...", "password": "..."}`
2. `AlertTemplate.ReceiveIdType` = `email`，`ReceiveId` = 收件地址（或后续扩展为 JSON 数组支持多个收件人）
3. 同一 SMTP 服务器可通过不同 Template 发给不同收件人，无需重复建 Channel

## 风险与注意事项

1. **API 兼容性**：前端创建/更新 AlertChannel 时不再需传 `receive_id_type` / `receive_id`；创建/更新 AlertTemplate 时变为必传。需协调前端同步修改。
2. **数据一致性**：迁移期间不能有新的 channel/template 创建操作，建议维护窗口执行。
3. **缓存失效**：Template 更新时已有逻辑清理关联 Channel 的 Redis 缓存。
4. **编译安全**：所有引用 `FeishuAppConfig.ReceiveIdType` / `ReceiveId` 的地方编译器会报错，不会遗漏。
5. **Template ReceiveIdType 枚举扩展**：当前 `oneof=open_id user_id email chat_id`，后续邮件等渠道可能需要扩展此枚举。

## 验证清单

- [x] Model 编译通过
- [x] Store 重新生成后编译通过
- [x] 创建 Template 时可传入接收者并持久化
- [x] 创建 Channel 时不再要求接收者字段
- [x] 告警发送到达正确的飞书会话
- [x] 数据迁移脚本执行后旧数据不丢失
