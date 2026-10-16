package coordination

import (
	"context"
	"database/sql"
)

type sourceCommitAuthorityKey struct{}
type sourceManagementAuthorityKey struct{}
type sourceCommitAuthority struct {
	Scope  ExecutionScope
	CallID string
}

func WithSourceCommitAuthority(ctx context.Context, scope ExecutionScope, callID string) context.Context {
	return context.WithValue(ctx, sourceCommitAuthorityKey{}, sourceCommitAuthority{Scope: scope, CallID: callID})
}

func WithSourceManagementAuthority(ctx context.Context, scope ExecutionScope, callID string) context.Context {
	return context.WithValue(ctx, sourceManagementAuthorityKey{}, sourceCommitAuthority{Scope: scope, CallID: callID})
}

func (s *OwnershipStore) validateSourceCommitAuthority(ctx context.Context, tx *sql.Tx, commit Commit) error {
	if proof, ok := ctx.Value(sourceManagementAuthorityKey{}).(sourceCommitAuthority); ok {
		if proof.CallID == "" || proof.Scope.TargetDeviceID != s.ownerID || commit.Scope.ResourceOwnerID != s.ownerID || commit.Scope.RoleOwnerID != s.ownerID {
			return ErrWrongOwner
		}
		original, current := proof.Scope, commit.Scope
		if !original.Coordinated && (current.RoleID != original.RoleID || original.ResourceOwnerID != s.ownerID || original.RoleOwnerID != s.ownerID) {
			return ErrWrongOwner
		}
		current.RequestID, current.RoleID, current.MemoryRoleID, current.ResourceOwnerID, current.RoleOwnerID = original.RequestID, original.RoleID, original.MemoryRoleID, original.ResourceOwnerID, original.RoleOwnerID
		if current != original {
			return ErrWrongOwner
		}
		for _, mutation := range commit.Mutations {
			if !mutation.Deleted {
				return ErrWrongOwner
			}
		}
		if err := validateSourceCallQuery(ctx, tx, original.SpaceID, proof.CallID); err != nil {
			return err
		}
		return validateSourceAuthorityQuery(ctx, tx, original)
	}
	proof, ok := ctx.Value(sourceCommitAuthorityKey{}).(sourceCommitAuthority)
	if !ok {
		return nil
	}
	if proof.Scope != commit.Scope || proof.Scope.Coordinated || proof.Scope.ResourceOwnerID != s.ownerID || proof.Scope.TargetDeviceID != s.ownerID || proof.Scope.RoleOwnerID != s.ownerID || proof.CallID == "" || len(proof.CallID) > 512 {
		return ErrWrongOwner
	}
	if err := validateSourceCallQuery(ctx, tx, proof.Scope.SpaceID, proof.CallID); err != nil {
		return err
	}
	return validateSourceAuthorityQuery(ctx, tx, proof.Scope)
}
