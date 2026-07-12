# Astro 静态内容站第一版开发计划

## 1. 目标与边界

建设一个独立的 Astro 静态站仓库，定位为单站点、多语言的 CMS 内容资源站。站点由 Cloudflare Pages 托管，构建时从本项目的公开 CMS API 拉取数据并生成静态 HTML。

第一版的重点是稳定的内容发布链路：CMS 管理内容 -> GitHub Actions 构建静态站 -> Cloudflare Pages 发布。管理后台可后续逐步加入，但不应阻塞公开内容站上线。

第一版不实现：

- SSR、用户登录态页面或浏览器端实时读取 CMS 数据。
- 增量构建、内容发布即时触发构建。
- 全文搜索、评论、文章版本恢复、协作编辑。
- 静态站仓库直接访问 PostgreSQL、Redis、对象存储或 Kafka。

## 2. 系统边界

```text
CMS 后端
  管理端写入文章、分类、标签与媒体
  公开 API 只返回已发布内容
        |
        v
GitHub Actions（静态站仓库）
  拉取 CMS 公开 API
  构建 Astro 静态页面、Sitemap 与重定向规则
        |
        v
Cloudflare Pages
  部署 dist 静态产物
  提供 CDN、TLS 与自定义域名
```

CMS 后端仍部署在独立运行环境。Cloudflare Pages 免费托管的是静态前端产物，不替代后端、数据库、消息队列或 Cron 服务。

## 3. 现有 CMS 可直接使用的能力

当前后端已可为静态构建提供：

- 多语言文章列表和文章详情，只返回已发布且发布时间已到的翻译。
- 多语言分类树、分类文章列表、标签文章列表。
- 文章详情中的 SEO 标题、描述、canonical URL、可用语言、面包屑、发布时间、更新时间和本地化封面信息。
- 按 locale 查询的 slug 重定向。
- Sitemap 条目数据。
- 使用 R2 或其他 S3 兼容对象存储的公开媒体 URL。

公开读取接口：

```text
GET /api/v1/public/:locale/articles
GET /api/v1/public/:locale/articles/:slug
GET /api/v1/public/:locale/categories
GET /api/v1/public/:locale/tags
GET /api/v1/public/:locale/categories/:slug/articles
GET /api/v1/public/:locale/tags/:slug/articles
GET /api/v1/public/:locale/redirects?source_path=...
GET /api/v1/public/:locale/sitemap-entries
```

所有 API 响应都需要检查 `success` 与 `error.code`，不可仅依据 HTTP 状态码判断，因为业务失败也使用 HTTP `200`。

## 4. 静态站信息架构

分类树决定整个网站的导航和目录布局。构建开始时，每种语言只获取一次分类树，并作为本次构建的导航快照供所有页面共享。

第一版页面：

```text
/
/{locale}/
/{locale}/articles/
/{locale}/articles/{slug}/
/{locale}/categories/{slug}/
/{locale}/tags/{slug}/
/404/
```

页面职责：

- 首页：站点介绍、顶级分类和最新文章。
- 语言首页：当前语言的分类导航和最新文章。
- 文章列表：分页列出该语言已发布文章。
- 文章详情：渲染正文、封面、SEO、面包屑和语言切换链接。
- 分类页：展示当前分类、子分类和分类文章列表。
- 标签页：预留路由；补齐公开标签发现接口后启用。
- 404 页：处理不存在的静态路径。

文章稳定 URL 采用：

```text
/{locale}/articles/{slug}/
```

分类 URL 采用：

```text
/{locale}/categories/{slug}/
```

文章 URL 不包含分类路径，分类移动不会改变文章 URL。

## 5. Astro 仓库结构

```text
cms-content-site/
  src/
    components/
      CategoryTree.astro
      ArticleCard.astro
      Breadcrumbs.astro
      LanguageSwitcher.astro
    layouts/
      BaseLayout.astro
      ArticleLayout.astro
    lib/
      cms-client.ts
      cms-build.ts
      routes.ts
      seo.ts
    pages/
      index.astro
      [locale]/
        index.astro
        articles/
          index.astro
          [slug].astro
        categories/
          [slug].astro
        tags/
          [slug].astro
      404.astro
  public/
    robots.txt
  scripts/
    generate-sitemap.mjs
    generate-redirects.mjs
  .github/workflows/
    deploy.yml
  astro.config.mjs
```

配置为静态输出：

```ts
export default defineConfig({
  output: "static",
})
```

## 6. 构建数据流程

构建器应定义统一 CMS 响应类型：

```ts
type CMSResponse<T> = {
  success: boolean
  data?: T
  error?: { code: string; message: string }
  meta?: { page: number; per_page: number; total: number; total_pages: number }
}
```

全量构建步骤：

1. 从环境变量读取 `CMS_API_BASE_URL` 和 locale 列表。
2. 对每种 locale 拉取分类树。
3. 翻页拉取文章列表，直到读取完 `meta.total` 对应的全部文章。
4. 逐篇拉取文章详情，生成文章静态路径和详情数据。
5. 基于分类树和文章数据生成语言首页、分类页与文章分页页。
6. 拉取 Sitemap 条目，生成 `sitemap.xml`。
7. 拉取完整重定向列表，生成 Cloudflare `_redirects` 文件。
8. 运行 Astro 构建，发布 `dist/`。

初期采取全量构建。它在内容量较小时实现简单、可重复，且不依赖当前尚未实现的内容事件。

## 7. CMS 构建读接口

以下只读接口已实现，可供静态站构建器直接使用：

### 7.1 启用语言列表

```text
GET /api/v1/public/locales
```

返回启用语言、默认语言和排序，静态仓库不需要硬编码 locale 列表。

### 7.2 标签发现

```text
GET /api/v1/public/:locale/tags
```

返回有已发布内容的标签列表，可用于生成标签页面和标签导航。

### 7.3 重定向导出

```text
GET /api/v1/public/:locale/redirects?page=...&per_page=...
```

不带 `source_path` 时分页导出所有记录，可生成 Cloudflare `_redirects` 文件；带该参数时保留单条重定向查询行为。

三项均为公开读模型，不改变 CMS 核心数据模型或管理端行为。

## 8. GitHub Actions 与 Cloudflare Pages

静态站仓库的 Action 触发条件：

```yaml
on:
  push:
    branches: [main]
  workflow_dispatch:
  schedule:
    - cron: "*/30 * * * *"
```

工作流步骤：

1. 检出代码，安装与锁定文件一致的 Node 版本和依赖。
2. 注入 `CMS_API_BASE_URL`，执行 Astro 构建。
3. 使用 Wrangler 将 `dist/` 发布到 Cloudflare Pages。
4. 构建或发布失败时保留 GitHub Actions 日志，Cloudflare Pages 继续保留上一次成功部署。

GitHub Secrets：

```text
CLOUDFLARE_API_TOKEN
CLOUDFLARE_ACCOUNT_ID
CMS_API_BASE_URL
```

Cloudflare API Token 只应存在于静态站仓库的 GitHub Secrets 中。不要把它放进 Go 后端配置，也不要由后端 Cron 执行 Astro 构建。

## 9. 实施阶段

### 阶段 1：验证 CMS 构建读模型

1. 使用公开 locale、标签列表和重定向导出接口作为构建数据源。
2. 确认构建器检查 `success`，并实现分页读取。
3. 依赖后端固定的分页上限和排序规则，保证构建可重复。

验收：构建程序不依赖硬编码内容，可完整获取所有 locale、文章、分类、标签和重定向。

### 阶段 2：初始化 Astro 静态站

1. 创建独立仓库，初始化 Astro 静态输出模式与 TypeScript。
2. 实现 CMS API 客户端、响应校验、分页获取和构建期错误处理。
3. 实现基础布局、分类树导航、文章卡片、面包屑与多语言切换。
4. 生成文章、分类和文章列表页；标签页保留到标签接口可用后实现。

验收：本地 `npm run build` 可生成所有已发布文章的静态 HTML，未发布内容不出现在产物中。

### 阶段 3：SEO 与 Cloudflare 部署

1. 从 CMS SEO 数据生成页面 title、description、canonical 与 hreflang。
2. 生成 `sitemap.xml`、`robots.txt` 和 `_redirects`。
3. 添加 GitHub Actions：手动触发、推送 `main` 和每 30 分钟全量构建。
4. 配置 Cloudflare Pages 项目、域名与环境变量。

验收：GitHub Actions 成功部署后，Cloudflare Pages 可访问多语言文章、分类导航、Sitemap 和重定向。

### 阶段 4：管理后台与事件触发

1. 使用同一套设计系统增加管理员登录和管理界面。
2. CMS 增加内容领域事件，通过 Outbox 投递发布、归档和 slug 变更。
3. 由事件触发 GitHub `repository_dispatch` 或 Cloudflare Deploy Hook，逐步替代定时构建。
4. 再评估增量构建、Redis 公开读缓存、搜索索引和 CDN 刷新。

## 10. 验收标准

- 所有启用 locale 均生成独立静态页面和导航。
- 分类树结构与 CMS 后端返回结构一致。
- 已发布文章能生成页面；草稿、归档和未来发布时间文章不会进入产物。
- 文章详情的封面、SEO、canonical、hreflang 和面包屑正确生成。
- Sitemap 与重定向文件可由 CMS 数据重复生成。
- 静态站构建失败不会覆盖 Cloudflare Pages 的上一个成功版本。
- 静态站不保存 CMS 数据库、对象存储或 Cloudflare 以外的基础设施凭证。
