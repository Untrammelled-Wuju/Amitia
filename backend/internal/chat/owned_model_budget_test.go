package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/mindruntime"
)

func TestRound77ModelCallReservesAndSettlesTokens(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	store, err := mindruntime.NewModelUsageStore(f.model.db, "space")
	if err != nil {
		t.Fatal(err)
	}
	f.model.SetUsageStore(store)
	var invoked atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invoked.Add(1)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)
	}))
	defer server.Close()
	budget := mindruntime.OwnedModelBudget{SpaceID: "space", RoleID: "role", OwnerID: "owner", ParentInteractionID: "parent", CoordinationID: "coord", AssignmentID: "child", TokenLimit: 133}
	ctx := mindruntime.WithOwnedModelBudget(context.Background(), budget)
	cfg := &ModelConfig{APIType: "openai-compatible", BaseURL: server.URL, ModelName: "model", ContextWindow: 128, MaxTokens: 64, TimeoutSeconds: 5}
	if _, _, err := f.model.callLLM(ctx, cfg, []map[string]interface{}{{"role": "user", "content": "test"}}); err != nil {
		t.Fatal(err)
	}
	sum, err := store.OwnedModelTokenUsage(ctx, "space", "coord")
	if err != nil || sum != 7 {
		t.Fatalf("actual token reconciliation %d %v", sum, err)
	}
	cfg.ContextWindow = 150
	if _, _, err := f.model.callLLM(ctx, cfg, []map[string]interface{}{{"role": "user", "content": "test"}}); !errors.Is(err, mindruntime.ErrOwnedModelBudgetExceeded) {
		t.Fatalf("unreserved model call admitted %v", err)
	}
	if invoked.Load() != 1 {
		t.Fatalf("provider invoked without admission %d", invoked.Load())
	}
	cfg.ContextWindow = 0
	if _, _, err := f.model.callLLM(ctx, cfg, []map[string]interface{}{{"role": "user", "content": "test"}}); err == nil {
		t.Fatal("missing context ceiling admitted")
	}
}
