package host_api

import "context"

type scopeContextKey struct{}

type ScopeContext struct {
	SpaceID        string
	CharacterID    string
	ConversationID string
}

func WithScopeContext(ctx context.Context, scope ScopeContext) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

func GetScopeContext(ctx context.Context) ScopeContext {
	scope, _ := ctx.Value(scopeContextKey{}).(ScopeContext)
	return scope
}
