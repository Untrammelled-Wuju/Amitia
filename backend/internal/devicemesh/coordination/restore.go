package coordination

import (
	"context"
	"time"
)

func (s *Service) Restore(ctx context.Context, expected ExecutionScope, roles DataPort) (context.Context, func(), error) {
	if expected.SpaceID == "" || expected.CoreID == "" || expected.AuthorizationRealm != expected.CoreID || expected.InitiatorDeviceID == "" || expected.TargetDeviceID == "" || expected.RoleID == "" || expected.RequestID == "" || expected.RoleRevision <= 0 || expected.ExecutionID == "" || expected.TurnID == "" {
		return ctx, nil, ErrScopeExpired
	}
	if roles == nil {
		return ctx, nil, ErrRoleRequired
	}
	policy, err := s.Get(ctx, expected.SpaceID, expected.TargetDeviceID)
	if err != nil {
		return ctx, nil, err
	}
	if policy.ModeRevision != expected.ModeRevision || policy.Coordinated != expected.Coordinated {
		return ctx, nil, ErrScopeExpired
	}
	current, actual, finish, err := s.BeginWithOwners(ctx, expected.SpaceID, expected.InitiatorDeviceID, expected.TargetDeviceID, expected.CoreID, expected.RoleID, expected.RequestID, expected.ResourceOwnerID, expected.RoleOwnerID)
	if err != nil {
		return ctx, nil, err
	}
	actual.RoleRevision = expected.RoleRevision
	actual.TurnID = expected.TurnID
	actual.ExecutionID = expected.ExecutionID
	actual.MemoryRoleID = expected.MemoryRoleID
	actual.MemoryViewAll = expected.MemoryViewAll
	actual.ContinuityRoleID = expected.ContinuityRoleID
	actual.ConversationRoleID = expected.ConversationRoleID
	if actual != expected {
		finish()
		return ctx, nil, ErrScopeExpired
	}
	current = WithScope(current, expected)
	if err := ValidateRoleRevision(current, roles, expected); err != nil {
		finish()
		return ctx, nil, err
	}
	guarded := WithAdditionalGuard(current, func(current context.Context) error {
		return ValidateRoleRevision(WithoutAdditionalGuard(current), roles, expected)
	})
	restored, cancel := context.WithCancelCause(guarded)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-restored.Done():
				return
			case <-ticker.C:
				if err := ValidateCurrent(restored); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	return restored, func() { cancel(nil); finish() }, nil
}
