package interaction

import (
	"context"
	"errors"
	"log"
	"strings"
)

var (
	ErrScopeBindingMissing    = errors.New("interaction scope binding is missing")
	ErrScopeBindingAmbiguous  = errors.New("interaction scope binding is ambiguous")
	ErrScopeBindingInactive   = errors.New("interaction scope binding is inactive")
	ErrScopeBindingConflict   = errors.New("interaction scope binding conflicts with request")
	ErrScopeConversationStale = errors.New("interaction scope conversation is stale")
)

type ScopeConfidence string

const (
	ScopeConfidenceExplicit        ScopeConfidence = "explicit"
	ScopeConfidenceBound           ScopeConfidence = "bound"
	ScopeConfidenceUnboundExplicit ScopeConfidence = "unbound_explicit"
)

type ScopeBindingState string

const (
	ScopeBindingStateActive  ScopeBindingState = "active"
	ScopeBindingStateDeleted ScopeBindingState = "deleted"
)

type ScopeResolveInput struct {
	UserID         string
	CharacterID    string
	ConversationID string
	Channel        string
	PeerID         string
	SessionID      string
	Source         string
	RequestID      string
}

type ScopeBinding struct {
	ID             string
	UserID         string
	CharacterID    string
	ConversationID string
	Channel        string
	PeerID         string
	Source         string
	State          ScopeBindingState
}

type ScopeResolution struct {
	Scope      InteractionScope
	BindingID  string
	Source     string
	Confidence ScopeConfidence
}

type ScopeBindingLookup interface {
	FindScopeBindings(ctx context.Context, channel, peerID string) ([]ScopeBinding, error)
}

type DefaultCharacterProvider interface {
	GetDefaultCharacterID(ctx context.Context) (string, error)
}

type ScopeResolver struct {
	lookup              ScopeBindingLookup
	defaultCharProvider DefaultCharacterProvider
}

func NewScopeResolver(lookup ScopeBindingLookup) ScopeResolver {
	return NewScopeResolverWithDefaultChar(lookup, nil)
}

func NewScopeResolverWithDefaultChar(lookup ScopeBindingLookup, defaultCharProvider DefaultCharacterProvider) ScopeResolver {
	return ScopeResolver{lookup: lookup, defaultCharProvider: defaultCharProvider}
}

func (r ScopeResolver) Resolve(ctx context.Context, input ScopeResolveInput) (ScopeResolution, error) {
	scope := InteractionScope{
		UserID:         input.UserID,
		CharacterID:    input.CharacterID,
		ConversationID: input.ConversationID,
		Channel:        input.Channel,
		PeerID:         input.PeerID,
		SessionID:      input.SessionID,
		Source:         input.Source,
		RequestID:      input.RequestID,
	}.Normalize()

	if scope.PeerID == "" {
		if err := scope.Validate(); err != nil {
			return ScopeResolution{}, err
		}
		return ScopeResolution{Scope: scope, Source: normalizeResolutionSource(scope.Source), Confidence: ScopeConfidenceExplicit}, nil
	}

	if scope.Channel == "" {
		return ScopeResolution{}, ErrScopeMissingChannel
	}
	if r.lookup == nil {
		if scope.CharacterID == "" && scope.ConversationID == "" {
			return ScopeResolution{}, ErrScopeBindingMissing
		}
		if err := scope.Validate(); err != nil {
			return ScopeResolution{}, err
		}
		scope = r.resolveDefaultCharacter(ctx, scope)
		return ScopeResolution{Scope: scope, Source: normalizeResolutionSource(scope.Source), Confidence: ScopeConfidenceUnboundExplicit}, nil
	}

	bindings, err := r.lookup.FindScopeBindings(ctx, scope.Channel, scope.PeerID)
	if err != nil {
		return ScopeResolution{}, err
	}
	if len(bindings) == 0 {
		if scope.CharacterID == "" && scope.ConversationID == "" {
			return ScopeResolution{}, ErrScopeBindingMissing
		}
		if err := scope.Validate(); err != nil {
			return ScopeResolution{}, err
		}
		scope = r.resolveDefaultCharacter(ctx, scope)
		return ScopeResolution{Scope: scope, Source: normalizeResolutionSource(scope.Source), Confidence: ScopeConfidenceUnboundExplicit}, nil
	}
	active := make([]ScopeBinding, 0, len(bindings))
	for _, binding := range bindings {
		normalized := normalizeBinding(binding)
		if normalized.State == ScopeBindingStateDeleted {
			continue
		}
		if normalized.State != "" && normalized.State != ScopeBindingStateActive {
			return ScopeResolution{}, ErrScopeBindingInactive
		}
		if normalized.Channel != scope.Channel || normalized.PeerID != scope.PeerID {
			return ScopeResolution{}, ErrScopeBindingConflict
		}
		active = append(active, normalized)
	}
	if len(active) == 0 {
		return ScopeResolution{}, ErrScopeBindingInactive
	}
	if len(active) > 1 {
		return ScopeResolution{}, ErrScopeBindingAmbiguous
	}

	binding := active[0]
	merged, err := mergeScopeBinding(scope, binding)
	if err != nil {
		return ScopeResolution{}, err
	}
	if err := merged.Validate(); err != nil {
		return ScopeResolution{}, err
	}
	return ScopeResolution{
		Scope:      merged,
		BindingID:  binding.ID,
		Source:     normalizeResolutionSource(merged.Source),
		Confidence: ScopeConfidenceBound,
	}, nil
}

func normalizeBinding(binding ScopeBinding) ScopeBinding {
	binding.ID = normalizeScopeValue(binding.ID)
	binding.UserID = normalizeScopeValue(binding.UserID)
	binding.CharacterID = normalizeScopeValue(binding.CharacterID)
	binding.ConversationID = normalizeScopeValue(binding.ConversationID)
	binding.Channel = strings.ToLower(normalizeScopeValue(binding.Channel))
	binding.PeerID = normalizeScopeValue(binding.PeerID)
	binding.Source = strings.ToLower(normalizeScopeValue(binding.Source))
	binding.State = ScopeBindingState(strings.ToLower(normalizeScopeValue(string(binding.State))))
	return binding
}

func mergeScopeBinding(scope InteractionScope, binding ScopeBinding) (InteractionScope, error) {
	if scope.UserID != "" && scope.UserID != DefaultUserID && binding.UserID != "" && scope.UserID != binding.UserID {
		return InteractionScope{}, ErrScopeBindingConflict
	}
	if scope.CharacterID != "" && binding.CharacterID != "" && scope.CharacterID != binding.CharacterID {
		return InteractionScope{}, ErrScopeBindingConflict
	}
	if scope.ConversationID != "" && binding.ConversationID != "" && scope.ConversationID != binding.ConversationID {
		return InteractionScope{}, ErrScopeBindingConflict
	}
	if scope.UserID == DefaultUserID && binding.UserID != "" {
		scope.UserID = binding.UserID
	}
	if scope.CharacterID == "" {
		scope.CharacterID = binding.CharacterID
	}
	if scope.ConversationID == "" {
		scope.ConversationID = binding.ConversationID
	}
	if scope.Source == "" {
		scope.Source = binding.Source
	}
	return scope.Normalize(), nil
}

func normalizeResolutionSource(source string) string {
	source = strings.ToLower(normalizeScopeValue(source))
	if source == "" {
		return "request"
	}
	return source
}

func (r ScopeResolver) resolveDefaultCharacter(ctx context.Context, scope InteractionScope) InteractionScope {
	if scope.CharacterID != "" || r.defaultCharProvider == nil {
		if r.defaultCharProvider == nil {
			log.Printf("[scope_resolver] defaultCharProvider is nil, skipping")
		}
		if scope.CharacterID != "" {
			log.Printf("[scope_resolver] CharacterID already set: %s", scope.CharacterID)
		}
		return scope
	}
	id, err := r.defaultCharProvider.GetDefaultCharacterID(ctx)
	log.Printf("[scope_resolver] resolved default character: id=%s err=%v", id, err)
	if err != nil {
		log.Printf("[scope_resolver] default character lookup failed: %v", err)
		return scope
	}
	scope.CharacterID = id
	return scope
}
