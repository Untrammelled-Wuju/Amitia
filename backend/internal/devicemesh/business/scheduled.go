package business

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type ScheduledMessage struct {
	MessageID      string                      `json:"messageId"`
	ConversationID string                      `json:"conversationId"`
	Content        string                      `json:"content"`
	Saved          bool                        `json:"saved"`
	Scope          coordination.ExecutionScope `json:"executionScope"`
}

func sameScheduledOwner(saved, current coordination.ExecutionScope) bool {
	return saved.SpaceID == current.SpaceID && saved.InitiatorDeviceID == current.InitiatorDeviceID && saved.TargetDeviceID == current.TargetDeviceID && saved.CoreID == current.CoreID && saved.RoleID == current.RoleID && saved.ResourceOwnerID == current.ResourceOwnerID && saved.RoleOwnerID == current.RoleOwnerID && saved.Coordinated == current.Coordinated
}

func (e *Engine) AuthorizeScheduledMessage(ctx context.Context, request Request, binding json.RawMessage) (coordination.ExecutionScope, error) {
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return scope, err
	}
	defer finish()
	if len(binding) > 0 {
		var saved coordination.ExecutionScope
		if err := json.Unmarshal(binding, &saved); err != nil {
			return scope, err
		}
		if saved.SpaceID != scope.SpaceID || saved.CoreID != scope.CoreID || saved.ResourceOwnerID != scope.ResourceOwnerID || saved.RoleOwnerID != scope.RoleOwnerID || saved.RoleID != scope.RoleID || saved.Coordinated != scope.Coordinated {
			return scope, coordination.ErrWrongOwner
		}
	}
	if request.ConversationID != "" {
		port, ok := e.data.(coordination.ResourcePort)
		if !ok {
			return scope, errors.New("日程对话数据端口未接通")
		}
		resource, err := port.Resource(ctx, scope, "conversation", request.ConversationID)
		if err != nil {
			return scope, err
		}
		if resource == nil {
			return scope, errors.New("来源对话不存在")
		}
		if resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID {
			return scope, coordination.ErrWrongOwner
		}
	}
	return scope, nil
}

func validateScheduledOwner(binding json.RawMessage, scope coordination.ExecutionScope) error {
	var saved coordination.ExecutionScope
	if err := json.Unmarshal(binding, &saved); err != nil {
		return err
	}
	if !sameScheduledOwner(saved, scope) {
		return coordination.ErrWrongOwner
	}
	return nil
}

type ScheduledCommitFence func(context.Context, func(context.Context) error) error

func (e *Engine) SendScheduledMessage(ctx context.Context, request Request, binding json.RawMessage, title, source string, fences ...ScheduledCommitFence) (ScheduledMessage, error) {
	if request.RequestID == "" || strings.TrimSpace(request.Message) == "" {
		return ScheduledMessage{}, errors.New("任务编号和内容不能为空")
	}
	request.ConversationID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.SpaceID+"\x00"+request.DeviceID+"\x00"+request.RequestID+"/conversation")).String()
	unlock := e.lock(request.SpaceID + "\x00" + request.DeviceID + "\x00" + request.ConversationID)
	defer unlock()
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return ScheduledMessage{}, err
	}
	defer finish()
	if err := validateScheduledOwner(binding, scope); err != nil {
		return ScheduledMessage{}, err
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return ScheduledMessage{}, errors.New("任务消息数据端口未接通")
	}
	checkpointID := "scheduled/" + request.RequestID
	previous, err := port.Resource(ctx, scope, "checkpoint", checkpointID)
	if err != nil {
		return ScheduledMessage{}, err
	}
	if previous != nil {
		var result ScheduledMessage
		if err := json.Unmarshal(previous.Body, &result); err != nil {
			return ScheduledMessage{}, err
		}
		if previous.Deleted || !result.Saved || result.ConversationID != request.ConversationID {
			return ScheduledMessage{}, ErrUncertainExecution
		}
		if len(fences) > 0 && fences[0] != nil {
			if err := coordination.CommitCurrentContext(ctx, func(guarded context.Context) error {
				return fences[0](guarded, func(context.Context) error { return nil })
			}); err != nil {
				return ScheduledMessage{}, err
			}
		}
		return result, nil
	}
	snapshot, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{RequestID: request.RequestID, ConversationID: request.ConversationID, Limit: 128})
	if err != nil {
		return ScheduledMessage{}, err
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return ScheduledMessage{}, err
	}
	generation, err := e.model.GenerateOwnedReply(ctx, Inference{Scope: scope, ConversationID: request.ConversationID, Snapshot: snapshot, PermissionMode: "ask", Message: request.Message, ScheduledMessage: true})
	if err != nil {
		return ScheduledMessage{}, err
	}
	if strings.TrimSpace(generation.Text) == "" {
		return ScheduledMessage{}, errors.New("AI 任务未生成有效内容")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return ScheduledMessage{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return ScheduledMessage{}, err
	}
	scope.TurnID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.RequestID+"/turn")).String()
	scope.ExecutionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.RequestID+"/execution")).String()
	result := ScheduledMessage{MessageID: request.RequestID + "/assistant", ConversationID: request.ConversationID, Content: generation.Text, Saved: true, Scope: scope}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	preview := ownedMessagePreview(generation.Text)
	mutations := []coordination.Mutation{
		{Kind: "conversation", ID: request.ConversationID, RoleID: scope.RoleID, Body: body(map[string]any{"id": request.ConversationID, "conversationId": request.ConversationID, "characterId": scope.RoleID, "title": title, "ownerId": scope.ResourceOwnerID, "coreId": scope.CoreID, "channel": "web", "source": source, "lastMessageSequence": 1, "lastMessagePreview": preview, "lastMessage": preview, "lastMessageRole": "assistant", "lastMessageType": "text", "createdAt": now, "updatedAt": now})},
		{Kind: "message", ID: result.MessageID, RoleID: scope.RoleID, Body: body(map[string]any{"id": result.MessageID, "conversationId": request.ConversationID, "characterId": scope.RoleID, "role": "assistant", "sequence": 1, "content": generation.Text, "reasoningContent": generation.Reasoning, "requestId": request.RequestID, "source": source, "executionScope": scope, "createdAt": now, "updatedAt": now})},
		{Kind: "checkpoint", ID: checkpointID, RoleID: scope.RoleID, Body: body(result)},
	}
	commit := func(commitCtx context.Context) error {
		return e.submit(coordination.WithScope(commitCtx, scope), scope, "scheduled-message", mutations)
	}
	if len(fences) > 0 && fences[0] != nil {
		err = coordination.CommitCurrentContext(ctx, func(guarded context.Context) error {
			return fences[0](guarded, commit)
		})
	} else {
		err = commit(ctx)
	}
	if err != nil {
		return ScheduledMessage{}, err
	}
	return result, nil
}
