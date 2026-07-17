# 数据库迁移

数据库结构变更通过 `db/migrations` 中的版本化 SQL 文件管理。应用进程启动时不得创建或修改表结构。

启动 PostgreSQL 并在 `.env` 或进程环境变量中设置 `DATABASE_DSN` 后，从仓库根目录执行迁移：

```powershell
go run ./cmd/migrate -direction up
go run ./cmd/migrate -version
```

在安装 GNU Make 的环境中，可使用等价的 `make migrate-up` 和 `make migrate-version`。

向下迁移具有破坏性，仅适用于隔离的本地数据库，且必须显式确认：

```powershell
go run ./cmd/migrate -direction down -steps 1 -allow-destructive
```

迁移失败时，必须先排查并修复数据库结构，再清除 dirty 状态。`-force-version` 只会修改迁移元数据，因此同样受破坏性确认参数保护；不得用它绕过未经核验的生产迁移：

```powershell
go run ./cmd/migrate -force-version 1 -allow-destructive
```

部署时，应在启动或更新 `server`、`worker`、`cron` 前，以独立发布任务执行一次 `up`。不得由多个应用副本并发执行迁移。

## 新增迁移

使用下一个连续版本号创建一对迁移文件：

```text
db/migrations/000002_add_example.up.sql
db/migrations/000002_add_example.down.sql
```

表、约束、索引和外键策略必须使用显式 SQL 定义。涉及已有生产数据的迁移必须采用兼容的“扩展、回填、切换、收缩”步骤；应用发布失败时通常只回滚应用代码，不执行破坏性的数据库回滚。

PostgreSQL 迁移集成测试会在隔离 schema 中执行完整的向上迁移，再执行对应的向下迁移：

```powershell
go test -tags=integration ./internal/infra/postgres
```
