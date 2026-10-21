package modelprotocol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type ollamaBudgetSink struct{}

func (ollamaBudgetSink) Emit(context.Context, ModelEvent) error { return nil }

func TestOllamaPreservesOutputAndContextBudgetsAcrossModes(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(map[bool]string{false: "generate", true: "stream"}[streaming], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var input struct {
					Stream  bool
					Options map[string]float64
				}
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				if input.Stream != streaming || input.Options["num_ctx"] != 8192 || input.Options["num_predict"] != 128 {
					t.Errorf("wrong independent budgets: %+v", input)
				}
				_, _ = w.Write([]byte("{\"message\":{\"content\":\"summary\"},\"done\":true}\n"))
			}))
			defer server.Close()
			cfg := ProviderConfig{BaseURL: server.URL, ModelName: "test", ContextWindow: 8192, MaxOutputTokens: 512}
			request := ModelRequest{MaxOutputTokens: 128}
			adapter := &OllamaAdapter{}
			var err error
			if streaming {
				_, err = adapter.Stream(t.Context(), cfg, request, ollamaBudgetSink{})
			} else {
				_, err = adapter.Generate(t.Context(), cfg, request)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
