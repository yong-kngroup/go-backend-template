package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type ArticleInput struct {
	Locale         string `json:"locale"`
	Title          string `json:"title"`
	Slug           string `json:"slug"`
	Summary        string `json:"summary,omitempty"`
	Content        string `json:"content,omitempty"`
	ContentFormat  string `json:"content_format,omitempty"`
	SEOTitle       string `json:"seo_title,omitempty"`
	SEODescription string `json:"seo_description,omitempty"`
	CanonicalURL   string `json:"canonical_url,omitempty"`
}

type CategoryInput struct {
	ParentID       *uint  `json:"parent_id,omitempty"`
	SortOrder      int    `json:"sort_order"`
	Locale         string `json:"locale"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description,omitempty"`
	SEOTitle       string `json:"seo_title,omitempty"`
	SEODescription string `json:"seo_description,omitempty"`
}

type CategoryStateInput struct {
	IsEnabled bool `json:"is_enabled"`
	SortOrder int  `json:"sort_order"`
}

type CategoryMoveInput struct {
	ParentID  *uint `json:"parent_id,omitempty"`
	SortOrder int   `json:"sort_order"`
}

type CategoryTranslationInput struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description,omitempty"`
	SEOTitle       string `json:"seo_title,omitempty"`
	SEODescription string `json:"seo_description,omitempty"`
}

type TagInput struct {
	Locale string `json:"locale"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
}

type TagTranslationInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type LocaleCreateInput struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsEnabled bool   `json:"is_enabled"`
	SortOrder int    `json:"sort_order"`
}

type LocaleUpdateInput struct {
	Name      string `json:"name"`
	IsEnabled bool   `json:"is_enabled"`
	SortOrder int    `json:"sort_order"`
	IsDefault bool   `json:"is_default"`
}

type TokenProvider interface {
	Token(context.Context) (string, error)
}

type writeOperationKey struct{}

// WithWriteOperation binds one MCP write intent to a stable idempotency key.
// Callers must reuse the same value only when retrying an uncertain outcome.
func WithWriteOperation(ctx context.Context, operationID string) context.Context {
	return context.WithValue(ctx, writeOperationKey{}, strings.TrimSpace(operationID))
}

type Client struct {
	baseURL string
	http    *http.Client
	tokens  TokenProvider
}

type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func New(baseURL string, httpClient *http.Client, tokens TokenProvider, allowInsecureHTTP bool) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u, err := url.Parse(baseURL)
	validScheme := u != nil && (u.Scheme == "https" || (allowInsecureHTTP && u.Scheme == "http"))
	if err != nil || !validScheme || u.Host == "" {
		return nil, fmt.Errorf("cms base URL must be an HTTPS URL")
	}
	if httpClient == nil || tokens == nil {
		return nil, fmt.Errorf("mcp HTTP client and token provider are required")
	}
	return &Client{baseURL: baseURL, http: httpClient, tokens: tokens}, nil
}

func (c *Client) Health(ctx context.Context) (json.RawMessage, error) {
	live, err := c.getPublic(ctx, "/healthz")
	if err != nil {
		return nil, err
	}
	ready, err := c.getPublic(ctx, "/readyz")
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]json.RawMessage{"live": live, "ready": ready})
}

func (c *Client) Locales(ctx context.Context) (json.RawMessage, error) {
	return c.getAdmin(ctx, "/api/v1/admin/cms/locales", nil)
}

func (c *Client) CreateLocale(ctx context.Context, input LocaleCreateInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/admin/cms/locales", input)
}

func (c *Client) UpdateLocale(ctx context.Context, code string, input LocaleUpdateInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPatch, "/api/v1/admin/cms/locales/"+url.PathEscape(code), input)
}

func (c *Client) Categories(ctx context.Context, locale string) (json.RawMessage, error) {
	data, err := c.getAdmin(ctx, "/api/v1/admin/cms/categories", url.Values{"locale": {locale}})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]json.RawMessage{"data": data})
}

func (c *Client) Tags(ctx context.Context, locale string, page, perPage int) (json.RawMessage, error) {
	return c.getAdmin(ctx, "/api/v1/admin/cms/tags", pageQuery(locale, page, perPage))
}

func (c *Client) Articles(ctx context.Context, locale string, page, perPage int) (json.RawMessage, error) {
	return c.getAdmin(ctx, "/api/v1/admin/cms/articles", pageQuery(locale, page, perPage))
}

func (c *Client) ArticleTranslation(ctx context.Context, articleID uint, locale string) (json.RawMessage, error) {
	if articleID == 0 || strings.TrimSpace(locale) == "" {
		return nil, &APIError{Code: "INVALID_INPUT", Message: "article_id and locale are required"}
	}
	return c.getAdmin(ctx, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/translations/"+url.PathEscape(locale), nil)
}

func (c *Client) CreateArticleDraft(ctx context.Context, input ArticleInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/admin/cms/articles", input)
}

func (c *Client) CreateArticleTranslation(ctx context.Context, articleID uint, input ArticleInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/translations", input)
}

func (c *Client) UpdateArticleTranslation(ctx context.Context, articleID uint, locale string, input ArticleInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPut, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/translations/"+url.PathEscape(locale), input)
}

func (c *Client) ReplaceArticleCategories(ctx context.Context, articleID uint, categoryIDs []uint, primaryCategoryID *uint) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPut, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/categories", map[string]any{"category_ids": categoryIDs, "primary_category_id": primaryCategoryID})
}

func (c *Client) ReplaceArticleTags(ctx context.Context, articleID uint, tagIDs []uint) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPut, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/tags", map[string]any{"tag_ids": tagIDs})
}

func (c *Client) PreviewPublish(ctx context.Context, articleID uint, locale string) (json.RawMessage, error) {
	return c.getAdmin(ctx, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/translations/"+url.PathEscape(locale)+"/publish-preview", nil)
}

func (c *Client) PublishArticleTranslation(ctx context.Context, articleID uint, locale string) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/translations/"+url.PathEscape(locale)+"/publish", nil)
}

func (c *Client) ArchiveArticleTranslation(ctx context.Context, articleID uint, locale string) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/translations/"+url.PathEscape(locale)+"/archive", nil)
}

func (c *Client) RestoreArticle(ctx context.Context, articleID uint) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/restore", nil)
}

func (c *Client) SetArticleCover(ctx context.Context, articleID uint, mediaID *uint) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPut, "/api/v1/admin/cms/articles/"+strconv.FormatUint(uint64(articleID), 10)+"/cover", map[string]any{"media_id": mediaID})
}

func (c *Client) CreateCategory(ctx context.Context, input CategoryInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/admin/cms/categories", input)
}

func (c *Client) UpdateCategory(ctx context.Context, categoryID uint, input CategoryStateInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPatch, "/api/v1/admin/cms/categories/"+strconv.FormatUint(uint64(categoryID), 10), input)
}

func (c *Client) MoveCategory(ctx context.Context, categoryID uint, input CategoryMoveInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPatch, "/api/v1/admin/cms/categories/"+strconv.FormatUint(uint64(categoryID), 10)+"/move", input)
}

func (c *Client) UpsertCategoryTranslation(ctx context.Context, categoryID uint, locale string, input CategoryTranslationInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPut, "/api/v1/admin/cms/categories/"+strconv.FormatUint(uint64(categoryID), 10)+"/translations/"+url.PathEscape(locale), input)
}

func (c *Client) CreateTag(ctx context.Context, input TagInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/admin/cms/tags", input)
}

func (c *Client) UpsertTagTranslation(ctx context.Context, tagID uint, locale string, input TagTranslationInput) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPut, "/api/v1/admin/cms/tags/"+strconv.FormatUint(uint64(tagID), 10)+"/translations/"+url.PathEscape(locale), input)
}

func pageQuery(locale string, page, perPage int) url.Values {
	values := url.Values{"locale": {locale}}
	if page > 0 {
		values.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		values.Set("per_page", strconv.Itoa(perPage))
	}
	return values
}

func (c *Client) getPublic(ctx context.Context, path string) (json.RawMessage, error) {
	return c.request(ctx, path, nil, false)
}

func (c *Client) getAdmin(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.request(ctx, path, query, true)
}

func (c *Client) request(ctx context.Context, path string, query url.Values, auth bool) (json.RawMessage, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	return c.do(req, auth)
}

func (c *Client) write(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode CMS request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	operationID, _ := ctx.Value(writeOperationKey{}).(string)
	if operationID == "" {
		operationID = correlationID()
	}
	req.Header.Set("X-Correlation-ID", operationID)
	req.Header.Set("Idempotency-Key", operationID)
	return c.do(req, true)
}

func (c *Client) do(req *http.Request, auth bool) (json.RawMessage, error) {
	if auth {
		token, err := c.tokens.Token(req.Context())
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call CMS API: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("CMS API HTTP %d", resp.StatusCode)
	}
	if !auth {
		return json.RawMessage(body), nil
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Meta    json.RawMessage `json:"meta"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode CMS response: %w", err)
	}
	if !envelope.Success {
		if envelope.Error == nil {
			return nil, &APIError{Code: "UNKNOWN", Message: "CMS request failed"}
		}
		return nil, &APIError{Code: envelope.Error.Code, Message: envelope.Error.Message}
	}
	if len(envelope.Meta) == 0 || string(envelope.Meta) == "null" {
		return envelope.Data, nil
	}
	return json.Marshal(map[string]json.RawMessage{"data": envelope.Data, "meta": envelope.Meta})
}

func correlationID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "mcp"
	}
	return hex.EncodeToString(buf)
}
