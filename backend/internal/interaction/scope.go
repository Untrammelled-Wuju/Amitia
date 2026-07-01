package interaction

import (
	"context"
	"errors"
	"strings"
)

const DefaultUserID = "default"

var (
	ErrScopeMissingTarget  = errors.New("interaction scope requires character_id or conversation_id")
	ErrScopeMissingChannel = errors.New("interaction scope requires channel when peer_id is present")
)

type InteractionScope struct {
	UserID         string `json:"userId,omitempty"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	Channel        string `json:"channel,omitempty"`
	PeerID         string `json:"peerId,omitempty"`
	SessionID      string `json:"sessionId,omitempty"`
	Source         string `json:"source,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
}

type interactionScopeContextKey struct{}

func (s InteractionScope) Normalize() InteractionScope {
	s.UserID = normalizeScopeValue(s.UserID)
	s.CharacterID = normalizeScopeValue(s.CharacterID)
	s.ConversationID = normalizeScopeValue(s.ConversationID)
	s.Channel = strings.ToLower(normalizeScopeValue(s.Channel))
	s.PeerID = normalizeScopeValue(s.PeerID)
	s.SessionID = normalizeScopeValue(s.SessionID)
	s.Source = strings.ToLower(normalizeScopeValue(s.Source))
	s.RequestID = normalizeScopeValue(s.RequestID)
	if s.UserID == "" {
		s.UserID = DefaultUserID
	}
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
