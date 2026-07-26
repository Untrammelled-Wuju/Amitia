package scope

import (
	"context"
	"fmt"
)

type CleanupHandler struct {
	manager ScopeManager
	store   ScopeStore
}

func NewCleanupHandler(manager ScopeManager, store ScopeStore) *CleanupHandler {
	return &CleanupHandler{manager: manager, store: store}
}

func (h *CleanupHandler) OnCharacterDeleted(ctx context.Context, characterID string) error {
	if err := h.manager.Invalidate(ctx, ScopeInvalidationFilter{CharacterID: characterID}); err != nil {
		return fmt.Errorf("invalidate character bindings: %w", err)
	}
	return nil
}

func (h *CleanupHandler) OnConversationDeleted(ctx context.Context, conversationID string) error {
	if err := h.manager.Invalidate(ctx, ScopeInvalidationFilter{ConversationID: conversationID}); err != nil {
		return fmt.Errorf("invalidate conversation bindings: %w", err)
	}
	return nil
}

func (h *CleanupHandler) OnExtensionDisabled(ctx context.Context, extensionID string) error {
	if err := h.manager.Invalidate(ctx, ScopeInvalidationFilter{ExtensionID: extensionID}); err != nil {
		return fmt.Errorf("invalidate extension bindings: %w", err)
	}
	return nil
}

func (h *CleanupHandler) OnModuleDisabled(ctx context.Context, extensionID, moduleID string) error {
	filter := ScopeInvalidationFilter{ExtensionID: extensionID, ModuleID: moduleID}
	if err := h.manager.Invalidate(ctx, filter); err != nil {
		return fmt.Errorf("invalidate module bindings: %w", err)
	}
	return nil
}

func (h *CleanupHandler) ListAffectedBindings(ctx context.Context, subjectType ScopeSubjectType, subjectID string) ([]ScopeBinding, error) {
	return h.manager.ListBindings(ctx, ScopeBindingFilter{
		SubjectType: subjectType,
		SubjectID:   subjectID,
	})
}
