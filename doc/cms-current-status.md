# CMS 当前实现现状

更新时间：2026-07-13

本文记录当前分支的实际实现，用于与 [CMS 方案](cms-plan.md) 对照。`cms-plan.md` 仍是目标与边界定义；由于其中的缓存、内容事件和版本增强尚未完成，当前不删除该文档。

## 当前定位

项目已经具备单站点、多语言 Headless CMS 的第一版可用后端：管理员可管理语言、分类、标签、文章翻译和媒体；公开 API 只读取已发布内容；媒体使用 S3 兼容对象存储，支持 Cloudflare R2 与 MinIO。

HTTP API 无论业务成功或失败均返回 HTTP `200`，调用方应依据 `success` 和 `error.code` 判断结果。

## 已完成能力

### 内容与多语言

- `locales` 提供默认语言、启用状态和排序管理。
- 文章与翻译分表：文章共享作者、封面、删除状态；标题、slug、正文、SEO 与发布状态按 locale 独立管理。
- 翻译支持草稿、发布、归档；公开读取仅返回已发布且发布时间已到的翻译。
- 缺失翻译不会回退到其他语言，公开 API 返回 `CONTENT_TRANSLATION_NOT_FOUND`。
- 文章支持软删除与恢复。

### 分类、标签与公开读模型

- 分类使用邻接表模型，支持排序、启停、移动和循环引用防护。
- 分类和标签均支持多语言翻译与 locale 内 slug 唯一约束。
- 文章支持多分类、唯一主分类和多标签关联。
- 公开 API 已提供启用语言、文章列表/详情、分类树、标签发现、分类文章列表、标签文章列表、重定向查询/导出与 Sitemap 条目：

```text
GET /api/v1/public/locales
GET /api/v1/public/:locale/articles
GET /api/v1/public/:locale/articles/:slug
GET /api/v1/public/:locale/categories
GET /api/v1/public/:locale/tags
GET /api/v1/public/:locale/categories/:slug/articles
GET /api/v1/public/:locale/tags/:slug/articles
GET /api/v1/public/:locale/redirects
GET /api/v1/public/:locale/sitemap-entries
```

- 文章详情包含 SEO 字段、canonical URL、可用语言、主分类、面包屑、发布时间、更新时间和封面信息。

### 后台与权限

- 管理端已提供语言、分类、标签、文章、翻译、发布/归档、文章分类/标签、封面和媒体管理路由。
- 路由由 RBAC 保护，当前已注册 `cms.article.create`、`cms.article.update`、`cms.article.publish`、`cms.article.archive`、`cms.category.manage`、`cms.tag.manage`、`cms.locale.manage`、`cms.media.upload` 权限。
- 关键 CMS 写操作写入审计事件，包括发布、归档、删除、恢复、slug 变更、分类移动、标签关联和封面变更。

### SEO 与重定向

- 文章和分类翻译保存独立 SEO 标题、描述；文章保存 canonical URL。
- 文章翻译 slug 变更会创建按 locale 区分的重定向记录。
- 重定向读取通过公开 API 暴露目标路径与状态码，由前端或边缘层执行浏览器 `301`/`308` 跳转。
- Sitemap 条目 API 返回文章和分类的稳定路径及最后更新时间；XML 渲染责任仍在前端、边缘层或后续独立适配器。重定向 API 不带 `source_path` 时分页导出记录，适合静态构建生成 Cloudflare `_redirects` 文件。

### 媒体与对象存储

- 使用通用 S3 适配器，配置 `endpoint`、`region`、`use_path_style` 等参数；可对接 R2，也可用 MinIO 进行集成测试。
- 上传闭环为：申请预签名 URL -> 客户端直传 -> 服务端确认 -> 对象元数据与真实图片内容校验 -> 标记为 ready。
- 仅接受 JPEG、PNG、WebP；同时限制大小、尺寸和像素数，并验证图片真实内容而非只信任 MIME 元数据。
- 媒体支持按 locale 保存 alt 文本和标题；文章封面只能引用 ready 媒体，公开文章返回本地化封面 URL 与文案。
- pending、expired、failed 上传由 Cron 回收。校验失败会优先删除对象；删除失败保留失败记录，等待后续 Cron 重试。

### 数据库与测试

- CMS 与媒体表均由版本化 SQL migration 创建，当前最新版本为 `000009_media_failure_reason`。
- 已覆盖 CMS/媒体 Repository 的 PostgreSQL 集成测试、真实 MinIO S3 适配器测试，以及“创建媒体记录 -> 上传 -> 确认 -> 设置文章封面”的跨层集成测试。
- 已覆盖公开内容 Handler 的基础路由测试；管理媒体 Handler 覆盖权限、非法请求、上传校验失败、上传过期和成功；管理 CMS Handler 覆盖封面设置权限、非法输入、设置与清除成功。
- CI 会运行单元测试、静态检查、可执行文件与镜像构建，并启动 PostgreSQL、Redis、Kafka、MinIO 执行集成测试。

## 与方案的差距

以下能力仍未实现，因而 `cms-plan.md` 仍需保留。

### 内容事件与缓存

- CMS 写操作目前只发布审计事件，没有发布 `cms.article_translation.published`、`cms.article_translation.archived`、`cms.article_translation.slug_changed`、`cms.category.changed`、`cms.media.uploaded` 等内容领域事件。
- Redis 当前用于会话、验证码和限流，尚未缓存公开文章、分类树、分类/标签文章列表或 Sitemap。
- 因此也没有基于 Outbox 的缓存失效消费者、CDN 刷新或搜索索引处理。

### 内容工作流增强

- 尚无 `article_revisions` 表、版本列表、版本恢复接口。
- 尚无定时发布字段、调度任务和对应的状态转换规则。
- 当前没有编辑协作、审批流、评论或全文搜索，这些本就不属于第一版必需范围。

### 媒体管理完善

- 管理端没有主动删除 ready 媒体的用例和路由，`cms.media.delete` 权限也尚未定义。
- 还未生成缩略图、图片衍生版本或异步安全扫描；当前只保存原图和确认时取得的宽高。

### API 与测试缺口

- 管理 CMS Handler 现有路由测试集中在文章封面；语言、分类、标签、文章翻译与发布状态机仍应逐步补齐成功、权限拒绝和领域错误映射测试。
- 媒体管理 Handler 尚未覆盖列表、翻译更新、对象存储未配置和媒体不存在场景。
- 公开读模型、缓存失效和内容事件尚无端到端式业务集成验证；在相关能力实现后，应优先补 Repository/Usecase 集成测试，再补少量路由测试。

## 推荐后续顺序

1. 定义 CMS 内容领域事件，并在发布、归档、slug/分类/封面变更的同一事务内通过 Outbox 写入。
2. 为公开读模型增加 Redis 缓存与幂等失效消费者，缓存 key 必须包含 locale。
3. 增加文章修订和定时发布，随后再引入 Sitemap 生成、搜索索引或 CDN 刷新消费者。
4. 根据实际运营需要补媒体删除、缩略图/变体和异步安全扫描。
5. 按功能闭环继续补管理端路由测试与跨层集成测试。
