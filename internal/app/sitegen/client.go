package sitegen

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Client reads only the CMS public content API.
type Client struct {
	baseURL *url.URL
	http    *http.Client
	perPage int
}

func NewClient(cfg Config) *Client {
	return &Client{
		baseURL: cfg.APIBaseURL,
		http:    &http.Client{Timeout: cfg.HTTPTimeout},
		perPage: cfg.PerPage,
	}
}

func (c *Client) ListLocales(ctx context.Context) ([]Locale, error) {
	return getData[[]Locale](ctx, c, "/api/v1/public/locales", nil)
}

func (c *Client) ListCategories(ctx context.Context, locale string) ([]Category, error) {
	return getData[[]Category](ctx, c, publicPath(locale, "/categories"), nil)
}

func (c *Client) ListArticles(ctx context.Context, locale string) ([]ArticleListItem, error) {
	return listAll(ctx, func(ctx context.Context, page int) ([]ArticleListItem, *pageMeta, error) {
		return getPage[ArticleListItem](ctx, c, publicPath(locale, "/articles"), page)
	})
}

func (c *Client) GetArticle(ctx context.Context, locale, slug string) (Article, error) {
	return getData[Article](ctx, c, publicPath(locale, "/articles/"+url.PathEscape(slug)), nil)
}

func (c *Client) ListTags(ctx context.Context, locale string) ([]Tag, error) {
	return listAll(ctx, func(ctx context.Context, page int) ([]Tag, *pageMeta, error) {
		return getPage[Tag](ctx, c, publicPath(locale, "/tags"), page)
	})
}

func (c *Client) ListCategoryArticles(ctx context.Context, locale, slug string) ([]ArticleListItem, error) {
	path := publicPath(locale, "/categories/"+url.PathEscape(slug)+"/articles")
	return listAll(ctx, func(ctx context.Context, page int) ([]ArticleListItem, *pageMeta, error) {
		return getPage[ArticleListItem](ctx, c, path, page)
	})
}

func (c *Client) ListTagArticles(ctx context.Context, locale, slug string) ([]ArticleListItem, error) {
	path := publicPath(locale, "/tags/"+url.PathEscape(slug)+"/articles")
	return listAll(ctx, func(ctx context.Context, page int) ([]ArticleListItem, *pageMeta, error) {
		return getPage[ArticleListItem](ctx, c, path, page)
	})
}

func (c *Client) ListSitemapEntries(ctx context.Context, locale string) ([]SitemapEntry, error) {
	return listAll(ctx, func(ctx context.Context, page int) ([]SitemapEntry, *pageMeta, error) {
		return getPage[SitemapEntry](ctx, c, publicPath(locale, "/sitemap-entries"), page)
	})
}

func (c *Client) ListRedirects(ctx context.Context, locale string) ([]Redirect, error) {
	return listAll(ctx, func(ctx context.Context, page int) ([]Redirect, *pageMeta, error) {
		return getPage[Redirect](ctx, c, publicPath(locale, "/redirects"), page)
	})
}

func publicPath(locale, suffix string) string {
	return "/api/v1/public/" + url.PathEscape(locale) + suffix
}

func getData[T any](ctx context.Context, c *Client, path string, query url.Values) (T, error) {
	data, _, err := get[T](ctx, c, path, query, false)
	return data, err
}

func getPage[T any](ctx context.Context, c *Client, path string, page int) ([]T, *pageMeta, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(c.perPage))
	return get[[]T](ctx, c, path, query, true)
}

func get[T any](ctx context.Context, c *Client, path string, query url.Values, requireMeta bool) (T, *pageMeta, error) {
	var zero T
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + path
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return zero, nil, fmt.Errorf("create CMS request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return zero, nil, fmt.Errorf("request %s: %w", u.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return zero, nil, fmt.Errorf("CMS request %s returned HTTP %d", u.Path, resp.StatusCode)
	}

	var payload apiResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return zero, nil, fmt.Errorf("decode CMS response %s: %w", u.Path, err)
	}
	if !payload.Success {
		if payload.Error == nil {
			return zero, nil, fmt.Errorf("CMS request %s failed without error details", u.Path)
		}
		return zero, nil, fmt.Errorf("CMS request %s failed: %s: %s", u.Path, payload.Error.Code, payload.Error.Message)
	}
	if requireMeta && payload.Meta == nil {
		return zero, nil, fmt.Errorf("CMS response %s is missing pagination metadata", u.Path)
	}
	return payload.Data, payload.Meta, nil
}

func listAll[T any](ctx context.Context, getPage func(context.Context, int) ([]T, *pageMeta, error)) ([]T, error) {
	first, meta, err := getPage(ctx, 1)
	if err != nil {
		return nil, err
	}
	if err := validateMeta(meta, 1); err != nil {
		return nil, err
	}
	all := append([]T(nil), first...)
	for page := 2; page <= meta.TotalPages; page++ {
		items, current, err := getPage(ctx, page)
		if err != nil {
			return nil, err
		}
		if err := validateMeta(current, page); err != nil {
			return nil, err
		}
		if current.TotalPages != meta.TotalPages || current.Total != meta.Total {
			return nil, fmt.Errorf("pagination metadata changed while reading page %d", page)
		}
		all = append(all, items...)
	}
	if int64(len(all)) != meta.Total {
		return nil, fmt.Errorf("pagination returned %d items, expected %d", len(all), meta.Total)
	}
	return all, nil
}

func validateMeta(meta *pageMeta, requestedPage int) error {
	if meta == nil {
		return fmt.Errorf("pagination metadata is missing")
	}
	if meta.Page != requestedPage || meta.PerPage < 1 || meta.Total < 0 || meta.TotalPages < 0 {
		return fmt.Errorf("invalid pagination metadata for page %d", requestedPage)
	}
	expectedPages := 0
	if meta.Total > 0 {
		expectedPages = int((meta.Total + int64(meta.PerPage) - 1) / int64(meta.PerPage))
	}
	if meta.TotalPages != expectedPages {
		return fmt.Errorf("invalid pagination total_pages for page %d", requestedPage)
	}
	return nil
}
