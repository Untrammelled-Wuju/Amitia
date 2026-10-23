package coordination

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/dbtransaction"
	"strings"
)

type PrivacyDeletionTarget struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	OwnerID        string         `json:"ownerId"`
	RoleID         string         `json:"roleId"`
	Legacy         bool           `json:"legacy"`
	Revision       int64          `json:"revision"`
	Fingerprint    string         `json:"fingerprint"`
	ExecutionScope ExecutionScope `json:"executionScope"`
}

type PrivacyDeletionRequest struct {
	LegacyDeletionID string                 `json:"legacyDeletionId,omitempty"`
	TargetID         string                 `json:"targetId"`
	TargetType       string                 `json:"targetType"`
	Selection        string                 `json:"scope"`
	Reason           string                 `json:"reason,omitempty"`
	ID               string                 `json:"id,omitempty"`
	Target           *PrivacyDeletionTarget `json:"target,omitempty"`
}

type privacyDeletionCommitKey struct{}

func (s *OwnershipStore) ApplyPrivacyDeletion(ctx context.Context, commits []Commit) error {
	if len(commits) == 0 || len(commits) > 4096 {
		return ErrPendingLimit
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ctx = dbtransaction.With(context.WithValue(ctx, privacyDeletionCommitKey{}, true), s.db, tx)
	for _, commit := range commits {
		for _, mutation := range commit.Mutations {
			if !mutation.Deleted && (mutation.Kind != "checkpoint" || !strings.HasPrefix(mutation.ID, "privacy-deletion/")) {
				return ErrWrongOwner
			}
			if mutation.Deleted && mutation.Kind == "checkpoint" && strings.HasPrefix(mutation.ID, "privacy-deletion/") {
				return ErrWrongOwner
			}
		}
		mutations := []Mutation{}
		for _, mutation := range commit.Mutations {
			if mutation.Deleted {
				row, err := s.Get(ctx, mutation.Kind, mutation.ID)
				if err != nil {
					return err
				}
				if row != nil && row.Deleted {
					continue
				}
			}
			mutations = append(mutations, mutation)
		}
		commit.Mutations = mutations
		if len(mutations) == 0 {
			continue
		}
		if _, err := s.Apply(ctx, commit); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func PrivacyDeletionReceipt(body json.RawMessage) error {
	var value struct {
		OwnerID string `json:"ownerId"`
		ID      string `json:"id"`
	}
	if json.Unmarshal(body, &value) != nil || value.OwnerID == "" || value.ID == "" {
		return errors.New("删除回执无效")
	}
	return nil
}

func (s *OwnershipStore) PrivacyDeletionProjectionComplete(ctx context.Context, kind, id string) (bool, error) {
	var pending int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_resources v LEFT JOIN kernel_device_owned_resources m ON m.owner_id=v.owner_id AND m.kind='memory' AND m.resource_id=v.source_id AND m.role_id=v.role_id LEFT JOIN kernel_device_owned_projections p ON p.owner_id=v.owner_id AND p.kind=v.kind AND p.resource_id=v.resource_id WHERE v.owner_id=? AND v.kind=? AND v.resource_id=? AND (v.deleted=0 OR COALESCE(p.projected_revision,0)<>v.revision OR COALESCE(p.projected_source_revision,0)<>COALESCE(m.revision,0))`, s.ownerID, kind, id).Scan(&pending)
	return pending == 0, err
}
