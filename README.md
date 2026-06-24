# api-server

基于 Go 的告警管理 API 服务 — 接收、静默、抑制、路由、通知，覆盖告警全生命周期。

[![Build](https://img.shields.io/github/actions/workflow/status/alert666/api-server/docker-publish.yml?branch=main)](https://github.com/alert666/api-server/actions)
[![License](https://img.shields.io/github/license/alert666/api-server)](https://github.com/alert666/api-server/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/alert666/api-server)](https://go.dev)

## 目录

- [api-server](#api-server)
  - [目录](#目录)
  - [简介](#简介)
  - [功能](#功能)
    - [告警管理](#告警管理)
    - [平台能力](#平台能力)
  - [技术栈](#技术栈)
  - [项目结构](#项目结构)
  - [快速开始](#快速开始)
    - [前置依赖](#前置依赖)
    - [本地运行](#本地运行)
  - [配置说明](#配置说明)
    - [Alertmanager 对接示例](#alertmanager-对接示例)
  - [API 文档](#api-文档)
  - [部署](#部署)
    - [Docker Compose](#docker-compose)
    - [Docker 镜像](#docker-镜像)
  - [可观测性](#可观测性)
    - [环境变量](#环境变量)
  - [告警抑制说明](#告警抑制说明)
  - [开发指南](#开发指南)
    - [添加新模块](#添加新模块)
    - [运行测试](#运行测试)
    - [提交规范](#提交规范)
  - [相关项目](#相关项目)
  - [License](#license)

## 简介

api-server 是一个告警管理后端服务，配合[前端 UI](https://github.com/alert666/ui) 使用，提供从告警接收、静默、抑制、模板渲染到多渠道通知的完整能力。

在线预览：[qqlx.net](https://qqlx.net/)（只读账号：`readonly@qqlx.net` / `12345678`）

## 功能

### 告警管理

- **告警接收** — 作为 Alertmanager Webhook 接收方，接收并持久化告警数据。支持通过 `extraSync` 参数为告警模板额外追加接收者，支持在飞书应用消息中覆盖 @ 提醒对象
- **告警历史** — 追踪告警生命周期（firing → resolved），支持多维筛选与分页查询
- **告警静默** — 按标签匹配创建静默规则，支持定时生效/失效，按租户统计活跃静默数
- **告警抑制** — 自定义抑制规则引擎，通过定时任务自动清理被抑制的告警，弥补 Alertmanager 原生抑制在恢复场景下的状态不一致问题
- **告警通道** — 管理通知渠道（飞书、邮件等），支持 CRUD
- **告警模板** — 基于 Go template 的通知模板，支持一键复制模板。新增 remote 接收者类型，从远程 HTTP 接口动态获取接收者列表并按租户过滤去重，接收者配置格式为 `url;;token;;receiveType`

### 平台能力

- **多租户（集群）管理** — 租户粒度的告警隔离，每个租户对应一个集群
- **用户管理** — 用户 CRUD，JWT 认证（支持 Refresh Token 续期与登出失效）
- **角色管理** — 角色定义与分配
- **接口权限管理** — 基于 Casbin 的 RBAC 细粒度 API 访问控制
- **OAuth2 登录** — 支持飞书、Keycloak 等多种 OAuth2 Provider
- **gRPC 数据隧道** — 双向流式 RPC，实现服务端与 Agent 的实时命令下发与结果回传
- **Agent 命令** — 通过 HTTP API 向 Agent 下发命令并同步等待结果
- **跨副本转发** — 多副本部署时自动将命令转发至目标 Agent 所在副本执行

## 技术栈

| 类别     | 技术                                                   |
| -------- | ------------------------------------------------------ |
| Web 框架 | [Gin](https://github.com/gin-gonic/gin)                |
| ORM      | [GORM](https://gorm.io/) + [gen](https://gorm.io/gen/) |
| 依赖注入 | [Wire](https://github.com/google/wire)                 |
| 访问控制 | [Casbin](https://casbin.org/)                          |
| 缓存     | [go-redis](https://redis.uptrace.dev/)                 |
| 日志     | [Zap](https://github.com/uber-go/zap)                  |
| 配置管理 | [Viper](https://github.com/spf13/viper)                |
| CLI      | [Cobra](https://github.com/spf13/cobra)                |
| 认证     | JWT + OAuth2（飞书 / Keycloak）                        |
| gRPC     | [google.golang.org/grpc](https://grpc.io/)             |
| 可观测性 | OpenTelemetry（Traces）+ Prometheus（Metrics）         |
| API 文档 | [Swagger](https://github.com/swaggo/swag)              |
| 数据库   | MySQL                                                  |
| 容器化   | Docker / Docker Compose                                |

## 项目结构

```bash
.
├── cmd/apiserver/     # 入口 + Wire 依赖注入
├── base/              # 核心框架：server、middleware、config、logger、types
├── controller/        # HTTP 请求处理层
├── service/v1/        # 业务逻辑层
├── store/             # 数据访问层（GORM + Redis）
├── model/             # GORM 数据模型
├── grpc/              # gRPC 服务端、拦截器、handler
├── pkg/               # 可复用包
│   ├── alertinhibit/  #   告警抑制引擎
│   ├── casbin/        #   Casbin 适配
│   ├── email/         #   邮件发送
│   ├── feishu/        #   飞书 SDK 封装
│   ├── jwt/           #   JWT 工具
│   ├── local_cache/   #   本地缓存
│   └── oauth/         #   OAuth2 集成
├── test/              # 测试（按 domain 分子目录）
├── docs/              # 文档、截图、部署配置
├── scripts/           # 工具脚本（gRPC 证书生成等）
└── gormgen/           # GORM gen 代码生成配置
```

调用链路：`controller → service/v1 → store`，依赖通过 Wire 注入。

## 快速开始

### 前置依赖

- Go 1.25+
- MySQL 8.0+
- Redis 7+

### 本地运行

```bash
# 克隆仓库
git clone https://github.com/alert666/api-server.git
cd api-server

# 安装依赖
go mod download

# 初始化数据库（使用 deploy 目录下的 schema.sql）
mysql -h 127.0.0.1 -P 3306 -u root -p < docs/deploy/schema.sql

# 准备配置文件
cp config-example.yaml config.yaml
# 编辑 config.yaml，填入数据库连接信息

# 生成 Wire 依赖注入代码
wire ./cmd/

# 运行
go run ./cmd/apiserver -c config.yaml
```

服务默认监听 `0.0.0.0:8080`。

## 配置说明

所有配置项均支持环境变量覆盖，环境变量前缀由 `SERVICE_NAME` 决定（默认 `API_SERVER`）。例如 `API_SERVER_SERVER_BIND=:9090` 等价于 `server.bind: ":9090"`。配置文件使用 YAML 格式，时间单位支持 `ns`、`us`（或 `µs`）、`ms`、`s`、`m`、`h`。

```yaml
server:
  bind: 0.0.0.0:8080          # HTTP 监听地址（默认 0.0.0.0:8080）
  timeZone: "Asia/Shanghai"   # 时区（默认 Asia/Shanghai）

log:
  level: info                 # 日志级别：debug / info / warn / error（默认 info）
  encoder: console            # 日志格式：console（文本）/ json（默认 console）

grpc:
  bind: 0.0.0.0:9090          # gRPC 监听地址（默认 0.0.0.0:9090）
  tls:                        # 可选，配置后启用 TLS
    certFile: /path/to/server.crt   # 服务端证书
    keyFile:  /path/to/server.key   # 服务端私钥
    caFile:   /path/to/ca.crt       # 可选，配置后启用 mTLS（双向认证）

mysql:
  debug: false                # 是否打印 SQL 日志
  username: root              # 必填
  password: ""                # 必填
  host: 127.0.0.1             # 必填
  port: 3306                  # 必填
  database: alert             # 必填
  maxIdleConns: 10            # 最大空闲连接数（默认 10）
  maxOpenConns: 30            # 最大打开连接数（默认 30）
  maxLifetime: 30m            # 连接最大存活时间（默认 30m）

redis:
  mode: single                # single 或 sentinel
  host: 127.0.0.1:6379        # single 模式必填
  username: ""                # Redis 6+ ACL 用户名（可选）
  password: ""                # 必填
  expireTime: 1h              # 缓存过期时间（默认 1h）
  keyPrefix: alert            # Redis key 前缀（必填）
  db: 0                       # 数据库编号
  poolSize: 50                # 连接池大小（默认 50）
  minIdleConns: 20            # 最小空闲连接数（默认 20）
  connMaxLifetime: 30m        # 连接最大存活时间（默认 30m）
  sentinel:                   # mode=sentinel 时必填
    hosts:                    #   Sentinel 节点列表
      - 10.0.0.1:26379
      - 10.0.0.2:26379
    masterName: mymaster      #   主节点名称
    password: ""              #   Sentinel 密码

jwt:
  issuer: api-server          # 签发者（默认 api-server）
  secret: "your-secret-key"   # 签名密钥（必填）
  accessExpireTime: 30h       # Token 过期时间（默认 30h）
  refreshExpireTime: 168h     # Refresh Token 过期时间（默认 168h，7 天）

oauth2:
  enable: true                # 是否启用 OAuth2
  providers:
    feishu:
      clientId: ""
      clientSecret: ""
      authUrl: https://accounts.feishu.cn/open-apis/authen/v1/authorize
      tokenUrl: https://open.feishu.cn/open-apis/authen/v2/oauth/token
      userInfoUrl: https://open.feishu.cn/open-apis/authen/v1/user_info
      redirectUrl: http://localhost:5173/oauth/login
    keycloak:
      clientId: ""
      clientSecret: ""
      scopes: ["openid", "email", "profile"]
      authUrl: https://keycloak.example.com/realms/myrealm/protocol/openid-connect/auth
      tokenUrl: https://keycloak.example.com/realms/myrealm/protocol/openid-connect/token
      userInfoUrl: https://keycloak.example.com/realms/myrealm/protocol/openid-connect/userinfo
      redirectUrl: http://localhost:5173/oauth/login

alert:
  receiveToken: ""            # 告警接收认证 token（不配置则不校验认证）
  # 示例（Alertmanager 侧需配置对应的 Authorization 头）：
  # receiveToken: "a8f5c2e3-9b1d-4f6e-8c2a-1d3e5f7a9b0c"
  tenantKey: cluster          # 租户标签键名，从告警 label 中提取（默认 cluster）
  repeatInterval: 4h          # 告警重复发送间隔
  extraSync:
    # alerts 接口 extraSync url 参数的值
    # 注意事项: 当前告警发送的 TemplateName 绑定的 AlertChannel 必须有权限发送消息至这个接收者
    # 假如: A TemplateName 绑定了 A AlertChannel, 那么 A AlertChannel 必须能发送至这个接收者
    idc:
      # 类型: map[string]string.

      # key: alert.labels.["cluster"], 作用是将指定集群的告警发送至指定的接收这个.

      # value: 发送给对应的接收的信息, 支持覆盖模板中的 at 用户
      # 格式: receiveID;;<at id=xxx></>
      # receiveID 如飞书群 id 或 邮箱发送时的接受者的邮箱.
      # <at id=xxx></> 仅支持飞书卡片, 可选

      # 效果: 当告警的 cluster 标签匹配 cn-henan-2 之后, 会额外发送告警至 oc_f63570b503b00bce9155bf92539b5dac 接收者。
      cn-henan-2:
        - oc_f63570b503b00bce9155bf92539b5dac
        # - oc_f63570b503b00bce9155bf92539b5dac;;<at id=userID></at>
  inhibit_rules:              # 告警抑制规则列表（详见「告警抑制说明」）
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

internal:
  token: internal-shared-secret   # 内部 API 认证 token（用于跨副本命令转发）
  advertiseAddr: ""               # 可选，手动指定本实例广播地址（默认自动探测）

```

### Alertmanager 对接示例

Prometheus Alertmanager 配置 webhook receiver 示例（token 需与 config.yaml 中 `alert.receiveToken` 一致）：

```yaml
# alertmanager.yml
route:
  receiver: 'api-server'
  group_by: ['alertname', 'cluster']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 4h
  http_config:
    authorization:
      type: Bearer
      credentials: '<your-global-token>'

receivers:
  - name: 'api-server'
    webhook_configs:
      - url: 'http://<api-server-host>:8080/api/v1/alerts?templateName=<template-name>'
        http_config:
          authorization:
            type: Bearer
            credentials: <token from config.yaml alert.receiveToken>
```

## API 文档

项目集成了 Swagger 自动文档。启动服务后访问：

```bash
http://localhost:8080/swagger/index.html
```

主要 API 分组：

| 分组             | 路径前缀                | 说明              |
| ---------------- | ----------------------- | ----------------- |
| 告警接收         | `/api/v1/alerts`        | Alertmanager 回调 |
| 告警历史         | `/api/v1/alertHistory`  | 告警生命周期查询  |
| 告警静默         | `/api/v1/alertSilence`  | 静默规则管理      |
| 告警通道         | `/api/v1/alertChannel`  | 通知渠道管理      |
| 告警模板         | `/api/v1/alertTemplate` | 通知模板管理      |
| 集群（租户）管理 | `/api/v1/cluster`       | 多租户管理        |
| 用户管理         | `/api/v1/user`          | 用户 CRUD         |
| Token 刷新       | `/api/v1/user/refresh`  | 刷新 Access Token |
| 角色管理         | `/api/v1/role`          | 角色 CRUD         |
| 接口权限         | `/api/v1/api`           | API 权限管理      |
| Agent 命令       | `/api/v1/agentCommand`  | 下发命令到 Agent  |

## 部署

### Docker Compose

```bash
# 构建前端资源
git clone -b main https://github.com/yiran15/ui.git
cd ui && make deploy

# 构建并启动服务
git clone https://github.com/alert666/api-server.git
cd api-server/docs/deploy
cp config-example.yaml config.yaml
# 修改 config.yaml 中的数据库和 Redis 配置
docker compose up -d
```

### Docker 镜像

预构建镜像可从以下仓库获取：

- GitHub Container Registry: `ghcr.io/alert666/api-server:latest`
- 阿里云容器镜像: `registry.cn-beijing.aliyuncs.com/qqlx/alertmanager:latest`

## 可观测性

基于 OpenTelemetry 提供分布式追踪（Traces）和指标（Metrics）。

### 环境变量

| 变量                            | 说明                | 示例值                     |
| ------------------------------- | ------------------- | -------------------------- |
| `OTEL_EXPORTER_OTLP_ENDPOINT`   | OTLP 收集器地址     | `http://10.10.10.10:30001` |
| `OTEL_EXPORTER_OTLP_PROTOCOL`   | 传输协议            | `grpc` / `http`            |
| `OTEL_SERVICE_NAME`             | 服务名称            | `api-server`               |
| `OTEL_METRICS_EXPORTER`         | Metrics 导出方式    | `prometheus`               |
| `OTEL_EXPORTER_PROMETHEUS_PORT` | Prometheus 指标端口 | `9464`                     |
| `OTEL_EXPORTER_PROMETHEUS_HOST` | Prometheus 指标主机 | `0.0.0.0`                  |

每次请求都会生成唯一的 `requestId`，通过它可以在日志和 Trace 系统中快速定位问题链路。

## 告警抑制说明

Alertmanager 原生抑制在告警恢复场景下存在已知问题：当 source 告警恢复后，target 告警的 recovery 通知也会被抑制，导致数据库中 target 告警状态与 Alertmanager 中的实际状态不一致。

本服务通过自定义抑制规则引擎解决此问题：

1. 将 Alertmanager 的所有抑制规则配置到本服务的抑制规则表中
2. 定时任务定期轮询抑制规则，检查 `source_matchers` 匹配的 firing 告警
3. 若存在 firing 的 source 告警，则查找符合 `target_matchers` 的 resolved 告警
4. 对满足 `equal` 标签匹配条件的 target 告警，自动标记为 resolved

抑制规则模型：

| 字段             | 说明                                                 |
| ---------------- | ---------------------------------------------------- |
| `sourceMatchers` | 源告警匹配器（抑制者），格式同 Prometheus 标签匹配器 |
| `targetMatchers` | 目标告警匹配器（被抑制者）                           |
| `equalLabels`    | 必须相等的标签列表                                   |

## 开发指南

### 添加新模块

遵循分层架构，新增功能时按以下顺序添加代码：

1. `model/` — 定义数据模型
2. `store/` — 实现数据访问（使用 GORM gen 生成基础 CRUD）
3. `service/v1/` — 实现业务逻辑
4. `controller/` — 实现 HTTP handler（添加 Swagger 注解）
5. `cmd/apiserver/` — 在 Wire 中注册 Provider，运行 `wire ./cmd/` 重新生成

### 运行测试

```bash
# 全部测试
go test ./...

# 指定 domain 测试
go test ./test/alert/
go test ./test/store/
```

### 提交规范

使用 Conventional Commits（中文描述）：

```bash
feat: 添加告警模板复制功能
fix: 修复静默恢复告警逻辑
refactor: 完成告警模块重构
```

## 相关项目

- 前端 UI：[alert666/ui](https://github.com/alert666/ui)
- Alertmanager Proto：[alert666/alertmanager-proto](https://github.com/alert666/alertmanager-proto)

## License

[MIT](LICENSE)
