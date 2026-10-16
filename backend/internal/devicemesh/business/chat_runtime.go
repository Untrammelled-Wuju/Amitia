package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

var ownedRuntimeManager = conversationstream.NewManager()

func RuntimeManager() *conversationstream.Manager { return ownedRuntimeManager }

func RuntimeKey(scope coordination.ExecutionScope, conversation string) string {
	digest := sha256.Sum256(body([]string{scope.SpaceID, scope.CoreID, scope.ResourceOwnerID, scope.RoleID, conversation}))
	return "owned/" + hex.EncodeToString(digest[:])
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
	block := ""
	switch event.Type {
	case "started":
		typeName = "turn.started"
	case "delta":
		typeName, block = "block.delta", "text"
		if event.Reasoning {
			block = "reasoning"
		}
		payload["delta"], payload["text"], payload["blockType"] = event.Text, event.Text, block
	case "tool-progress":
		typeName = "tool.progress"
		if err := json.Unmarshal(event.Progress, &payload); err != nil {
			return err
		}
	case "task":
		typeName, payload["task"] = "task.updated", event.Task
	default:
		return nil
	}
	_, err := ownedRuntimeManager.Publish(ctx, conversationstream.AgentUIEvent{ConversationID: RuntimeKey(inference.Scope, inference.ConversationID), RequestID: inference.Scope.RequestID, TurnID: inference.Scope.TurnID, ExecutionID: inference.Scope.ExecutionID, BlockID: inference.Scope.TurnID + "/" + block, Type: typeName, Status: status, Payload: payload}, false)
	return err
}
