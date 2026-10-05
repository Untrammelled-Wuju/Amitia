package coordination

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type InterruptedReply struct {
	Scope          ExecutionScope `json:"scope"`
	ConversationID string         `json:"conversationId"`
	Text           string         `json:"text"`
	Reasoning      string         `json:"reasoning,omitempty"`
	Reason         string         `json:"reason"`
}

type InterruptionPort interface {
	SaveInterrupted(context.Context, InterruptedReply) error
}

func (s *OwnershipStore) SaveInterrupted(ctx context.Context, reply InterruptedReply) error {
	scope := reply.Scope
	if scope.ResourceOwnerID != s.ownerID || scope.RequestID == "" || reply.ConversationID == "" || len(reply.Text)+len(reply.Reasoning) > 256<<10 {
		return ErrWrongOwner
	}
	var recorded struct {
		Hash           string          `json:"hash"`
		Status         string          `json:"status"`
		ConversationID string          `json:"conversationId"`
		Scope          *ExecutionScope `json:"scope"`
		Reason         string          `json:"reason"`
	}
	var checkpointBody json.RawMessage
	if err := s.db.QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='checkpoint' AND resource_id=? AND role_id=? AND deleted=0`, s.ownerID, "turn/"+scope.RequestID, scope.RoleID).Scan(&checkpointBody); err != nil {
		return err
	}
	if err := json.Unmarshal(checkpointBody, &recorded); err != nil {
		return err
	}
	if recorded.Scope == nil || *recorded.Scope != scope || recorded.ConversationID != reply.ConversationID {
		return ErrRequestConflict
	}
	if recorded.Status == "interrupted" {
		if recorded.Reason != reply.Reason {
			return ErrRequestConflict
		}
		if reply.Text == "" && reply.Reasoning == "" {
			return nil
		}
		var body json.RawMessage
		if err := s.db.QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND resource_id=? AND role_id=? AND deleted=0`, s.ownerID, scope.RequestID+"/assistant", scope.RoleID).Scan(&body); err != nil {
			return err
		}
		var message struct {
			Text      string `json:"content"`
			Reasoning string `json:"reasoningContent"`
		}
		if err := json.Unmarshal(body, &message); err != nil {
			return err
		}
		if message.Text != reply.Text || message.Reasoning != reply.Reasoning {
			return ErrRequestConflict
		}
		return nil
	}
	if recorded.Status != "running" {
		return ErrRequestConflict
	}
	checkpoint, err := json.Marshal(map[string]any{"hash": recorded.Hash, "status": "interrupted", "conversationId": reply.ConversationID, "scope": scope, "reason": reply.Reason})
	if err != nil {
		return err
	}
	mutations := []Mutation{{Kind: "checkpoint", ID: "turn/" + scope.RequestID, RoleID: scope.RoleID, ExpectedRevision: 1, Body: checkpoint}}
	if reply.Text != "" || reply.Reasoning != "" {
		message, err := json.Marshal(map[string]any{"id": scope.RequestID + "/assistant", "conversationId": reply.ConversationID, "characterId": scope.RoleID, "role": "assistant", "content": reply.Text, "reasoningContent": reply.Reasoning, "requestId": scope.RequestID, "createdAt": time.Now().UTC().Format(time.RFC3339Nano), "executionScope": scope, "status": "interrupted", "interruptionReason": reply.Reason})
		if err != nil {
			return err
		}
		mutations = append(mutations, Mutation{Kind: "message", ID: scope.RequestID + "/assistant", RoleID: scope.RoleID, Body: message})
	}
	commitScope := scope
	commitScope.RequestID += "|interrupted"
	_, err = s.apply(ctx, Commit{Scope: commitScope, Mutations: mutations}, func(tx *sql.Tx) error {
		var current json.RawMessage
		if err := tx.QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='checkpoint' AND resource_id=? AND role_id=? AND deleted=0`, s.ownerID, "turn/"+scope.RequestID, scope.RoleID).Scan(&current); err != nil {
			return err
		}
		if string(current) != string(checkpointBody) {
			return ErrResourceVersion
		}
		return nil
	})
	return err
}
