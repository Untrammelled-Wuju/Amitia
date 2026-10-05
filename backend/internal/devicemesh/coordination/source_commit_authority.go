package coordination

import (
	"context"
	"database/sql"
)

type sourceCommitAuthorityKey struct{}
type sourceCommitAuthority struct {
	Scope  ExecutionScope
	CallID string
}

func WithSourceCommitAuthority(ctx context.Context, scope ExecutionScope, callID string) context.Context {
	return context.WithValue(ctx, sourceCommitAuthorityKey{}, sourceCommitAuthority{Scope: scope, CallID: callID})
}

func (s *OwnershipStore) validateSourceCommitAuthority(ctx context.Context, tx *sql.Tx, commit Commit) error {
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
