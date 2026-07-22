# Go 静态内容站生成器开发方案

## 1. 决策与目标

本方案在当前仓库实现一个由 Go 驱动的静态内容站生成器。生成器只调用 CMS 公开 API，读取已发布的 Markdown 内容并输出可部署的静态 HTML 文件。

本方案是后续静态站开发的实施基线，不采用 `astro-static-site-plan.md` 中的独立 Astro 仓库、Node.js 构建或前端框架设计。旧文档保留为历史方案，不得与本方案混合实现。

第一版目标：

- 多语言文章、分类、标签、文章列表和分页页面。
- 由 Markdown 构建文章正文的完整静态 HTML。
- 生成 title、description、canonical、hreflang、JSON-LD、Sitemap、robots 和重定向规则。
- 使用原生 ES Module JavaScript 提供少量渐进增强；页面核心内容和导航不依赖 JavaScript。
- 将 `dist/` 发布到任意静态托管服务，例如 Cloudflare Pages、对象存储 CDN 或 Nginx。

第一版不实现：

- SSR、登录态、浏览器端实时读取 CMS API、客户端路由或 SPA。
- 直接读取 PostgreSQL、Redis、Kafka、对象存储元数据或使用 CMS 管理端接口。
- 内容发布后即时增量构建；先使用 CI 定时全量构建。
- 搜索、评论、全文索引、图像处理或文章版本恢复。

## 2. 系统边界

```text
CMS 后端
  管理内容、发布状态、媒体、SEO 和重定向
  公开 API 仅返回已发布内容
        |
        v
sitegen（Go CLI）
  调用公开 API
  渲染 Markdown 与 HTML 模板
  写入 dist/ 静态产物
        |
        v
静态托管服务
  CDN、TLS、重定向和自定义域名
```

生成器必须通过公开 API 读取数据，即使它和 API 服务位于同一仓库。这样可以复用 CMS 已有的发布可见性、多语言和公开响应规则，且构建器可在 CI 中独立运行。

生成器不是 `cmd/server`、`cmd/cron` 或 `cmd/worker` 的一部分。构建和发布由开发者命令或 CI 工作流触发；后端 Cron 不负责构建前端产物。

## 3. 现有数据契约

生成器使用以下公开接口：

```text
GET /api/v1/public/locales
GET /api/v1/public/:locale/categories
GET /api/v1/public/:locale/tags?page=&per_page=
GET /api/v1/public/:locale/articles?page=&per_page=
GET /api/v1/public/:locale/articles/:slug
GET /api/v1/public/:locale/categories/:slug/articles?page=&per_page=
GET /api/v1/public/:locale/tags/:slug/articles?page=&per_page=
GET /api/v1/public/:locale/sitemap-entries?page=&per_page=
GET /api/v1/public/:locale/redirects?page=&per_page=
```

所有业务 API 都使用统一响应信封。HTTP `200` 不代表业务成功，客户端必须检查：

```go
type response[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Meta *struct {
		Page       int   `json:"page"`
		PerPage    int   `json:"per_page"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"total_pages"`
	} `json:"meta"`
}
```

任何 `Success == false`、响应格式错误、分页元数据不一致、网络超时或 JSON 解码失败，都必须使本次构建失败。不得跳过失败的文章后继续发布不完整站点。

文章详情中只允许 `content_format == "markdown"`。若 API 返回已发布的 `html` 或未知格式内容，构建必须失败，并输出 locale、slug 和内容格式；这是本方案“只渲染 Markdown”边界的一部分。

## 4. 代码与目录设计

新增一个可执行入口及应用包：

```text
cmd/sitegen/
  main.go                       # 解析参数、初始化日志、调用 app

internal/app/sitegen/
  app.go                        # 构建编排与依赖注入
  config.go                     # 配置校验
  client.go                     # CMS HTTP 客户端和分页读取
  types.go                      # 与公开 API 对齐的 DTO
  build.go                      # 构建快照、页面任务和错误汇总
  routes.go                     # URL、输出路径与相对链接
  markdown.go                   # Markdown 安全转换
  seo.go                        # 页面元数据、JSON-LD、Sitemap
  render.go                     # html/template 加载和页面渲染
  write.go                      # 原子输出、资源复制和清理
  templates/
    base.html
    partials.html
    home.html
    article.html
    listing.html
    category.html
    tag.html
    404.html
  assets/
    app.js
    site.css
```

`cmd/sitegen` 仅负责进程入口和配置，业务流程放在 `internal/app/sitegen`。该包不得依赖 GORM、Gin、Repository、Usecase 或 CMS 内部领域实体；它只依赖标准库、Markdown/净化库和公开 API DTO。

建议命令：

```bash
go run ./cmd/sitegen \
  -api-base https://cms.example.com \
  -site-url https://www.example.com \
  -out ./dist
```

后续可增加 `-config` 配置文件参数，但配置来源和优先级必须与仓库现有配置原则一致：命令行显式参数优先，敏感值不写入仓库。

## 5. 配置

第一版至少需要以下配置：

| 配置 | 说明 | 是否必填 |
| --- | --- | --- |
| `SITEGEN_CMS_API_BASE_URL` | CMS 公开 API 的 HTTPS 基地址 | 是 |
| `SITEGEN_SITE_URL` | 不含路径的正式站点绝对地址，用于 canonical、Sitemap 和 JSON-LD | 是 |
| `SITEGEN_OUTPUT_DIR` | 静态产物目录，默认 `dist` | 否 |
| `SITEGEN_PER_PAGE` | 列表页尺寸，范围 1 到 100，默认 20 | 否 |
| `SITEGEN_HTTP_TIMEOUT_SECONDS` | 每个 API 请求超时，默认 15 秒 | 否 |
| `SITEGEN_CONCURRENCY` | 文章详情并发抓取数，默认 4 | 否 |

配置校验规则：

- API 基地址和站点地址必须是绝对 URL；生产环境只允许 HTTPS。
- `SITEGEN_SITE_URL` 不得包含查询参数、片段或路径前缀。
- 输出目录不得是仓库根目录、当前目录或文件系统根目录，防止误写。
- 并发数必须有上限；初始上限为 16，避免对公开 API 施加突发压力。

## 6. 路由与静态输出

所有 URL 使用尾随斜杠，生成目标文件使用目录内 `index.html`：

| 页面 | 公开 URL | 输出文件 |
| --- | --- | --- |
| 默认语言入口 | `/` | 由托管层重定向，不生成内容副本 |
| 语言首页 | `/{locale}/` | `{locale}/index.html` |
| 文章列表 | `/{locale}/articles/` | `{locale}/articles/index.html` |
| 文章列表第 N 页 | `/{locale}/articles/page/{n}/` | `{locale}/articles/page/{n}/index.html` |
| 文章详情 | `/{locale}/articles/{slug}/` | `{locale}/articles/{slug}/index.html` |
| 分类页 | `/{locale}/categories/{slug}/` | `{locale}/categories/{slug}/index.html` |
| 分类分页 | `/{locale}/categories/{slug}/page/{n}/` | `{locale}/categories/{slug}/page/{n}/index.html` |
| 标签页 | `/{locale}/tags/{slug}/` | `{locale}/tags/{slug}/index.html` |
| 标签分页 | `/{locale}/tags/{slug}/page/{n}/` | `{locale}/tags/{slug}/page/{n}/index.html` |
| 404 页 | `/404/` | `404.html` |

根路径 `/` 必须由静态托管平台返回到默认语言首页的 `301`。Cloudflare Pages 使用 `_redirects`，Nginx 或对象存储 CDN 使用各自的重定向规则。不能为 `/` 生成默认语言首页的重复内容。

链接由 `routes.go` 统一生成，模板禁止拼接 URL 字符串，避免尾随斜杠、URL 编码和相对路径不一致。

## 7. 数据读取与构建流程

构建按以下顺序执行：

1. 读取并校验配置，创建带超时的 HTTP 客户端。
2. 调用 `public/locales`，识别启用语言和唯一默认语言。
3. 对每种语言读取分类树、所有标签、所有文章列表、所有 Sitemap 条目和所有重定向。
4. 根据文章列表的 `meta.total` 翻页读取，直到全部文章读取完成。
5. 使用受限 worker pool 并发读取每篇文章详情；详情响应中的 locale 和 slug 必须与列表项一致。
6. 把每种语言的数据固定为本次构建快照；页面渲染不得再发起 HTTP 请求。
7. 将 Markdown 转换为净化后的文章 HTML，构造页面 View Model。
8. 写入所有 HTML、`sitemap.xml`、`robots.txt`、`_redirects` 和静态资源。
9. 对临时产物执行基本完整性检查，通过后再替换输出目录。

站点应使用全量构建。当前 CMS 没有内容发布领域事件和构建触发机制，因此不在第一版引入增量构建或按内容更新的局部发布。

所有分页读取按 API 返回的 `meta.total_pages` 停止；同时验证每页 `meta.page` 与请求页码一致。每个 locale 的 API 读取可以并行，但文章详情读取必须经过全局并发限制。

## 8. Markdown 与内容安全

新增依赖建议：

- `github.com/yuin/goldmark`：Markdown 到 HTML。
- `github.com/microcosm-cc/bluemonday`：输出 HTML 白名单净化。

转换规则：

1. 使用 Goldmark 渲染 CommonMark、表格、删除线、任务列表和自动标题 ID。
2. 不启用 Goldmark 的 unsafe/raw HTML 选项。
3. 对生成结果使用受限白名单净化，只保留文章排版所需元素和属性。
4. 只允许 `https`、`http`、`mailto` 和同站点相对 URL；禁止 `javascript:`、`data:` 和未知 scheme。
5. 外链添加 `rel="noopener noreferrer"`；图片必须具有可用替代文本或由编辑内容修复。
6. 净化结果才可转为 `template.HTML` 交给 `html/template` 输出。模板中任何其他 CMS 字段都保持正常自动转义。

第一版不解析或下载 Markdown 内嵌图片。正文图片 URL 必须是可公开访问的绝对 URL 或以 `/` 开头的站内路径；相对路径在构建时判为错误，防止在不同文章层级下解析出不同资源地址。

## 9. HTML、CSS 与原生 JavaScript

页面使用 Go `html/template` 生成，模板按语义 HTML 编写：`header`、`nav`、`main`、`article`、`aside`、`footer`、`time`、`ol` 等。每个页面必须在没有 JavaScript 的情况下提供完整导航、正文、分页和链接。

样式策略：

- 基础样式使用固定版本的 Pico CSS CDN，以语义 HTML 获得响应式排版。
- 页面引用本地 `assets/site.css`，仅放品牌色、布局、代码块和必要的覆盖样式。
- CDN URL 必须锁定精确版本；生产上线前为 CDN 样式添加 SRI 和 `crossorigin="anonymous"`。
- CSS CDN 不承载站点关键内容。CDN 不可用时，HTML 仍需可读、可导航。

浏览器脚本仅使用本地原生 ES Module：

```html
<script type="module" src="/assets/app.js"></script>
```

`app.js` 第一版仅可实现主题偏好切换、移动端导航展开和其他非关键体验增强。它不得请求 CMS API、注入文章正文、承担页面路由或生成 SEO 信息。

## 10. SEO 规则

每个页面生成唯一的 title、description 和 canonical。规则如下：

| 字段 | 文章页规则 |
| --- | --- |
| title | `seo_title` 非空时使用，否则使用文章 `title` |
| description | `seo_description` 非空时使用，否则从 `summary` 取净化后的纯文本 |
| canonical | 默认使用 `SITEGEN_SITE_URL + article route`；仅允许经校验的同站点 `canonical_url` 覆盖 |
| `hreflang` | 根据 `available_locales` 输出已发布翻译；仅当默认语言翻译已发布时，增加 `x-default` |
| Open Graph | 复用 title、description、canonical 和封面公开 URL |
| JSON-LD | 文章页输出 `Article` 与 `BreadcrumbList` |

列表、分类和标签页也必须有唯一 title、description、canonical。第 2 页及以后 canonical 指向自身，不指向第一页；可补充 `rel="prev"` 与 `rel="next"` 方便用户导航，但不依赖其 SEO 效果。

`sitemap.xml` 从公开 Sitemap API 构建，URL 必须转为 `SITEGEN_SITE_URL` 下的绝对 URL，并带 API 返回的 `lastmod`。`robots.txt` 至少包含：

```text
User-agent: *
Allow: /
Sitemap: https://www.example.com/sitemap.xml
```

`_redirects` 使用 CMS 导出的 source path、target path 和状态码生成。CMS 当前保存的文章和分类路径可能没有尾随斜杠；生成器必须保留该 source path，并为可归一化的站内内容路径额外输出尾随斜杠变体。target path 则统一为本方案的尾随斜杠 canonical 路径。构建时应拒绝站外目标、非法状态码和重复 source path；根路径到默认语言首页的 301 规则单独写入。

## 11. 输出一致性与失败处理

生成器不得在构建失败时留下可部署的半成品目录。实现方式：

1. 在输出目录的同级临时目录写入全部产物。
2. 写入完成后检查预期文件、Sitemap 和重定向规则。
3. 成功后将旧输出目录改名为备份目录，将临时目录改名为正式输出目录。
4. 删除备份目录；删除失败只记录警告，不影响已成功生成的输出。

CI 中发布步骤只使用构建成功后的 `dist/`。构建过程产生的文章数量、页面数量、语言数量、耗时和失败原因必须记录到结构化日志。

## 12. 部署与触发

第一版使用 CI 全量构建和静态发布。建议 GitHub Actions 的触发条件：

```yaml
on:
  workflow_dispatch:
  push:
    branches: [main]
  schedule:
    - cron: "*/30 * * * *"
```

工作流职责：检出代码、安装固定 Go 版本、执行 `go run ./cmd/sitegen`、验证 `dist/`、发布静态目录。CMS API 地址、站点地址和静态托管凭证仅存放在 CI Secrets 或部署环境变量中。

构建失败时不得覆盖上一次成功部署。静态托管平台应保留上一个成功版本。

## 13. 测试计划

实现按风险补充下列测试：

- `client_test.go`：成功信封、`success: false`、HTTP 超时、错误 JSON、分页完整读取和分页元数据不一致。
- `markdown_test.go`：标题、代码块、普通链接、raw HTML、`javascript:` URL、相对图片 URL 和净化结果。
- `routes_test.go`：尾随斜杠、locale、slug、分页以及输出路径映射。
- `seo_test.go`：title/description 回退、canonical 校验、hreflang、JSON-LD、Sitemap、robots 和重定向规则。
- `render_test.go`：基于固定 API fixture 的 golden HTML，确保正文、封面、面包屑、分页和转义结果正确。
- `build_test.go`：使用 `httptest.Server` 模拟 CMS，验证完整 `dist/` 以及任一接口失败时不替换旧产物。

完成后至少执行：

```bash
go test ./...
go vet ./...
go build ./cmd/sitegen
```

## 14. 实施阶段与验收

### 阶段 1：构建器骨架和公开 API 客户端

1. 新增 `cmd/sitegen` 与 `internal/app/sitegen`。
2. 完成配置校验、HTTP 客户端、响应信封和分页读取。
3. 使用假 HTTP 服务覆盖错误和分页测试。

验收：命令可以读取全部 locale、分类、标签、文章、Sitemap 和重定向数据，不读取数据库或 CMS 内部包。

### 阶段 2：Markdown 与基础 HTML

1. 引入 Markdown 和 HTML 净化依赖。
2. 实现基础模板、本地资源与首页、文章详情、列表页。
3. 完成安全测试和输出目录原子替换。

验收：对包含多个语言和文章的 fixture 可生成完全静态、无 JavaScript 依赖的文章页面；不安全 Markdown 无法进入产物。

### 阶段 3：分类、标签和 SEO

1. 增加分类页、标签页、分页、面包屑和语言切换。
2. 增加 canonical、hreflang、Open Graph、JSON-LD、Sitemap、robots 和 `_redirects`。
3. 用 golden tests 固定 HTML 与 XML 输出。

验收：静态产物可被标准静态服务器直接托管，搜索引擎无需执行 JavaScript 即可读取每页内容与 SEO 元数据。

### 阶段 4：CI 与真实环境验证

1. 添加构建和静态发布工作流。
2. 配置生产 CMS API、站点域名和托管凭证。
3. 使用真实已发布内容执行一次全量构建，抽查语言、重定向、Sitemap 和搜索引擎抓取结果。

验收：构建失败不影响线上旧版本；新文章在下一次计划构建后出现在静态站中。

## 15. 后续演进

当内容量、发布频率或运营需求验证后，再考虑：

- CMS 在发布、归档、slug 变更和分类变更时通过 Outbox 发出内容事件。
- CI 或构建服务接收事件后触发构建，逐步替代轮询定时任务。
- 通过内容哈希和页面依赖图实现增量构建。
- 本地化静态资源、图片变体、代码高亮、搜索索引和预览环境。

这些演进不得改变“只从公开 API 读取”和“静态页面核心内容不依赖浏览器端框架”的边界。
