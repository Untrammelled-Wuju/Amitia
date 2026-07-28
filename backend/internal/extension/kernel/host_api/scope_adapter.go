package host_api

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type ScopeSnapshotStore interface {
	Get(ctx context.Context, snapshotID string) (*scope.ScopeSnapshot, error)
}

type ScopeSnapshotReader interface {
	GetSnapshot(ctx context.Context, snapshotID string) (scope.ScopeSnapshot, error)
}

type SnapshotStoreAdapter struct {
	Reader ScopeSnapshotReader
}

func NewSnapshotStoreAdapter(reader ScopeSnapshotReader) *SnapshotStoreAdapter {
	return &SnapshotStoreAdapter{Reader: reader}
}

func (a *SnapshotStoreAdapter) Get(ctx context.Context, snapshotID string) (*scope.ScopeSnapshot, error) {
	if a == nil || a.Reader == nil {
		return nil, fmt.Errorf("host_api: snapshot reader not wired")
	}
	snap, err := a.Reader.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

type ManagerScopeChecker struct {
	Manager       scope.ScopeManager
	SnapshotStore ScopeSnapshotStore
	Now           func() time.Time
}

func NewManagerScopeChecker(manager scope.ScopeManager, store ScopeSnapshotStore) *ManagerScopeChecker {
	return &ManagerScopeChecker{Manager: manager, SnapshotStore: store, Now: time.Now}
}

func (c *ManagerScopeChecker) Check(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, scopeSnapshotID string, policy ScopePolicy) error {
	if c == nil || c.Manager == nil || c.SnapshotStore == nil {
		return ErrScopeDenied
	}

	if scopeSnapshotID == "" {
		if !isGlobalRoute(policy) {
			return fmt.Errorf("%w: empty scope snapshot for non-global route", ErrScopeDenied)
		}
		return nil
	}

	snap, err := c.SnapshotStore.Get(ctx, scopeSnapshotID)
	if err != nil || snap == nil {
		return fmt.Errorf("%w: snapshot %s not found", ErrScopeDenied, scopeSnapshotID)
	}

	now := c.Now()
	if snap.ExpiresAt != nil && now.After(*snap.ExpiresAt) {
		return fmt.Errorf("%w: snapshot %s expired", ErrScopeDenied, scopeSnapshotID)
	}

	extID := string(identity.ExtensionID)
	modID := string(identity.ModuleID)
	if snap.ExtensionID != "" && snap.ExtensionID != extID {
		return fmt.Errorf("%w: snapshot extension %s does not match caller %s", ErrScopeDenied, snap.ExtensionID, extID)
	}
	if modID != "" && snap.ModuleID != "" && snap.ModuleID != modID {
		return fmt.Errorf("%w: snapshot module %s does not match caller %s", ErrScopeDenied, snap.ModuleID, modID)
	}
	if identity.Generation > 0 && snap.Generation != identity.Generation {
		return fmt.Errorf("%w: snapshot generation %d does not match caller generation %d", ErrScopeDenied, snap.Generation, identity.Generation)
	}

	if !isGlobalRoute(policy) {
		subjectType, subjectID := ScopeSubjectTypeFromIdentity(identity)
		if subjectType == "" || subjectID == "" {
			return fmt.Errorf("%w: cannot derive scope subject", ErrScopeDenied)
		}
		decision := c.Manager.Evaluate(ctx, scope.ScopeEvaluationRequest{
			SubjectType:    toScopeSubjectType(subjectType),
			SubjectID:      subjectID,
			CharacterID:    snap.CharacterID,
			ConversationID: snap.ConversationID,
			ExtensionID:    snap.ExtensionID,
			ModuleID:       snap.ModuleID,
			InvocationID:   snap.InvocationID,
			Generation:     snap.Generation,
			ParentSnapshot: snap,
		})
		if !decision.Allowed {
			return fmt.Errorf("%w: scope manager denied (reasons=%v)", ErrScopeDenied, decision.Reasons)
		}
	}

	if len(policy.RequireRoles) > 0 {
		if !roleSatisfied(policy.RequireRoles, snap) {
			return fmt.Errorf("%w: required roles %v not satisfied", ErrScopeDenied, policy.RequireRoles)
		}
	}

	return nil
}

func isGlobalRoute(policy ScopePolicy) bool {
	if len(policy.RequireRoles) == 0 && !policy.Namespaced {
		return true
	}
	return false
}

func roleSatisfied(roles []string, snap *scope.ScopeSnapshot) bool {
	if snap == nil {
		return false
	}
	matched := make(map[string]bool, len(roles))
	for _, r := range roles {
		matched[r] = false
	}
	for _, s := range snap.ResolvedScopes {
		if _, ok := matched[string(s.Type)]; ok {
			matched[string(s.Type)] = true
		}
	}
	for _, ok := range matched {
		if !ok {
			return false
		}
	}
	return true
}

func toScopeSubjectType(s string) scope.ScopeSubjectType {
	return scope.ScopeSubjectType(s)
}
