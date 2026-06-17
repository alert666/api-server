 # 实现计划：告警模板多接收者 & 邮箱渠道
 
 ## 总览
 
 - 飞书渠道：一个模板支持多个 ReceiveId（JSON 数组）
 - 邮箱渠道：新增渠道类型，SMTP 凭证在渠道 Config 中，收件人在模板 ReceiveId 中
 - 模板统一：所有渠道的接收者信息都放在模板侧，渠道只存凭证
 
 ---
 
 ### 任务 1：`AlertTemplate.ReceiveId` 改为 JSON 数组
 
 **现状**: `ReceiveId` 是单值 `varchar(255)`
 **目标**: 改为 JSON 字符串数组，如 `["chat_111","chat_222"]`
 
 **修改文件**:
 
 - [x] `model/alert_template.go` — `ReceiveId` 类型改为 `string` 不动（存 JSON），数据库注释改为"接收者ID列表(JSON 数组)"；`AlertChannelID` 类型改为 `int`
 - [x] `deploy/schema.sql` — `alert_templates.receive_id` 列类型从 `varchar(255)` 改为 `text`
 - [x] `base/types/alert_template.go` — `AlertTemplateCreateRequest` 和 `AlertTemplateUpdateRequest` 的 `ReceiveId` 从 `string` 改为 `[]string`，binding 改为 `required` 保留；`ReceiveIdType` 的 `oneof` 增加 `email`
 - [x] `base/helper/alert.go` — `ValidateTemplateRecipient` 入参改为 `[]string`，遍历每项校验；增加 `ChannelTypeEmail` 分支到 `VerificationAlertConfig`
 - [x] `base/helper/feishu.go` （如果涉及模板渲染） — 确认无硬编码单值依赖
 
 **Service 层**:
 
 - [x] `service/v1/alert_template.go` — `CreateAlerTemplate`：请求的 `ReceiveId []string` 经 `json.Marshal` 后存入 `model.ReceiveId`；校验调用改为传数组
 - [x] `service/v1/alert_template.go` — `UpdateTemplate`：同上
 - [x] `service/v1/alert_template.go` — `CopyTemplate`：同上
 
 **发送逻辑**:
 
 - [x] `pkg/feishu/feishu.go` — `Notify()` 中从 `alertTemplate.ReceiveId` 解析 `[]string`，循环调用 `renderAndSend`
 - [x] `pkg/feishu/feishu.go` — `singleSend()` 同理循环
 - [x] `service/v1/alerts.go` — `getTemplate()` 确认缓存逻辑不受影响
 
 ---
 
 ### 任务 2：新增邮箱渠道类型
 
 **现状**: `EmailConfig` 已定义但包含 `To []string`，`ChannelType` 没有 `email`，无发送实现
 **目标**: 新增 `email` 渠道，`EmailConfig` 去掉 `To`，创建 `pkg/email/` 发送模块
 
 **修改文件**:
 
 - [x] `model/alert_channel.go` — 添加 `ChannelTypeEmail ChannelType = "email"`；`EmailConfig` 移除 `To []string` 字段
 - [x] `base/types/alert_channel.go` — 所有 binding `oneof` 增加 `email`：`AlertChannelCreateRequest`、`AlertChannelUpdateRequest`、`AlertChannelListRequest`
 - [x] `base/helper/alert.go` — `VerificationAlertConfig` 增加 `case model.ChannelTypeEmail:`，校验 `smtp_host`、`smtp_port`、`username`、`password` 不为空
 - [x] `model/alert_template.go` — 无变动（ReceiveId 复用数组机制，`receive_id_type` 传 `email`）
 
 **新增文件**:
 
 - [x] `pkg/email/email.go` — `Emailer` 接口 + 实现
   - `Notify(ctx, notifyReq) -> (*types.NotifySendResult, error)`
   - 解析渠道 Config 为 `EmailConfig`
   - 从模板 `ReceiveId` 解析收件人列表（JSON 数组）
   - 渲染模板（Go `text/template`）
   - 使用 `net/smtp` 或 `gomail.v2` 发送邮件
   - 支持聚合/非聚合两种模式（参考 feishu 的 `Notify` 实现结构）
 
 - [x] `pkg/email/provider.go` — Wire Provider 函数
   ```go
   package email
   import "github.com/google/wire"
   var EmailProviderSet = wire.NewSet(NewEmailer)
   ```
 
 **发送集成**:
 
 - [x] `service/v1/alerts.go` — `alertsService` 增加 `emailImpl email.Emailer` 字段
 - [x] `service/v1/alerts.go` — `SendAlert` switch 增加 `case model.ChannelTypeEmail:` 分支
 - [x] `service/v1/alerts.go` — `getTemplate` 增加 `case model.ChannelTypeEmail:`（无客户端初始化，只需校验渠道配置合法）
 - [x] `service/v1/alerts.go` — `NewAlertsServicer` 入参增加 `emailImpl`
 
 **DI 注入**:
 
 - [x] `pkg/provider.go` — `PkgProviderSet` 添加 `email.NewEmailer`
 - [x] `cmd/wire.go` — 无需改动（通过 `PkgProviderSet` 自动引入）
 - [x] `service/provider.go` — 无需改动（`ServiceProviderSet` 已包含 `NewAlertsServicer`，更新构造函数后自动生效）
 - [x] 执行 `wire ./cmd/` — 已重新生成 DI
 
 **渠道 CRUD 额外处理**:
 
 - [x] `service/v1/alert_channel.go` — CreateAlerChannel` 中 `publish` 事件跳过 email 渠道（无飞书 SDK 客户端）
 - [x] `service/v1/alert_channel.go` — UpdateChannel` 同理跳过 email
 - [x] `service/v1/alert_channel.go` — DeleteChannel` 同理跳过 email
 
 ---
 
### 任务 3：数据迁移

 - [x] `deploy/schema.sql` 同步更新建表语句（已在任务 1 完成）
 - [ ] 生产库执行 SQL 迁移：

```sql
-- 1. 修改列类型
ALTER TABLE alert_templates MODIFY COLUMN receive_id TEXT NOT NULL COMMENT '接收者ID列表(JSON 数组)';

-- 2. 已有单值数据包裹为 JSON 数组
UPDATE alert_templates 
SET receive_id = CONCAT('["', receive_id, '"]') 
WHERE receive_id != '' AND receive_id NOT LIKE '[%';
```

---

### 依赖（按需确认）
 
 - [ ] 邮件发送方式：用 `net/smtp` 标准库（无需加依赖）vs `gomail.v2`（需 `go get gopkg.in/mail.v2`）— **推荐用 `net/smtp` 减少依赖**
 
 ---
 
 ## 执行顺序
 
 ```
 任务 1 (ReceiveId 数组化) → 任务 2 (邮箱渠道) → 任务 3 (数据迁移)
 ```
 
 任务 1 是基础，邮件渠道依赖 ReceiveId 数组机制来存放多收件人。
