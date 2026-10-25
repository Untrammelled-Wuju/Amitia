package kernel

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type HostAPIScopeContext = host_api.ScopeContext

func WithHostAPIScope(ctx context.Context, sc HostAPIScopeContext) context.Context {
	return host_api.WithScopeContext(ctx, sc)
}

func GetHostAPIScope(ctx context.Context) HostAPIScopeContext {
	return host_api.GetScopeContext(ctx)
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
		SpaceID:        snap.SpaceID,
		CharacterID:    snap.CharacterID,
		ConversationID: snap.ConversationID,
	})
}
