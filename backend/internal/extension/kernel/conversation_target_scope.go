package kernel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

const conversationTargetSnapshotPrefix = "conversation-target-"

type ScopeSnapshotWriter interface {
	SaveSnapshot(ctx context.Context, snapshot scope.ScopeSnapshot) error
}

type ScopeSnapshotDeleter interface {
	DeleteSnapshot(ctx context.Context, snapshotID string) error
}

func createConversationTargetSnapshot(ctx context.Context, deps HostAPIRouteDeps, req host_api.CallRequest, conversationID string) (string, error) {
	if deps.ScopeSnapshotWriter == nil {
		return "", fmt.Errorf("conversation target snapshot writer not configured")
	}
	hostScope := GetHostAPIScope(ctx)
	conversationID = strings.TrimSpace(conversationID)
	if hostScope.SpaceID == "" || hostScope.CharacterID == "" || conversationID == "" {
		return "", fmt.Errorf("conversation target snapshot requires space, character and conversation")
	}
	extensionID := string(req.RuntimeIdentity.ExtensionID)
	moduleID := string(req.RuntimeIdentity.ModuleID)
	snapshot := scope.CreateSnapshotWithOwner(
		req.InvocationID,
		[]scope.ScopeRef{
			scope.NewExtensionScope(extensionID),
			scope.NewModuleScope(extensionID, moduleID),
			scope.NewCharacterScope(hostScope.CharacterID),
			scope.NewConversationScope(conversationID),
		},
		hostScope.SpaceID,
		hostScope.CharacterID,
		conversationID,
		extensionID,
		moduleID,
		req.RuntimeIdentity.Generation,
	)
	snapshot.SnapshotID = conversationTargetSnapshotPrefix + uuid.NewString()
	if err := deps.ScopeSnapshotWriter.SaveSnapshot(ctx, snapshot); err != nil {
		return "", fmt.Errorf("save conversation target snapshot: %w", err)
	}
	return snapshot.SnapshotID, nil
}

func resolveConversationTargetScope(ctx context.Context, deps HostAPIRouteDeps, req host_api.CallRequest, conversationID string) (HostAPIScopeContext, error) {
	hostScope := GetHostAPIScope(ctx)
	targetID := strings.TrimSpace(req.TargetScopeSnapshotID)
	if targetID == "" {
		return hostScope, nil
	}
	if deps.ScopeSnapshotStore == nil {
		return HostAPIScopeContext{}, fmt.Errorf("conversation target snapshot store not configured")
	}
	snapshot, err := deps.ScopeSnapshotStore.Get(ctx, targetID)
	if err != nil {
		return HostAPIScopeContext{}, fmt.Errorf("load conversation target snapshot: %w", err)
	}
	if snapshot == nil || !strings.HasPrefix(snapshot.SnapshotID, conversationTargetSnapshotPrefix) {
		return HostAPIScopeContext{}, fmt.Errorf("conversation target snapshot is invalid")
	}
	if snapshot.ExpiresAt != nil && time.Now().After(*snapshot.ExpiresAt) {
		return HostAPIScopeContext{}, fmt.Errorf("conversation target snapshot expired")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || snapshot.ConversationID != conversationID {
		return HostAPIScopeContext{}, fmt.Errorf("conversation target snapshot does not match conversation")
	}
	if snapshot.SpaceID != hostScope.SpaceID || snapshot.CharacterID != hostScope.CharacterID {
		return HostAPIScopeContext{}, fmt.Errorf("conversation target snapshot owner does not match current scope")
	}
	if snapshot.ExtensionID != string(req.RuntimeIdentity.ExtensionID) || snapshot.ModuleID != string(req.RuntimeIdentity.ModuleID) {
		return HostAPIScopeContext{}, fmt.Errorf("conversation target snapshot extension does not match caller")
	}
	if req.RuntimeIdentity.Generation > 0 && snapshot.Generation != req.RuntimeIdentity.Generation {
		return HostAPIScopeContext{}, fmt.Errorf("conversation target snapshot generation does not match caller")
	}
	hostScope.ConversationID = conversationID
	return hostScope, nil
}

func deleteConversationTargetSnapshot(ctx context.Context, deps HostAPIRouteDeps, req host_api.CallRequest) error {
	targetID := strings.TrimSpace(req.TargetScopeSnapshotID)
	if targetID == "" || deps.ScopeSnapshotDeleter == nil {
		return nil
	}
	return deps.ScopeSnapshotDeleter.DeleteSnapshot(ctx, targetID)
}
