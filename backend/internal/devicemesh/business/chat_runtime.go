package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

var ownedRuntimeManager = conversationstream.NewManager()

func RuntimeManager() *conversationstream.Manager { return ownedRuntimeManager }

func RuntimeKey(scope coordination.ExecutionScope, conversation string) string {
	digest := sha256.Sum256(body([]string{scope.SpaceID, scope.CoreID, scope.ResourceOwnerID, scope.RoleID, conversation}))
	return "owned/" + hex.EncodeToString(digest[:])
}

func (e *Engine) ownedCheckpointRevision(ctx context.Context, scope coordination.ExecutionScope) (int64, error) {
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return 0, errors.New("Owner 执行日志未接通")
	}
	row, err := port.Resource(ctx, scope, "checkpoint", "turn/"+scope.RequestID)
	if err != nil {
		return 0, err
	}
	if row == nil || row.Deleted {
		return 0, ErrUncertainExecution
	}
	return row.Revision, nil
}

func (e *Engine) persistOwnedRuntime(ctx context.Context, scope coordination.ExecutionScope, conversation string, turnErr error) error {
	unlock := e.lock("runtime-checkpoint\x00" + RuntimeKey(scope, conversation))
	defer unlock()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return errors.New("Owner 执行日志未接通")
	}
	row, err := port.Resource(ctx, scope, "checkpoint", "turn/"+scope.RequestID)
	if err != nil {
		return err
	}
	if row == nil || row.Deleted {
		return ErrUncertainExecution
	}
	var saved checkpoint
	if json.Unmarshal(row.Body, &saved) != nil || saved.ConversationID != conversation {
		return coordination.ErrWrongOwner
	}
	if saved.Status == "completed" {
		return nil
	}
	runtime := ownedRuntimeManager.RuntimeSnapshot(RuntimeKey(scope, conversation)).ActiveTurn
	if turnErr != nil {
		runtimeError := DescribeRuntimeError(turnErr)
		saved.Error = &runtimeError
		saved.Status = "failed"
		if errors.Is(turnErr, ErrInterrupted) || errors.Is(turnErr, context.Canceled) {
			saved.Status = "interrupted"
		}
	}
	if runtime == nil || runtime.RequestID != scope.RequestID || runtime.TurnID != scope.TurnID || runtime.ExecutionID != scope.ExecutionID {
		if turnErr == nil {
			return coordination.ErrWrongOwner
		}
	} else {
		saved.Runtime = runtime
		if turnErr != nil && saved.Status != "interrupted" {
			saved.Runtime.Status = "failed"
		}
		if turnErr != nil && saved.Status == "interrupted" {
			saved.Runtime.Status = "interrupted"
		}
		if runtime.Status == "failed" || runtime.Status == "interrupted" {
			saved.Status = runtime.Status
		}
	}
	commitScope := scope
	commitScope.RequestID = fmt.Sprintf("runtime/%s/%d", scope.RequestID, row.Revision)
	_, err = e.commit(ctx, coordination.Commit{Scope: commitScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: row.ID, RoleID: scope.RoleID, ExpectedRevision: row.Revision, Body: body(saved)}}})
	return err
}

func (e *Engine) loadOwnedRetryRuntime(ctx context.Context, scope coordination.ExecutionScope, request Request) (*conversationstream.ActiveTurnState, error) {
	if len(request.RetryRequestID) > 128 || request.RetryRequestID == request.RequestID {
		return nil, coordination.ErrWrongOwner
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return nil, ErrUncertainExecution
	}
	row, err := port.Resource(ctx, scope, "checkpoint", "turn/"+request.RetryRequestID)
	if err != nil {
		return nil, err
	}
	if row == nil || row.Deleted {
		return nil, ErrConversationUnavailable
	}
	var saved checkpoint
	if json.Unmarshal(row.Body, &saved) != nil || saved.ConversationID != request.ConversationID || saved.Scope == nil || !sameQueueAuthority(scope, *saved.Scope) {
		return nil, coordination.ErrWrongOwner
	}
	if saved.Status == "completed" || ownedRuntimeManager.HasExecution(RuntimeKey(scope, request.ConversationID), saved.Scope.TurnID) {
		return nil, ErrUncertainExecution
	}
	input, err := port.Resource(ctx, scope, "message", request.RetryRequestID+"/user")
	if err != nil {
		return nil, err
	}
	if input == nil || input.Deleted {
		return nil, ErrUncertainExecution
	}
	var user struct {
		ConversationID string       `json:"conversationId"`
		Content        string       `json:"content"`
		Attachments    []Attachment `json:"attachments"`
	}
	if json.Unmarshal(input.Body, &user) != nil || user.ConversationID != request.ConversationID || user.Content != request.Message || string(body(user.Attachments)) != string(body(request.Attachments)) {
		return nil, coordination.ErrRequestConflict
	}
	runtime := saved.Runtime
	if runtime == nil || runtime.RequestID != request.RetryRequestID || runtime.TurnID != saved.Scope.TurnID || runtime.ExecutionID != saved.Scope.ExecutionID {
		return nil, ErrUncertainExecution
	}
	for _, block := range runtime.Blocks {
		if block.Type == "tool_call" && (block.Status != "completed" && block.Status != "failed" || block.CallID == "" || block.Payload["result"] == nil) {
			return nil, ErrUncertainExecution
		}
	}
	return runtime, nil
}

func (e *Engine) savedRuntime(ctx context.Context, scope coordination.ExecutionScope, conversation string) (*conversationstream.ActiveTurnState, error) {
	if runtime := ownedRuntimeManager.RuntimeSnapshot(RuntimeKey(scope, conversation)).ActiveTurn; runtime != nil && runtime.RequestID == scope.RequestID {
		return runtime, nil
	}
	if port, ok := e.data.(coordination.ResourcePort); ok {
		row, err := port.Resource(ctx, scope, "checkpoint", "turn/"+scope.RequestID)
		if err != nil {
			return nil, err
		}
		if row != nil && !row.Deleted {
			var saved checkpoint
			if err := json.Unmarshal(row.Body, &saved); err != nil {
				return nil, err
			}
			return saved.Runtime, nil
		}
	}
	return nil, nil
}

func beginRuntime(ctx context.Context, inference Inference, interrupt context.CancelCauseFunc) (func(), error) {
	key := RuntimeKey(inference.Scope, inference.ConversationID)
	control, cancel, started := ownedRuntimeManager.BeginExecution(key, inference.Scope.TurnID)
	if !started {
		return nil, errors.New("当前会话已有正在执行的任务")
	}
	stop := context.AfterFunc(control, func() { interrupt(ErrInterrupted) })
	return func() { stop(); cancel(); ownedRuntimeManager.ClearExecution(key, inference.Scope.TurnID) }, nil
}

func publishRuntime(ctx context.Context, inference Inference, event Event) error {
	typeName, status := "", "running"
	payload := map[string]any{}
	block, call := "", ""
	switch event.Type {
	case "started":
		typeName = "turn.started"
	case "reasoning-completed":
		typeName, block, status = "reasoning.completed", "reasoning", "completed"
	case "output-completed":
		typeName, block, status = "text.completed", "text", "completed"
	case "delta":
		typeName, block = "text.delta", "text"
		if event.Reasoning {
			typeName, block = "reasoning.delta", "reasoning"
		}
		payload["delta"], payload["text"], payload["blockType"] = event.Text, event.Text, block
	case "tool-started", "tool-progress", "tool-completed":
		typeName = "tool.progress"
		if err := json.Unmarshal(event.Progress, &payload); err != nil {
			return err
		}
		call, _ = payload["toolCallId"].(string)
		if call == "" {
			return errors.New("工具进度缺少调用编号")
		}
		block = "tool/" + call
		payload["blockType"] = "tool_call"
		if event.Type == "tool-started" {
			typeName = "tool.started"
		} else if event.Type == "tool-completed" {
			typeName, status = "tool.completed", "completed"
			toolStatus := strings.ToUpper(strings.TrimSpace(fmt.Sprint(payload["status"])))
			errorCode, _ := payload["errorCode"].(string)
			failedStatus := toolStatus == "FAILED" || toolStatus == "FAILURE" || toolStatus == "ERROR" || toolStatus == "DENIED" || toolStatus == "REJECTED" || toolStatus == "CANCELLED" || toolStatus == "CANCELED" || toolStatus == "TIMED_OUT" || toolStatus == "TIMEOUT" || toolStatus == "UNAVAILABLE" || toolStatus == "UNKNOWN" || toolStatus == "UNCERTAIN"
			if payload["error"] != nil || strings.TrimSpace(errorCode) != "" || failedStatus {
				typeName, status = "tool.failed", "failed"
			}
		}
	case "task":
		typeName, payload["task"] = "task.updated", event.Task
	default:
		return nil
	}
	blockID := ""
	if block != "" {
		blockID = inference.Scope.TurnID + "/" + block
	}
	payload["spaceId"] = inference.Scope.SpaceID
	payload["conversationId"] = inference.ConversationID
	payload["characterId"] = inference.Scope.RoleID
	if inference.Ingress != nil {
		payload["source"] = inference.Ingress.Source
	}
	_, err := ownedRuntimeManager.Publish(ctx, conversationstream.AgentUIEvent{ConversationID: RuntimeKey(inference.Scope, inference.ConversationID), RequestID: inference.Scope.RequestID, TurnID: inference.Scope.TurnID, ExecutionID: inference.Scope.ExecutionID, BlockID: blockID, CallID: call, Type: typeName, Status: status, Payload: payload}, false)
	return err
}
