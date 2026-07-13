package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type TokenProvider interface {
	Token(context.Context) (string, error)
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

func New(baseURL string, httpClient *http.Client, tokens TokenProvider) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
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
	if auth {
		token, err := c.tokens.Token(ctx)
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
