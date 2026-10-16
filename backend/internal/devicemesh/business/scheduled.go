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
	Degraded       bool                        `json:"degraded"`
	Scope          coordination.ExecutionScope `json:"executionScope"`
}

func sameScheduledOwner(saved, current coordination.ExecutionScope) bool {
	return saved.SpaceID == current.SpaceID && saved.InitiatorDeviceID == current.InitiatorDeviceID && saved.TargetDeviceID == current.TargetDeviceID && saved.CoreID == current.CoreID && saved.RoleID == current.RoleID && saved.ResourceOwnerID == current.ResourceOwnerID && saved.RoleOwnerID == current.RoleOwnerID && saved.Coordinated == current.Coordinated
}

func (e *Engine) PrepareScheduledConversation(ctx context.Context, request Request, title string, binding json.RawMessage) (string, coordination.ExecutionScope, error) {
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return "", scope, err
	}
	defer finish()
	if len(binding) > 0 {
		var saved coordination.ExecutionScope
		if json.Unmarshal(binding, &saved) != nil || saved.SpaceID != scope.SpaceID || saved.CoreID != scope.CoreID || saved.ResourceOwnerID != scope.ResourceOwnerID || saved.RoleOwnerID != scope.RoleOwnerID || saved.RoleID != scope.RoleID || saved.Coordinated != scope.Coordinated {
			return "", scope, coordination.ErrWrongOwner
		}
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return "", scope, errors.New("日程对话数据端口未接通")
	}
	id := request.ConversationID
	create := id == ""
	if create {
		id = uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.SpaceID+"\x00"+request.DeviceID+"\x00"+request.RequestID+"/conversation")).String()
	}
	resource, err := port.Resource(ctx, scope, "conversation", id)
	if err != nil {
		return "", scope, err
	}
	if resource != nil {
		if resource.Deleted || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID {
			return "", scope, coordination.ErrWrongOwner
		}
		return id, scope, nil
	}
	if !create {
		return "", scope, errors.New("绑定对话不存在")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := e.submit(ctx, scope, "scheduled-conversation", []coordination.Mutation{{Kind: "conversation", ID: id, RoleID: scope.RoleID, Body: body(map[string]any{"id": id, "conversationId": id, "characterId": scope.RoleID, "title": title, "ownerId": scope.ResourceOwnerID, "coreId": scope.CoreID, "createdAt": now, "updatedAt": now})}}); err != nil {
		return "", scope, err
	}
	return id, scope, nil
}

type ScheduledCommitFence func(context.Context, func(context.Context) error) error

func (e *Engine) SendScheduledMessage(ctx context.Context, request Request, binding json.RawMessage, source, fallback string, fences ...ScheduledCommitFence) (ScheduledMessage, error) {
	if request.ConversationID == "" || request.RequestID == "" || strings.TrimSpace(request.Message) == "" {
		return ScheduledMessage{}, errors.New("任务对话、编号和内容不能为空")
	}
	unlock := e.lock(request.SpaceID + "\x00" + request.DeviceID + "\x00" + request.ConversationID)
	defer unlock()
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return ScheduledMessage{}, err
	}
	defer finish()
	var savedScope coordination.ExecutionScope
	if json.Unmarshal(binding, &savedScope) != nil || !sameScheduledOwner(savedScope, scope) {
		return ScheduledMessage{}, coordination.ErrWrongOwner
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return ScheduledMessage{}, errors.New("任务消息数据端口未接通")
	}
	conversation, err := port.Resource(ctx, scope, "conversation", request.ConversationID)
	if err != nil {
		return ScheduledMessage{}, err
	}
	if conversation == nil || conversation.Deleted || conversation.RoleID != scope.RoleID || conversation.OwnerID != scope.ResourceOwnerID {
		return ScheduledMessage{}, coordination.ErrWrongOwner
	}
	checkpointID := "scheduled/" + request.RequestID
	previous, err := port.Resource(ctx, scope, "checkpoint", checkpointID)
	if err != nil {
		return ScheduledMessage{}, err
	}
	if previous != nil {
		var result ScheduledMessage
		if previous.Deleted || json.Unmarshal(previous.Body, &result) != nil || !result.Saved || result.ConversationID != request.ConversationID {
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
	generation, generateErr := e.model.GenerateOwnedReply(ctx, Inference{Scope: scope, ConversationID: request.ConversationID, Snapshot: snapshot, PermissionMode: "ask", Message: request.Message, ScheduledMessage: true})
	degraded := false
	if generateErr != nil || strings.TrimSpace(generation.Text) == "" {
		if strings.TrimSpace(fallback) == "" {
			if generateErr != nil {
				return ScheduledMessage{}, generateErr
			}
			return ScheduledMessage{}, errors.New("主动任务未生成有效内容")
		}
		generation = Generation{Text: fallback}
		degraded = true
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return ScheduledMessage{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return ScheduledMessage{}, err
	}
	scope.TurnID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.RequestID+"/turn")).String()
	scope.ExecutionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(request.RequestID+"/execution")).String()
	result := ScheduledMessage{MessageID: request.RequestID + "/assistant", ConversationID: request.ConversationID, Content: generation.Text, Saved: true, Degraded: degraded, Scope: scope}
	var document map[string]any
	if json.Unmarshal(conversation.Body, &document) != nil {
		return ScheduledMessage{}, errors.New("绑定对话格式无效")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	document["updatedAt"] = now
	mutations := []coordination.Mutation{
		{Kind: "conversation", ID: request.ConversationID, RoleID: scope.RoleID, ExpectedRevision: conversation.Revision, Body: body(document)},
		{Kind: "message", ID: result.MessageID, RoleID: scope.RoleID, Body: body(map[string]any{"id": result.MessageID, "conversationId": request.ConversationID, "characterId": scope.RoleID, "role": "assistant", "content": generation.Text, "reasoningContent": generation.Reasoning, "requestId": request.RequestID, "source": source, "executionScope": scope, "createdAt": now, "updatedAt": now, "generationDegraded": degraded})},
		{Kind: "checkpoint", ID: checkpointID, RoleID: scope.RoleID, Body: body(result)},
	}
	commit := func(commitCtx context.Context) error {
		return e.submit(coordination.WithScope(commitCtx, scope), scope, "scheduled-message", mutations, coordination.ResourceVersion{Kind: "conversation", ID: request.ConversationID, Revision: conversation.Revision})
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
