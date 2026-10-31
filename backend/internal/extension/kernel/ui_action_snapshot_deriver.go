package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	amitialog "log"
)

func newUIActionSnapshotDeriver(scopeStore scope.ScopeStore, permissionStore permission.PermissionSnapshotStore, validator *permission.PermissionIDValidator) UIActionSnapshotDeriver {
	return func(ctx context.Context, scopeSnapshotID, permissionSnapshotID, targetExtensionID, targetModuleID string) (string, string, func(), error) {
		if scopeStore == nil || permissionStore == nil {
			return "", "", nil, fmt.Errorf("execution snapshot stores unavailable")
		}
		if scopeSnapshotID == "" || permissionSnapshotID == "" || targetExtensionID == "" || targetModuleID == "" {
			return "", "", nil, fmt.Errorf("execution snapshot identity incomplete")
		}
		now := time.Now().UTC()
		sourceScope, err := scopeStore.GetSnapshot(ctx, scopeSnapshotID)
		if err != nil {
			return "", "", nil, err
		}
		if sourceScope.ExtensionID != "" && sourceScope.ExtensionID != targetExtensionID {
			return "", "", nil, fmt.Errorf("scope snapshot extension mismatch")
		}
		if sourceScope.ExpiresAt != nil && now.After(*sourceScope.ExpiresAt) {
			return "", "", nil, fmt.Errorf("scope snapshot expired")
		}
		sourcePermission, err := permissionStore.GetSnapshot(ctx, permissionSnapshotID)
		if err != nil {
			return "", "", nil, err
		}
		if sourcePermission.ExtensionID != "" && sourcePermission.ExtensionID != targetExtensionID {
			return "", "", nil, fmt.Errorf("permission snapshot extension mismatch")
		}
		if !sourcePermission.IsValid(now) {
			return "", "", nil, fmt.Errorf("permission snapshot expired or revoked")
		}

		derivedScope := sourceScope
		derivedScope.SnapshotID = "ssnap-derived-" + uuid.NewString()
		derivedScope.ExtensionID = targetExtensionID
		derivedScope.ModuleID = targetModuleID
		derivedScope.CreatedAt = now
		for index := range derivedScope.ResolvedScopes {
			ref := &derivedScope.ResolvedScopes[index]
			if ref.Type != scope.ScopeModule {
				continue
			}
			if ref.ExtensionID != "" && ref.ExtensionID != targetExtensionID {
				continue
			}
			ref.ExtensionID = targetExtensionID
			ref.ModuleID = targetModuleID
		}
		if err := scopeStore.SaveSnapshot(ctx, derivedScope); err != nil {
			return "", "", nil, err
		}

		lifetime := time.Duration(0)
		if sourcePermission.ExpiresAt != nil {
			lifetime = time.Until(*sourcePermission.ExpiresAt)
			if lifetime <= 0 {
				if err := scopeStore.DeleteSnapshot(context.Background(), derivedScope.SnapshotID); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
				return "", "", nil, fmt.Errorf("permission snapshot expired")
			}
		}
		derivedPermission := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{
			SessionID:      sourcePermission.SessionID,
			ExtensionID:    targetExtensionID,
			ModuleID:       targetModuleID,
			Generation:     sourcePermission.Generation,
			CharacterID:    sourcePermission.CharacterID,
			ConversationID: sourcePermission.ConversationID,
			ResourceIDs:    sourcePermission.ResourceIDs,
			GrantedPerms:   sourcePermission.GrantedPerms,
			GrantedScopes:  sourcePermission.GrantedScopes,
			Lifetime:       lifetime,
			ExecutionContext: permission.PermissionExecutionContext{
				Placement:          sourcePermission.ExecutionPlacement,
				SpaceID:            sourcePermission.SpaceID,
				DeviceID:           sourcePermission.DeviceID,
				RuntimeID:          sourcePermission.RuntimeID,
				ProviderID:         sourcePermission.ProviderID,
				ProviderInstanceID: sourcePermission.ProviderInstanceID,
				ExtensionID:        targetExtensionID,
				ModuleID:           targetModuleID,
				Source:             "ui_action_cross_module",
			},
		})
		if err := derivedPermission.ValidateGrantedPerms(validator); err != nil {
			if err := scopeStore.DeleteSnapshot(context.Background(), derivedScope.SnapshotID); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return "", "", nil, err
		}
		if err := permissionStore.SaveSnapshot(ctx, derivedPermission); err != nil {
			if err := scopeStore.DeleteSnapshot(context.Background(), derivedScope.SnapshotID); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return "", "", nil, err
		}
		cleanup := func() {
			if err := scopeStore.DeleteSnapshot(context.Background(), derivedScope.SnapshotID); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			if err := permissionStore.DeleteSnapshot(context.Background(), derivedPermission.SnapshotID); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
		return derivedScope.SnapshotID, derivedPermission.SnapshotID, cleanup, nil
	}
}
