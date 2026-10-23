package coordination

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
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
		if !original.Coordinated && (original.ResourceOwnerID != s.ownerID || original.RoleOwnerID != s.ownerID) {
			return ErrWrongOwner
		}
		current.RequestID, current.RoleID, current.MemoryRoleID, current.ResourceOwnerID, current.RoleOwnerID = original.RequestID, original.RoleID, original.MemoryRoleID, original.ResourceOwnerID, original.RoleOwnerID
		if current != original {
			return ErrWrongOwner
		}
		for _, mutation := range commit.Mutations {
			deletion, _ := ctx.Value(privacyDeletionCommitKey{}).(bool)
			if mutation.Deleted && !deletion && !original.Coordinated && commit.Scope.RoleID != original.RoleID {
				return ErrWrongOwner
			}
			if !mutation.Deleted {
				if deletion && mutation.Kind == "checkpoint" && strings.HasPrefix(mutation.ID, "privacy-deletion/") {
					if err := PrivacyDeletionReceipt(mutation.Body); err != nil {
						return err
					}
					continue
				}
				if err := s.validatePrivacyRedaction(ctx, tx, mutation); err != nil {
					return err
				}
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

func (s *OwnershipStore) validatePrivacyRedaction(ctx context.Context, tx *sql.Tx, mutation Mutation) error {
	if mutation.Kind != "message" || mutation.ExpectedRevision < 1 {
		return ErrWrongOwner
	}
	var raw []byte
	var role string
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT body,role_id,revision FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND resource_id=? AND deleted=0`, s.ownerID, mutation.ID).Scan(&raw, &role, &revision); err != nil {
		return err
	}
	if revision != mutation.ExpectedRevision || role != mutation.RoleID {
		return ErrResourceVersion
	}
	var before, after map[string]json.RawMessage
	if json.Unmarshal(raw, &before) != nil || json.Unmarshal(mutation.Body, &after) != nil {
		return ErrWrongOwner
	}
	var content, reasoning, safety string
	if json.Unmarshal(after["content"], &content) != nil || content != "[已脱敏]" || json.Unmarshal(after["reasoningContent"], &reasoning) != nil || reasoning != "" || json.Unmarshal(after["safetyLevel"], &safety) != nil || safety != "masked" {
		return ErrWrongOwner
	}
	for _, key := range []string{"content", "reasoningContent", "safetyLevel", "updatedAt"} {
		delete(before, key)
		delete(after, key)
	}
	left, _ := json.Marshal(before)
	right, _ := json.Marshal(after)
	if !bytes.Equal(left, right) {
		return ErrWrongOwner
	}
	return nil
}
