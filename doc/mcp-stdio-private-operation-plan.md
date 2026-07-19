# 私有 stdio MCP 内容运营方案

更新时间：2026-07-13

## 1. 目标与范围

为本项目新增 `cmd/mcp`，使用官方 `modelcontextprotocol/go-sdk` 实现一个仅供本机 Codex 使用的 stdio MCP Server。运营人员可通过自然语言完成站点状态查询、文章草稿维护、分类标签维护和文章发布。

第一期采用本机 stdio transport：Codex 启动 `cmd/mcp` 子进程，并以标准输入输出传输 MCP 消息。服务不监听 TCP 端口，不提供 SSE 或 Streamable HTTP，不接入 ChatGPT 远程 MCP，也不实现 OAuth 用户委托。

本方案的目标是以最小改动复用既有 CMS 的 HTTP API、JWT、RBAC、输入校验和审计链路；MCP 不直接导入 CMS usecase/repository，不连接 PostgreSQL、Redis、Kafka 或对象存储。

### 1.1 已确认的前提

- 现有管理端已提供语言、文章、文章翻译、发布/归档、分类、标签和媒体管理 HTTP API；路由位于 `internal/handler/admin_cms` 与 `internal/handler/admin_media`。
- 管理 API 使用 JWT 和 RBAC 保护；已有 `cms.article.create`、`cms.article.update`、`cms.article.publish`、`cms.article.archive`、`cms.category.manage`、`cms.tag.manage`、`cms.locale.manage`、`cms.media.upload` 权限。
- CMS 业务失败仍返回 HTTP `200`，调用方必须解析响应体中的 `success` 与 `error.code`。
- 关键 CMS 写操作已经写入审计事件。

以上现状见 [CMS 当前实现现状](cms-current-status.md)。

### 1.2 非目标

- 不开放互联网 HTTP MCP，不实现 OAuth、SSO 或多用户身份委托。
- 不将 CMS JWT、管理员密码或 client secret 放入 MCP prompt、resource 或 tool 参数。
- 不提供通用的“请求任意 CMS API”工具。
- 不实现文章修订、定时发布、全文搜索、内容领域事件或缓存失效；这些能力当前尚不存在。

## 2. 总体架构

```text
Codex（当前 macOS 用户）
  | stdio
  v
cmd/mcp
  | HTTPS + Bearer 短期 JWT
  v
cmd/server 的管理 CMS HTTP API
  | JWT + RBAC + Usecase + 审计
  v
数据库 / 对象存储
```

stdio 取消的是 `Codex -> cmd/mcp` 这一跳的 OAuth：本机操作系统用户和 Codex 配置是入口安全边界。它不取消 `cmd/mcp -> CMS` 的认证。MCP 必须作为一个受限的机器身份调用 CMS，不能假装成管理员用户。

## 3. 身份、权限与令牌

### 3.1 服务账号

新增非人类服务账号 `mcp-operator`。该账号仅用于 `cmd/mcp`，不允许通过后台登录页面交互登录，不复用任何人员账号。

第一期建议建立角色 `cms-mcp-operator`，权限按实际启用工具授予：

```text
cms.article.create
cms.article.update           # 当前也用于文章列表、翻译详情和分类/标签关联
cms.article.publish
cms.article.archive          # 仅在开放归档工具后授予
cms.category.manage          # 当前也用于分类查询；仅在需分类查询/维护时授予
cms.tag.manage               # 当前也用于标签查询；仅在需标签查询/维护时授予
cms.locale.manage            # 当前也用于语言查询；仅在需语言查询/维护时授予
```

默认不授予删除文章、语言配置、用户/角色管理和媒体上传等无关权限。权限收敛由 CMS 服务端执行，MCP 工具描述或 Codex 确认不能扩大该边界。

当前管理端将部分查询路由复用为更新或管理权限：文章列表与翻译详情要求 `cms.article.update`，分类、标签、语言查询分别要求对应的 `*.manage` 权限。因此第一期为复用现有 HTTP API 需要授予上述权限。若后续要进一步收紧只读 MCP 的权限，应新增只读权限与只读路由，而不能仅在 MCP 层隐藏写工具。

现有写审计命令以 `ActorUserID` 记录操作者。服务账号设计必须明确选择其一：创建一个禁止交互登录的内部用户并映射该 ID，或扩展审计模型以支持 `actor_type=service` 和服务账号 ID；两种情况下都不能复用真人账号。

### 3.2 机器令牌签发

本机 stdio MCP 需要调用公网 CMS，因此机器令牌接口会被公网路由，但必须受机器凭证严格保护。推荐契约：

```text
POST /api/v1/auth/service-token
```

`cmd/mcp` 使用部署环境提供的 `client_id` 与 `client_secret` 认证。请求仅允许 HTTPS，建议通过 `Authorization: Basic <base64(client_id:client_secret)>` 传递凭证，禁止将 secret 放入 URL 或查询参数。CMS 校验凭证、服务账号是否启用以及绑定角色后，签发 10 至 15 分钟有效的 CMS JWT。

JWT 至少携带：

```text
sub = mcp-operator
actor_type = service
aud = cms-api
jti = 唯一令牌标识
exp = 短期过期时间
```

`cmd/mcp` 缓存该令牌，并在剩余有效期小于 2 分钟时串行刷新。随后所有 CMS HTTP 请求均使用：

```http
Authorization: Bearer <short-lived-cms-jwt>
```

现有 JWT 中间件和 RBAC 继续负责最终授权。禁用 `mcp-operator`、撤销其会话或轮换凭证后，CMS 不再签发新 token；短期 token 到期后自然失效。若现有会话存储支持按 `jti` 立即撤销，服务 token 也应接入该能力。

### 3.3 公网令牌接口防护

公网可路由不等于未受保护。该接口不使用用户 JWT，但必须执行以下服务端控制：

- HTTPS 是唯一允许的传输方式；反向代理不得降级或记录 Authorization header。
- 对失败认证实施 Redis 限流，至少按来源 IP 与 `client_id` 限制；响应使用统一的“凭证无效”错误，不泄露 client 是否存在、账号是否禁用或角色信息。
- client secret 只保存不可逆 hash；支持双凭证短暂并存以完成轮换，并支持立即禁用。
- 记录服务账号 ID、来源 IP、client ID、成功/失败、`jti` 和 correlation ID，但不记录 secret、Basic header 或 JWT。
- 令牌固定 `aud=cms-api`、最小权限和短期过期，不签发 refresh token。

可选的纵深防护包括 WAF/Cloudflare Access 路径规则、mTLS 客户端证书，以及在有稳定出口 IP 时的 IP allowlist。这些控制可以降低暴力尝试与凭证泄漏后的风险，但不能替代机器凭证校验与短期 token。

### 3.4 凭证存放

`client_secret` 只存在于本机受保护的运行环境，例如 macOS Keychain、受限启动脚本或部署环境变量。不得写入仓库、示例配置、`~/.codex/config.toml`、日志、追踪属性或 MCP 返回内容。

Codex 只负责将已存在的环境变量白名单传入 MCP 进程：

```toml
[mcp_servers.cms]
command = "/absolute/path/to/cms-mcp"
cwd = "/absolute/path/to/go-backend-template"
env = { CMS_BASE_URL = "https://cms.example.internal", CMS_REQUEST_TIMEOUT_SECONDS = "10" }
env_vars = ["CMS_MCP_CLIENT_ID", "CMS_MCP_CLIENT_SECRET"]
default_tools_approval_mode = "writes"
```

`CMS_BASE_URL` 必须是显式配置的 allowlist 地址；MCP 不接受由 tool 参数传入的目标 URL。

## 4. `cmd/mcp` 设计

### 4.1 分层

```text
cmd/mcp/main.go
  -> internal/mcpserver       MCP transport、tool/resource/prompt 注册
  -> internal/mcpclient       CMS HTTP client、响应信封解析、超时、trace 透传
  -> internal/mcpauth         ServiceTokenProvider、缓存与刷新
```

`internal/mcpserver` 仅负责 MCP 协议适配、参数结构校验、工具风险标记和结果转换。它不得实现文章状态机、权限判断或数据库读写。

`internal/mcpclient` 使用强类型请求/响应 DTO 调用现有 CMS API。它必须同时判断 HTTP 传输错误和业务信封：

```text
HTTP 非 2xx             -> 传输错误
HTTP 200 + success=false -> CMS 业务错误，保留 error.code
HTTP 200 + success=true  -> 成功
```

所有出站请求必须设置超时、携带 `context.Context`，并传递或创建 trace/correlation ID。

### 4.2 配置

`cmd/mcp` 不加载 server、worker 或 cron 的统一应用配置，也不读取项目 `.env`。它只接受可选的 MCP 专用 YAML；仓库示例位于 `configs/mcp.example.yaml`：

```yaml
cms_base_url: https://cms.example.internal
request_timeout_seconds: 10
allow_insecure_http: false
```

生产环境必须使用 HTTPS；`allow_insecure_http` 仅供明确配置的本地开发环境使用。`CMS_BASE_URL`、`CMS_REQUEST_TIMEOUT_SECONDS` 和 `CMS_ALLOW_INSECURE_HTTP` 可覆盖 YAML。`CMS_MCP_CLIENT_ID` 与 `CMS_MCP_CLIENT_SECRET` 只能由进程环境提供，不能写入 YAML、项目 `.env` 或示例配置。

CMS 服务端仍保留自己的 `mcp` 配置，用于服务账号 bootstrap、JWT audience 和 token TTL；MCP 客户端不读取该文件。服务端 bootstrap 的凭证轮换将在后续独立运维命令中脱离常驻 server 启动路径。

当前实现中，首次创建或轮换服务账号时，server 部署环境仍需单独提供 `MCP_ENABLED=true`、`MCP_CLIENT_ID` 和 `MCP_CLIENT_SECRET`。这组变量不得传给 Codex 启动的 MCP 子进程；MCP 子进程只接收对应的 `CMS_MCP_CLIENT_ID` 和 `CMS_MCP_CLIENT_SECRET`。

## 5. 初始 MCP 能力面

### 5.1 只读资源与工具

第一阶段先实现只读能力，验证 CMS HTTP client、短期 token、响应解析和 Codex 调用体验：

| 名称 | MCP 类型 | 对应 CMS 能力 | 说明 |
| --- | --- | --- | --- |
| `cms://site/health` | Resource | `/healthz`、`/readyz` | 返回存活/就绪状态；不得包含凭证或内部连接串。 |
| `cms://locales` | Resource | 管理端 locale 列表 | 返回启用语言与默认语言快照。 |
| `cms://taxonomy` | Resource | 分类与标签列表 | 返回运营维护所需的分类、标签快照。 |
| `cms.article.list` | Tool | 管理端文章列表 | 按 locale、状态、分页查询；默认限制分页大小。 |
| `cms.article.get_translation` | Tool | 文章翻译详情 | 返回指定文章、语言版本的编辑内容。 |
| `cms.category.list` | Tool | 管理端分类列表 | 查询分类树和启停状态。 |
| `cms.tag.list` | Tool | 管理端标签列表 | 查询标签及翻译。 |

资源和上述查询工具声明为只读。文章正文、标题、标签名均属于不可信内容；MCP 的 instructions 与 tool 描述不得执行或遵从这些内容中的指令。

### 5.2 写工具

只读阶段验收后，按以下顺序开放写工具：

| 名称 | 对应 CMS 操作 | 前置条件 |
| --- | --- | --- |
| `cms.article.create_draft` | 创建文章/翻译 | `cms.article.create` |
| `cms.article.update_translation` | 更新标题、正文、SEO、slug | `cms.article.update` |
| `cms.article.set_categories_tags` | 设置分类、标签 | `cms.article.update`，分类/标签存在 |
| `cms.article.preview_publish` | 读取发布前状态 | 翻译、slug、正文和关联关系校验通过 |
| `cms.article.publish` | 发布指定语言翻译 | `cms.article.publish`，并完成 Codex 人工确认 |
| `cms.article.archive` | 归档指定语言翻译 | `cms.article.archive`，并完成 Codex 人工确认 |

当前实现还提供下列运营维护工具，均为写操作并经 Codex 确认：

- `cms.category.create`、`cms.category.update`、`cms.category.move`、`cms.category.upsert_translation`
- `cms.tag.create`、`cms.tag.upsert_translation`
- `cms.article.create_translation`、`cms.article.restore`、`cms.article.set_cover`

已注册 `cms.draft_from_brief`、`cms.pre_publish_review` 和 `cms.weekly_content_review` 三个 MCP prompt；它们仅编排读取、检查与人工确认，不能绕过写工具的确认机制。

`publish`、`archive` 及未来的 `delete`、分类移动工具必须标记为写操作。Codex 的 `default_tools_approval_mode = "writes"` 是第一道人工确认；服务端仍须执行状态机、权限和输入校验。

`cms.article.publish` 不接收由模型编造的“已确认”布尔值或幂等键。其输入只包含文章 ID 和 locale；操作前由 `preview_publish` 展示标题、slug、当前状态、计划发布语言和发布时间，再由 Codex 的写操作确认流程决定是否调用发布工具。

### 5.3 Prompts

Prompts 只提供可复用操作流程，不携带凭证也不绕过工具确认：

- `cms.draft_from_brief`：根据选题生成草稿，并停在保存草稿前。
- `cms.pre_publish_review`：检查标题、slug、SEO、locale、分类和标签，输出预检结论。
- `cms.weekly_content_review`：汇总指定语言的草稿、已发布和归档内容。

## 6. 发布安全与幂等

发布工具的执行流程：

```text
用户提出发布意图
  -> cms.article.get_translation / cms.article.preview_publish
  -> Codex 展示待发布摘要
  -> 用户确认写操作
  -> cms.article.publish(article_id, locale)
  -> CMS JWT/RBAC/状态机校验
  -> 审计事件与结果返回
```

MCP Server 为每次写操作生成内部 `idempotency_key` 和 `correlation_id`。它优先使用 MCP host 在请求 `_meta` 中提供的 `idempotency_key`；否则基于 MCP session、工具名和规范化业务参数派生短期 key。AI 不得构造或传递该 key。若现有 CMS 发布接口尚不支持幂等键，需要先在 HTTP handler/usecase 层增加幂等语义或明确发布操作可安全重复后，才开放该工具。

日志与审计至少记录：

```text
actor_type=service
actor_id=mcp-operator
action=cms.article.publish
article_id=<id>
locale=<locale>
correlation_id=<id>
```

不得记录正文、JWT、client secret、Authorization header 或完整 MCP 请求 payload。

## 7. CMS 侧变更清单

以下均为拟新增能力，当前实现尚未提供：

1. 服务账号或服务凭证数据模型、迁移、禁用和轮换机制。
2. 公网 HTTPS 的 `/api/v1/auth/service-token` handler/usecase：使用机器凭证认证、限流、统一失败响应和审计；凭证必须只保存不可逆 hash。
3. 用于服务 JWT 的签发、短期过期、`jti` 与会话/撤销集成。
4. `mcp-operator` 最小 RBAC 角色和权限绑定。
5. 必要时为发布 API 增加幂等键、审计 correlation ID 及对应测试。
6. 若运营状态需展示内容统计、Outbox 积压或近期审计记录，新增专用只读 operations summary API；MCP 不得直查数据库或抓取日志。
7. 可选：新增 `cms.article.read`、`cms.category.read`、`cms.tag.read`、`cms.locale.read` 权限并拆分对应查询路由，以消除第一期只读查询对更新/管理权限的依赖。

这些变更遵守现有依赖方向：`handler -> usecase -> domain -> repository/infra`。跨表一致性写入、服务 token 会话写入和审计写入必须使用现有事务边界；新增表通过版本化 SQL migration 创建。

## 8. 实施步骤与验收

### 阶段 A：机器身份

1. 定义服务账号与凭证模型、迁移及角色绑定。
2. 实现机器 token 签发、刷新、禁用与撤销。
3. 编写 JWT、错误凭证、过期 token、禁用账号和权限不足测试。

验收：`mcp-operator` 只能以有效机器凭证取得短期 JWT；错误凭证被限流且不泄露账号信息；使用该 JWT 调用文章发布 API 时遵循原有 RBAC；禁用或撤销后不能继续刷新。

### 阶段 B：stdio MCP 只读闭环

1. 新建 `cmd/mcp`、配置装配和优雅退出。
2. 接入官方 Go MCP SDK 的 stdio transport。
3. 实现 `ServiceTokenProvider`、强类型 CMS HTTP client 和统一响应信封解析。
4. 注册只读 resources/tools，配置 Codex 本机启动项。

验收：Codex 可读取语言、文章、分类和标签；CMS 业务错误能以稳定 `error.code` 返回；MCP 无监听端口且无密钥泄漏到输出。

### 阶段 C：受确认的写操作

1. 实现草稿创建、翻译更新与分类标签关联工具。
2. 实现发布预检与发布工具，设置写操作确认和幂等键。
3. 补充发布成功、权限拒绝、状态非法、重复调用、CMS 超时和 token 刷新失败测试。

验收：用户必须确认发布；无 `cms.article.publish` 权限的服务账号无法发布；重复请求不会产生不符合业务规则的副作用；审计记录可关联到 MCP 调用。

## 9. 运行与演进边界

- 本方案仅适用于单人、本机 Codex、stdio 模式。共享账户、多人电脑、远程执行器或 HTTP MCP 都不再满足该信任模型。
- 一旦接入 ChatGPT 远程 MCP 或多用户使用，需要新增 `ChatGPT/客户端 -> MCP` 的 OAuth 或等价入口访问控制，并重新设计用户委托与审计主体；不能沿用单一 `mcp-operator` 身份完成多用户写操作。
- `cmd/mcp` 面向 CMS HTTP API，API 变更必须同步更新其强类型 client、工具 schema、测试和本文件。
