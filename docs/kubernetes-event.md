# Kubernetes Event 接入

API Server 通过 kube-eventer 的 webhook sink 接收 Kubernetes Event。接收地址为：

```text
POST /api/v1/kubernetesEvent
```

请求必须携带 `Authorization: Bearer <alert.receiveToken>` 和
`X-Tenant-Id: <cluster>`。集群名称以请求头为准，不从 body 接受覆盖。

## kube-eventer ConfigMap

kube-eventer 默认 webhook body 只有五个字段，不能满足事件检索和去重要求。
必须创建以下自定义 body ConfigMap：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: api-server-kubernetes-event-body
  namespace: kube-system
data:
  content: |-
    {
      "eventUID": {{ printf "%q" .UID }},
      "eventName": {{ printf "%q" .Name }},
      "type": {{ printf "%q" .Type }},
      "reason": {{ printf "%q" .Reason }},
      "message": {{ printf "%q" .Message }},
      "count": {{ .Count }},
      "firstTimestamp": {{ .FirstTimestamp.UTC.Format "2006-01-02T15:04:05.999999999Z07:00" | printf "%q" }},
      "lastTimestamp": {{ .LastTimestamp.UTC.Format "2006-01-02T15:04:05.999999999Z07:00" | printf "%q" }},
      "eventTime": {{ .EventTime.UTC.Format "2006-01-02T15:04:05.999999999Z07:00" | printf "%q" }},
      "action": {{ printf "%q" .Action }},
      "source": {
        "component": {{ printf "%q" .Source.Component }},
        "host": {{ printf "%q" .Source.Host }}
      },
      "reportingController": {{ printf "%q" .ReportingController }},
      "reportingInstance": {{ printf "%q" .ReportingInstance }},
      "involvedObject": {
        "kind": {{ printf "%q" .InvolvedObject.Kind }},
        "namespace": {{ printf "%q" .InvolvedObject.Namespace }},
        "name": {{ printf "%q" .InvolvedObject.Name }},
        "uid": {{ printf "%q" .InvolvedObject.UID }},
        "apiVersion": {{ printf "%q" .InvolvedObject.APIVersion }},
        "resourceVersion": {{ printf "%q" .InvolvedObject.ResourceVersion }},
        "fieldPath": {{ printf "%q" .InvolvedObject.FieldPath }}
      }
    }
```

kube-eventer ServiceAccount 需要对该 ConfigMap 具有 `get` 权限。然后增加 webhook sink：

```yaml
- '--sink=webhook:https://api-server.example.com/api/v1/kubernetesEvent?method=POST&level=Normal&header=Content-Type=application/json&header=Authorization=Bearer your-alert-receive-token&header=X-Tenant-Id=cluster-a&custom_body_configmap=api-server-kubernetes-event-body&custom_body_configmap_namespace=kube-system'
```

`level=Normal` 表示同时接收 Normal 和 Warning，改为 `Warning` 可只接收异常事件。

## 告警规则

规则接口为 `/api/v1/kubernetesEventRule`，支持创建、更新、删除、详情和分页查询。
规则中的多个 matcher 按 AND 计算，可用字段为：

```text
type, reason, namespace, kind, name, source,
reportingController, action, message
```

支持的操作符为 `=`, `!=`, `=~`, `!~`。示例：

```json
{
  "name": "PodFailedScheduling",
  "global": false,
  "status": 1,
  "severity": "warning",
  "alertTemplateID": 1,
  "description": "Pod 调度失败事件",
  "matchers": [
    {"name": "type", "type": "=", "value": "Warning"},
    {"name": "reason", "type": "=~", "value": "FailedScheduling|FailedCreate"},
    {"name": "kind", "type": "=", "value": "Pod"}
  ]
}
```

`global=true` 创建全局规则，否则规则绑定 `X-Tenant-Id` 指定的集群。规则命中后使用
关联的 AlertTemplate 调用现有告警发送流程。
