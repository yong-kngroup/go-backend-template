# 测试指南

项目将测试分为无需外部服务的单元测试和依赖真实基础设施的集成测试。

本地运行服务时，配置加载器会自动读取工作目录下可选的 `.env`。`.env` 不提交；从 `.env.example` 创建个人配置。系统环境变量优先于 `.env` 和 YAML 配置。

## 单元测试

单元测试使用 stub、fake 或内存实现验证领域规则、Usecase 编排、Handler 响应映射和基础设施适配逻辑。它们不依赖 PostgreSQL、Redis 或 Kafka。

```bash
make test-unit
```

`make test` 是 `make test-unit` 的别名，适合本地快速反馈和每个 PR 的默认质量门槛。

## PostgreSQL 集成测试

集成测试使用 `integration` build tag，不会在普通 `go test ./...` 中编译或执行。执行前必须显式设置 `TEST_DATABASE_DSN`；未设置会失败，避免测试意外连接开发数据库。

先启动本地依赖：

```bash
docker compose -f deploy/docker-compose.yml up -d postgres
```

PowerShell：

```powershell
$env:TEST_DATABASE_DSN = "host=localhost user=postgres password=postgres dbname=go_backend port=5432 sslmode=disable TimeZone=Asia/Shanghai"
make test-db-integration
```

集成测试必须：

- 使用 `internal/testsupport.OpenPostgres` 建立连接；不得内置默认 DSN。
- 使用测试专用数据库，绝不使用生产数据库。`testsupport.OpenPostgres` 会为每个测试创建并在结束后删除独立 PostgreSQL schema。
- 使用唯一测试数据并在 `t.Cleanup` 中清理。
- 覆盖数据库约束、事务、并发和真实 SQL 查询语义。

## Redis 集成测试

Redis 集成测试覆盖刷新会话、验证码存储和 `pkg/ratelimit` 的固定窗口限流行为。

Redis 集成测试使用同一个 `integration` build tag，要求显式配置 Redis 地址和测试专用 DB：

```powershell
$env:TEST_REDIS_ADDR = "localhost:6379"
$env:TEST_REDIS_PASSWORD = ""
$env:TEST_REDIS_DB = "15"
make test-redis-integration
```

测试只能创建和清理带唯一前缀的 key，禁止执行 `FLUSHDB`。建议保留 Redis DB 15 仅供本地集成测试使用。

在本地同时运行 PostgreSQL 与 Redis 集成测试：

```powershell
make test-integration
```

## Kafka 集成测试

Kafka 集成测试要求显式配置 broker：

```powershell
$env:TEST_KAFKA_BROKERS = "localhost:9092"
make test-kafka-integration
```

每个测试创建唯一 topic 并在结束时请求删除。当前覆盖项目 Publisher 到真实 Kafka 的 key、payload、事件 header 和 trace header 映射。

## CI

GitHub Actions 在 PR 与 `main` 提交时依次执行单元测试、静态检查、四个进程镜像构建和真实基础设施集成测试。CI 仅启动 PostgreSQL、Redis、Kafka，并通过 Compose healthcheck 等待依赖就绪；Jaeger 不参与 CI。

镜像只在 CI 中构建验证，不推送到镜像仓库。模板 tag 的镜像发布流程将在后续单独添加。

CI 应启动独立 PostgreSQL、Redis 和 Kafka 服务，并注入 `TEST_DATABASE_DSN`、`TEST_REDIS_ADDR`、`TEST_REDIS_DB`、`TEST_KAFKA_BROKERS`，随后执行：

```bash
make test-ci
go vet ./...
go build ./cmd/server ./cmd/worker ./cmd/cron
```

Redis 和 Kafka 集成测试采用相同约定：使用独立 build tag、显式环境变量和 CI 服务容器，不能隐式依赖开发者本机服务。
