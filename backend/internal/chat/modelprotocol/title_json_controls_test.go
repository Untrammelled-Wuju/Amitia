package modelprotocol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeminiTitleJSONMode(t *testing.T) {
	adapter := &GeminiAdapter{}
	config := adapter.buildGenConfig(ProviderConfig{MaxOutputTokens: 128}, ModelRequest{ResponseFormat: ModelResponseFormat{Type: "json_object"}, DisableThinking: true, ReasoningEffort: "high"})
	if config["responseMimeType"] != "application/json" || config["thinkingConfig"] != nil {
		t.Fatalf("JSON title controls missing: %v", config)
	}
}

func TestOllamaTitleJSONMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["format"] != "json" {
			t.Errorf("JSON title format missing: %v", body["format"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": map[string]interface{}{"role": "assistant", "content": `{"title":"修复日志"}`}, "done": true})
	}))
	defer server.Close()
	adapter := &OllamaAdapter{}
	_, err := adapter.Generate(context.Background(), ProviderConfig{BaseURL: server.URL, ModelName: "test", MaxOutputTokens: 128}, ModelRequest{ResponseFormat: ModelResponseFormat{Type: "json_object"}})
	if err != nil {
		t.Fatal(err)
	}
}
