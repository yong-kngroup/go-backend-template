package mcpauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderCachesTokenUntilRefreshWindow(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v1/auth/service-token" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		id, secret, ok := r.BasicAuth()
		if !ok || id != "client" || secret != "secret" {
			t.Fatal("service credentials were not sent with basic auth")
		}
		_, _ = fmt.Fprint(w, `{"success":true,"data":{"access_token":"token","expires_in":600}}`)
	}))
	defer server.Close()

	provider, err := New(server.URL, "client", "secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		token, err := provider.Token(context.Background())
		if err != nil || token != "token" {
			t.Fatalf("Token() = %q, %v", token, err)
		}
	}
	if calls != 1 {
		t.Fatalf("service token calls = %d, want 1", calls)
	}
}

func TestProviderRejectsInsecureURL(t *testing.T) {
	if _, err := New("http://cms.example", "client", "secret", http.DefaultClient); err == nil {
		t.Fatal("New() expected error for HTTP URL")
	}
}
