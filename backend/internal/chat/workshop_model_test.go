package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallLLMJSONRequestsJSONObjectFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		format, ok := request["response_format"].(map[string]interface{})
		if !ok || format["type"] != "json_object" {
			t.Errorf("response_format = %#v", request["response_format"])
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": `{}`}}}, "usage": map[string]interface{}{"total_tokens": 1}})
	}))
	defer server.Close()

	service := &service{}
	content, _, err := service.callLLMJSON(context.Background(), &ModelConfig{BaseURL: server.URL, APIKey: "test", ModelName: "test", MaxTokens: 100}, []map[string]interface{}{{"role": "user", "content": "json"}})
	if err != nil {
		t.Fatal(err)
	}
	if content != `{}` {
		t.Fatalf("content = %q", content)
	}
}
