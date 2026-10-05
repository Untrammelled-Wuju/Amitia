package coordination

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type CommitLeaseProof struct {
	ContinuityID   string `json:"continuityId"`
	LeaseID        string `json:"leaseId"`
	RequestID      string `json:"requestId"`
	AllowCompleted bool   `json:"allowCompleted,omitempty"`
}

type commitLeaseKey struct{}

func WithCommitLease(ctx context.Context, proof CommitLeaseProof) context.Context {
	return context.WithValue(ctx, commitLeaseKey{}, proof)
}

func CommitLease(ctx context.Context) *CommitLeaseProof {
	proof, ok := ctx.Value(commitLeaseKey{}).(CommitLeaseProof)
	if !ok {
		return nil
	}
	return &proof
}

func sameLeaseAuthority(first, second ExecutionScope) bool {
	first.RequestID, first.TurnID, first.ExecutionID = "", "", ""
	second.RequestID, second.TurnID, second.ExecutionID = "", "", ""
	return first == second
}

func (s *OwnershipStore) validateCommitLease(ctx context.Context, tx *sql.Tx, commit Commit) error {
	proof := commit.LeaseProof
	if proof == nil {
		return nil
	}
	if proof.ContinuityID == "" || proof.LeaseID == "" || proof.RequestID == "" || len(proof.ContinuityID) > 512 || len(proof.LeaseID) > 128 || len(proof.RequestID) > 128 {
		return ErrWrongOwner
	}
	var role string
	var deleted bool
	var raw json.RawMessage
	err := tx.QueryRowContext(ctx, `SELECT role_id,deleted,CAST(body AS BLOB) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='continuity' AND resource_id=?`, s.ownerID, proof.ContinuityID).Scan(&role, &deleted, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrResourceVersion
	}
	if err != nil {
		return err
	}
	var document struct {
		Scope   ExecutionScope `json:"executionScope"`
		CoreID  string         `json:"coreId"`
		OwnerID string         `json:"ownerId"`
		Thread  struct {
			Status string `json:"status"`
		} `json:"thread"`
		Lease *struct {
			ID        string    `json:"id"`
			RequestID string    `json:"requestId"`
			State     string    `json:"state"`
			ExpiresAt time.Time `json:"expiresAt"`
		} `json:"lease"`
	}
	if deleted || role != commit.Scope.RoleID || json.Unmarshal(raw, &document) != nil || document.Lease == nil || document.CoreID != commit.Scope.CoreID || document.OwnerID != s.ownerID || !sameLeaseAuthority(document.Scope, commit.Scope) || document.Lease.ID != proof.LeaseID || document.Lease.RequestID != proof.RequestID {
		return ErrResourceVersion
	}
	if document.Lease.State == "running" && document.Lease.ExpiresAt.After(time.Now()) && document.Thread.Status != "paused" && document.Thread.Status != "completed" && document.Thread.Status != "cancelled" {
		return nil
	}
	if !proof.AllowCompleted || document.Lease.State != "completed" || commit.Scope.RequestID != proof.RequestID+"|memory" {
		return ErrResourceVersion
	}
	for _, mutation := range commit.Mutations {
		switch mutation.Kind {
		case "memory", "working", "profile", "episodic", "fact", "vector", "graph", "summary":
		default:
			return ErrWrongOwner
		}
	}
	var checkpointRaw json.RawMessage
	if err := tx.QueryRowContext(ctx, `SELECT CAST(body AS BLOB) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='checkpoint' AND resource_id=? AND role_id=? AND deleted=0`, s.ownerID, "turn/"+proof.RequestID, commit.Scope.RoleID).Scan(&checkpointRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrResourceVersion
		}
		return err
	}
	var checkpoint struct {
		Status   string `json:"status"`
		Response *struct {
			Saved bool           `json:"saved"`
			Scope ExecutionScope `json:"executionScope"`
		} `json:"response"`
	}
	if json.Unmarshal(checkpointRaw, &checkpoint) != nil || checkpoint.Status != "completed" || checkpoint.Response == nil || !checkpoint.Response.Saved || checkpoint.Response.Scope.RequestID != proof.RequestID || !sameLeaseAuthority(checkpoint.Response.Scope, commit.Scope) {
		return ErrResourceVersion
	}
	return nil
}
