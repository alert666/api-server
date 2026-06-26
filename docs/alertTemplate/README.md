# 告警模板

告警模板定义了告警通知的展示内容和格式。模板使用 **Go 模板引擎** 渲染，支持条件判断、循环、自定义函数等能力，实现根据告警标签动态生成通知内容。

## 模板位置

模板文件存放在 `docs/alertTemplate/` 目录下，按通知渠道分类：

```
docs/alertTemplate/
├── email/            # 邮件通知模板（HTML）
│   ├── alert.html          # 单条告警邮件模板
│   └── aggregationAlert.html  # 聚合告警邮件模板
├── feishu-app/       # 飞书应用消息模板（YAML + 卡片）
│   ├── alert.yaml          # 单条告警飞书卡片模板
│   ├── aggregationAlert.yaml  # 聚合告警飞书卡片模板
│   └── 告警卡片.card        # 飞书卡片定义参考
```

> 这些是示例模板，用于展示模板语法和可用变量。实际使用中，模板内容通过前端 UI 在 `告警模板` 页面进行配置和管理，持久化存储在数据库中，而非直接读取文件。

## 模板分类

每个通知渠道包含两类模板：

| 模板类型                               | 说明                     | 适用场景                          |
| -------------------------------------- | ------------------------ | --------------------------------- |
| **单条告警模板**（`alert`）            | 渲染一条告警的详细信息   | 单次触发告警时发送                |
| **聚合告警模板**（`aggregationAlert`） | 将多条告警聚合并渲染摘要 | Alertmanager 合并后的批量告警通知 |

## 模板变量

### 单条告警模板可用变量

| 变量            | 类型                | 说明                                        |
| --------------- | ------------------- | ------------------------------------------- |
| `.Labels`       | `map[string]string` | 告警标签（alertname、severity、cluster 等） |
| `.Annotations`  | `map[string]string` | 告警注解                                    |
| `.StartsAt`     | `time.Time`         | 告警开始时间                                |
| `.EndsAt`       | `time.Time`         | 告警结束时间                                |
| `.GeneratorURL` | `string`            | 告警来源链接（如 Grafana）                  |

### 聚合告警模板可用变量

| 变量               | 类型      | 说明                                        |
| ------------------ | --------- | ------------------------------------------- |
| `.Alerts`          | `[]Alert` | 告警列表，取 `index .Alerts 0` 获取首条告警 |
| `.Alerts.Firing`   | `[]Alert` | 当前 firing 的告警列表                      |
| `.Alerts.Resolved` | `[]Alert` | 当前 resolved 的告警列表                    |
| `len .Alerts`      | `int`     | 聚合告警数量                                |

## 自定义函数

模板中可使用以下自定义函数：

| 函数                    | 说明                               | 示例                                                                             |
| ----------------------- | ---------------------------------- | -------------------------------------------------------------------------------- |
| `timeFormat`            | 格式化时间                         | `{{ timeFormat .StartsAt }}`                                                     |
| `getEndTime`            | 格式化结束时间，未结束显示默认文本 | `{{ getEndTime .EndsAt "告警未恢复" }}`                                          |
| `getClusterLabel`       | 格式化集群标签                     | `{{ getClusterLabel (index .Labels "cluster") }}`                                |
| `getDescript`           | 获取告警描述（支持单条和聚合）     | `{{ getDescript . }}` / `{{ getDescript .Alerts }}`                              |
| `newViewLink`           | 生成 Grafana 跳转链接              | `{{ newViewLink (getGrafanaExploreLink "https://xxx" .GeneratorURL "thanos") }}` |
| `getGrafanaExploreLink` | 构造 Grafana Explore 链接          | `{{ getGrafanaExploreLink "https://xxx" .GeneratorURL "thanos" }}`               |
| `newAlertManagerLink`   | 生成平台告警历史跳转链接           | `{{ newAlertManagerLink "https://xxx/history" (index .Labels "cluster") }}`      |

## 通知渠道差异

| 渠道             | 模板格式                       | 推送方式                  | 特性                                    |
| ---------------- | ------------------------------ | ------------------------- | --------------------------------------- |
| **飞书应用消息** | YAML（模板变量） + 飞书卡片 ID | 飞书开放 API 发送应用消息 | 支持 @ 指定用户、富文本卡片、自定义跳转 |
| **飞书群机器人** | YAML 或消息模板                | Webhook URL 推送          | 群内通知，支持简单 @                    |
| **邮件**         | HTML                           | SMTP 发送                 | 可自定义 HTML 样式，适合邮件客户端      |

## 相关配置

告警模板通过以下方式与系统联动：

1. **Alertmanager 对接** — 在 Webhook URL 的 `templateName` query 参数中指定模板名称
2. **告警通道绑定** — 模板绑定到告警通道（AlertChannel），决定通过哪个渠道发送通知
3. **extraSync** — 可通过 `config.yaml` 的 `alert.extraSync` 配置，按标签将告警额外发送到指定接收者
