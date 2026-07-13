package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	client, err := New(server.URL, server.Client(), tokenProviderStub{})
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
	if _, err := New("http://cms.example", http.DefaultClient, tokenProviderStub{}); err == nil {
		t.Fatal("New() expected error for HTTP URL")
	}
}

type tokenProviderStub struct{}

func (tokenProviderStub) Token(context.Context) (string, error) { return "access-token", nil }
