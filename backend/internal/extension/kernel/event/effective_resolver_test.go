package event

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/glebarez/sqlite"
)

type mockGenerationResolver struct {
	gen int64
	err error
}

func (m *mockGenerationResolver) CurrentGeneration(_ context.Context, _ string) (int64, error) {
	return m.gen, m.err
}

func newFailClosedTestResolver(genResolver GenerationResolver) *DefaultEffectiveResolver {
	return NewDefaultEffectiveResolver(
		&mockPermissionChecker{granted: true},
		&mockScopeChecker{valid: true},
		&mockDependencyChecker{ready: true},
		&mockRuntimeChecker{available: true},
		&mockCircuitLookup{},
		genResolver,
	)
}

func TestResolveForDelivery_ProducerGenerationCheckError_FailClosed(t *testing.T) {
	resolver := newFailClosedTestResolver(&mockGenerationResolver{err: errors.New("db unavailable")})
	def := EventSubscriptionDefinition{Enabled: true, Generation: 0}
	envelope := EventEnvelope{
		ProducerID:         "ext-producer",
		ProducerType:       "extension",
		ProducerGeneration: 1,
	}
	state := resolver.ResolveForDelivery(context.Background(), def, envelope)
	if state.IsActive() {
		t.Fatalf("expected inactive on producer generation check error, got active")
	}
	if state.Reason != "generation_check_error" {
		t.Fatalf("expected generation_check_error, got %s", state.Reason)
	}
}

func TestResolveForDelivery_SubscriberGenerationCheckError_FailClosed(t *testing.T) {
	resolver := newFailClosedTestResolver(&mockGenerationResolver{err: errors.New("db unavailable")})
	def := EventSubscriptionDefinition{Enabled: true, Generation: 1}
	envelope := EventEnvelope{
		ProducerID:         "host",
		ProducerType:       "host",
		ProducerGeneration: 0,
	}
	state := resolver.ResolveForDelivery(context.Background(), def, envelope)
	if state.IsActive() {
		t.Fatalf("expected inactive on subscriber generation check error, got active")
	}
	if state.Reason != "generation_check_error" {
		t.Fatalf("expected generation_check_error, got %s", state.Reason)
	}
}

func TestResolveForDelivery_StaleProducer_Rejected(t *testing.T) {
	resolver := newFailClosedTestResolver(&mockGenerationResolver{gen: 5})
	def := EventSubscriptionDefinition{Enabled: true, Generation: 0}
	envelope := EventEnvelope{
		ProducerID:         "ext-producer",
		ProducerType:       "extension",
		ProducerGeneration: 1,
	}
	state := resolver.ResolveForDelivery(context.Background(), def, envelope)
	if state.IsActive() {
		t.Fatalf("expected inactive on stale producer, got active")
	}
	if state.Reason != "reject_stale_producer" {
		t.Fatalf("expected reject_stale_producer, got %s", state.Reason)
	}
}

func TestResolveForDelivery_StaleSubscription_Cancelled(t *testing.T) {
	resolver := newFailClosedTestResolver(&mockGenerationResolver{gen: 5})
	def := EventSubscriptionDefinition{Enabled: true, Generation: 1}
	envelope := EventEnvelope{
		ProducerID:         "host",
		ProducerType:       "host",
		ProducerGeneration: 0,
	}
	state := resolver.ResolveForDelivery(context.Background(), def, envelope)
	if state.IsActive() {
		t.Fatalf("expected inactive on stale subscription, got active")
	}
	if state.Reason != "cancel_stale_subscription" {
		t.Fatalf("expected cancel_stale_subscription, got %s", state.Reason)
	}
}

func TestResolveForDelivery_GenerationOK_Active(t *testing.T) {
	resolver := newFailClosedTestResolver(&mockGenerationResolver{gen: 1})
	def := EventSubscriptionDefinition{Enabled: true, Generation: 1}
	envelope := EventEnvelope{
		ProducerID:         "ext-producer",
		ProducerType:       "extension",
		ProducerGeneration: 1,
	}
	state := resolver.ResolveForDelivery(context.Background(), def, envelope)
	if !state.IsActive() {
		t.Fatalf("expected active when generation matches, got inactive: %s", state.Reason)
	}
}

func TestSetEffectiveResolver_RejectsNoop(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	svc, err := NewService(DefaultServiceConfig().WithDB(db))
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	if err := svc.SetEffectiveResolver(NewNoopEffectiveResolver()); err == nil {
		t.Fatalf("expected error when setting NoopEffectiveResolver")
	}
}

func TestSetEffectiveResolver_AcceptsDefault(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	svc, err := NewService(DefaultServiceConfig().WithDB(db))
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	resolver := NewDefaultEffectiveResolver(
		&mockPermissionChecker{granted: true},
		&mockScopeChecker{valid: true},
		&mockDependencyChecker{ready: true},
		&mockRuntimeChecker{available: true},
		&mockCircuitLookup{},
		&mockGenerationResolver{gen: 1},
	)
	if err := svc.SetEffectiveResolver(resolver); err != nil {
		t.Fatalf("expected no error for DefaultEffectiveResolver, got %v", err)
	}
}
