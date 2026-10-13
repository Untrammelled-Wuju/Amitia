package business

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"strings"
	"time"
)

func (e *Engine) SubmitMemoryCandidate(ctx context.Context, request Request, input MemoryCandidateInput) (MemoryManagementResponse, error) {
	if request.RequestID == "" || len(request.RequestID) > 128 || request.ConversationID == "" || strings.TrimSpace(input.Key) == "" || len(input.Key) > 512 || strings.TrimSpace(input.Value) == "" || len(input.Value) > 128<<10 || input.Importance < 0 || input.Importance > 10 {
		return MemoryManagementResponse{}, errors.New("候选记忆内容或来源无效")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return MemoryManagementResponse{}, coordination.ErrWrongOwner
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00candidate-submission\x00" + request.RequestID)
	defer unlock()
	receiptID := "memory-candidate-operation/" + request.RequestID
	fingerprint := memoryManagementHash(struct {
		Scope        coordination.ExecutionScope
		Conversation string
		Input        MemoryCandidateInput
	}{summaryAuthority(scope), request.ConversationID, input})
	if pending, _, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
		return MemoryManagementResponse{}, err
	} else if pending != nil {
		if _, err := e.commit(ctx, *pending); err != nil {
			return MemoryManagementResponse{}, err
		}
	}
	if receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID); err != nil {
		return MemoryManagementResponse{}, err
	} else if receipt != nil {
		var saved struct {
			Hash     string                    `json:"hash"`
			Response *MemoryManagementResponse `json:"response"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint || saved.Response == nil {
			return MemoryManagementResponse{}, coordination.ErrRequestConflict
		}
		if summaryAuthority(saved.Response.Scope) != summaryAuthority(scope) {
			return MemoryManagementResponse{}, coordination.ErrScopeExpired
		}
		if err := e.coordination.Validate(ctx, scope); err != nil {
			return MemoryManagementResponse{}, err
		}
		return *saved.Response, coordination.ValidateRoleRevision(ctx, e.data, scope)
	}
	messages, dependencies, err := e.summaryHistory(ctx, request, scope)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	if len(messages) == 0 {
		return MemoryManagementResponse{}, errors.New("候选记忆没有已保存的对话来源")
	}
	id := "memory-candidate/" + request.RequestID + "/0"
	candidate := OwnedMemoryCandidate{Type: "owned-memory-candidate", ID: id, Key: input.Key, Value: input.Value, MemoryType: input.MemoryType, Importance: input.Importance, State: "pending", DerivedKind: "fact", Derived: body(map[string]any{"value": input.Value, "source": "reflection"}), ConversationID: request.ConversationID, ConversationOrigin: request.ConversationOrigin, ProvenanceHash: memoryManagementHash(messages), Dependencies: dependencies, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), ExecutionScope: scope}
	return e.memoryCandidateCommit(ctx, scope, []coordination.Mutation{{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: body(candidate)}}, dependencies, receiptID, 0, fingerprint)
}
