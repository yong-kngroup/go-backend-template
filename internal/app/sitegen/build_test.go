package sitegen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildGeneratesMultilingualStaticSite(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(fixtureCMSHandler(t))
	t.Cleanup(server.Close)
	outputDir := filepath.Join(t.TempDir(), "dist")
	cfg, err := NewConfig(ConfigInput{
		APIBaseURL: server.URL, SiteURL: "https://docs.example.test", OutputDir: outputDir,
		PerPage: 20, Concurrency: 2, HTTPTimeout: time.Second, SiteName: "Docs",
	})
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	stats, err := New(cfg).Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if stats.Locales != 2 || stats.Articles != 2 {
		t.Fatalf("stats = %+v, want two locales and two articles", stats)
	}

	zhArticle := readOutput(t, outputDir, "zh-CN/articles/go-jing-tai-zhan/index.html")
	if !strings.Contains(zhArticle, `href="/en-US/articles/go-static-site/"`) {
		t.Fatalf("Chinese article is missing its translated article route:\n%s", zhArticle)
	}
	if strings.Contains(zhArticle, "<script>alert") {
		t.Fatalf("raw HTML from Markdown reached article output")
	}
	if !strings.Contains(zhArticle, `id="section"`) {
		t.Fatalf("article is missing generated heading ID for the table of contents")
	}

	for _, path := range []string{
		"en-US/articles/go-static-site/index.html",
		"zh-CN/categories/backend/index.html",
		"zh-CN/tags/go/index.html",
		"sitemap.xml",
		"robots.txt",
		"_redirects",
		"assets/site.css",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, path)); err != nil {
			t.Errorf("expected generated %s: %v", path, err)
		}
	}
	if redirects := readOutput(t, outputDir, "_redirects"); !strings.Contains(redirects, "/zh-CN/articles/old-go /zh-CN/articles/go-jing-tai-zhan/ 301") {
		t.Fatalf("redirect output did not normalize target:\n%s", redirects)
	}
	if sitemap := readOutput(t, outputDir, "sitemap.xml"); !strings.Contains(sitemap, "https://docs.example.test/zh-CN/articles/go-jing-tai-zhan/") {
		t.Fatalf("sitemap did not contain an absolute canonical URL:\n%s", sitemap)
	}
}

func TestMarkdownRendererRejectsRelativeImagesAndSanitizesRawHTML(t *testing.T) {
	t.Parallel()
	renderer := NewMarkdownRenderer()
	if _, err := renderer.Render("![bad](images/example.png)"); err == nil {
		t.Fatal("relative Markdown image was accepted")
	}
	if _, err := renderer.Render("[bad](//example.com)"); err == nil {
		t.Fatal("protocol-relative Markdown link was accepted")
	}
	rendered, err := renderer.Render("## Section\n\n<script>alert(1)</script>\n\n[link](https://example.com)")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(rendered.HTML), "<script") || !strings.Contains(string(rendered.HTML), `rel="noreferrer"`) {
		t.Fatalf("sanitized HTML = %s", rendered.HTML)
	}
}

func fixtureCMSHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
		locales := []Locale{
			{Code: "zh-CN", Name: "简体中文", IsEnabled: true, IsDefault: true, SortOrder: 1},
			{Code: "en-US", Name: "English", IsEnabled: true, SortOrder: 2},
		}
		zhList := []ArticleListItem{{ID: 1, Locale: "zh-CN", Title: "Go 静态站", Slug: "go-jing-tai-zhan", Summary: "使用 Go 构建静态站。", ContentFormat: "markdown", PublishedAt: &now, UpdatedAt: now, PrimaryCategory: &CategoryRef{ID: 10, Name: "后端", Slug: "backend"}}}
		enList := []ArticleListItem{{ID: 1, Locale: "en-US", Title: "Go Static Sites", Slug: "go-static-site", Summary: "Build static sites with Go.", ContentFormat: "markdown", PublishedAt: &now, UpdatedAt: now, PrimaryCategory: &CategoryRef{ID: 10, Name: "Backend", Slug: "backend"}}}
		article := func(locale string) Article {
			if locale == "zh-CN" {
				return Article{ID: 1, Locale: locale, Title: "Go 静态站", Slug: "go-jing-tai-zhan", Summary: "使用 Go 构建静态站。", Content: "## Section\n\n安全的正文。\n\n<script>alert(1)</script>", ContentFormat: "markdown", PublishedAt: &now, UpdatedAt: now, AvailableLocales: []ArticleLocale{{Locale: "zh-CN", Slug: "go-jing-tai-zhan"}, {Locale: "en-US", Slug: "go-static-site"}}, Breadcrumbs: []CategoryRef{{ID: 10, Name: "后端", Slug: "backend"}}}
			}
			return Article{ID: 1, Locale: locale, Title: "Go Static Sites", Slug: "go-static-site", Summary: "Build static sites with Go.", Content: "## Section\n\nSafe body.", ContentFormat: "markdown", PublishedAt: &now, UpdatedAt: now, AvailableLocales: []ArticleLocale{{Locale: "zh-CN", Slug: "go-jing-tai-zhan"}, {Locale: "en-US", Slug: "go-static-site"}}, Breadcrumbs: []CategoryRef{{ID: 10, Name: "Backend", Slug: "backend"}}}
		}

		switch r.URL.Path {
		case "/api/v1/public/locales":
			writeAPI(t, w, locales, nil)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/public/"), "/")
		if len(parts) < 2 {
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
		locale := parts[0]
		items := zhList
		category := Category{ID: 10, Name: "后端", Slug: "backend", Description: "后端工程文章"}
		if locale == "en-US" {
			items = enList
			category = Category{ID: 10, Name: "Backend", Slug: "backend", Description: "Backend engineering articles"}
		}
		path := strings.Join(parts[1:], "/")
		switch path {
		case "categories":
			writeAPI(t, w, []Category{category}, nil)
		case "tags":
			writeAPI(t, w, []Tag{{ID: 20, Locale: locale, Name: "Go", Slug: "go"}}, pageMetaFor(1))
		case "articles":
			writeAPI(t, w, items, pageMetaFor(1))
		case "articles/go-jing-tai-zhan", "articles/go-static-site":
			writeAPI(t, w, article(locale), nil)
		case "categories/backend/articles", "tags/go/articles":
			writeAPI(t, w, items, pageMetaFor(1))
		case "sitemap-entries":
			writeAPI(t, w, []SitemapEntry{{URL: "/" + locale + "/articles/" + items[0].Slug, LastModified: now}}, pageMetaFor(1))
		case "redirects":
			if locale == "zh-CN" {
				writeAPI(t, w, []Redirect{{SourcePath: "/zh-CN/articles/old-go", TargetPath: "/zh-CN/articles/go-jing-tai-zhan", StatusCode: 301}}, pageMetaFor(1))
			} else {
				writeAPI(t, w, []Redirect{}, pageMetaFor(0))
			}
		default:
			t.Fatalf("unexpected CMS request %s", r.URL.Path)
		}
	})
}

func pageMetaFor(total int64) *pageMeta {
	pages := 0
	if total > 0 {
		pages = 1
	}
	return &pageMeta{Page: 1, PerPage: 20, Total: total, TotalPages: pages}
}

func writeAPI(t *testing.T, w http.ResponseWriter, data any, meta *pageMeta) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(apiResponse[any]{Success: true, Data: data, Meta: meta}); err != nil {
		t.Fatalf("write test response: %v", err)
	}
}

func readOutput(t *testing.T, outputDir, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(outputDir, path))
	if err != nil {
		t.Fatalf("read generated %s: %v", path, err)
	}
	return string(data)
}
