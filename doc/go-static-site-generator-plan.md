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
    theme-init.js                 # 样式加载前恢复已保存的主题
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

`app.js` 第一版实现主题偏好切换、移动端导航展开和其他非关键体验增强。它不得请求 CMS API、注入文章正文、承担页面路由或生成 SEO 信息。

### 9.1 UI 设计规范

#### 视觉方向

站点定位为内容优先的多语言知识库，而不是管理后台、营销落地页或卡片式信息流。视觉应延续现有 `modern-js-lab` 的清洁、克制和易读基调：浅色画布、深色文字、小圆角和绿色交互强调；但不复用任务面板的密集卡片布局。

- 首页和列表页以内容层级、留白和分隔线组织，页面区块不使用悬浮大卡片。
- 文章页以连续阅读为核心，正文、目录、元数据和相关链接应有清楚的层级，不制造装饰性视觉元素。
- 不使用渐变、发光圆形、背景插画、玻璃拟态或大面积深色背景。
- CMS 文章封面是唯一的主要视觉媒体；有封面时展示实际图片，不以模糊、裁切过度或纯氛围图替代。

#### 设计令牌

`assets/site.css` 必须先定义下列 CSS 自定义属性，模板和脚本不得内联颜色、尺寸或阴影值。Pico CSS 仅作为 reset 和基础元素样式，以下令牌覆盖站点视觉。浅色主题是令牌默认值；深色主题仅覆盖颜色令牌，排版、间距、宽度和圆角不变。

```css
:root {
  color-scheme: light;
  --color-canvas: #f7f8f6;
  --color-surface: #ffffff;
  --color-ink: #1d2730;
  --color-muted: #62707a;
  --color-line: #d9e0e2;
  --color-brand: #0f766e;
  --color-brand-hover: #0b5f59;
  --color-link: #155e9a;
  --color-code-bg: #17212b;
  --color-code-ink: #e8f0f2;
  --color-notice-bg: #edf6f2;
  --color-notice-line: #9fcfbe;
  --font-ui: Inter, "PingFang SC", "Microsoft YaHei", ui-sans-serif, system-ui, sans-serif;
  --font-reading: "Noto Serif CJK SC", "Songti SC", "STSong", Georgia, ui-serif, serif;
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-5: 24px;
  --space-6: 32px;
  --space-7: 48px;
  --radius-control: 4px;
  --radius-card: 8px;
  --shadow-subtle: 0 6px 20px rgb(29 39 48 / 0.06);
  --content-max: 1200px;
  --reading-max: 720px;
}

html[data-theme="dark"] {
  color-scheme: dark;
  --color-canvas: #11191e;
  --color-surface: #19242a;
  --color-ink: #e7eef0;
  --color-muted: #afbec4;
  --color-line: #394a52;
  --color-brand: #54cbb4;
  --color-brand-hover: #79dbc8;
  --color-link: #82cbff;
  --color-code-bg: #0c1318;
  --color-code-ink: #e5edef;
  --color-notice-bg: #17332d;
  --color-notice-line: #4d9d89;
}

@media (prefers-color-scheme: dark) {
  html:not([data-theme]) {
    color-scheme: dark;
    --color-canvas: #11191e;
    --color-surface: #19242a;
    --color-ink: #e7eef0;
    --color-muted: #afbec4;
    --color-line: #394a52;
    --color-brand: #54cbb4;
    --color-brand-hover: #79dbc8;
    --color-link: #82cbff;
    --color-code-bg: #0c1318;
    --color-code-ink: #e5edef;
    --color-notice-bg: #17332d;
    --color-notice-line: #4d9d89;
  }
}
```

基础文字使用 `--font-ui`，文章正文使用 `--font-reading`。正文基准字号为 `18px`、行高 `1.8`；正文列最大宽度为 `720px`。标题使用固定字号阶梯，不得用视口单位缩放字体：`h1` 40px、`h2` 28px、`h3` 22px、`h4` 18px。移动端只将 `h1` 调整为 32px，正文仍保持 18px 以保障阅读性。深色主题不得使用纯黑背景或纯白正文，以减轻夜间阅读眩光；封面图片、正文图片和代码块不强制反相或滤镜处理。

#### 页面框架

每页使用相同的四段结构：

```text
Skip link
Site header: 品牌 / 主导航 / 语言切换 / 主题切换
Main: 当前页面的唯一主要内容
Site footer: 版权、语言入口、Sitemap 链接
```

- 页面最大内容宽度为 `1200px`，桌面左右内边距 32px，移动端为 16px。
- 顶部导航高度固定为 64px，使用 `--color-surface` 的不透明背景和 `--color-line` 底部分隔线；滚动后可使用轻微阴影，但不得遮挡正文。
- 品牌名称为顶部左侧的首要识别元素，使用文字或现有正式 Logo；不要使用只有图标而没有可访问名称的品牌入口。
- 桌面端显示主导航、当前语言和主题控制；移动端将导航收纳为可访问的展开区，初始状态和无 JavaScript 状态必须可用。
- 所有页面在 `main` 前提供可见焦点的“跳至正文”链接。

#### 明暗主题行为

站点第一版必须同时提供浅色与深色主题。首次访问时，CSS 使用 `prefers-color-scheme` 匹配系统偏好；用户随后可以通过页头中固定尺寸的主题图标按钮在浅色和深色间切换。按钮使用 Lucide 的 `Sun` 和 `Moon` 图标，具有随状态变化的 `aria-label`，例如“切换至深色主题”。图标按钮的可点击区域不小于 40px，并配有 hover、focus-visible 和 active 状态。

主题选择使用 `localStorage` 的 `site-theme` 键保存为 `light` 或 `dark`。页面加载时，初始化脚本必须在首次可见渲染前读取该键并设置 `html[data-theme]`；没有保存值时不设置属性，让 CSS 的系统偏好规则生效。`app.js` 使用相同规则响应按钮点击。主题切换只修改 `data-theme` 和按钮标签，不得重新请求 API、重新渲染文章或改变布局尺寸。

为避免首次加载出现错误主题闪烁，`base.html` 在样式表之前加载本地的阻塞脚本 `assets/theme-init.js`：它只读取 `site-theme`，值为 `light` 或 `dark` 时设置 `document.documentElement.dataset.theme`，不包含其他业务逻辑。该方案不需要为静态 HTML 维护 CSP nonce；若 JavaScript 不可用，CSS 的 `prefers-color-scheme` 仍能提供适合系统的主题。

主题切换应只对 `background-color`、`color`、`border-color` 和阴影使用不超过 150ms 的过渡；用户设置 `prefers-reduced-motion: reduce` 时禁用过渡。深色主题必须保持与浅色主题一致的正文宽度、行高、标题尺寸、图片比例、代码滚动行为和目录位置，确保用户切换主题时文章阅读位置不发生跳动。

#### 页面布局

| 页面 | 必须包含 | 布局规则 |
| --- | --- | --- |
| 首页 | 站点名称、简短说明、顶级分类、最新文章 | 顶部是无卡片的内容介绍区；分类为最多三列的轻量链接网格，文章使用纵向列表，首屏应看到部分最新文章。 |
| 语言首页 | 当前语言名称、分类导航、最新文章 | 与首页相同结构，但所有文案和链接属于当前 locale。 |
| 文章列表 | 标题、文章数量、文章卡片、分页 | 单列列表；每项展示分类、标题、摘要、发布时间和可选缩略封面。 |
| 文章详情 | 面包屑、标题、摘要、元数据、可选封面、正文、目录、语言链接 | 桌面端采用 `minmax(0, 720px) 200px` 两列，正文在左、目录在右；目录可 sticky。小屏幕将目录移动到正文前。 |
| 分类页 | 分类名称、描述、子分类、文章列表、分页 | 子分类为普通链接列表，不包裹在嵌套卡片中；文章列表复用同一组件。 |
| 标签页 | 标签名称、文章列表、分页 | 与分类文章列表一致，不额外制造装饰区。 |
| 404 | 简短错误说明、返回首页链接、分类入口 | 不使用插画或营销式文案。 |

“文章卡片”是唯一允许的重复内容卡片：使用 `--color-surface` 背景和 `--color-line` 的 1px 边框、最多 8px 圆角、内边距 20px。它可在悬停时变更边框和阴影，但高度、封面比例、标题行距和元数据区域必须稳定，不能因内容变化跳动。文章卡片不得嵌套在其他卡片内。

#### 文章语言切换

多语言是文章页的必备能力。每篇文章详情页必须根据详情接口的 `available_locales` 渲染语言切换器，并用构建开始时读取的 locale 列表将 locale code 映射为本地化显示名称。

- 只展示 `available_locales` 中的已发布翻译，禁止为未翻译语言生成禁用项、占位链接或自动回退内容。
- 每个选项的链接由该条记录的 `locale` 和 `slug` 共同生成，例如英文翻译链接到 `/{locale}/articles/{translated-slug}/`；不得假定所有翻译共用 slug。
- 当前语言显示为当前项，添加 `aria-current="page"`，且不生成指向自身的链接；其他选项使用 `<a hreflang="..." lang="...">`。
- 有两种及以上已发布翻译时，在文章标题与元数据区域中提供语言菜单；只有一种翻译时不显示空的语言切换控件。
- 控件使用原生 `<details>` 和 `<summary>` 实现可展开菜单，`summary` 显示当前语言名称并具有清晰的“选择文章语言”无障碍名称。这样即使 JavaScript 不可用，用户仍能展开并跳转。
- 语言切换只导航到另一个已生成的静态页面，不通过浏览器 API 请求翻译内容；切换后不尝试复制当前阅读进度或正文锚点，因为不同语言的内容结构可能不同。

建议的静态 HTML 结构：

```html
<nav class="article-language-switcher" aria-label="选择文章语言">
  <details>
    <summary>简体中文</summary>
    <ul>
      <li><span aria-current="page" lang="zh-CN">简体中文</span></li>
      <li><a href="/en-US/articles/go-cms-design/" hreflang="en-US" lang="en-US">English</a></li>
    </ul>
  </details>
</nav>
```

站点页头的全局语言入口与文章切换器职责不同：全局入口用于前往其他语言首页；进入文章详情后，文章切换器才负责跳转到该文章存在的翻译版本。两者不得混用，避免用户被带到不存在的翻译 URL。

#### 文章排版与 Markdown 映射

`markdown.go` 的输出和 `site.css` 必须满足以下对应关系：

| Markdown 元素 | HTML/CSS 规则 |
| --- | --- |
| `h2` - `h4` | 标题前留出 48px，具有稳定 `id` 供目录链接；标题不使用负字距。 |
| 段落 | 段间距 20px，禁止首行缩进，长英文 URL 必须自动换行。 |
| 链接 | 使用 `--color-link` 和明显的下划线；仅颜色变化不能是唯一状态提示。 |
| 引用 | 左侧 3px `--color-brand` 边线、`--color-notice-bg` 背景；不是圆角大卡片。 |
| 无序/有序列表 | 保留列表标记，使用足够的缩进和项间距。 |
| 表格 | 外层允许横向滚动；表头使用浅色背景，单元格有分隔线。 |
| 代码块 | 深色背景、浅色文字、16px 内边距、4px 圆角、可横向滚动。 |
| 行内代码 | 使用主题适配的弱化背景和等宽字体，不与链接样式混淆。 |
| 图片 | 最大宽度 100%，保留原始纵横比；使用 Markdown alt 文本作为 `alt`。 |

文章详情页标题下依次展示发布时间、更新时间、主分类和预计阅读时长。阅读时长按净化后的正文纯文本计算，采用每分钟 400 个中文字符或 200 个英文单词的较大值，并向上取整；该信息是辅助阅读信息，不能替代发布日期。

#### 响应式与交互状态

布局断点固定为 768px 与 1024px：

- 小于 768px：单列，导航折叠，文章目录置于正文前，封面占满内容宽度。
- 768px 至 1023px：列表可为两列，文章页仍单列，保留 24px 页面内边距。
- 1024px 及以上：首页分类最多三列，文章详情启用正文和目录双列布局。

交互仅用于增强。链接、按钮、输入控件必须有 default、hover、focus-visible、active 和 disabled 状态；键盘焦点使用不少于 3px 的高对比轮廓。主题切换保存到 `localStorage`，没有用户选择时遵循 `prefers-color-scheme`。

#### 无障碍与 UI 验收

- 常规文本与背景对比度至少为 4.5:1，大号文字和交互边界至少为 3:1。
- 图像使用 CMS 返回的本地化 alt 文本；装饰图使用空 alt。
- 导航、语言切换、目录和分页具有明确的 `aria-label`、当前项状态及键盘访问路径。
- 每个页面只有一个 `h1`，标题层级连续；标题、链接和按钮的可点击区域不小于 40px。
- 在 320px、768px、1440px 三种宽度下验证文字不溢出、控件不重叠、文章正文可阅读。
- 在浅色、深色和系统自动三种主题来源下验证正文、链接、代码块、引用、表格、导航和焦点轮廓的对比度及可读性；主题切换前后正文滚动位置和页面布局不变化。
- 对一篇具有不同 slug 的多语言文章验证：只显示已发布翻译、当前语言不可点击、所有翻译链接均指向已生成页面；对于只有一种翻译的文章不渲染语言菜单。
- 使用浏览器 Lighthouse 检查 Accessibility、SEO 和 Best Practices；视觉实现以生成后的实际页面截图为准，而不是仅检查模板源代码。

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
