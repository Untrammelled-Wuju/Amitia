package business

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"time"
)

func (e *Engine) ObserveMemory(ctx context.Context, request Request, generation Generation) (coordination.Acknowledgement, error) {
	if request.ConversationID == "" || request.RequestID == "" || request.Message == "" || generation.Text == "" {
		return coordination.Acknowledgement{}, errors.New("记忆观察缺少已完成对话")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	defer finish()
	if request.ExpectedScope != nil && summaryAuthority(*request.ExpectedScope) != summaryAuthority(scope) {
		return coordination.Acknowledgement{}, coordination.ErrScopeExpired
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return coordination.Acknowledgement{}, coordination.ErrWrongOwner
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00memory-observation\x00" + request.RequestID)
	defer unlock()
	fingerprint := memoryManagementHash(struct {
		Scope                        coordination.ExecutionScope
		Conversation, Message, Reply string
		Evidence                     []coordination.ResourceVersion
	}{summaryAuthority(scope), request.ConversationID, request.Message, generation.Text, request.RecallEvidence})
	receiptID := "memory-observation/" + request.RequestID
	if pending, _, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
		return coordination.Acknowledgement{}, err
	} else if pending != nil {
		return e.commit(ctx, *pending)
	}
	receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	if receipt != nil {
		var saved struct {
			Hash            string                       `json:"hash"`
			Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint {
			return coordination.Acknowledgement{}, coordination.ErrRequestConflict
		}
		return saved.Acknowledgement, e.coordination.Validate(ctx, scope)
	}
	snapshot, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{ConversationID: request.ConversationID, Limit: 128})
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return coordination.Acknowledgement{}, err
	}
	derived, err := e.model.ExtractOwnedMemory(ctx, Inference{Scope: scope, ConversationID: request.ConversationID, Snapshot: snapshot, Message: request.Message}, generation)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	mutations, err := e.prepareMemoryMutations(ctx, scope, request.ConversationID, modelMemoryDerived(derived), &snapshot)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	recalled := coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: snapshot.Role}
	for _, evidence := range request.RecallEvidence {
		if evidence.Kind != "memory" || evidence.ID == "" || evidence.Revision < 1 {
			return coordination.Acknowledgement{}, coordination.ErrWrongOwner
		}
		resource, err := port.Resource(ctx, scope, evidence.Kind, evidence.ID)
		if err != nil {
			return coordination.Acknowledgement{}, err
		}
		if resource == nil || resource.Deleted || resource.Revision != evidence.Revision {
			return coordination.Acknowledgement{}, coordination.ErrResourceVersion
		}
		recalled.Resources = append(recalled.Resources, *resource)
	}
	mutations = reinforceMemoryMutations(mutations, recalled, scope, time.Now())
	ack := coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"checkpoint/" + receiptID: 1}}
	for _, mutation := range mutations {
		ack.Versions[mutation.Kind+"/"+mutation.ID] = mutation.ExpectedRevision + 1
	}
	mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(map[string]any{"hash": fingerprint, "acknowledgement": ack})})
	commit := coordination.Commit{Scope: scope, Mutations: mutations, Dependencies: request.RecallEvidence}
	result, err := e.commit(ctx, commit)
	if err != nil && !errors.Is(err, coordination.ErrResourceVersion) && !errors.Is(err, coordination.ErrScopeExpired) && !errors.Is(err, coordination.ErrRequestConflict) {
		if pendingErr := e.coordination.Enqueue(ctx, commit); pendingErr != nil {
			return result, errors.Join(err, pendingErr)
		}
	}
	return result, err
}
