 # Repository Guidelines
 
 ## Project Structure & Module Organization
 
 This is a Go web API server built with Gin, GORM, and Wire. The source is organized into layered packages:
 
 | Directory     | Purpose                                                                        |
 | ------------- | ------------------------------------------------------------------------------ |
 | `cmd/`        | Application entry point and Wire dependency injection                          |
 | `base/`       | Core framework — server setup, middleware, types, logging, config              |
 | `controller/` | HTTP request handlers                                                          |
 | `service/v1/` | Business logic layer                                                           |
 | `store/`      | Data access (GORM queries and cache logic)                                     |
 | `model/`      | GORM data models                                                               |
 | `grpc/`       | gRPC server and handlers                                                       |
 | `pkg/`        | Reusable packages (Casbin, Feishu, JWT, OAuth2, alert inhibition, local cache) |
 | `test/`       | Tests organized by domain subdirectory                                         |
 | `deploy/`     | Dockerfile, docker-compose.yaml, nginx.conf, schema.sql                        |
 | `scripts/`    | Utility scripts (e.g., gRPC certificate generation)                            |
 | `docs/`       | Documentation and screenshots                                                  |
 | `gormgen/`    | GORM model code generation                                                     |
 
 ## Build, Test, and Development Commands
 
 ```bash
 # Run all tests
 go test ./...
 
 # Run tests in a specific domain directory
 go test ./test/alert/
 
 # Build the binary
 go build ./cmd/apiserver
 
 # Run the server locally (requires config.yaml and a running MySQL/Redis)
 go run ./cmd/apiserver -c config.yaml
 
 # Update Wire dependency injection
 wire ./cmd/
 
 # Docker build (from deploy/ directory)
 cd deploy && make build
 ```
 
 ## Coding Style & Naming Conventions
 
 - **Language**: Go, using `gofmt` (or `goimports`) for formatting.
 - **Indentation**: Tabs (Go standard).
 - **Naming**:
   - Files: `snake_case.go`.
   - Types, functions, exported fields: PascalCase.
   - Unexported fields, local variables: camelCase.
   - Constants: PascalCase or camelCase depending on export scope.
 - **Error handling**: Errors are returned explicitly; `log.Fatal` is reserved for initialization failures only.
 - **Imports**: Grouped into stdlib, third-party, and internal (`github.com/alert666/api-server/...`) blocks separated by blank lines.
 - **Linting**: No project-wide linter config is checked in; follow Go's own `go vet` and standard conventions.
 
 ## Testing Guidelines
 
 - **Framework**: Standard Go `testing` package.
 - **Location**: Tests live in the `test/` directory, organized in subdirectories by domain (e.g., `test/alert/`, `test/store/`, `test/casbin/`).
 - **Names**: Test functions use the pattern `TestXxx`, where `Xxx` describes the unit under test.
 - **Coverage**: No formal coverage threshold; contributors are expected to cover new logic with domain-level tests.
 - **Running tests**: Use `go test ./test/<domain>/` or `go test ./...` for the full suite.
 
 ## Commit & Pull Request Guidelines
 
 This project uses **Conventional Commits** with Chinese descriptions. Examples from the repository:
 
 ```
 feat: add alertname Options interface
 fix: resolve silence recovery alert logic
 refactor: complete alert refactoring
 update: update Dockerfile
 ```
 
 **PR requirements**:
 - Base branch: `main` (or `refactor/alert` for large feature branches).
 - Title and description should clearly state the change and the problem it solves.
 - Link related GitHub issues where applicable.
 - Include screenshots for UI or API changes.
 - CI (GitHub Actions) must pass before merging.
 
 ## CI/CD Pipeline
 
 GitHub Actions builds and pushes Docker images on every push to `main` or `refactor/alert`. Images are published to both Alibaba Cloud Container Registry (`registry.cn-beijing.aliyuncs.com/qqlx/alertmanager`) and GitHub Container Registry (`ghcr.io/alert666/api-server`). Tags are generated from the branch name, short commit SHA, and timestamp.
 
 ## Observability
 
 The server exposes OpenTelemetry traces and Prometheus-formatted metrics. Configure these via environment variables:
 
 - `OTEL_EXPORTER_OTLP_ENDPOINT` — OTLP collector address
 - `OTEL_EXPORTER_OTLP_PROTOCOL` — transport protocol (grpc/http)
 - `OTEL_SERVICE_NAME` — service identifier
 - `OTEL_METRICS_EXPORTER` — set to `prometheus` to enable metrics
 - `OTEL_EXPORTER_PROMETHEUS_PORT` — metrics HTTP port
 
 ## Agent-Specific Instructions
 
 When contributing to this repository via an AI agent:
 
 - Read the existing codebase to understand layered architecture (controller → service/v1 → store → model).
 - Respect Wire's dependency injection: add new providers to `cmd/wire.go` and regenerate with `wire ./cmd/`.
 - Place new Go source files in the appropriate layer directory, not a flat layout.
 - Keep new packages inside `pkg/` unless they are layer-specific.
 - Use GORM for database operations and go-redis for caching; avoid raw SQL without justification.
 - For API changes, ensure Swagger annotations are updated alongside the controller code.
