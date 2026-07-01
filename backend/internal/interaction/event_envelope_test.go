package interaction

import (
	"context"
	"errors"
	"testing"
)

func TestEventEnvelopeNormalizeDefaultsVersionAndTrimsFields(t *testing.T) {
	envelope := EventEnvelope{
		EventID:        " event-1 ",
		EventType:      " message.created ",
		Source:         " USER ",
		Status:         " PENDING ",
		IdempotencyKey: " idem-1 ",
		Scope: InteractionScope{
			UserID:         " user-1 ",
			CharacterID:    " char-1 ",
			ConversationID: " conv-1 ",
			Channel:        " Web ",
		},
		Causation: EventCausation{
			CorrelationID: " corr-1 ",
			CausationID:   " cause-1 ",
			ParentEventID: " parent-1 ",
			Chain:         []string{" root-1 ", "", " parent-1 "},
		},
		StateVersion: 7,
	}

	normalized := envelope.Normalize()

	if normalized.Version != EventEnvelopeVersionV1 {
		t.Fatalf("expected default version, got %q", normalized.Version)
	}
	if normalized.EventID != "event-1" || normalized.EventType != "message.created" {
		t.Fatalf("expected trimmed event ids, got %#v", normalized)
	}
	if normalized.Source != EventSourceUser || normalized.Status != EventStatusPending {
		t.Fatalf("expected normalized source/status, got %#v", normalized)
	}
	if normalized.IdempotencyKey != "idem-1" {
		t.Fatalf("expected trimmed idempotency key, got %q", normalized.IdempotencyKey)
	}
	if normalized.Scope.UserID != "user-1" || normalized.Scope.CharacterID != "char-1" || normalized.Scope.Channel != "web" {
		t.Fatalf("expected normalized scope, got %#v", normalized.Scope)
	}
	if normalized.Causation.CorrelationID != "corr-1" || normalized.Causation.CausationID != "cause-1" || normalized.Causation.ParentEventID != "parent-1" {
		t.Fatalf("expected normalized causation ids, got %#v", normalized.Causation)
	}
	if len(normalized.Causation.Chain) != 2 || normalized.Causation.Chain[0] != "root-1" || normalized.Causation.Chain[1] != "parent-1" {
		t.Fatalf("expected compact causation chain, got %#v", normalized.Causation.Chain)
	}
	if envelope.EventID != " event-1 " {
		t.Fatalf("normalize must not mutate original envelope, got %q", envelope.EventID)
	}
}

func TestEventEnvelopeValidateRequiresCoreFields(t *testing.T) {
	base := EventEnvelope{
		EventType:      "message.created",
		Source:         EventSourceUser,
		Status:         EventStatusPending,
		IdempotencyKey: "idem-1",
		Scope:          InteractionScope{CharacterID: "char-1"},
	}

	cases := []struct {
		name     string
		mutate   func(EventEnvelope) EventEnvelope
		expected error
	}{
		{
			name: "missing event type",
			mutate: func(envelope EventEnvelope) EventEnvelope {
				envelope.EventType = " "
				return envelope
			},
			expected: ErrEventEnvelopeMissingType,
		},
		{
			name: "missing source",
			mutate: func(envelope EventEnvelope) EventEnvelope {
				envelope.Source = ""
				return envelope
			},
			expected: ErrEventEnvelopeMissingSource,
		},
		{
			name: "missing status",
			mutate: func(envelope EventEnvelope) EventEnvelope {
				envelope.Status = ""
				return envelope
			},
			expected: ErrEventEnvelopeMissingStatus,
		},
		{
			name: "missing idempotency key",
			mutate: func(envelope EventEnvelope) EventEnvelope {
				envelope.IdempotencyKey = " "
				return envelope
			},
			expected: ErrEventEnvelopeMissingIdempotencyKey,
		},
		{
			name: "missing scope target",
			mutate: func(envelope EventEnvelope) EventEnvelope {
				envelope.Scope = InteractionScope{}
				return envelope
			},
			expected: ErrScopeMissingTarget,
		},
	}

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			err := item.mutate(base).Validate()
			if !errors.Is(err, item.expected) {
				t.Fatalf("expected %v, got %v", item.expected, err)
			}
		})
	}

	if err := base.Validate(); err != nil {
		t.Fatalf("expected base envelope to validate, got %v", err)
	}
}

func TestBuildEventIdempotencyKeyIsStableAndTrimsParts(t *testing.T) {
	first := BuildEventIdempotencyKey(" conv-1 ", "message.created", "42")
	second := BuildEventIdempotencyKey("conv-1", "message.created", "42")
	changed := BuildEventIdempotencyKey("conv-1", "message.created", "43")

	if first == "" {
		t.Fatal("expected idempotency key")
	}
	if first != second {
		t.Fatalf("expected stable idempotency key, got %q and %q", first, second)
	}
	if first == changed {
		t.Fatalf("expected different input to change idempotency key, got %q", changed)
	}
	if len(first) != 64 {
		t.Fatalf("expected sha256 hex length, got %d", len(first))
	}
}

func TestEventEnvelopeWithContextStoresNormalizedScope(t *testing.T) {
	envelope := EventEnvelope{
		Scope: InteractionScope{
			UserID:         " user-1 ",
			CharacterID:    " char-1 ",
			ConversationID: " conv-1 ",
			Channel:        " QQ ",
			PeerID:         " peer-1 ",
			Source:         " API ",
			RequestID:      " req-1 ",
		},
	}

	stored, ok := FromContext(envelope.WithContext(context.Background()))
	if !ok {
		t.Fatal("expected scope to be stored in context")
	}
	if stored.UserID != "user-1" || stored.CharacterID != "char-1" || stored.ConversationID != "conv-1" {
		t.Fatalf("unexpected stored scope ids: %#v", stored)
	}
	if stored.Channel != "qq" || stored.Source != "api" || stored.PeerID != "peer-1" || stored.RequestID != "req-1" {
		t.Fatalf("unexpected stored scope metadata: %#v", stored)
	}
}
