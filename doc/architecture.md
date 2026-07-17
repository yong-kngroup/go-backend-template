# 架构导航

本文档帮助维护者从运行入口追踪一次请求或消息的完整路径。它补充
`engineering-constraints.md` 的维护规则，不替代领域规则、接口契约和测试。

## 运行进程

| 进程 | 入口 | 职责 |
| --- | --- | --- |
| HTTP 服务 | `cmd/server` | 装配 HTTP 路由，处理同步 API 请求。 |
| 消息消费者 | `cmd/worker` | 消费领域事件，执行异步副作用。 |
| 定时任务 | `cmd/cron` | 发布 Outbox、执行清理和死信巡检/回放。 |
| 数据库迁移 | `cmd/migrate` | 以独立发布任务执行版本化 SQL 迁移。 |

`cmd` 只负责依赖装配和进程生命周期；业务规则不应放入其中。

## 同步请求链路

```text
HTTP request
  -> middleware（认证、授权、限流、幂等）
  -> handler（协议转换、输入校验、响应映射）
  -> usecase（流程编排、事务边界）
  -> domain（实体、规则、端口）
  -> domain 端口
  -> repository（业务数据）/ platform（应用协作能力）/ infra（技术适配）
```

关键入口：

- 路由注册：`internal/handler/registry.go` 与各 handler 的 `RegisterRoutes`。
- HTTP 中间件：`internal/handler/middleware`。
- 用例：`internal/usecase/<领域>`。
- 领域契约：`internal/domain/<领域>`。
- 具体持久化：`internal/repository/<领域>`。
- 平台能力：`internal/platform/<能力>`，例如 Outbox、幂等与消息消费状态。
- Redis 客户端与追踪：`internal/infra/redis`；认证刷新令牌会话：`internal/repository/auth`。

Handler 不直接访问 GORM、Redis 或 Kafka。Usecase 只依赖 domain 定义的端口，具体技术实现由 `cmd` 注入。

## 事务与 Outbox

跨表一致性写入由 Usecase 调用 `shared.TxManager.Do` 包裹。事务回调获得的
`context.Context` 携带当前数据库事务；Repository 必须通过
`repository.DB(ctx, db)` 取得连接，不能绕过该 context。

```text
usecase transaction
  -> 更新业务表
  -> 写入审计记录或领域事件
  -> EventBus 写入 outbox_events
commit
  -> cron 扫描未发布 Outbox
  -> 发布到 Kafka
  -> 成功后标记 published_at
```

Outbox 提供至少一次投递。发布成功但回写 `published_at` 失败时，允许再次发布；消费者必须以消息 key 实现幂等处理。

关键入口：

- 事务端口：`internal/domain/shared/tx.go`。
- PostgreSQL 连接与迁移：`internal/infra/postgres`；事务上下文实现：`internal/repository/tx.go`。
- 事件落库与扫描发布：`internal/platform/outbox`；发布任务仅由 `cmd/cron` 调用。

## 消息消费、重试与死信

```text
Kafka record
  -> 校验消息格式；无效消息写入 DLQ 后提交原 offset
  -> 以 consumer_group + message_key 领取消费记录
  -> 已完成/已死信：提交 offset
  -> 仍被其他实例锁定：返回错误，保留 offset
  -> 执行业务 handler
     -> 成功：标记 done，再提交 offset
     -> 可重试失败：写 retry topic，标记 failed，再提交 offset
     -> 不可重试或超过上限：写 DLQ，标记 dead，再提交 offset
```

“写入下一跳消息、更新消费状态、提交原 offset”的顺序不能随意调整。若下一跳写入失败，返回错误以保留原消息，等待重新投递。

关键入口：

- Kafka 消费适配器：`internal/infra/mq/consumer_adapter.go`。
- 消费状态与持久化：`internal/platform/messaging`。
- 死信巡检与回放流程：`internal/platform/messaging`；Kafka 死信读写：`internal/infra/mq/dead_letter_adapter.go`。

## 幂等、认证与授权

写接口可选用 `Idempotency-Key`。同一用户、HTTP 方法、路由和 key 组成幂等命名空间：

- 相同请求体且已完成：重放已保存的响应；
- 相同 key、不同请求体：拒绝；
- 相同请求仍在执行：拒绝并提示处理中；
- 未提供 key：不启用幂等保护。

认证负责识别访问令牌，授权负责判断角色和权限。权限、角色及用户角色关系由 authorization 领域维护；默认权限初始化由 server 启动阶段的 bootstrap 用例幂等执行，运行时授权用例不进行懒初始化。

关键入口：

- 幂等中间件与平台存储：`internal/handler/middleware/idempotency.go`、`internal/platform/idempotency`。
- 认证与授权中间件：`internal/handler/middleware/auth.go`、`internal/handler/middleware/admin.go`。
- 授权用例与领域：`internal/usecase/authorization`、`internal/domain/authorization`。

## CMS 领域

CMS 的用例负责 locale、分类树、标签、文章翻译、发布状态、公开查询、slug 和 URL 重定向的一致性。修改可公开访问的 slug 时，需要在同一事务中验证路径可用性、保存新值、必要时写入永久重定向，并记录审计事件。

关键入口：

- 用例编排：`internal/usecase/cms/service.go`。
- 领域端口与实体：`internal/domain/cms`。
- PostgreSQL 查询和映射：`internal/repository/cms/repository.go`。

## 注释维护规则

代码注释只记录无法从命名直接得出的信息：跨层契约、事务边界、并发语义、状态转换、副作用顺序、安全限制和兼容性原因。业务行为的可执行依据仍是测试；修改这些行为时，必须同时更新测试、注释和本文档中的链路说明。
