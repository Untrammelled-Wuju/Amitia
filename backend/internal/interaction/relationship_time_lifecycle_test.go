package interaction

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/temporal"
)

type relationshipTimeCoordinatorStub struct {
	mu           sync.Mutex
	prepareCalls []temporal.PrepareInboundInput
	releaseCalls []relationshipTimeReleaseCall
	releaseErrs  []error
	prepareErr   error
	releaseErr   error
	claim        bool
}

type relationshipTimeReleaseCall struct {
	InteractionID string
	Reason        string
}

func (s *relationshipTimeCoordinatorStub) PrepareInbound(_ context.Context, input temporal.PrepareInboundInput) (temporal.RelationshipTimeContext, error) {
	s.mu.Lock()
	s.prepareCalls = append(s.prepareCalls, input)
	s.mu.Unlock()
	if s.prepareErr != nil {
		return temporal.RelationshipTimeContext{}, s.prepareErr
	}
	result := temporal.RelationshipTimeContext{
		Version:     temporal.RelationshipTimeVersion,
		UserID:      input.UserID,
		CharacterID: input.CharacterID,
		NowUTC:      input.ObservedAt,
	}
	if s.claim {
		result.Reunion = &temporal.ReunionContext{
			EpisodeID:              "episode-1",
			State:                  temporal.ReunionStateClaimed,
			ClaimedByInteractionID: input.InteractionID,
			ClaimExpiresAt:         input.ObservedAt.Add(temporal.ReunionClaimTTL),
		}
	}
	return result, nil
}

func (s *relationshipTimeCoordinatorStub) ReleaseClaim(ctx context.Context, interactionID string, reason string) error {
	s.mu.Lock()
	s.releaseCalls = append(s.releaseCalls, relationshipTimeReleaseCall{InteractionID: interactionID, Reason: reason})
	s.releaseErrs = append(s.releaseErrs, ctx.Err())
	s.mu.Unlock()
	return s.releaseErr
}

func (s *relationshipTimeCoordinatorStub) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.prepareCalls), len(s.releaseCalls)
}

type relationshipTimeErrorProcessor struct{ err error }

func (p *relationshipTimeErrorProcessor) ProcessMessageCtx(context.Context, *ProcessRequest) (*ProcessResponse, error) {
	return nil, p.err
}

type relationshipTimeBlockingProcessor struct{ started chan struct{} }

func (p *relationshipTimeBlockingProcessor) ProcessMessageCtx(ctx context.Context, _ *ProcessRequest) (*ProcessResponse, error) {
	close(p.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRelationshipTimePrepareRunsOnceForIdempotentRequest(t *testing.T) {
	coordinator := &relationshipTimeCoordinatorStub{claim: true}
	orch := NewOrchestrator(DefaultOrchestratorConfig(), &stubMessageProcessor{prefix: "ok-"})
	orch.SetRelationshipTimeCoordinator(coordinator)
	orch.SetReady(true)
	req := &ProcessRequest{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Channel:        "web",
		PeerID:         "peer-1",
		Source:         "web",
		RequestID:      "request-1",
		Message:        "hello",
	}
	first, err := orch.Process(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := orch.Process(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.InteractionID != second.InteractionID {
		t.Fatalf("idempotent request created different interactions: %s %s", first.InteractionID, second.InteractionID)
	}
	prepareCount, releaseCount := coordinator.counts()
	if prepareCount != 1 || releaseCount != 0 {
		t.Fatalf("unexpected lifecycle calls prepare=%d release=%d", prepareCount, releaseCount)
	}
	input := coordinator.prepareCalls[0]
	if input.UserID != "user-1" || input.CharacterID != "char-1" || input.InteractionID != first.InteractionID || input.RequestID != "request-1" {
		t.Fatalf("unexpected prepare input: %#v", input)
	}
	if input.ObservedAt.IsZero() || input.ObservedAt.Location() != time.UTC {
		t.Fatalf("observed time must be UTC: %v", input.ObservedAt)
	}
}

func TestRelationshipTimeClaimReleasedOnProcessorFailure(t *testing.T) {
	coordinator := &relationshipTimeCoordinatorStub{claim: true}
	orch := NewOrchestrator(DefaultOrchestratorConfig(), &relationshipTimeErrorProcessor{err: errors.New("generation failed")})
	orch.SetRelationshipTimeCoordinator(coordinator)
	orch.SetReady(true)
	result, err := orch.Process(context.Background(), &ProcessRequest{UserID: "user-1", CharacterID: "char-1", RequestID: "request-fail", Message: "hello"})
	if err == nil || result == nil {
		t.Fatalf("expected failed result: result=%#v err=%v", result, err)
	}
	prepareCount, releaseCount := coordinator.counts()
	if prepareCount != 1 || releaseCount != 1 {
		t.Fatalf("unexpected lifecycle calls prepare=%d release=%d", prepareCount, releaseCount)
	}
	if coordinator.releaseCalls[0].InteractionID != result.InteractionID || coordinator.releaseCalls[0].Reason != "processor_failed" {
		t.Fatalf("unexpected release: %#v", coordinator.releaseCalls[0])
	}
}

func TestRelationshipTimeClaimReleasedOnDeadline(t *testing.T) {
	coordinator := &relationshipTimeCoordinatorStub{claim: true}
	cfg := DefaultOrchestratorConfig()
	cfg.DefaultTimeout = 20 * time.Millisecond
	orch := NewOrchestrator(cfg, &stubMessageProcessor{prefix: "late-", delay: time.Second})
	orch.SetRelationshipTimeCoordinator(coordinator)
	orch.SetReady(true)
	result, err := orch.Process(context.Background(), &ProcessRequest{UserID: "user-1", CharacterID: "char-1", RequestID: "request-timeout", Message: "hello"})
	if !errors.Is(err, context.DeadlineExceeded) || result == nil || result.Outcome != OutcomeCancelled {
		t.Fatalf("expected deadline cancellation: result=%#v err=%v", result, err)
	}
	_, releaseCount := coordinator.counts()
	if releaseCount != 1 || coordinator.releaseCalls[0].Reason != "processor_cancelled" {
		t.Fatalf("unexpected release calls: %#v", coordinator.releaseCalls)
	}
}

func TestRelationshipTimeClaimReleaseSurvivesCallerCancellation(t *testing.T) {
	coordinator := &relationshipTimeCoordinatorStub{claim: true}
	processor := &relationshipTimeBlockingProcessor{started: make(chan struct{})}
	orch := NewOrchestrator(DefaultOrchestratorConfig(), processor)
	orch.SetRelationshipTimeCoordinator(coordinator)
	orch.SetReady(true)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := orch.Process(ctx, &ProcessRequest{UserID: "user-1", CharacterID: "char-1", RequestID: "request-cancelled", Message: "hello"})
		done <- err
	}()
	select {
	case <-processor.started:
	case <-time.After(time.Second):
		t.Fatal("processor did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("process did not stop")
	}
	_, releaseCount := coordinator.counts()
	if releaseCount != 1 || coordinator.releaseErrs[0] != nil {
		t.Fatalf("release inherited cancelled context: calls=%#v errors=%#v", coordinator.releaseCalls, coordinator.releaseErrs)
	}
}

func TestRelationshipTimePrepareFailureDoesNotBlockChat(t *testing.T) {
	coordinator := &relationshipTimeCoordinatorStub{prepareErr: errors.New("repository unavailable")}
	orch := NewOrchestrator(DefaultOrchestratorConfig(), &stubMessageProcessor{prefix: "ok-"})
	orch.SetRelationshipTimeCoordinator(coordinator)
	orch.SetReady(true)
	result, err := orch.Process(context.Background(), &ProcessRequest{UserID: "user-1", CharacterID: "char-1", RequestID: "request-prepare-fail", Message: "hello"})
	if err != nil || result == nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("prepare failure blocked chat: result=%#v err=%v", result, err)
	}
	prepareCount, releaseCount := coordinator.counts()
	if prepareCount != 1 || releaseCount != 0 {
		t.Fatalf("unexpected lifecycle calls prepare=%d release=%d", prepareCount, releaseCount)
	}
}

func TestRelationshipTimeUnclaimedContextIsNeverReleased(t *testing.T) {
	coordinator := &relationshipTimeCoordinatorStub{}
	orch := NewOrchestrator(DefaultOrchestratorConfig(), &relationshipTimeErrorProcessor{err: errors.New("generation failed")})
	orch.SetRelationshipTimeCoordinator(coordinator)
	orch.SetReady(true)
	_, _ = orch.Process(context.Background(), &ProcessRequest{UserID: "user-1", CharacterID: "char-1", RequestID: "request-unclaimed", Message: "hello"})
	prepareCount, releaseCount := coordinator.counts()
	if prepareCount != 1 || releaseCount != 0 {
		t.Fatalf("unexpected lifecycle calls prepare=%d release=%d", prepareCount, releaseCount)
	}
}
