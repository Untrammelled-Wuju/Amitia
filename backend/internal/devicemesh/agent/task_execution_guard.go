package agent

import (
	"context"
	"database/sql"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func executeOwnedTaskGuard(ctx context.Context, db *sql.DB, credentials *CredentialStore, credential *StoredCredential, scope coordination.ExecutionScope, callID string, roles []coordination.SourceRoleExecutionGuard, run func(context.Context) (*protocol.RuntimeResultPayload, error)) (*protocol.RuntimeResultPayload, error) {
	current, cancel := context.WithDeadline(ctx, credential.ExpiresAt)
	defer cancel()
	guarded := coordination.WithAdditionalGuard(coordination.WithScope(current, scope), func(check context.Context) error {
		return credentials.WithActiveCredential(check, credential, func(locked context.Context) error {
			if err := coordination.ValidateSourceCall(locked, db, scope.SpaceID, callID); err != nil {
				return err
			}
			if err := coordination.ValidateSourceAuthority(locked, db, scope); err != nil {
				return err
			}
			if !scope.Coordinated {
				if len(roles) == 0 || roles[0] == nil {
					return coordination.ErrRoleRequired
				}
				return roles[0].WithSourceRole(locked, scope, func() error { return nil })
			}
			return nil
		})
	})
	if err := coordination.ValidateCurrent(guarded); err != nil {
		return nil, err
	}
	result, err := run(guarded)
	if err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(guarded); err != nil {
		return nil, err
	}
	return result, nil
}
