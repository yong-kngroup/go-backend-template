package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpclient "github.com/freeDog-wy/go-backend-template/internal/app/mcp/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestLocaleCodes(t *testing.T) {
	codes, err := localeCodes(json.RawMessage(`[{"code":"zh-CN"},{"code":""},{"code":"en-US"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 2 || codes[0] != "zh-CN" || codes[1] != "en-US" {
		t.Fatalf("localeCodes() = %v", codes)
	}
}

func TestFilterArticleStatus(t *testing.T) {
	output := map[string]any{"data": []any{
		map[string]any{"id": float64(1), "status": "draft"},
		map[string]any{"id": float64(2), "status": "published"},
	}}
	filterArticleStatus(output, "draft")
	items := output["data"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["status"] != "draft" {
		t.Fatalf("filtered data = %#v", output["data"])
	}
}

func TestOperationalInputValidation(t *testing.T) {
	if err := validateArticleReference(articleReferenceInput{ArticleID: 7, Locale: "zh-CN"}); err != nil {
		t.Fatalf("validateArticleReference() error = %v", err)
	}
	if err := validateArticleReference(articleReferenceInput{ArticleID: 7}); err == nil {
		t.Fatal("validateArticleReference() accepted missing locale")
	}
	if err := validateNamedTranslation("zh-CN", "Engineering", "engineering"); err != nil {
		t.Fatalf("validateNamedTranslation() error = %v", err)
	}
	if err := validateNamedTranslation("zh-CN", "", "engineering"); err == nil {
		t.Fatal("validateNamedTranslation() accepted missing name")
	}
	if err := validateLocaleInput("en-US", "English (United States)"); err != nil {
		t.Fatalf("validateLocaleInput() error = %v", err)
	}
	if err := validateLocaleInput("en_US", "English"); err == nil {
		t.Fatal("validateLocaleInput() accepted an invalid code")
	}
}

func TestOperationIDForUsesHostValueOrSessionFingerprint(t *testing.T) {
	input := articleIDInput{ArticleID: 7}
	if got := operationIDFor("session-1", "host-operation", "cms.article.restore", input); got != "host-operation" {
		t.Fatalf("host operation ID = %q", got)
	}
	first := operationIDFor("session-1", "", "cms.article.restore", input)
	second := operationIDFor("session-1", "", "cms.article.restore", input)
	if first != second {
		t.Fatalf("same session and input generated %q and %q", first, second)
	}
	if changed := operationIDFor("session-1", "", "cms.article.restore", articleIDInput{ArticleID: 8}); changed == first {
		t.Fatalf("different input generated identical operation ID %q", changed)
	}
}

func TestServerRegistersOperationalToolsAndPrompts(t *testing.T) {
	ctx := context.Background()
	server := New((*mcpclient.Client)(nil))
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	wantTools := map[string]bool{
		"cms.article.create_translation":  false,
		"cms.article.archive":             false,
		"cms.article.restore":             false,
		"cms.article.set_cover":           false,
		"cms.category.create":             false,
		"cms.category.update":             false,
		"cms.category.move":               false,
		"cms.category.upsert_translation": false,
		"cms.tag.create":                  false,
		"cms.tag.upsert_translation":      false,
		"cms.locale.create":               false,
		"cms.locale.update":               false,
	}
	for tool, err := range clientSession.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
		}
	}
	for name, found := range wantTools {
		if !found {
			t.Errorf("tool %q was not registered", name)
		}
	}

	wantPrompts := map[string]bool{
		"cms.draft_from_brief":      false,
		"cms.pre_publish_review":    false,
		"cms.weekly_content_review": false,
	}
	for prompt, err := range clientSession.Prompts(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := wantPrompts[prompt.Name]; ok {
			wantPrompts[prompt.Name] = true
		}
	}
	for name, found := range wantPrompts {
		if !found {
			t.Errorf("prompt %q was not registered", name)
		}
	}
	result, err := clientSession.GetPrompt(ctx, &mcp.GetPromptParams{Name: "cms.draft_from_brief", Arguments: map[string]string{"locale": "zh-CN", "brief": "Write about content operations"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 1 || !strings.Contains(result.Messages[0].Content.(*mcp.TextContent).Text, "Write about content operations") {
		t.Fatalf("draft prompt = %#v", result)
	}
}
