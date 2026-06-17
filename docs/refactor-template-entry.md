# 重构：告警入口参数从 channelName 改为 templateName

## 问题背景

上一轮重构已将接收者配置（`ReceiveIdType` / `ReceiveId`）从 `AlertChannel` 移动到 `AlertTemplate`。但告警入口仍然通过 `channelName` 定位。当前 Channel ↔ Template 是 1:1 绑定（`AlertChannel.AlertTemplateID`），一个飞书 App 要发给不同群仍需多个 Channel，与"一个 App 一个 Channel"的初衷矛盾。

## 目标

- 反转 FK 关系：`AlertTemplate` 持有 `AlertChannelID`，实现 **一个 Channel → 多个 Template**
- 告警入口参数改为 `templateName`，语义上明确"用哪个模板发给谁"
- Alertmanager Webhook URL 从 `?channelName=xxx` 变为 `?templateName=xxx`

## 数据模型变更

### 变更前

```
AlertChannel ───(AlertTemplateID)───> AlertTemplate
  1:1，每个 Channel 只能绑一个 Template
```

### 变更后

```
AlertTemplate ───(AlertChannelID)───> AlertChannel
  N:1，同一个 Channel 可绑多个 Template，每个 Template 指定不同接收者
```

## 影响范围总览

| 层级    | 文件                           | 变更类型                                                                         |
| ------- | ------------------------------ | -------------------------------------------------------------------------------- |
| Model   | `model/alert_channel.go`       | 移除 `AlertTemplateID` / `AlertTemplate` 字段                                    |
| Model   | `model/alert_template.go`      | 新增 `AlertChannelID` / `AlertChannel` 字段                                      |
| Types   | `base/types/alert_channel.go`  | 移除 `TemplateID` 字段                                                           |
| Types   | `base/types/alert_template.go` | Create/Update 请求新增 `AlertChannelID`                                          |
| Types   | `base/types/alerts.go`         | `AlertReceiveReq.ChannelName` → `TemplateName`                                   |
| Service | `service/v1/alerts.go`         | `getChannel` → `getTemplate`；通过 Template 反查 Channel 获取凭据                |
| Service | `service/v1/alert_channel.go`  | Create/Update/缓存逻辑移除模板相关代码                                           |
| Service | `service/v1/alert_template.go` | Create/Update/Delete 加入 Channel 关联；更新时需要同步缓存                       |
| Service | `pkg/feishu/feishu.go`         | `Notify` 从 `template.AlertChannel` 取凭据                                       |
| App     | `base/app/app.go`              | 启动初始化从 Template 预加载 Channel 建连；订阅逻辑调整                          |
| Helper  | `base/helper/alert.go`         | 校验逻辑调整                                                                     |
| DB      | Migration                      | `alert_templates` 加 `alert_channel_id`；`alert_channels` 删 `alert_template_id` |
| Store   | `store/`                       | 重新生成                                                                         |

## 详细变更步骤

### Step 1: Model — 反转 FK

**`model/alert_channel.go`** — 移除：
```go
AlertTemplateID int            `gorm:"column:alert_template_id;index;..."`
AlertTemplate   *AlertTemplate `gorm:"foreignKey:AlertTemplateID" json:"alertTemplate,omitempty"`
```

**`model/alert_template.go`** — 新增：
```go
AlertChannelID int           `gorm:"column:alert_channel_id;index;not null;comment:关联的告警渠道ID" json:"alertChannelID"`
AlertChannel   *AlertChannel `gorm:"foreignKey:AlertChannelID" json:"alertChannel,omitempty"`
```

### Step 2: Types — 调整请求结构

**`base/types/alert_channel.go`** — `AlertChannelUpdateRequest` 移除 `TemplateID` 字段。

**`base/types/alert_template.go`** — `AlertTemplateCreateRequest` / `AlertTemplateUpdateRequest` 新增：
```go
AlertChannelID int `json:"alertChannelID" binding:"required"`
```

**`base/types/alerts.go`** — `AlertReceiveReq`：
```go
// 变更前
ChannelName string `form:"channelName" binding:"required"`

// 变更后
TemplateName string `form:"templateName" binding:"required"`
```

### Step 3: Service — SendAlert 入口改造

**`service/v1/alerts.go`** — `SendAlert`：
```go
// 变更前: 通过 channelName 找 Channel → 取 Template
alertChannel, err := receiver.getChannel(ctx, req.ChannelName)

// 变更后: 通过 templateName 找 Template → 取 Channel
alertTemplate, err := receiver.getTemplate(ctx, req.TemplateName)
```

`getTemplate` 新方法：
```go
func (receiver *alertsService) getTemplate(ctx context.Context, templateName string) (*model.AlertTemplate, error) {
    // 1. 尝试从缓存获取
    // 2. 缓存缺失时从 DB 加载，Preload AlertChannel
    // 3. 根据 AlertChannel.Type 初始化/获取 SDK 客户端
    // 4. 缓存 template
}
```

`Notify` 调用处需调整：`notifyReq` 中传入 `alertTemplate` 而非 `alertChannel`。

### Step 4: Service — AlertChannel CRUD 简化

**`service/v1/alert_channel.go`**：
- `CreateAlerChannel`: 移除 `AlertTemplateID` 相关代码
- `UpdateChannel`: 移除模板绑定/解绑/缓存更新逻辑（这些逻辑移到 Template 服务）
- `ListChannel`: 不再 Preload AlertTemplate

### Step 5: Service — AlertTemplate CRUD 改造

**`service/v1/alert_template.go`**：
- `CreateAlerTemplate`: 校验 `AlertChannelID` 对应的 Channel 存在且启用
- `UpdateTemplate`: 若 `AlertChannelID` 变更，需清理旧 Channel 的缓存、更新新 Channel 的缓存
- `DeleteTemplate`: 不再检查是否有 Channel 绑定（因为 FK 方向反了）
- `ListTemplate`: 可 Preload AlertChannel

### Step 6: Service — Feishu Notify 调整

**`pkg/feishu/feishu.go`** — `Notify`：

当前从 `notifyReq.AlertChannel` 同时取凭据（`GetFeishuAppConfig`）和模板（`AlertTemplate`）。改造后两者来源于不同路径：
- 凭据：`notifyReq.AlertTemplate.AlertChannel.GetFeishuAppConfig()`
- 模板内容 + 接收者：`notifyReq.AlertTemplate`

```go
func (receiver *FeiShu) Notify(ctx context.Context, notifyReq *types.NotifyReq) {
    alertTemplate := notifyReq.AlertTemplate
    alertChannel := alertTemplate.AlertChannel
    
    feishuAppConf, err := alertChannel.GetFeishuAppConfig()
    larkCli, err := receiver.GetCli(alertChannel.Name, feishuAppConf.AppID, feishuAppConf.AppSecret)
    
    // 接收者从模板取
    receiveIdType := alertTemplate.ReceiveIdType
    receiveId := alertTemplate.ReceiveId
    // ...
}
```

### Step 7: App — 初始化逻辑调整

**`base/app/app.go`** — `Init`：

当前遍历 `AlertChannel`（Preload AlertTemplate）初始化 SDK 客户端。改造后遍历 `AlertTemplate`（Preload AlertChannel），按 Channel 去重初始化。

```go
// 遍历启用的 Template，Preload AlertChannel
templates, err := store.AlertTemplate.
    Preload(store.AlertTemplate.AlertChannel).
    Joins("JOIN alert_channels ON alert_channels.id = alert_templates.alert_channel_id").
    Where("alert_channels.status = ?", model.StatusEnabled).
    Find()

// 按 Channel 去重初始化 SDK 客户端
seen := map[int]bool{}
for _, t := range templates {
    if seen[t.AlertChannelID] { continue }
    seen[t.AlertChannelID] = true
    switch t.AlertChannel.Type {
    case model.ChannelTypeFeishuApp:
        // ...
    }
}
```

频道更新/删除订阅逻辑也需调整：收到 publish 消息后，通过 Template 查找关联该 Channel 的所有 Template 来决策是否需要关闭客户端。

### Step 8: DB Migration

项目使用逻辑外键（GORM `foreignKey` 标签 + SQL 普通索引），无数据库级 `FOREIGN KEY` 约束，迁移只需处理列和索引。

```sql
-- Stage 1: alert_templates 新增 channel 关联列
ALTER TABLE alert_templates 
    ADD COLUMN alert_channel_id INT NOT NULL DEFAULT 0 COMMENT '关联的告警渠道ID';
CREATE INDEX idx_alert_templates_alert_channel_id ON alert_templates (alert_channel_id);

-- Stage 2: 数据迁移前检查边界情况

-- 2a. 是否有多个 Channel 绑了同一个 Template？
--     如果有结果，需人工决定：合并为一个模板 or 拆成多个模板各绑不同 Channel
SELECT alert_template_id, COUNT(*) c FROM alert_channels 
WHERE alert_template_id IS NOT NULL AND alert_template_id > 0 
GROUP BY alert_template_id HAVING c > 1;

-- 2b. 是否有 Template 未被任何 Channel 绑定？
--     如果有结果，需人工决定：绑定到某个 Channel or 删除
SELECT id, name FROM alert_templates 
WHERE id NOT IN (SELECT DISTINCT alert_template_id FROM alert_channels WHERE alert_template_id > 0);

-- Stage 3: 从 alert_channels.alert_template_id 反填 alert_templates.alert_channel_id
UPDATE alert_templates t
JOIN alert_channels c ON c.alert_template_id = t.id
SET t.alert_channel_id = c.id;

-- Stage 4: 验证无遗漏（应为 0 行）
SELECT id, name FROM alert_templates WHERE alert_channel_id = 0;

-- Stage 5: 确认无误后移除旧列和索引
ALTER TABLE alert_channels DROP INDEX idx_alert_channels_alert_template_id;
ALTER TABLE alert_channels DROP COLUMN alert_template_id;
```

### Step 9: GORM Gen 重新生成 Store

### Step 10: Alertmanager 配置变更

所有 Webhook URL 从 `?channelName=xxx` 改为 `?templateName=xxx`。

**示例 — Prometheus Operator `alertmanager.yaml` Secret：**

```yaml
# 变更前
receivers:
  - name: 'prometheusalert'
    webhook_configs:
      - url: 'https://qqlx.net/api/v1/alerts?channelName=feishu'

# 变更后
receivers:
  - name: 'prometheusalert'
    webhook_configs:
      - url: 'https://qqlx.net/api/v1/alerts?templateName=feishu'
```

**多个 receiver 对应不同模板：**

```yaml
receivers:
  - name: 'feishu-ops'
    webhook_configs:
      - url: 'https://qqlx.net/api/v1/alerts?templateName=ops-template'
  - name: 'feishu-dev'
    webhook_configs:
      - url: 'https://qqlx.net/api/v1/alerts?templateName=dev-template'
```

**Reload：**

```bash
# Alertmanager API
curl -X POST http://alertmanager:9093/-/reload

# 或 K8s
kubectl rollout restart statefulset alertmanager-main -n monitoring
```

## 风险

1. **Alertmanager 配置需要同步更新**，否则现有告警会因参数名不匹配而失败
2. **数据迁移**需在维护窗口执行，确保 `alert_channel_id` 填充完成后再删旧列
3. **缓存键**从 `channelName` 变为 `templateName`，需要在 Redis 中同步清理旧缓存
4. **NotifyReq 结构体**需要把 `AlertChannel` 字段改为 `AlertTemplate`（或两者都保留作为过渡）

## 验证清单

- [x] Model 反转 FK 后编译通过
- [x] Template 创建时可绑定 Channel
- [x] 同一个 Channel 可绑多个 Template
- [x] 通过 `templateName` 接收告警并正确路由到对应 Channel + 接收者
- [x] DB 迁移后旧数据不丢失
- [x] Alertmanager 配置更新后告警正常送达

# SQL 记录 - 2026-06-16

## 1. alert_templates 唯一索引改为普通索引
```sql
ALTER TABLE alert_templates DROP INDEX idx_alert_templates_name;
ALTER TABLE alert_templates ADD INDEX idx_alert_templates_name (name);
```

## 2. alert_channels 唯一索引改为普通索引
```sql
ALTER TABLE alert_channels DROP INDEX idx_alert_channels_name;
ALTER TABLE alert_channels ADD INDEX idx_alert_channels_name (name);
```

## 3. alert_templates.receive_id 修复 NULL 值
```sql
UPDATE alert_templates SET receive_id = '' WHERE receive_id IS NULL;
```
