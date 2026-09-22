package interaction

import (
	"context"
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/requestidentity"
)

var (
	ErrScopeMissingTarget  = errors.New("interaction scope requires character_id or conversation_id")
	ErrScopeMissingChannel = errors.New("interaction scope requires channel when peer_id is present")
)

type InteractionScope struct {
	SpaceID        string `json:"spaceId,omitempty"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	ThreadID       string `json:"threadId,omitempty"`
	Channel        string `json:"channel,omitempty"`
	PeerID         string `json:"peerId,omitempty"`
	SessionID      string `json:"sessionId,omitempty"`
	Source         string `json:"source,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
}

type interactionScopeContextKey struct{}

func (s InteractionScope) Normalize() InteractionScope {
	s.SpaceID = normalizeScopeValue(s.SpaceID)
	s.CharacterID = normalizeScopeValue(s.CharacterID)
	s.ConversationID = normalizeScopeValue(s.ConversationID)
	s.ThreadID = normalizeScopeValue(s.ThreadID)
	s.Channel = strings.ToLower(normalizeScopeValue(s.Channel))
	s.PeerID = normalizeScopeValue(s.PeerID)
	s.SessionID = normalizeScopeValue(s.SessionID)
	s.Source = strings.ToLower(normalizeScopeValue(s.Source))
	s.RequestID = normalizeScopeValue(s.RequestID)
	s.SpaceID = requestidentity.NormalizeSpaceID(s.SpaceID)
	return s
}

func (s InteractionScope) Validate() error {
	s = s.Normalize()
	if s.CharacterID == "" && s.ConversationID == "" {
		return ErrScopeMissingTarget
	}
	if s.PeerID != "" && s.Channel == "" {
		return ErrScopeMissingChannel
	}
	return nil
}

func (s InteractionScope) WithContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, interactionScopeContextKey{}, s.Normalize())
}

func FromContext(ctx context.Context) (InteractionScope, bool) {
	if ctx == nil {
		return InteractionScope{}, false
	}
	scope, ok := ctx.Value(interactionScopeContextKey{}).(InteractionScope)
	if !ok {
		return InteractionScope{}, false
	}
	return scope.Normalize(), true
}

func normalizeScopeValue(value string) string {
	return strings.TrimSpace(value)
}
