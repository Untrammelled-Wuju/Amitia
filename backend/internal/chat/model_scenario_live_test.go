package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestModelScenarioActualHTTPInference(t *testing.T) {
	for _, mode := range []string{"local", "cloud"} {
		t.Run(mode, func(t *testing.T) {
			f := newOwnedExecutionFixture(t)
			var mu sync.Mutex
			var calls []string
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var input struct {
					Model    string
					Stream   bool
					Messages []map[string]any
				}
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				mu.Lock()
				calls = append(calls, input.Model)
				mu.Unlock()
				if r.URL.Path != "/chat/completions" {
					t.Errorf("unexpected protocol endpoint: %s", r.URL.Path)
				}
				text := `{"items":[]}`
				if input.Model == "summary" {
					text = `{"summaryText":"真实链路摘要"}`
				}
				if input.Stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\ndata: [DONE]\n\n", input.Model)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": text}}}})
			}))
			defer provider.Close()
			if err := f.model.db.Model(&ModelConfig{}).Where("id = 1").Updates(map[string]any{"base_url": provider.URL, "protocol": "openai_chat", "model_name": "default"}).Error; err != nil {
				t.Fatal(err)
			}
			for id, name := range map[int]string{2: "chat", 3: "summary", 4: "memory", 5: "explicit"} {
				if err := f.model.db.Create(&ModelConfig{ID: id, Name: name, APIType: "openai-compatible", Protocol: "openai_chat", BaseURL: provider.URL, ModelName: name, MaxTokens: 128}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := f.model.repo.UpdateModelRoutes([]map[string]interface{}{{"scenario": "chat", "modelConfigId": 2}, {"scenario": "summary", "modelConfigId": 3}, {"scenario": "memory_extract", "modelConfigId": 4}}); err != nil {
				t.Fatal(err)
			}
			request := f.request(mode + "-chat")
			if mode == "cloud" {
				request.CoreID = "cloud-core"
			}
			response, err := f.engine.Run(t.Context(), request)
			if err != nil || response.Generation.Text != "chat" || !response.Saved {
				t.Fatalf("chat: %+v %v", response, err)
			}
			scope := response.Scope
			ctx := coordination.WithScope(t.Context(), scope)
			snapshot, err := f.port.Snapshot(ctx, scope, coordination.DataQuery{})
			if err != nil {
				t.Fatal(err)
			}
			inference := business.Inference{Scope: scope, Snapshot: snapshot, Message: "已确认的用户事实", ModelConfigID: 5, Preview: true}
			generation, err := f.model.GenerateOwnedReply(ctx, inference)
			if err != nil || generation.Text != "explicit" {
				t.Fatalf("explicit: %+v %v", generation, err)
			}
			if _, err := f.model.ExtractOwnedMemoryCandidates(ctx, inference, business.Generation{Text: "确认"}); err != nil {
				t.Fatal(err)
			}
			if _, err := f.model.ExtractOwnedMemory(ctx, inference, business.Generation{Text: "确认"}); err != nil {
				t.Fatal(err)
			}
			text, err := f.model.GenerateOwnedSummary(ctx, business.SummaryInference{Scope: scope, Role: snapshot.Role, Messages: []business.SummaryMessage{{Role: "user", Content: "已确认的用户事实"}}})
			if err != nil || text != "真实链路摘要" {
				t.Fatalf("summary: %s %v", text, err)
			}
			mu.Lock()
			got := fmt.Sprint(calls)
			mu.Unlock()
			if got != "[chat explicit memory memory summary]" {
				t.Fatalf("wrong actual provider calls: %s", got)
			}
		})
	}
}
