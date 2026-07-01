package interaction

import (
	"context"
	"errors"
	"testing"
)

type fakeScopeBindingLookup struct {
	bindings []ScopeBinding
	err      error
}

func (f fakeScopeBindingLookup) FindScopeBindings(ctx context.Context, channel, peerID string) ([]ScopeBinding, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.bindings, nil
}

func TestScopeResolverResolvesBoundPeer(t *testing.T) {
	resolver := NewScopeResolver(fakeScopeBindingLookup{bindings: []ScopeBinding{
		{
			ID:             " bind-1 ",
			UserID:         " user-1 ",
			CharacterID:    " char-1 ",
			ConversationID: " conv-1 ",
			Channel:        " QQ ",
			PeerID:         " peer-1 ",
			Source:         " qq ",
			State:          ScopeBindingStateActive,
		},
	}})

	result, err := resolver.Resolve(context.Background(), ScopeResolveInput{
		Channel:   " qq ",
		PeerID:    " peer-1 ",
		RequestID: " req-1 ",
	})
	if err != nil {
		t.Fatalf("expected scope to resolve, got %v", err)
	}
	if result.Scope.UserID != "user-1" || result.Scope.CharacterID != "char-1" || result.Scope.ConversationID != "conv-1" {
		t.Fatalf("unexpected resolved scope: %#v", result.Scope)
	}
	if result.Scope.Channel != "qq" || result.Scope.PeerID != "peer-1" || result.BindingID != "bind-1" {
		t.Fatalf("unexpected binding result: %#v", result)
	}
	if result.Confidence != ScopeConfidenceBound || result.Source != "qq" {
		t.Fatalf("unexpected resolution metadata: %#v", result)
	}
}

func TestScopeResolverRejectsAmbiguousBindings(t *testing.T) {
	resolver := NewScopeResolver(fakeScopeBindingLookup{bindings: []ScopeBinding{
		{ID: "bind-1", UserID: "user-1", CharacterID: "char-1", Channel: "qq", PeerID: "peer-1", State: ScopeBindingStateActive},
		{ID: "bind-2", UserID: "user-1", CharacterID: "char-2", Channel: "qq", PeerID: "peer-1", State: ScopeBindingStateActive},
	}})

	_, err := resolver.Resolve(context.Background(), ScopeResolveInput{Channel: "qq", PeerID: "peer-1"})
	if !errors.Is(err, ErrScopeBindingAmbiguous) {
		t.Fatalf("expected ErrScopeBindingAmbiguous, got %v", err)
	}
}

func TestScopeResolverRejectsMissingBindingWithoutExplicitTarget(t *testing.T) {
	resolver := NewScopeResolver(fakeScopeBindingLookup{})

	_, err := resolver.Resolve(context.Background(), ScopeResolveInput{Channel: "wechat", PeerID: "peer-1"})
	if !errors.Is(err, ErrScopeBindingMissing) {
		t.Fatalf("expected ErrScopeBindingMissing, got %v", err)
	}
}

func TestScopeResolverAllowsUnboundPeerWithExplicitTarget(t *testing.T) {
	resolver := NewScopeResolver(fakeScopeBindingLookup{})

	result, err := resolver.Resolve(context.Background(), ScopeResolveInput{
		CharacterID: "char-1",
		Channel:     "wechat",
		PeerID:      "peer-1",
		Source:      "wechat",
	})
	if err != nil {
		t.Fatalf("expected explicit target to resolve, got %v", err)
	}
	if result.Confidence != ScopeConfidenceUnboundExplicit {
		t.Fatalf("expected unbound explicit confidence, got %#v", result)
	}
	if result.Scope.CharacterID != "char-1" || result.Scope.Channel != "wechat" {
		t.Fatalf("unexpected scope: %#v", result.Scope)
	}
}

func TestScopeResolverRejectsBindingConflict(t *testing.T) {
	resolver := NewScopeResolver(fakeScopeBindingLookup{bindings: []ScopeBinding{
		{ID: "bind-1", UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1", Channel: "qq", PeerID: "peer-1", State: ScopeBindingStateActive},
	}})

	_, err := resolver.Resolve(context.Background(), ScopeResolveInput{
		CharacterID: "char-2",
		Channel:     "qq",
		PeerID:      "peer-1",
	})
	if !errors.Is(err, ErrScopeBindingConflict) {
		t.Fatalf("expected ErrScopeBindingConflict, got %v", err)
	}
}

func TestScopeResolverSkipsDeletedBindingsButRejectsIfNoActiveBinding(t *testing.T) {
	resolver := NewScopeResolver(fakeScopeBindingLookup{bindings: []ScopeBinding{
		{ID: "bind-1", UserID: "user-1", CharacterID: "char-1", Channel: "qq", PeerID: "peer-1", State: ScopeBindingStateDeleted},
	}})

	_, err := resolver.Resolve(context.Background(), ScopeResolveInput{Channel: "qq", PeerID: "peer-1"})
	if !errors.Is(err, ErrScopeBindingInactive) {
		t.Fatalf("expected ErrScopeBindingInactive, got %v", err)
	}
}

func TestScopeResolverRejectsPeerWithoutChannelBeforeLookup(t *testing.T) {
	resolver := NewScopeResolver(fakeScopeBindingLookup{bindings: []ScopeBinding{
		{ID: "bind-1", UserID: "user-1", CharacterID: "char-1", Channel: "qq", PeerID: "peer-1", State: ScopeBindingStateActive},
	}})

	_, err := resolver.Resolve(context.Background(), ScopeResolveInput{PeerID: "peer-1"})
	if !errors.Is(err, ErrScopeMissingChannel) {
		t.Fatalf("expected ErrScopeMissingChannel, got %v", err)
	}
}
