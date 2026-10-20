package interaction

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/temporal"
)

type recordingEntryExecutor struct {
	request *UnifiedEntryRequest
	scope   InteractionScope
}

func (e *recordingEntryExecutor) ExecuteEntry(_ context.Context, req *UnifiedEntryRequest) (*OrchestrationResult, error) {
	e.request = req
	return &OrchestrationResult{InteractionID: "owned-parent", Outcome: OutcomeCompleted}, nil
}
func (e *recordingEntryExecutor) CancelEntryScope(scope InteractionScope) int {
	e.scope = scope
	return 1
}
func (e *recordingEntryExecutor) EntryInteractionActive(id string) bool { return id == "owned-parent" }
func (e *recordingEntryExecutor) EntryReady() bool                      { return true }

func TestEntryExecutorOwnsExecutionWithoutLegacyResolverOrOrchestrator(t *testing.T) {
	entry := NewUnifiedEntry(nil, ScopeResolver{}, temporal.SystemClock{})
	executor := &recordingEntryExecutor{}
	entry.SetEntryExecutor(executor)
	request := &UnifiedEntryRequest{Channel: "test", Message: "hello", RequestID: "", CharacterID: "role"}
	result, err := entry.Handle(t.Context(), request)
	if err != nil || result.InteractionID != "owned-parent" || executor.request == nil || executor.request.RequestID == "" {
		t.Fatalf("canonical dispatch failed: %+v %v", result, err)
	}
	if request.RequestID != "" {
		t.Fatal("entry mutated caller request")
	}
	if !entry.IsReady() || !entry.IsInteractionActive("owned-parent") {
		t.Fatal("entry still depends on legacy readiness")
	}
	if count := entry.CancelByPeer("test", "peer"); count != 1 || executor.scope.Channel != "test" || executor.scope.PeerID != "peer" {
		t.Fatal("cancel was not routed to canonical executor")
	}
}

func TestEntryExecutorRejectsNilRequest(t *testing.T) {
	entry := NewUnifiedEntry(nil, ScopeResolver{}, temporal.SystemClock{})
	if _, err := entry.Handle(t.Context(), nil); err == nil {
		t.Fatal("nil request accepted")
	}
}
