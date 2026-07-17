# 测试指南

项目将测试分为无需外部服务的单元测试，以及依赖真实基础设施的集成测试。集成测试使用 `integration` build tag，不会在普通 `go test ./...` 中编译或执行。

本地运行服务时，使用示例配置的显式路径，例如 `go run ./cmd/server -config configs/app.example.yaml`。生产环境应通过 `-config` 提供受限的运行配置文件；加载器不再从 `internal/` 源码目录搜索配置。配置加载器会自动读取工作目录下可选的 `.env`。`.env` 不提交；从 `.env.example` 创建个人配置。系统环境变量优先于 `.env` 和 YAML 配置。

## 单元测试

单元测试使用 stub、fake 或内存实现验证领域规则、Usecase 编排、Handler 响应映射和基础设施适配逻辑。它们不依赖 PostgreSQL、Redis、Kafka 或 S3。

```bash
make test-unit
```

该命令以 verbose 模式执行 `go test ./...`，适合本地快速反馈。CI 也会执行相同范围的 `go test ./...`。

## 完整测试

```bash
make test
```

`make test` 会执行 `test-unit` 与全部集成测试。运行前需启动 PostgreSQL、Redis、Kafka 和 MinIO，并设置下文列出的测试环境变量。

本地可一次启动全部依赖：

```bash
docker compose -f deploy/docker-compose.yml up -d postgres redis kafka minio
```

## PostgreSQL 集成测试

PostgreSQL 集成测试要求显式设置 `TEST_DATABASE_DSN`；未设置会失败，避免测试意外连接开发数据库。

```powershell
$env:TEST_DATABASE_DSN = "host=localhost user=postgres password=postgres dbname=go_backend port=5432 sslmode=disable TimeZone=Asia/Shanghai"
make test-db-integration
```

该命令覆盖数据库迁移和 `internal/repository/...` 下的真实 SQL、事务、约束与并发行为。

集成测试必须：

- 使用 `internal/testsupport.OpenPostgres` 建立连接；不得内置默认 DSN。
- 使用测试专用数据库，绝不使用生产数据库。`testsupport.OpenPostgres` 会为每个测试创建并在结束后删除独立 PostgreSQL schema。
- 使用唯一测试数据并在 `t.Cleanup` 中清理。

## Bootstrap 集成测试

```powershell
make test-bootstrap-integration
```

该命令独立验证启动阶段的默认授权数据初始化，包括默认权限、超级管理员角色及其关联的幂等性。它需要与 PostgreSQL 集成测试相同的 `TEST_DATABASE_DSN`，但不包含在 `test-db-integration` 中。

## Redis 集成测试

Redis 集成测试覆盖刷新会话、验证码存储和 `pkg/ratelimit` 的固定窗口限流行为。执行前设置测试专用 Redis DB：

```powershell
$env:TEST_REDIS_ADDR = "localhost:6379"
$env:TEST_REDIS_PASSWORD = ""
$env:TEST_REDIS_DB = "15"
make test-redis-integration
```

测试只能创建和清理带唯一前缀的 key，禁止执行 `FLUSHDB`。建议保留 Redis DB 15 仅供本地集成测试使用。

## Kafka 集成测试

Kafka 集成测试要求显式配置 broker：

```powershell
$env:TEST_KAFKA_BROKERS = "localhost:9092"
make test-kafka-integration
```

每个测试创建唯一 topic 并在结束时请求删除。当前覆盖项目 Publisher 到真实 Kafka 的 key、payload、事件 header 和 trace header 映射。

## S3 与媒体集成测试

S3 存储适配器测试使用 `test-s3-integration`；CMS 媒体用例测试使用 `test-media-integration`，后者还依赖 `TEST_DATABASE_DSN`。

```powershell
$env:TEST_S3_ENDPOINT = "http://localhost:9000"
$env:TEST_S3_REGION = "us-east-1"
$env:TEST_S3_ACCESS_KEY_ID = "minioadmin"
$env:TEST_S3_SECRET_ACCESS_KEY = "minioadmin"
$env:TEST_S3_BUCKET = "go-backend-template-test"
make test-s3-integration
make test-media-integration
```

测试会按需创建测试 bucket 和对象；应使用专用 bucket，避免与开发或生产对象混用。

## 全部集成测试

```bash
make test-integration
```

该聚合命令包含 PostgreSQL、Redis、Kafka、S3、CMS 媒体和 Bootstrap 六类集成测试。需要所有对应服务和环境变量均已就绪。

`make test-consumption-integration` 用于单独、verbose 地排查消费仓储集成测试；其测试范围已被 `test-db-integration` 覆盖，因此不重复纳入聚合命令。

## CI

GitHub Actions 会在 PR 与 `main` 分支提交时依次执行：

- `go test ./...`
- `go vet ./...`
- 构建 `server`、`worker`、`cron`、`migrate` 与 `mcp` 五个可执行文件
- `make docker-build`，验证 server、worker 与 cron 三个运行镜像可构建
- 启动 PostgreSQL、Redis、Kafka 和 MinIO，随后执行 `make test-integration`

CI 通过 Compose healthcheck 等待依赖就绪，并注入 `TEST_DATABASE_DSN`、Redis、Kafka 和 S3 所需的全部测试环境变量。镜像只在 CI 中构建验证，不推送到镜像仓库；Jaeger 不参与 CI。
