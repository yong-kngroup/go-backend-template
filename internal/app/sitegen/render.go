package sitegen

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"
)

//go:embed templates/*.html assets/*
var siteFiles embed.FS

type renderer struct{}

type labels struct {
	Home            string
	Articles        string
	Categories      string
	Tags            string
	LatestArticles  string
	Published       string
	Updated         string
	ReadingTime     string
	Minutes         string
	TableOfContents string
	ArticleLanguage string
	SelectLanguage  string
	Previous        string
	Next            string
	Page            string
	NotFound        string
	BackToHome      string
	SkipToContent   string
	Menu            string
	ToggleDark      string
	ToggleLight     string
}

type headView struct {
	Title       string
	Description string
	Canonical   string
	Hreflangs   []hreflangView
	Image       string
	Kind        string
}

type hreflangView struct {
	Locale  string
	URL     string
	Default bool
}

type localeOptionView struct {
	Code    string
	Name    string
	URL     string
	Current bool
}

type categoryNavView struct {
	Name string
	URL  string
}

type pageBaseView struct {
	SiteName      string
	CurrentLocale localeOptionView
	Locales       []localeOptionView
	Navigation    []categoryNavView
	Labels        labels
	Head          headView
	HomeURL       string
}

type articleCardView struct {
	Title       string
	Summary     string
	URL         string
	PublishedAt *time.Time
	Category    *categoryNavView
	Cover       *Cover
}

type paginationView struct {
	Current       int
	Total         int
	Previous      string
	Next          string
	PreviousLabel string
	NextLabel     string
	PageLabel     string
}

type homeView struct {
	pageBaseView
	Heading    string
	Categories []categoryNavView
	Articles   []articleCardView
}

type listingView struct {
	pageBaseView
	Heading     string
	Description string
	Articles    []articleCardView
	Pagination  paginationView
}

type categoryView struct {
	listingView
	Children []categoryNavView
}

type articleLanguageView struct {
	Name    string
	Code    string
	URL     string
	Current bool
}

type articleView struct {
	pageBaseView
	Article          Article
	Body             template.HTML
	TOC              []TOCEntry
	ReadingMinutes   int
	Languages        []articleLanguageView
	ShowLanguageMenu bool
}

type notFoundView struct {
	pageBaseView
}

func newRenderer() *renderer { return &renderer{} }

func (r *renderer) Render(templateName string, data any) ([]byte, error) {
	funcs := template.FuncMap{
		"formatDate": func(value *time.Time) string {
			if value == nil {
				return ""
			}
			return value.Format("2006-01-02")
		},
		"formatUpdated": func(value time.Time) string { return value.Format("2006-01-02") },
		"hasCover":      func(cover *Cover) bool { return cover != nil && strings.TrimSpace(cover.URL) != "" },
	}
	tmpl, err := template.New("base.html").Funcs(funcs).ParseFS(siteFiles,
		"templates/base.html",
		"templates/partials.html",
		"templates/"+templateName,
	)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", templateName, err)
	}
	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "base", data); err != nil {
		return nil, fmt.Errorf("render %s: %w", templateName, err)
	}
	return output.Bytes(), nil
}

func localizedLabels(locale string) labels {
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return labels{
			Home: "首页", Articles: "文章", Categories: "分类", Tags: "标签", LatestArticles: "最新文章",
			Published: "发布于", Updated: "更新于", ReadingTime: "阅读约", Minutes: "分钟",
			TableOfContents: "目录", ArticleLanguage: "文章语言", SelectLanguage: "选择文章语言",
			Previous: "上一页", Next: "下一页", Page: "第", NotFound: "页面不存在", BackToHome: "返回首页",
			SkipToContent: "跳至正文", Menu: "导航菜单", ToggleDark: "切换至深色主题", ToggleLight: "切换至浅色主题",
		}
	}
	return labels{
		Home: "Home", Articles: "Articles", Categories: "Categories", Tags: "Tags", LatestArticles: "Latest articles",
		Published: "Published", Updated: "Updated", ReadingTime: "Reading time", Minutes: "min",
		TableOfContents: "On this page", ArticleLanguage: "Article language", SelectLanguage: "Choose article language",
		Previous: "Previous", Next: "Next", Page: "Page", NotFound: "Page not found", BackToHome: "Back to home",
		SkipToContent: "Skip to content", Menu: "Navigation menu", ToggleDark: "Switch to dark theme", ToggleLight: "Switch to light theme",
	}
}
