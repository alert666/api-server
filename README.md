# api-server — 告警管理 API 服务

[![Go Version](https://img.shields.io/github/go-mod/go-version/alert666/api-server)](https://go.dev)
[![Build](https://img.shields.io/github/actions/workflow/status/alert666/api-server/docker-publish.yml?branch=main)](https://github.com/alert666/api-server/actions)
[![License](https://img.shields.io/github/license/alert666/api-server)](https://github.com/alert666/api-server/blob/main/LICENSE)

基于 Go + Gin + GORM 构建的告警管理后端服务。配合[前端 UI](https://github.com/alert666/ui)，提供从告警接收、静默、抑制、模板渲染到多渠道通知的完整能力，同时支持多租户隔离、RBAC 权限控制、Agent 命令下发等平台级功能。

在线预览：[qqlx.net](https://qqlx.net/)（只读账号：`readonly@qqlx.net` / `12345678`）


## 简介

api-server 覆盖告警管理全生命周期：

1. **接收** — 作为 Prometheus Alertmanager Webhook Receiver，接收并持久化告警
2. **静默** — 基于标签匹配的静默规则，支持定时生效/失效
3. **抑制** — 自定义抑制规则引擎，解决 Alertmanager 原生抑制在告警恢复场景下的缺陷
4. **路由** — 多维度告警分发，支持模板化通知内容
5. **通知** — 多渠道推送（飞书应用消息/群机器人、邮件），支持 @ 提醒与富文本卡片
6. **Agent 通道** — 通过 gRPC Data Tunnel 向 Agent 下发命令并获取执行结果
7. **内部转发** — 多副本间通过内部 API 转发 Agent 命令，支持横向扩展


## 功能

| 模块           | 说明                                                                                            |
| -------------- | ----------------------------------------------------------------------------------------------- |
| **告警接收**   | Alertmanager Webhook 回调，支持 `extraSync` 为告警追加额外接收者，支持飞书消息中覆盖 @ 提醒对象 |
| **告警历史**   | 追踪告警完整生命周期（firing → resolved），多维筛选与分页查询                                   |
| **告警静默**   | 按标签匹配创建静默规则，支持定时生效/失效，按租户统计活跃静默数                                 |
| **告警抑制**   | 自定义抑制规则引擎，解决 Alertmanager 原生抑制在告警恢复场景下的缺陷                            |
| **告警通道**   | 支持飞书应用消息、飞书群机器人、邮件等多种通知渠道                                              |
| **告警模板**   | Go 模板引擎自定义通知内容，支持标签动态渲染与模板复制                                           |
| **Agent 命令** | 通过 gRPC Data Tunnel 向 Agent 下发命令并同步等待结果，支持跨副本转发                           |

### 平台能力

| 功能           | 说明                                                |
| -------------- | --------------------------------------------------- |
| **多租户**     | 租户级告警数据隔离，独立配置告警接收 token          |
| **RBAC**       | 基于 Casbin 的细粒度角色权限控制，支持 API 级别授权 |
| **OAuth2**     | 集成飞书、Keycloak 等第三方身份认证                 |
| **用户管理**   | 用户 CRUD、登录/登出、Token 刷新、头像管理          |
| **Swagger**    | 自动生成 API 文档，启动后访问 `/swagger/index.html` |
| **可观测性**   | 基于 OpenTelemetry 的分布式追踪与 Prometheus 指标   |
| **多副本支持** | 内部 API + gRPC Data Tunnel 支持服务横向扩展        |

> 告警模板的详细语法、变量、自定义函数及各渠道差异说明，参见 [docs/alertTemplate/README.md](docs/alertTemplate/README.md)。



## 技术栈

**语言与框架**
- Go 1.25 + Gin 路由框架 + GORM ORM + Wire 依赖注入
- gRPC 用于 Agent 数据通道
- Casbin 提供 RBAC 权限控制

**数据层**
- MySQL 持久化存储
- Redis 缓存（告警模板、频道、Token、Session）
- go-cache 本地缓存

**中间件/集成**
- JWT + OAuth2（飞书、Keycloak）身份认证
- Viper 配置管理 + Cobra 命令行
- Zap 日志 + OpenTelemetry 可观测性
- Cron 定时任务（缓存清理、抑制规则轮询）v
- Swagger 自动 API 文档

**通知渠道**
- 飞书应用消息 & 群机器人
- 邮件（SMTP）

**部署**
- Docker / Docker Compose
- CI/CD：GitHub Actions → 阿里云 ACR + GitHub Container Registry


## 项目结构

```
.
├── cmd/            # 应用入口 + Wire 依赖注入
├── base/           # 框架层
│   ├── app/        #   Application 定义与初始化
│   ├── bind/       #   请求参数绑定
│   ├── conf/       #   配置管理
│   ├── constant/   #   常量
│   ├── data/       #   数据库与 Redis 连接
│   ├── helper/     #   工具函数
│   ├── log/        #   日志初始化
│   ├── middleware/ #   HTTP 中间件（认证、鉴权、租户）
│   ├── router/     #   路由注册
│   ├── server/     #   服务生命周期管理
│   └── types/      #   共享类型定义
├── controller/     # HTTP 请求处理器
├── service/v1/     # 业务逻辑层
├── store/          # 数据访问层（GORM gen + Redis 缓存）
├── model/          # GORM 数据模型
├── grpc/           # gRPC 服务端
│   ├── handler/    #   gRPC 处理器
│   ├── interceptor/#   gRPC 拦截器
│   └── server/     #   gRPC 服务
├── pkg/            # 可复用包
│   ├── alertinhibit#   告警抑制引擎
│   ├── casbin/     #   RBAC 权限控制
│   ├── email/      #   邮件发送
│   ├── feishu/     #   飞书消息推送
│   ├── jwt/        #   JWT 令牌
│   ├── local_cache/#   本地缓存
│   └── oauth/      #   OAuth2 认证
├── test/           # 按领域组织的测试
├── docs/           # 文档、Swagger 部署文件
│   ├── deploy/     #   Docker、Nginx、SQL 脚本
│   │   ├── certs/  #   gRPC TLS 证书
│   │   └── nginx/  #   Nginx 配置
│   └── img/        #   截图
├── scripts/        # 工具脚本
├── gormgen/        # GORM gen 代码生成模板
└── .github/workflows/ # CI/CD 流水线
```


## 快速开始

需要以下依赖运行完整服务：

- Go 1.25+
- MySQL 8.0+
- Redis 7+

### 1. 克隆与配置

```bash
git clone https://github.com/alert666/api-server.git
cd api-server

# 导入数据库 schema
mysql -u root -p < docs/deploy/schema.sql

# 复制并编辑配置
cp docs/deploy/config-example.yaml config.yaml
# 修改 config.yaml 中的 mysql.host、redis.host 等连接信息
```

### 2. 运行

```bash
# 启动服务（默认监听 :8080）
go run ./cmd/apiserver -c config.yaml
```

首次启动会自动初始化，创建以下默认数据：

| 用户名  | 密码       | 角色  | 说明                      |
| ------- | ---------- | ----- | ------------------------- |
| `admin` | `12345678` | admin | 超级管理员，全部 API 权限 |

### 3. 使用 `init` 命令

仅执行数据库初始化（创建默认 API、角色、管理员用户），不启动 HTTP 服务：

```bash
go run ./cmd/apiserver init -c config.yaml
```


## 配置说明

使用 `config.yaml` 配置，基于 Viper 加载，支持环境变量覆盖。完整参考 `docs/deploy/config-example.yaml`。

```yaml
server:
  bind: 0.0.0.0:8080
  timeZone: "Asia/Shanghai"

redis:
  mode: single          # single / sentinel / cluster
  host: localhost:6379
  expireTime: 300s
  keyPrefix: tutu

# ... 更多配置项见 config-example.yaml
```

### Alertmanager 对接示例

在 Prometheus Alertmanager 中配置 Webhook Receiver：

```yaml
route:
  receiver: 'api-server'
  group_by: ['alertname', 'cluster', 'severity']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 4h

receivers:
  - name: 'api-server'
    webhook_configs:
      - url: 'http://<api-server>:8080/api/v1/alerts?templateName=<模板名称>'
        http_config:
          authorization:
            type: Bearer
            credentials: <config.yaml 中 alert.receiveToken>
```

`templateName` 为必填 query 参数，指定该路由下的告警使用哪个通知模板。

> 告警数据按 `alert.receiveToken` 字段校验；若为空白字符串则不校验认证（仅建议开发环境使用）。


## API 文档

服务启动后访问 Swagger UI：

```bash
http://localhost:8080/swagger/index.html
```

### 主要 API 分组

| 分组       | 路径                                | 认证           | 说明                      |
| ---------- | ----------------------------------- | -------------- | ------------------------- |
| 告警接收   | `POST /api/v1/alerts`               | Bearer Token   | Alertmanager Webhook 回调 |
| 告警历史   | `/api/v1/alertHistory`              | JWT + RBAC     | 告警生命周期查询          |
| 告警静默   | `/api/v1/alertSilence`              | JWT + RBAC     | 静默规则 CRUD             |
| 告警通道   | `/api/v1/alertChannel`              | JWT + RBAC     | 通知渠道 CRUD             |
| 告警模板   | `/api/v1/alertTemplate`             | JWT + RBAC     | 通知模板 CRUD             |
| 租户管理   | `/api/v1/tenant`                    | JWT + RBAC     | 多租户 CRUD               |
| 用户管理   | `/api/v1/user`                      | JWT + RBAC     | 用户 CRUD、登录/登出      |
| Token 刷新 | `POST /api/v1/user/refresh`         | Bearer Token   | 刷新 Access Token         |
| 角色管理   | `/api/v1/role`                      | JWT + RBAC     | 角色 CRUD                 |
| API 权限   | `/api/v1/api`                       | JWT + RBAC     | 接口权限定义              |
| Agent 命令 | `/api/v1/agents/commands/wait`      | JWT + RBAC     | 下发命令并等待结果        |
| OAuth2     | `/api/v1/oauth2`                    | Session        | 飞书/Keycloak 登录        |
| 健康检查   | `GET /api/v1/healthz`               | 无             | 服务健康状态              |
| 内部转发   | `POST /internal/v1/forward-command` | Internal Token | 跨副本命令转发            |


## 部署

### Docker Compose（推荐）

```bash
git clone https://github.com/alert666/api-server.git
cd api-server/docs/deploy

cp config-example.yaml config.yaml
# 修改 config.yaml 中 MySQL 和 Redis 连接信息
docker compose up -d
```

### Docker 镜像

预构建镜像（CI/CD 自动推送）：

- **GitHub Container Registry** — `ghcr.io/alert666/api-server:latest`
- **阿里云容器镜像服务** — `registry.cn-beijing.aliyuncs.com/qqlx/alertmanager:latest`

镜像标签格式：`{branch}-{short-sha}-{timestamp}`。


## 可观测性

### 构建时自动插桩

本项目使用**阿里云开源的 OpenTelemetry 自动插桩工具**（`otel`）在编译阶段自动注入可观测性能力，无需修改业务代码即可获得分布式追踪、指标和日志。

构建时通过 `OTEL` 参数控制：

```bash
# 启用 OpenTelemetry 插桩（默认）
docker build --build-arg OTEL=true -t api-server .

# 不启用 OpenTelemetry
docker build --build-arg OTEL=false -t api-server .
```

CI/CD 流水线始终使用 `OTEL=true` 构建。

### 运行时配置

编译注入的 OpenTelemetry 组件通过以下环境变量配置：

| 变量                            | 说明                | 示例值                     |
| ------------------------------- | ------------------- | -------------------------- |
| `OTEL_EXPORTER_OTLP_ENDPOINT`   | OTLP 收集器地址     | `http://10.10.10.10:30001` |
| `OTEL_EXPORTER_OTLP_PROTOCOL`   | 传输协议            | `grpc` / `http`            |
| `OTEL_SERVICE_NAME`             | 服务名称            | `api-server`               |
| `OTEL_METRICS_EXPORTER`         | Metrics 导出方式    | `prometheus`               |
| `OTEL_EXPORTER_PROMETHEUS_PORT` | Prometheus 指标端口 | `9464`                     |
| `OTEL_EXPORTER_PROMETHEUS_HOST` | Prometheus 指标主机 | `0.0.0.0`                  |

### 请求链路

无论是否开启 OTEL，每个 HTTP 请求都会自动生成唯一 `requestId`，通过它可在日志和 Trace 系统中快速定位问题链路。


## 告警抑制说明

Alertmanager 原生抑制存在已知问题：当 source 告警恢复后，target 告警的 recovery 通知也被抑制，导致数据库与 Alertmanager 状态不一致。

本服务通过自定义抑制规则引擎解决此问题：

1. 将抑制规则配置存储在数据库中
2. 定时任务轮询规则，检查 `sourceMatchers` 是否匹配到当前 firing 的告警
3. 如果存在 firing 的 source 告警，查找匹配 `targetMatchers` 且已 resolved 的 target 告警
4. 对满足 `equal`（标签相等）条件的 target 告警自动标记为 resolved

| 字段             | 说明                                                 |
| ---------------- | ---------------------------------------------------- |
| `sourceMatchers` | 源告警匹配器（抑制者），同 Prometheus 标签匹配器格式 |
| `targetMatchers` | 目标告警匹配器（被抑制者）                           |
| `equalLabels`    | 抑制生效需相等的标签键列表                           |

配置示例：

```yaml
alert:
  inhibit_rules:
    - source_matchers:
        - alertname = "节点磁盘空间不足"
        - severity = "critical"
      target_matchers:
        - alertname = "节点磁盘空间不足"
        - severity = "warning"
      equal:
        - instance
        - device
        - mountpoint
```


## Agent 数据通道

通过 gRPC 双向流实现服务端与 Agent 之间的命令下发通道：

- Agent 启动后通过 gRPC 注册到 api-server，建立长连接
- 服务端通过 `POST /api/v1/agents/commands/wait` 下发命令并同步等待结果
- 多副本场景下，通过内部 API `POST /internal/v1/forward-command` 将命令转发到 Agent 所连接的实例

**证书生成：**

```bash
bash scripts/gen-certs.sh
```

gRPC 使用 TLS 双向认证，证书文件路径在 `config.yaml` 的 `grpc.tls` 中配置。


## 开发指南

### 分层架构

代码严格分层：**controller → service/v1 → store → model**。

新增功能的开发流程：

1. `model/` — 定义 GORM 数据模型
2. `store/` — 使用 GORM gen 生成基础 CRUD 方法，实现缓存逻辑
3. `service/v1/` — 实现业务逻辑，定义 Service 接口
4. `controller/` — 实现 HTTP handler（添加 Swagger 注解）
5. `cmd/wire.go` — 在 Wire 中注册 Provider
6. 运行 `wire ./cmd/` 重新生成依赖注入代码

### GORM gen

`.gen.go` 文件由 GORM gen 工具生成，提供类型安全的数据库查询方法。生成的代码在 `store/` 目录中，勿手动编辑。

生成器定义位于 `gormgen/` 目录。

### 运行测试

```bash
# 运行全部测试
go test ./...

# 指定领域测试
go test ./test/alert/
go test ./test/store/
go test ./test/cache/
```

### 提交规范

遵循 Conventional Commits，使用中文描述：

```bash
feat: 添加告警模板复制功能
fix: 修复静默恢复告警逻辑
refactor: 完成告警模块重构
update: 更新 Dockerfile 基础镜像
```


## 相关项目

- **前端 UI** — [alert666/ui](https://github.com/alert666/ui)（Vue 3 + TypeScript）
- **gRPC Proto** — [alert666/alertmanager-proto](https://github.com/alert666/alertmanager-proto)（Agent 数据通道协议定义）

## License

[MIT](LICENSE)

