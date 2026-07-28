package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type ManagerScopeChecker struct {
	Manager       scope.ScopeManager
	Store         ScheduleStore
	SnapshotStore scope.ScopeStore
}

func NewManagerScopeChecker(manager scope.ScopeManager, store ScheduleStore, snapshotStore scope.ScopeStore) *ManagerScopeChecker {
	return &ManagerScopeChecker{Manager: manager, Store: store, SnapshotStore: snapshotStore}
}

func (c *ManagerScopeChecker) CheckScope(ctx context.Context, scheduleID string, rule ScopeRule) (bool, string, error) {
	if c == nil {
		return false, "scope checker not configured", nil
	}
	if c.Manager == nil {
		return false, "scope manager not configured", nil
	}
	if rule.ScopeType == "" || rule.ScopeType == "global" {
		return true, "", nil
	}

	def, err := c.Store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return false, "schedule definition not found", nil
	}

	subjectType := scope.SubjectExtension
	subjectID := def.ExtensionID
	if def.ModuleID != "" {
		subjectType = scope.SubjectModule
		subjectID = def.ModuleID
	}

	decision := c.Manager.Evaluate(ctx, scope.ScopeEvaluationRequest{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		ExtensionID: def.ExtensionID,
		ModuleID:    def.ModuleID,
	})

	if !decision.Allowed {
		reason := "scope denied"
		if len(decision.Reasons) > 0 {
			reason = fmt.Sprint(decision.Reasons)
		}
		return false, reason, nil
	}
	return true, "", nil
}

func (c *ManagerScopeChecker) CreateSnapshot(ctx context.Context, scheduleID, invocationID string, rule ScopeRule) (string, error) {
	if c == nil {
		return "", fmt.Errorf("scope checker not configured")
	}

	def, err := c.Store.GetDefinition(ctx, scheduleID)
	if err != nil || def == nil {
		return "", fmt.Errorf("schedule definition not found")
	}

	scopeType := scope.ScopeType(rule.ScopeType)
	if scopeType == "" {
		scopeType = scope.ScopeExtension
	}
	resolvedScope := scope.ScopeRef{
		Type:        scopeType,
		ExtensionID: def.ExtensionID,
		ModuleID:    def.ModuleID,
	}
	for _, id := range rule.ScopeIDs {
		switch scopeType {
		case scope.ScopeCharacter:
			resolvedScope.CharacterID = id
		case scope.ScopeConversation:
			resolvedScope.ConversationID = id
		case scope.ScopeResource:
			resolvedScope.ResourceID = id
		}
	}

	snap := scope.ScopeSnapshot{
		SnapshotID:     "snap-" + uuid.NewString(),
		InvocationID:   invocationID,
		ResolvedScopes: []scope.ScopeRef{resolvedScope},
		ExtensionID:    def.ExtensionID,
		ModuleID:       def.ModuleID,
		CreatedAt:      time.Now().UTC(),
	}

	if c.SnapshotStore != nil {
		if err := c.SnapshotStore.SaveSnapshot(ctx, snap); err != nil {
			return "", fmt.Errorf("scope: save snapshot: %w", err)
		}
	}
	return snap.SnapshotID, nil
}

var _ ScopeChecker = (*ManagerScopeChecker)(nil)
