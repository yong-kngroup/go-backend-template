package sitegen

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func localeRoute(locale string) string {
	return "/" + url.PathEscape(locale) + "/"
}

func articlesRoute(locale string, page int) string {
	base := localeRoute(locale) + "articles/"
	if page <= 1 {
		return base
	}
	return base + "page/" + strconvItoa(page) + "/"
}

func articleRoute(locale, slug string) string {
	return localeRoute(locale) + "articles/" + url.PathEscape(slug) + "/"
}

func categoryRoute(locale, slug string, page int) string {
	base := localeRoute(locale) + "categories/" + url.PathEscape(slug) + "/"
	if page <= 1 {
		return base
	}
	return base + "page/" + strconvItoa(page) + "/"
}

func tagRoute(locale, slug string, page int) string {
	base := localeRoute(locale) + "tags/" + url.PathEscape(slug) + "/"
	if page <= 1 {
		return base
	}
	return base + "page/" + strconvItoa(page) + "/"
}

func outputPath(route string) string {
	clean := strings.Trim(route, "/")
	if clean == "" {
		return "index.html"
	}
	return filepath.Join(filepath.FromSlash(clean), "index.html")
}

func canonicalContentRoute(path string) (string, bool) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "", false
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 3 || (segments[1] != "articles" && segments[1] != "categories" && segments[1] != "tags") {
		return "", false
	}
	return "/" + strings.Join(segments, "/") + "/", true
}

func strconvItoa(value int) string {
	// Avoid exposing route construction to templates; this helper keeps it local.
	return fmt.Sprintf("%d", value)
}
