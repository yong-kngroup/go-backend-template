package mcpserver

import (
	"encoding/json"
	"testing"
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
