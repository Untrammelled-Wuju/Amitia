package business

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) pendingMutation(ctx context.Context, scope coordination.ExecutionScope, receiptID, fingerprint string) (*coordination.Commit, json.RawMessage, error) {
	pending, err := e.coordination.PendingRequest(ctx, scope.ResourceOwnerID, scope.RequestID)
	if err != nil || pending == nil {
		return nil, nil, err
	}
	if pending.Commit.Scope != scope {
		return nil, nil, coordination.ErrRequestConflict
	}
	for _, mutation := range pending.Commit.Mutations {
		if mutation.Kind != "checkpoint" || mutation.ID != receiptID {
			continue
		}
		var proof struct {
			Hash string `json:"hash"`
		}
		if mutation.Deleted || json.Unmarshal(mutation.Body, &proof) != nil || proof.Hash != fingerprint {
			return nil, nil, coordination.ErrRequestConflict
		}
		return &pending.Commit, mutation.Body, nil
	}
	return nil, nil, coordination.ErrRequestConflict
}
