package mcpclient

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

type tokenProviderStub struct{}

func (tokenProviderStub) Token(context.Context) (string, error) { return "access-token", nil }
