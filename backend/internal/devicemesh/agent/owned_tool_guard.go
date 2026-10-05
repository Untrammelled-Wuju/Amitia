package agent

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type RuntimeExecutionGuard func(context.Context, protocol.RuntimeInvokePayload, func(context.Context) (*protocol.RuntimeResultPayload, error)) (*protocol.RuntimeResultPayload, error)

func NewOwnedToolGuard(db *sql.DB, dataDir string, roleGuards ...coordination.SourceRoleExecutionGuard) RuntimeExecutionGuard {
	credentials := NewCredentialStore(dataDir)
	return func(ctx context.Context, invocation protocol.RuntimeInvokePayload, run func(context.Context) (*protocol.RuntimeResultPayload, error)) (*protocol.RuntimeResultPayload, error) {
		var scope coordination.ExecutionScope
		if db == nil || invocation.AuthorityCallID == "" || len(invocation.AuthorityCallID) > 512 || len(invocation.OwnedExecutionScope) > 64<<10 || json.Unmarshal(invocation.OwnedExecutionScope, &scope) != nil {
			return nil, coordination.ErrWrongOwner
		}
		credential, err := credentials.LoadCredential()
		if err != nil {
			return nil, err
		}
		if credential == nil || invocation.SpaceID != credential.SpaceID || invocation.DeviceID != credential.DeviceID || invocation.RuntimeID != credential.RuntimeID || scope.SpaceID != invocation.SpaceID.String() || scope.CoreID != scope.SpaceID || scope.AuthorizationRealm != scope.CoreID || scope.TargetDeviceID != invocation.DeviceID.String() || scope.InitiatorDeviceID == "" || scope.RoleID == "" || scope.RequestID == "" {
			return nil, coordination.ErrWrongOwner
		}
		if scope.PermissionRevision < 1 || scope.TargetPermissionRevision < 1 || scope.ModeRevision < 1 || scope.ProviderEpoch < 1 || scope.TargetProviderEpoch < 1 || scope.RoleRevision < 1 || len(scope.RoleID) > 512 || len(scope.RequestID) > 512 || len(scope.InitiatorDeviceID) > 512 {
			return nil, coordination.ErrWrongOwner
		}
		owner := scope.TargetDeviceID
		if scope.Coordinated {
			owner = scope.CoreID
		}
		if scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner {
			return nil, coordination.ErrWrongOwner
		}
		if invocation.RuntimeType == "task" {
			if scope.ExecutionID == "" || scope.TurnID == "" {
				return nil, coordination.ErrWrongOwner
			}
			return executeOwnedTaskGuard(ctx, db, credentials, credential, scope, invocation.AuthorityCallID, roleGuards, run)
		}
		var result *protocol.RuntimeResultPayload
		err = credentials.WithActiveCredential(ctx, credential, func(current context.Context) error {
			if err := coordination.ValidateSourceCall(current, db, scope.SpaceID, invocation.AuthorityCallID); err != nil {
				return err
			}
			if err := coordination.ValidateSourceAuthority(current, db, scope); err != nil {
				return err
			}
			execute := func() error {
				var runErr error
				guarded := coordination.WithAdditionalGuard(coordination.WithScope(current, scope), func(check context.Context) error {
					if err := coordination.ValidateSourceCall(check, db, scope.SpaceID, invocation.AuthorityCallID); err != nil {
						return err
					}
					return coordination.ValidateSourceAuthority(check, db, scope)
				})
				result, runErr = run(guarded)
				if runErr == nil && current.Err() != nil {
					return context.Cause(current)
				}
				return runErr
			}
			if !scope.Coordinated {
				if len(roleGuards) == 0 || roleGuards[0] == nil {
					return coordination.ErrRoleRequired
				}
				return roleGuards[0].WithSourceRole(current, scope, execute)
			}
			return execute()
		})
		return result, err
	}
}
