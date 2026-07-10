# 用户系统落地蓝图

## 1. 文档目标

本文档用于指导 `go-backend-template` 从当前的演示级用户注册样例，演进为一个可复用、可扩展、可作为项目起点的用户系统模板。

目标不是一次性实现完整身份平台，而是在保持现有分层习惯的前提下，分阶段交付：

- 可用的用户注册与邮箱验证闭环
- 可用的登录、刷新、登出能力
- 可用的后台用户管理与 RBAC
- 可沉淀为模板的基础设施约束

## 2. 当前仓库现状

当前代码已经具备一套可继续演进的基础骨架：

- HTTP 入口在 `cmd/server`
- 事件消费入口在 `cmd/worker`
- 已确认将新增 `cmd/cron` 作为单实例定时任务入口
- Handler / Usecase / Domain / Repository / Infra 已分层
- 注册、邮箱验证、登录、刷新、登出、找回密码、后台用户管理与 RBAC 已有基础实现

关键证据：

- 服务装配：`cmd/server/wire.go`
- Worker 装配：`cmd/worker/wire.go`
- 用户注册编排：`internal/usecase/identity/service.go`
- 认证编排：`internal/usecase/auth/service.go`
- 权限编排：`internal/usecase/authorization/service.go`
- 启动期管理员初始化：`internal/usecase/bootstrap/service.go`
- 共享授权默认模型安装：`internal/usecase/support/authorization_defaults.go`
- Kafka 事件适配层：`internal/infra/mq`

当前已确认问题：

- 事件发布与数据库事务未实现强一致，`outbox` 尚未落地
- 事件消费失败仍然会 `ack`
- Access Token 撤销策略尚未完整实现，请求链路未校验 session 有效性
- 登录失败锁定、权限缓存、登录限流仍未落地
- 文档中历史 `service/user`、`domain/user` 路径描述已过时

## 3. 本次蓝图的固定约束

本蓝图按以下已确认条件设计：

- Access Token 使用 JWT
- Refresh Token 使用随机不透明串，存储于 Redis
- 首发不要求多端会话管理
- 首发要求管理后台与 RBAC 一起上线
- 邮箱未验证前不允许登录

## 4. 总体设计原则

### 4.1 分层原则

- `handler` 仅负责协议转换、参数校验、响应映射
- `usecase` 仅负责用例编排与事务边界
- `domain` 负责实体、值对象、领域规则、领域事件、仓储契约
- `repository` 负责持久化实现，不承载业务规则
- `infra` 负责数据库、缓存、消息、日志、邮件、加密、追踪等基础设施适配
- `cmd/*` 负责进程入口与依赖装配，不承载业务规则

### 4.2 安全原则

- Access Token 短有效期
- Refresh Token 必须轮换
- Token 必须有明确过期与撤销策略
- 邮箱未验证用户禁止登录
- 安全关键操作必须写审计日志

### 4.3 可扩展原则

- 不再把所有能力继续堆进 `user` 模块
- 用户本体、认证、验证、授权四类能力分包演进
- 异步副作用默认经事件流转，不在请求链路直接串外部副作用

### 4.4 `cmd/cron` 职责边界

- `cmd/cron` 是单实例部署的定时任务进程，用于执行“非请求驱动、非消息驱动”的后台任务
- `cmd/cron` 负责装配依赖、注册任务、配置调度周期与启动调度器，不承载具体业务规则
- 具体任务逻辑应放在对应模块的 `usecase/<module>/cron.go` 中，按业务归属组织，而不是按触发方式集中堆放
- `cmd/cron` 与 `cmd/worker` 的边界应明确：
  - `cmd/worker` 负责消费已投递的异步事件
  - `cmd/cron` 负责扫描、补偿、清理、归档、发布等定时任务
- 调度器实现可以依赖外部库，并在 `pkg/scheduler` 下做轻量抽象；`usecase` 层不直接依赖具体调度库
- 当前阶段按单实例假设设计，但任务本身仍必须满足幂等性要求

## 5. 目标目录结构

建议按以下方式重构用户系统相关目录：

```text
cmd/
  server/
  worker/
  cron/

internal/
  domain/
    identity/
    auth/
    verification/
    authorization/
    shared/
  usecase/
    identity/
    auth/
    verification/
    authorization/
    bootstrap/
    support/
  handler/
    auth/
    me/
    admin_user/
    admin_role/
    captcha/
  repository/
    identity/
    auth/
    verification/
    authorization/
  model/
    identity/
    auth/
    verification/
    authorization/
  infra/
    cache/
    crypto/
    database/
    logging/
    mq/
    token/
    tracing/

pkg/
  captcha/
  email/
  logger/
  scheduler/
```

## 6. 子系统边界

### 6.1 identity

职责：

- 用户本体
- 账号状态机
- 基本资料
- 邮箱验证状态

核心对象：

- `User`
- `UserStatus`

建议状态：

- `pending_verification`
- `active`
- `locked`
- `banned`
- `deleted`

### 6.2 auth

职责：

- 登录
- Access Token 签发
- Refresh Token 轮换
- 登出
- 登录失败锁定

核心对象：

- `Session`
- `AccessClaims`
- `RefreshSession`

### 6.3 verification

职责：

- 图形验证码
- 邮箱验证 token
- 密码重置 token

核心对象：

- `EmailVerificationToken`
- `PasswordResetToken`

### 6.4 authorization

职责：

- 角色
- 权限
- 用户角色绑定
- 接口级授权

核心对象：

- `Role`
- `Permission`

## 7. 核心业务规则

### 7.1 注册

流程：

1. 校验图形验证码
2. 校验邮箱唯一性
3. 创建用户，状态设为 `pending_verification`
4. 创建密码凭证
5. 生成邮箱验证 token
6. 事务内写业务数据与 outbox 事件
7. 异步发送验证邮件

### 7.2 邮箱验证

流程：

1. 校验 token 是否存在、未过期、未消费
2. 激活用户
3. 标记 token 已消费
4. 写审计日志

### 7.3 登录

规则：

- `pending_verification` 不允许登录
- `locked` 不允许登录
- `banned` 不允许登录
- 只有 `active` 可登录

流程：

1. 根据邮箱查用户
2. 校验状态
3. 校验密码
4. 失效该用户现存 refresh 会话
5. 新建单会话 refresh session
6. 签发 access token + refresh token

### 7.4 刷新

流程：

1. 校验 refresh token
2. 对比 Redis 中存储的 token hash
3. 校验是否过期
4. 执行 rotation
5. 签发新的 access token 与 refresh token

### 7.5 登出

流程：

1. 根据用户当前 session 删除 Redis 中的 refresh session
2. 写审计日志

## 8. 数据模型设计

### 8.1 users

用途：用户本体。

建议字段：

- `id`
- `name`
- `email`
- `status`
- `email_verified`
- `last_login_at`
- `created_at`
- `updated_at`
- `deleted_at`

说明：

- `email_verified` 与 `status` 同时保留，便于快速判断与统计
- 注册后初始状态为 `pending_verification`

### 8.2 user_credentials

用途：认证凭证，与用户本体拆分。

建议字段：

- `user_id`
- `password_hash`
- `password_changed_at`
- `created_at`
- `updated_at`

### 8.3 email_verification_tokens

用途：邮箱验证凭证。

建议字段：

- `id`
- `user_id`
- `token_hash`
- `expires_at`
- `consumed_at`
- `created_at`

规则：

- 只保存 hash，不保存明文
- 一次性使用
- 默认有效期建议 `30m`

### 8.4 password_reset_tokens

用途：找回密码凭证。

建议字段：

- `id`
- `user_id`
- `token_hash`
- `expires_at`
- `consumed_at`
- `created_at`

规则：

- 一次性使用
- 默认有效期建议 `30m`
- 使用成功后强制失效当前 refresh session

### 8.5 roles

用途：角色定义。

建议字段：

- `id`
- `code`
- `name`
- `description`
- `created_at`
- `updated_at`

### 8.6 permissions

用途：权限定义。

建议字段：

- `id`
- `code`
- `name`
- `description`
- `created_at`
- `updated_at`

权限命名建议采用 `<resource>.<action>`：

- `user.read`
- `user.write`
- `user.ban`
- `role.read`
- `role.write`
- `audit.read`

### 8.7 user_roles

用途：用户与角色绑定。

建议字段：

- `user_id`
- `role_id`
- `created_at`

### 8.8 role_permissions

用途：角色与权限绑定。

建议字段：

- `role_id`
- `permission_id`
- `created_at`

### 8.9 audit_logs

用途：记录安全与管理关键行为。

建议字段：

- `id`
- `actor_user_id`
- `target_type`
- `target_id`
- `action`
- `result`
- `ip`
- `user_agent`
- `trace_id`
- `metadata`
- `created_at`

建议记录的动作：

- 注册
- 邮箱验证
- 登录成功
- 登录失败
- 刷新 token
- 登出
- 重置密码
- 修改用户状态
- 角色变更

### 8.10 outbox_events

用途：保证数据库写入与消息投递的最终一致性。

建议字段：

- `id`
- `event_name`
- `aggregate_type`
- `aggregate_id`
- `payload`
- `status`
- `published_at`
- `created_at`

## 9. Redis Key 设计

### 9.1 验证码

- `captcha:<captcha_id>`

### 9.2 Refresh Session

- `auth:refresh:<session_id>`
- `auth:user_session:<user_id>`

建议值内容：

- `user_id`
- `refresh_token_hash`
- `expires_at`

说明：

- `auth:user_session:<user_id>` 保存当前生效的 `session_id`
- 因为首发不做多端管理，用户重新登录时直接覆盖旧 session

### 9.3 登录失败计数

- `auth:login_fail:<email>`

建议用途：

- 累加失败次数
- 达阈值后触发临时锁定策略

### 9.4 权限缓存

- `authz:user_permissions:<user_id>`

建议用途：

- 缓存用户当前权限集合
- 角色变更后主动清理

## 10. Token 设计

### 10.1 Access Token

类型：JWT

建议：

- 签名算法：`HS256` 或 `RS256`
- 首版优先 `HS256`，简单且落地快
- 有效期：`15m`

建议 Claims：

- `sub`：用户 ID
- `sid`：会话 ID
- `typ`：固定为 `access`
- `iss`：签发方
- `aud`：受众
- `iat`
- `exp`

不建议直接把完整权限列表放入 JWT。

原因：

- RBAC 权限会变更
- 权限变更后 JWT 难以及时失效
- 更适合只在 JWT 中保存身份与会话信息，再由鉴权层查询权限

### 10.2 Refresh Token

类型：随机不透明串

规则：

- 服务端只存 hash
- 每次刷新都进行 rotation
- 默认有效期建议 `7d`
- 被覆盖、登出、重置密码后立即失效

## 11. 接口蓝图

### 11.1 用户侧接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/captcha` | 获取图形验证码 |
| POST | `/api/v1/auth/register` | 注册 |
| POST | `/api/v1/auth/resend-verification` | 重发验证邮件 |
| POST | `/api/v1/auth/verify-email` | 验证邮箱 |
| POST | `/api/v1/auth/login` | 登录 |
| POST | `/api/v1/auth/refresh` | 刷新 access token |
| POST | `/api/v1/auth/logout` | 登出 |
| POST | `/api/v1/auth/forgot-password` | 发起找回密码 |
| POST | `/api/v1/auth/reset-password` | 重置密码 |
| GET | `/api/v1/me` | 获取当前用户信息 |
| PATCH | `/api/v1/me/profile` | 修改资料 |
| PATCH | `/api/v1/me/password` | 修改密码 |

### 11.2 管理后台接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/v1/admin/auth/login` | 管理端登录 |
| GET | `/api/v1/admin/users` | 用户列表 |
| POST | `/api/v1/admin/users` | 创建用户 |
| PATCH | `/api/v1/admin/users/:id/status` | 修改用户状态 |
| GET | `/api/v1/admin/roles` | 角色列表 |
| POST | `/api/v1/admin/roles` | 创建角色 |
| PATCH | `/api/v1/admin/roles/:id` | 修改角色 |
| GET | `/api/v1/admin/permissions` | 权限列表 |

## 12. DTO 草案

### 12.1 Register Request

```json
{
  "name": "testuser",
  "email": "test@example.com",
  "password": "12345678",
  "captcha_id": "captcha-id",
  "captcha_code": "123456"
}
```

### 12.2 Login Request

```json
{
  "email": "test@example.com",
  "password": "12345678"
}
```

### 12.3 Login Response

```json
{
  "user": {
    "id": 1,
    "name": "testuser",
    "email": "test@example.com",
    "status": "active"
  },
  "access_token": "jwt",
  "refresh_token": "opaque-token",
  "expires_in": 900
}
```

### 12.4 Verify Email Request

```json
{
  "token": "verification-token"
}
```

### 12.5 Refresh Request

```json
{
  "refresh_token": "opaque-token"
}
```

## 13. RBAC 设计

### 13.1 首发范围

首发只做接口级授权，不在第一阶段同步实现：

- 菜单系统
- 按钮级前端权限点编排
- 数据权限域隔离

### 13.2 授权流程

1. 中间件解析 JWT
2. 提取用户 ID 与 session ID
3. 查询权限集合
4. 判断接口所需权限
5. 拒绝未授权访问
6. 写审计日志

### 13.3 建议中间件层

- `AuthMiddleware`：校验 JWT，提取用户上下文
- `PermissionMiddleware`：检查权限码
- `AdminOnlyMiddleware`：管理后台统一入口约束

## 14. 异步事件与 Outbox

### 14.1 首发事件

- `user.registered`
- `user.email_verification_requested`
- `user.password_reset_requested`
- `user.password_changed`

### 14.2 推荐链路

```text
HTTP Handler
-> Usecase
-> DB Transaction
-> 写业务表
-> 写 outbox_events
-> Publisher Job（cmd/cron，单实例）
-> Kafka
-> Worker Consumer
-> 邮件发送 / 审计 / 其他副作用
```

### 14.3 为什么要引入 Outbox

当前实现是“业务写库 + 定时发布到 Kafka”。

问题在于：

- 数据库事务与 Redis 写入不在同一事务边界
- 可能出现库写成功但消息丢失
- 也可能出现消息发出但事务失败

因此模板升级时应优先引入 outbox。

## 15. 中间件与通用基础设施建议

建议新增或沉淀以下通用组件：

- JWT Token Provider
- 当前用户上下文提取器
- 统一错误码定义
- 接口级权限映射器
- 审计日志记录器
- Outbox Publisher
- 统一分页响应
- 登录限流器

### 15.1 `cmd/cron` 首批适用任务

建议优先将以下能力设计为 `cmd/cron` 驱动：

- Outbox 发布与补偿任务
- 过期 token / 临时数据清理任务
- 审计数据归档与保留策略任务

这些任务都应遵循以下约束：

- 任务逻辑按业务归属放在对应 `usecase/<module>/cron.go`
- 调度注册统一放在 `cmd/cron/wire.go`
- 任务默认要求幂等，可安全重复执行

## 16. 分阶段实施计划

### Phase 1：重构基础边界

目标：

- 将 `user` 拆分为 `identity/auth/verification/authorization`
- 修复当前注册 ID 与事件载荷问题

主要改动点：

- `internal/domain/identity`
- `internal/usecase/identity`
- `internal/repository/identity`
- `internal/model/identity`
- `cmd/server/wire.go`
- `cmd/worker/wire.go`

### Phase 2：注册与邮箱验证闭环

目标：

- 注册后进入 `pending_verification`
- 完成邮箱验证后激活
- 未验证邮箱禁止登录

主要交付：

- `email_verification_tokens`
- `register`
- `resend-verification`
- `verify-email`

### Phase 3：认证与单会话

目标：

- JWT access token
- Redis refresh token
- refresh rotation
- 单会话覆盖登录

主要交付：

- `login`
- `refresh`
- `logout`

### Phase 4：后台用户管理与 RBAC

目标：

- 管理端登录
- 用户管理
- 角色权限模型
- 接口级授权

主要交付：

- `roles`
- `permissions`
- `user_roles`
- `role_permissions`
- admin middleware

### Phase 5：安全完善与模板化

目标：

- 找回密码
- 登录失败锁定
- 审计日志
- outbox
- 文档与测试基座

## 17. 第一批任务清单

建议按以下顺序开工：

1. 拆分 `user` 子系统边界
2. 修复注册成功后的 ID 回填与领域事件内容
3. 增加用户状态机与未验证邮箱不可登录规则
4. 增加邮箱验证 token 表与验证接口
5. 引入 JWT access token 与 token provider
6. 引入 Redis refresh token 与单会话 rotation
7. 增加 RBAC 表结构与管理端鉴权
8. 引入审计日志
9. 引入 outbox

## 18. 对当前仓库的落地建议

### 18.1 可以保留的部分

- `cmd/server` / `cmd/worker` 双入口
- 新增 `cmd/cron` 作为单实例定时任务入口的思路
- `handler -> usecase -> domain -> repository -> infra` 的总体分层
- `pkg/captcha`
- `pkg/email`
- `pkg/logger`
- `pkg/scheduler` 这一类对外部调度器的轻量抽象

### 18.2 应优先重构的部分

- `outbox_events` 模型、仓储、发布任务尚未落地
- 现有消息发布已调整为“业务写库 + outbox + 定时发布到 Kafka”
- 现有事件消费仍为“失败也 ack”
- Access Token 撤销校验尚未纳入请求链路
- 登录失败锁定、权限缓存、登录限流尚未落地
- 文档与代码当前目录命名已从 `service` 演进为 `usecase`

### 18.3 不建议的演进方式

- 不建议重新引入宽泛的 `user` 大模块承载登录、权限、后台管理
- 不建议让 `usecase` 之间直接相互依赖
- 不建议把完整权限集直接编码进 JWT
- 不建议继续保持“失败也 ack”的事件消费策略
- 不建议把定时任务调度细节放入 `usecase` 层

## 19. 待确认项

以下项不影响第一阶段蓝图，但在正式开发前建议确认：

- JWT 使用 `HS256` 还是 `RS256`
- 是否需要迁移工具而非继续依赖 `AutoMigrate`
- 管理员种子账号与种子角色如何初始化
- 登录失败锁定阈值与解锁策略
- 审计日志保留周期
