package kernel

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type hostAPIScopeContextKey struct{}

type HostAPIScopeContext struct {
	CharacterID    string
	ConversationID string
}

func WithHostAPIScope(ctx context.Context, sc HostAPIScopeContext) context.Context {
	return context.WithValue(ctx, hostAPIScopeContextKey{}, sc)
}

func GetHostAPIScope(ctx context.Context) HostAPIScopeContext {
	sc, _ := ctx.Value(hostAPIScopeContextKey{}).(HostAPIScopeContext)
	return sc
}

func resolveHostAPIScope(ctx context.Context, store host_api.ScopeSnapshotStore, snapshotID string) context.Context {
	if store == nil || snapshotID == "" {
		return ctx
	}
	snap, err := store.Get(ctx, snapshotID)
	if err != nil || snap == nil {
		return ctx
	}
	return WithHostAPIScope(ctx, HostAPIScopeContext{
		CharacterID:    snap.CharacterID,
		ConversationID: snap.ConversationID,
	})
}
