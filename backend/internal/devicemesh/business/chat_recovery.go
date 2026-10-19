package business

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type OwnedTurnReconciliation struct {
	Status   string    `json:"status"`
	Response *Response `json:"response,omitempty"`
}

func (e *Engine) ReconcileOwnedTurn(ctx context.Context, expected coordination.ExecutionScope, conversation string) (OwnedTurnReconciliation, error) {
	result := OwnedTurnReconciliation{}
	err := e.WithExecutionScope(ctx, expected, func(current context.Context) error {
		port, ok := e.data.(coordination.ResourcePort)
		if !ok || conversation == "" || expected.RequestID == "" {
			return coordination.ErrWrongOwner
		}
		if ownedRuntimeManager.HasExecution(RuntimeKey(expected, conversation), expected.TurnID) {
			result.Status = "running"
			return nil
		}
		row, err := port.Resource(current, expected, "checkpoint", "turn/"+expected.RequestID)
		if err != nil {
			return err
		}
		if row == nil || row.Deleted {
			return ErrUncertainExecution
		}
		var saved checkpoint
		if err := json.Unmarshal(row.Body, &saved); err != nil {
			return err
		}
		if saved.ConversationID != conversation {
			return coordination.ErrWrongOwner
		}
		if saved.Response != nil && saved.Status == "completed" {
			response := saved.Response
			if response.Scope != expected || response.RequestID != expected.RequestID || response.ConversationID != conversation || !response.Saved {
				return coordination.ErrScopeExpired
			}
			if !response.Suppressed && (saved.Ingress == nil || !saved.Ingress.SuppressReplyPersistence) {
				message, err := port.Resource(current, expected, "message", expected.RequestID+"/assistant")
				if err != nil {
					return err
				}
				if message == nil || message.Deleted {
					return ErrUncertainExecution
				}
				var committed struct {
					ConversationID string                      `json:"conversationId"`
					RequestID      string                      `json:"requestId"`
					Content        string                      `json:"content"`
					Scope          coordination.ExecutionScope `json:"executionScope"`
				}
				if json.Unmarshal(message.Body, &committed) != nil || committed.ConversationID != conversation || committed.RequestID != expected.RequestID || committed.Scope != expected || committed.Content != response.Text {
					return ErrUncertainExecution
				}
			}
			result.Status, result.Response = "completed", response
			return nil
		}
		if saved.Scope == nil || *saved.Scope != expected {
			return coordination.ErrScopeExpired
		}
		result.Status = "needs_reconciliation"
		if saved.Status == result.Status {
			return nil
		}
		saved.Status = result.Status
		if saved.Runtime != nil {
			saved.Runtime.Status = result.Status
		}
		scope := expected
		scope.RequestID = "reconcile/" + expected.RequestID
		_, err = e.commit(current, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: row.ID, RoleID: expected.RoleID, ExpectedRevision: row.Revision, Body: body(saved)}}})
		return err
	})
	if errors.Is(err, ErrUncertainExecution) {
		result.Status = "needs_reconciliation"
	}
	return result, err
}
