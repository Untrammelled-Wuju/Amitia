package interaction

import (
	"context"
	"errors"
	"testing"
)

func TestInteractionScopeNormalizeAssignsDefaultUserOnlyWhenMissing(t *testing.T) {
	scope := InteractionScope{
		CharacterID:    " char-1 ",
		ConversationID: " conv-1 ",
		Channel:        " WeB ",
		PeerID:         " peer-1 ",
		SessionID:      " sess-1 ",
		Source:         " HTTP ",
		RequestID:      " req-1 ",
	}

	normalized := scope.Normalize()

	if normalized.UserID != DefaultUserID {
		t.Fatalf("expected default user id, got %q", normalized.UserID)
	}
	if normalized.CharacterID != "char-1" || normalized.ConversationID != "conv-1" {
		t.Fatalf("expected trimmed ids, got %#v", normalized)
	}
	if normalized.Channel != "web" || normalized.Source != "http" {
		t.Fatalf("expected normalized channel/source, got %#v", normalized)
	}
	if normalized.PeerID != "peer-1" || normalized.SessionID != "sess-1" || normalized.RequestID != "req-1" {
		t.Fatalf("expected trimmed peer/session/request ids, got %#v", normalized)
	}
	if scope.UserID != "" {
		t.Fatalf("normalize must not mutate original scope, got %q", scope.UserID)
	}
}

func TestInteractionScopeNormalizeKeepsExplicitUserID(t *testing.T) {
	scope := InteractionScope{
		UserID:         " user-1 ",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
	}

	normalized := scope.Normalize()

	if normalized.UserID != "user-1" {
		t.Fatalf("expected explicit user id to be preserved, got %q", normalized.UserID)
	}
}

func TestInteractionScopeValidateRejectsMissingTarget(t *testing.T) {
	scope := InteractionScope{
		UserID:  "user-1",
		Channel: "web",
	}

	err := scope.Validate()
	if !errors.Is(err, ErrScopeMissingTarget) {
		t.Fatalf("expected ErrScopeMissingTarget, got %v", err)
	}
}

func TestInteractionScopeValidateRejectsPeerWithoutChannel(t *testing.T) {
	scope := InteractionScope{
		UserID:      "user-1",
		CharacterID: "char-1",
		PeerID:      "peer-1",
	}

	err := scope.Validate()
	if !errors.Is(err, ErrScopeMissingChannel) {
		t.Fatalf("expected ErrScopeMissingChannel, got %v", err)
	}
}

func TestInteractionScopeValidateAcceptsCharacterOrConversationBoundary(t *testing.T) {
	cases := []InteractionScope{
		{CharacterID: "char-1"},
		{ConversationID: "conv-1"},
		{CharacterID: "char-1", ConversationID: "conv-1", Channel: "web", PeerID: "peer-1"},
	}

	for _, scope := range cases {
		if err := scope.Validate(); err != nil {
			t.Fatalf("expected scope to validate, scope=%#v err=%v", scope, err)
		}
	}
}

func TestInteractionScopeContextRoundTrip(t *testing.T) {
	scope := InteractionScope{
		UserID:         " user-1 ",
		CharacterID:    " char-1 ",
		ConversationID: " conv-1 ",
		Channel:        " Web ",
		Source:         " API ",
		RequestID:      " req-1 ",
	}

	ctx := scope.WithContext(context.Background())
	stored, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected scope in context")
	}
	if stored.UserID != "user-1" || stored.CharacterID != "char-1" || stored.ConversationID != "conv-1" {
		t.Fatalf("unexpected scope from context: %#v", stored)
	}
	if stored.Channel != "web" || stored.Source != "api" || stored.RequestID != "req-1" {
		t.Fatalf("unexpected normalized values from context: %#v", stored)
	}
}

func TestInteractionScopeFromContextHandlesNilAndMissingValues(t *testing.T) {
	if scope, ok := FromContext(nil); ok || scope != (InteractionScope{}) {
		t.Fatalf("expected nil context to return empty scope, got %#v ok=%v", scope, ok)
	}

	if scope, ok := FromContext(context.Background()); ok || scope != (InteractionScope{}) {
		t.Fatalf("expected missing context value to return empty scope, got %#v ok=%v", scope, ok)
	}
}
