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

func TestRound78UnreportedProviderUsageDoesNotRefundSharedTokens(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	store, err := mindruntime.NewModelUsageStore(f.model.db, "space")
	if err != nil {
		t.Fatal(err)
	}
	f.model.SetUsageStore(store)
	var called atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Add(1)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"unreported"}}]}`)
	}))
	defer provider.Close()
	ctx := mindruntime.WithOwnedModelBudget(context.Background(), mindruntime.OwnedModelBudget{SpaceID: "space", RoleID: "role", OwnerID: "owner", ParentInteractionID: "parent", CoordinationID: "round78", AssignmentID: "child", TokenLimit: 256})
	cfg := &ModelConfig{APIType: "openai-compatible", BaseURL: provider.URL, ModelName: "test", MaxTokens: 64, ContextWindow: 128, TimeoutSeconds: 5}
	for range 2 {
		if _, _, err := f.model.callLLM(ctx, cfg, []map[string]interface{}{{"role": "user", "content": "test"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := f.model.callLLM(ctx, cfg, []map[string]interface{}{{"role": "user", "content": "test"}}); !errors.Is(err, mindruntime.ErrOwnedModelBudgetExceeded) {
		t.Fatalf("budget admitted a third model call after unknown usage: %v", err)
	}
	usage, err := store.OwnedModelTokenUsage(ctx, "space", "round78")
	if err != nil || usage != 256 || called.Load() != 2 {
		t.Fatalf("reservation=%d calls=%d err=%v", usage, called.Load(), err)
	}
}

func TestRound78OutputCeilingRequiresReservedHeadroom(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	store, err := mindruntime.NewModelUsageStore(f.model.db, "space")
	if err != nil {
		t.Fatal(err)
	}
	f.model.SetUsageStore(store)
	ctx := mindruntime.WithOwnedModelBudget(context.Background(), mindruntime.OwnedModelBudget{SpaceID: "space", RoleID: "role", OwnerID: "owner", ParentInteractionID: "parent", CoordinationID: "round78-ceiling", AssignmentID: "child", TokenLimit: 4096})
	cfg := &ModelConfig{APIType: "anthropic", MaxTokens: 64, ContextWindow: 128, ReasoningEffort: "high"}
	if _, err := f.model.meterModel(ctx, cfg); err == nil {
		t.Fatal("anthropic thinking could expand output beyond reservation")
	}
	cfg = &ModelConfig{APIType: "openai", MaxTokens: 256, ContextWindow: 128}
	if _, err := f.model.meterModel(ctx, cfg); err == nil {
		t.Fatal("output limit larger than reservation admitted")
	}
	cfg = &ModelConfig{APIType: "openai", ContextWindow: 128}
	if _, err := f.model.meterModel(ctx, cfg); err == nil {
		t.Fatal("implicit default output limit larger than reservation admitted")
	}
}
