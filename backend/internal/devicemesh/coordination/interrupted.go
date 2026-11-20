package coordination

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
		if err := s.db.QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND resource_id=? AND deleted=0`, s.ownerID, scope.RequestID+"/assistant").Scan(&body); err != nil {
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
	conversationRole := StorageRole(scope, "conversation")
	var conversationRevision int64
	var conversation map[string]any
	var conversationBody json.RawMessage
	err = s.db.QueryRowContext(ctx, `SELECT revision,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='conversation' AND resource_id=? AND role_id=? AND deleted=0`, s.ownerID, reply.ConversationID, conversationRole).Scan(&conversationRevision, &conversationBody)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		conversationRevision = 0
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(conversationBody, &conversation); err != nil {
			return err
		}
	}
	sequence := interruptedSequence(conversation)
	mutations := []Mutation{{Kind: "checkpoint", ID: "turn/" + scope.RequestID, RoleID: scope.RoleID, ExpectedRevision: 1, Body: checkpoint}}
	if reply.Text != "" || reply.Reasoning != "" {
		document := map[string]any{"id": scope.RequestID + "/assistant", "conversationId": reply.ConversationID, "characterId": scope.RoleID, "role": "assistant", "content": reply.Text, "reasoningContent": reply.Reasoning, "requestId": scope.RequestID, "createdAt": time.Now().UTC().Format(time.RFC3339Nano), "executionScope": scope, "status": "interrupted", "interruptionReason": reply.Reason}
		if sequence > 0 {
			document["sequence"] = sequence
		}
		message, err := json.Marshal(document)
		if err != nil {
			return err
		}
		mutations = append(mutations, Mutation{Kind: "message", ID: scope.RequestID + "/assistant", RoleID: StorageRole(scope, "message"), Body: message})
	}
	if conversation != nil {
		if preview := MessagePreview(reply.Text); preview != "" {
			conversation["lastMessage"] = preview
			conversation["lastMessagePreview"] = preview
			conversation["lastMessageRole"] = "assistant"
			conversation["lastMessageType"] = "text"
			if sequence > 0 {
				conversation["lastMessageSequence"] = sequence
			}
		}
		conversation["updatedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
		updated, err := json.Marshal(conversation)
		if err != nil {
			return err
		}
		mutations = append(mutations, Mutation{Kind: "conversation", ID: reply.ConversationID, RoleID: conversationRole, ExpectedRevision: conversationRevision, Body: updated})
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
		if conversation != nil {
			var latest json.RawMessage
			if err := tx.QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='conversation' AND resource_id=? AND role_id=? AND deleted=0`, s.ownerID, reply.ConversationID, conversationRole).Scan(&latest); err != nil {
				return err
			}
			if string(latest) != string(conversationBody) {
				return ErrResourceVersion
			}
		}
		return nil
	})
	return err
}

func interruptedSequence(conversation map[string]any) int64 {
	if conversation == nil {
		return 0
	}
	switch value := conversation["lastMessageSequence"].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case json.Number:
		number, err := value.Int64()
		if err != nil {
			return 0
		}
		return number
	default:
		return 0
	}
}
