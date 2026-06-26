# 仓库指南

## 项目结构与模块组织

这是一个基于 Gin、GORM 和 Wire 构建的 Go Web API 服务。源码按分层包组织：

| 目录          | 说明                                                      |
| ------------- | --------------------------------------------------------- |
| `cmd/`        | 应用入口与 Wire 依赖注入                                  |
| `base/`       | 核心框架 — 服务启动、中间件、类型定义、日志、配置         |
| `controller/` | HTTP 请求处理器                                           |
| `service/v1/` | 业务逻辑层                                                |
| `store/`      | 数据访问层（GORM 查询与缓存逻辑）                         |
| `model/`      | GORM 数据模型                                             |
| `grpc/`       | gRPC 服务器与处理器                                       |
| `pkg/`        | 可复用包（Casbin、飞书、JWT、OAuth2、告警抑制、本地缓存） |
| `test/`       | 按领域子目录组织的测试                                    |
| `deploy/`     | Dockerfile、docker-compose.yaml、nginx.conf、schema.sql   |
| `scripts/`    | 工具脚本（如 gRPC 证书生成）                              |
| `docs/`       | 文档与截图                                                |
| `gormgen/`    | GORM 模型代码生成                                         |

## 构建、测试与开发命令

```bash
# 运行所有测试
go test ./...

# 运行指定领域目录的测试
go test ./test/alert/

# 编译二进制
go build ./cmd/apiserver

# 本地运行服务（需要 config.yaml 和运行中的 MySQL/Redis）
go run ./cmd/apiserver -c config.yaml

# 更新 Wire 依赖注入
wire ./cmd/

# Docker 构建（从 deploy/ 目录执行）
cd deploy && make build
```

## 编码风格与命名规范

- **语言**: Go，使用 `gofmt`（或 `goimports`）格式化。
- **缩进**: Tab（Go 标准）。
- **命名**:
  - 文件：`snake_case.go`。
  - 类型、函数、导出字段：PascalCase。
  - 非导出字段、局部变量：camelCase。
  - 常量：根据导出范围使用 PascalCase 或 camelCase。
- **错误处理**: 错误显式返回；`log.Fatal` 仅用于初始化失败。
- **导入分组**: 按标准库、第三方库、内部库（`github.com/alert666/api-server/...`）分组，各组间空行分隔。
- **Linting**: 没有项目级 linter 配置；遵循 Go 自身的 `go vet` 和标准惯例。

## 测试指南

- **框架**: Go 标准 `testing` 包。
- **位置**: 测试位于 `test/` 目录，按领域子目录组织（如 `test/alert/`、`test/store/`、`test/casbin/`）。
- **命名**: 测试函数使用 `TestXxx` 模式，`Xxx` 描述被测单元。
- **覆盖率**: 无正式的覆盖率阈值；贡献者应对新逻辑编写领域级测试。
- **运行测试**: 使用 `go test ./test/<domain>/` 或 `go test ./...` 运行全量测试。

## 提交与 PR 规范

本项目使用带中文描述的 **Conventional Commits**。示例：

```
feat: 添加 alertname 下拉选项接口
fix: 修复静默恢复告警逻辑
refactor: 完成告警模块重构
update: 更新 Dockerfile
```

**PR 要求**：
- 目标分支：`main`（大型功能分支可使用 `refactor/alert`）
- 标题和描述应清晰说明变更内容及解决的问题
- 关联相关的 GitHub Issue
- UI 或 API 变更需附带截图
- CI（GitHub Actions）必须通过后方可合并

## CI/CD 流水线

每次推送到 `main` 或 `refactor/alert` 分支时，GitHub Actions 自动构建并推送 Docker 镜像。镜像同时发布到阿里云容器镜像服务（`registry.cn-beijing.aliyuncs.com/qqlx/alertmanager`）和 GitHub Container Registry（`ghcr.io/alert666/api-server`）。标签由分支名、短 commit SHA 和时间戳生成。

## 可观测性

服务暴露 OpenTelemetry 追踪和 Prometheus 格式指标。通过以下环境变量配置：

- `OTEL_EXPORTER_OTLP_ENDPOINT` — OTLP 收集器地址
- `OTEL_EXPORTER_OTLP_PROTOCOL` — 传输协议（grpc/http）
- `OTEL_SERVICE_NAME` — 服务标识
- `OTEL_METRICS_EXPORTER` — 设为 `prometheus` 启用指标
- `OTEL_EXPORTER_PROMETHEUS_PORT` — 指标 HTTP 端口

## 面向 AI Agent 的特别说明

当通过 AI Agent 为本仓库贡献代码时：

- 阅读现有代码库，理解分层架构（controller → service/v1 → store → model）。
- 遵循 Wire 依赖注入：在 `cmd/wire.go` 中添加新 Provider，并用 `wire ./cmd/` 重新生成。
- 将新的 Go 源文件放在对应的分层目录中，不要平铺。
- 新包放在 `pkg/` 中，除非它是特定层级的。
- 使用 GORM 进行数据库操作，使用 go-redis 进行缓存；无正当理由避免使用原生 SQL。
- 对于 API 变更，确保 Swagger 注解随 controller 代码一同更新。
