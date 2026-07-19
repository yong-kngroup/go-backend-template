package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientReturnsCMSBusinessError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatal("missing bearer token")
		}
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":"LOCALE_NOT_FOUND","message":"locale is not enabled"}}`))
	}))
	defer server.Close()

	client, err := New(server.URL, server.Client(), tokenProviderStub{}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Articles(context.Background(), "zh-CN", 1, 20)
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "LOCALE_NOT_FOUND" {
		t.Fatalf("Articles() error = %#v, want LOCALE_NOT_FOUND API error", err)
	}
}

func TestClientRejectsInsecureURL(t *testing.T) {
	if _, err := New("http://cms.example", http.DefaultClient, tokenProviderStub{}, false); err == nil {
		t.Fatal("New() expected error for HTTP URL")
	}
}

func TestClientAllowsInsecureURLInDevelopment(t *testing.T) {
	if _, err := New("http://cms.example", http.DefaultClient, tokenProviderStub{}, true); err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
}

func TestCreateArticleDraftSendsAuthenticatedJSON(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/admin/cms/articles" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer access-token" || r.Header.Get("X-Correlation-ID") == "" {
			t.Fatalf("headers = %#v", r.Header)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || !strings.Contains(string(body), `"title":"Draft"`) {
			t.Fatalf("body = %q, err = %v", body, err)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":7,"status":"draft"}}`))
	}))
	defer server.Close()

	client, err := New(server.URL, server.Client(), tokenProviderStub{}, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := client.CreateArticleDraft(context.Background(), ArticleInput{Locale: "zh-CN", Title: "Draft", Slug: "draft"})
	if err != nil || !strings.Contains(string(data), `"id":7`) {
		t.Fatalf("CreateArticleDraft() = %s, %v", data, err)
	}
}

func TestWriteOperationReusesStableRequestHeaders(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") != "operation-42" || r.Header.Get("X-Correlation-ID") != "operation-42" {
			t.Fatalf("headers = %#v", r.Header)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":7}}`))
	}))
	defer server.Close()

	client, err := New(server.URL, server.Client(), tokenProviderStub{}, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithWriteOperation(context.Background(), "operation-42")
	if _, err := client.CreateArticleDraft(ctx, ArticleInput{Locale: "zh-CN", Title: "Draft", Slug: "draft"}); err != nil {
		t.Fatal(err)
	}
}

func TestOperationalWritesUseExpectedCMSRoutes(t *testing.T) {
	mediaID := uint(9)
	cases := []struct {
		name    string
		method  string
		path    string
		bodyHas string
		call    func(context.Context, *Client) error
	}{
		{
			name: "create article translation", method: http.MethodPost, path: "/api/v1/admin/cms/articles/7/translations", bodyHas: `"title":"Translated"`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateArticleTranslation(ctx, 7, ArticleInput{Locale: "en-US", Title: "Translated", Slug: "translated"})
				return err
			},
		},
		{
			name: "archive article translation", method: http.MethodPost, path: "/api/v1/admin/cms/articles/7/translations/zh-CN/archive", bodyHas: "null",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ArchiveArticleTranslation(ctx, 7, "zh-CN")
				return err
			},
		},
		{
			name: "restore article", method: http.MethodPost, path: "/api/v1/admin/cms/articles/7/restore", bodyHas: "null",
			call: func(ctx context.Context, c *Client) error { _, err := c.RestoreArticle(ctx, 7); return err },
		},
		{
			name: "set article cover", method: http.MethodPut, path: "/api/v1/admin/cms/articles/7/cover", bodyHas: `"media_id":9`,
			call: func(ctx context.Context, c *Client) error { _, err := c.SetArticleCover(ctx, 7, &mediaID); return err },
		},
		{
			name: "create category", method: http.MethodPost, path: "/api/v1/admin/cms/categories", bodyHas: `"name":"Engineering"`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateCategory(ctx, CategoryInput{Locale: "zh-CN", Name: "Engineering", Slug: "engineering"})
				return err
			},
		},
		{
			name: "update category", method: http.MethodPatch, path: "/api/v1/admin/cms/categories/5", bodyHas: `"is_enabled":false`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpdateCategory(ctx, 5, CategoryStateInput{IsEnabled: false, SortOrder: 2})
				return err
			},
		},
		{
			name: "move category", method: http.MethodPatch, path: "/api/v1/admin/cms/categories/5/move", bodyHas: `"sort_order":2`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.MoveCategory(ctx, 5, CategoryMoveInput{SortOrder: 2})
				return err
			},
		},
		{
			name: "translate category", method: http.MethodPut, path: "/api/v1/admin/cms/categories/5/translations/en-US", bodyHas: `"slug":"engineering"`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpsertCategoryTranslation(ctx, 5, "en-US", CategoryTranslationInput{Name: "Engineering", Slug: "engineering"})
				return err
			},
		},
		{
			name: "create tag", method: http.MethodPost, path: "/api/v1/admin/cms/tags", bodyHas: `"name":"Go"`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateTag(ctx, TagInput{Locale: "zh-CN", Name: "Go", Slug: "go"})
				return err
			},
		},
		{
			name: "translate tag", method: http.MethodPut, path: "/api/v1/admin/cms/tags/3/translations/en-US", bodyHas: `"slug":"go"`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpsertTagTranslation(ctx, 3, "en-US", TagTranslationInput{Name: "Go", Slug: "go"})
				return err
			},
		},
		{
			name: "create locale", method: http.MethodPost, path: "/api/v1/admin/cms/locales", bodyHas: `"is_enabled":false`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateLocale(ctx, LocaleCreateInput{Code: "en-US", Name: "English", IsEnabled: false})
				return err
			},
		},
		{
			name: "update locale", method: http.MethodPatch, path: "/api/v1/admin/cms/locales/en-US", bodyHas: `"is_default":true`,
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpdateLocale(ctx, "en-US", LocaleUpdateInput{Name: "English", IsEnabled: true, SortOrder: 1, IsDefault: true})
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Fatalf("request = %s %s, want %s %s", r.Method, r.URL.Path, tc.method, tc.path)
				}
				if r.Header.Get("Authorization") != "Bearer access-token" || r.Header.Get("X-Correlation-ID") == "" || r.Header.Get("Idempotency-Key") == "" {
					t.Fatalf("headers = %#v", r.Header)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || !strings.Contains(string(body), tc.bodyHas) {
					t.Fatalf("body = %q, want %q, err = %v", body, tc.bodyHas, err)
				}
				_, _ = w.Write([]byte(`{"success":true,"data":{"id":7}}`))
			}))
			defer server.Close()
			client, err := New(server.URL, server.Client(), tokenProviderStub{}, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.call(context.Background(), client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type tokenProviderStub struct{}

func (tokenProviderStub) Token(context.Context) (string, error) { return "access-token", nil }
