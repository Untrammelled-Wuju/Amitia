package interaction

import (
	"context"
	"github.com/u-ai/backend/internal/temporal"
	"sync"
	"testing"
	"time"
)

type canonicalTestExecutor struct {
	entry    *UnifiedEntry
	requests []*UnifiedEntryRequest
	request  *UnifiedEntryRequest
	scope    InteractionScope
	entered  chan struct{}
	release  chan struct{}
}

func (e *canonicalTestExecutor) ExecuteEntry(ctx context.Context, req *UnifiedEntryRequest) (*OrchestrationResult, error) {
	resolution, err := e.entry.ResolveScope(ctx, req)
	if err != nil {
		return nil, err
	}
	copied := *req
	copied.SpaceID, copied.CharacterID, copied.ConversationID = resolution.Scope.SpaceID, resolution.Scope.CharacterID, resolution.Scope.ConversationID
	copied.Source, copied.Channel, copied.PeerID = resolution.Source, resolution.Scope.Channel, resolution.Scope.PeerID
	e.request = &copied
	e.requests = append(e.requests, &copied)
	return &OrchestrationResult{Outcome: OutcomeCompleted, Response: &ProcessResponse{ConversationID: copied.ConversationID, CharacterID: copied.CharacterID, RequestID: copied.RequestID, Reply: "ok"}}, nil
}
func (e *canonicalTestExecutor) CancelEntryScope(scope InteractionScope) int {
	e.scope = scope
	if e.entered != nil {
		close(e.entered)
	}
	if e.release != nil {
		<-e.release
	}
	return 1
}
func (*canonicalTestExecutor) EntryInteractionActive(string) bool { return false }
func (*canonicalTestExecutor) EntryReady() bool                   { return true }
func TestUnifiedEntryCanonicalEnvelopeAndGeneratedIdentity(t *testing.T) {
	for _, id := range []string{"client-id", ""} {
		t.Run(id, func(t *testing.T) {
			entry := NewUnifiedEntry(NewRuntimeContextService(NewInMemoryTracker(), 16), NewScopeResolver(nil), temporal.SystemClock{})
			executor := &canonicalTestExecutor{entry: entry}
			entry.SetEntryExecutor(executor)
			result, err := entry.Handle(t.Context(), &UnifiedEntryRequest{SpaceID: "space", CharacterID: "role", ConversationID: "conversation", Channel: "web", Source: "web", PeerID: "peer", SessionID: "session", RequestID: id, Message: "hello"})
			if err != nil {
				t.Fatal(err)
			}
			if executor.request == nil || result.Response == nil || result.Response.RequestID == "" {
				t.Fatalf("identity lost: %+v", result)
			}
			if id != "" && result.Response.RequestID != id {
				t.Fatal("client id replaced")
			}
			if executor.request.SpaceID != "space" || executor.request.PeerID != "peer" || executor.request.SessionID != "session" {
				t.Fatalf("envelope lost: %+v", executor.request)
			}
		})
	}
}
func TestUnifiedEntryHasNoLegacyProcessorFallback(t *testing.T) {
	entry := NewUnifiedEntry(NewRuntimeContextService(NewInMemoryTracker(), 16), NewScopeResolver(nil), temporal.SystemClock{})
	if _, err := entry.Handle(t.Context(), &UnifiedEntryRequest{Channel: "web", Message: "hello"}); err != ErrOrchestratorNotReady {
		t.Fatalf("unexpected fallback: %v", err)
	}
}
func TestUnifiedEntryBackpressureConfigNormalizesExtremeValues(t *testing.T) {
	orch := NewRuntimeContextService(NewInMemoryTracker(), 16)
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil), temporal.SystemClock{})
	executor := &canonicalTestExecutor{entry: entry}
	entry.SetEntryExecutor(executor)
	entry.SetBackpressureConfig(BackpressureConfig{
		MaxQueueDepth: -1,
		WarningRatio:  2,
		SheddingRatio: -0.1,
		CooldownBase:  -time.Second,
		CooldownMax:   -time.Second,
	})

	result, err := entry.Handle(context.Background(), &UnifiedEntryRequest{
		Channel:        "web",
		Source:         "web",
		SpaceID:        "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Message:        "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeCompleted {
		t.Fatalf("unexpected outcome: %s", result.Outcome)
	}
	if status := entry.GetBackpressureStatus(); status != BackpressureNormal {
		t.Fatalf("unexpected backpressure status: %s", status)
	}
	if executor.requests == nil {
		t.Fatal("canonical executor was not invoked")
	}
}

func TestUnifiedEntryBackpressureRecoveryTimeoutCapsCooldown(t *testing.T) {
	state := &BackpressureState{QueueDepth: 10}
	cfg := BackpressureConfig{
		MaxQueueDepth:   10,
		WarningRatio:    0.5,
		SheddingRatio:   0.8,
		CooldownBase:    time.Second,
		CooldownMax:     time.Minute,
		RecoveryTimeout: 10 * time.Millisecond,
	}
	before := time.Now()
	duration := state.applyCooldownLocked(cfg, before)
	after := time.Now()
	if duration > cfg.RecoveryTimeout {
		t.Fatalf("cooldown was not capped by recovery timeout: got %s want <= %s", duration, cfg.RecoveryTimeout)
	}
	if state.CooldownUntil.Before(before) || state.CooldownUntil.After(after.Add(cfg.RecoveryTimeout+time.Millisecond)) {
		t.Fatalf("cooldown until was not bounded by recovery timeout: %s", state.CooldownUntil)
	}
}

func TestUnifiedEntryBackpressureConcurrentConfigAndHandle(t *testing.T) {
	orch := NewRuntimeContextService(NewInMemoryTracker(), 16)
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil), temporal.SystemClock{})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entry.SetBackpressureConfig(BackpressureConfig{
				MaxQueueDepth: 1 + i%5,
				WarningRatio:  0.2,
				SheddingRatio: 0.8,
				CooldownBase:  time.Millisecond,
				CooldownMax:   10 * time.Millisecond,
			})
		}(i)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = entry.Handle(context.Background(), &UnifiedEntryRequest{
				Channel:        "web",
				Source:         "web",
				SpaceID:        "user-1",
				CharacterID:    "char-1",
				ConversationID: "conv-concurrent",
				Message:        "hello",
				RequestID:      "req-concurrent",
			})
		}(i)
	}
	wg.Wait()
}

func TestUnifiedEntryCancelByPeerDoesNotHoldBackpressureLock(t *testing.T) {
	tracker := NewInMemoryTracker()
	orch := NewRuntimeContextService(tracker, 16)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil), temporal.SystemClock{})
	executor := &canonicalTestExecutor{
		entry:   entry,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	entry.SetEntryExecutor(executor)

	done := make(chan struct{})
	go func() {
		entry.CancelByPeer("web", "peer-1")
		close(done)
	}()

	select {
	case <-executor.entered:
	case <-time.After(time.Second):
		t.Fatal("cancel did not reach the canonical executor")
	}
	if executor.scope.Channel != "web" || executor.scope.PeerID != "peer-1" {
		t.Fatalf("cancel scope was not delegated: %#v", executor.scope)
	}

	configDone := make(chan struct{})
	go func() {
		entry.SetBackpressureConfig(BackpressureConfig{
			MaxQueueDepth: 10,
			WarningRatio:  0.2,
			SheddingRatio: 0.8,
			CooldownBase:  time.Millisecond,
			CooldownMax:   10 * time.Millisecond,
		})
		close(configDone)
	}()

	select {
	case <-configDone:
	case <-time.After(time.Second):
		t.Fatal("backpressure config was blocked by cancel")
	}

	close(executor.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancel did not finish")
	}
}

func TestInteractionRecordActiveCoversAllNonTerminalStates(t *testing.T) {
	activeStatuses := []InteractionStatus{
		InteractionStatusReceived,
		InteractionStatusNormalized,
		InteractionStatusQueued,
		InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided,
		InteractionStatusGenerated,
		InteractionStatusCommitted,
		InteractionStatusDeliveryPending,
		InteractionStatusDelivered,
	}
	for _, status := range activeStatuses {
		rec := NewInteractionRecord(InteractionScope{CharacterID: "char", ConversationID: "conv"})
		rec.Status = status
		if !rec.IsActive() {
			t.Fatalf("expected %s to be active", status)
		}
	}
	terminalStatuses := []InteractionStatus{
		InteractionStatusCompleted,
		InteractionStatusSuperseded,
		InteractionStatusCancelled,
		InteractionStatusFailed,
		InteractionStatusInterrupted,
		InteractionStatusArchived,
	}
	for _, status := range terminalStatuses {
		rec := NewInteractionRecord(InteractionScope{CharacterID: "char", ConversationID: "conv"})
		rec.Status = status
		if rec.IsActive() {
			t.Fatalf("expected %s to be terminal", status)
		}
	}
}
