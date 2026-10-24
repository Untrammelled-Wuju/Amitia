package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/middleware/security"
)

type ownedRuntimeRequest struct {
	business.Request
	TurnID        string `json:"turnId"`
	Content       string `json:"content"`
	ApprovalID    string `json:"approvalId"`
	Approved      bool   `json:"approved"`
	AfterSequence int64  `json:"afterSequence"`
	Cursor        string `json:"cursor"`
	Limit         int    `json:"limit"`
}

func registerMeshChatRuntimeRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	for _, operation := range []string{"snapshot", "turns", "events", "interrupt", "steer", "retry", "approval", "queue/status"} {
		operation := operation
		mesh.POST("/chat-runtime/"+operation, func(c *gin.Context) {
			actor := security.GetActor(c)
			if actor == nil || actor.DeviceID == "" {
				c.AbortWithStatus(401)
				return
			}
			var request ownedRuntimeRequest
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
			if c.ShouldBindJSON(&request) != nil || request.ExpectedScope == nil || request.ConversationID == "" || len(request.ConversationID) > 512 || len(request.Content) > 128<<10 || request.AfterSequence < 0 || len(request.Cursor) > 8192 || request.Limit < 0 || request.Limit > 128 {
				c.JSON(400, gin.H{"message": "聊天运行控制参数无效"})
				return
			}
			if operation == "events" && validateOwnedRuntimeEventCursor(request.AfterSequence, c.GetHeader("Last-Event-ID")) != nil {
				c.JSON(400, gin.H{"message": "事件游标与请求不匹配，请重新加载快照"})
				return
			}
			request.SpaceID, request.DeviceID, request.CoreID = actor.SpaceID.String(), actor.DeviceID.String(), coreID
			if request.ExpectedScope.SpaceID != request.SpaceID || request.ExpectedScope.InitiatorDeviceID != request.DeviceID || request.ExpectedScope.TargetDeviceID != request.TargetDeviceID || request.ExpectedScope.CoreID != coreID || request.ExpectedScope.RoleID != request.RoleID {
				c.JSON(409, gin.H{"message": "聊天数据归属已变化，请刷新"})
				return
			}
			err := services.OwnedBusiness.WithExecutionScope(c.Request.Context(), *request.ExpectedScope, func(ctx context.Context) error {
				if operation == "queue/status" {
					status, err := services.OwnedBusiness.ResumeQueuedChat(ctx, *request.ExpectedScope, request.RequestID, request.ConversationID)
					if err != nil {
						return err
					}
					if status.ConversationID != request.ConversationID {
						return coordination.ErrWrongOwner
					}
					c.JSON(200, gin.H{"code": 200, "data": status})
					return nil
				}
				query := coordination.DataQuery{ConversationID: request.ConversationID, Management: true, RuntimeTurns: true, Limit: request.Limit}
				if operation == "turns" {
					query.ResourceKind, query.Cursor = "checkpoint", request.Cursor
				}
				view, err := services.OwnedBusiness.Query(ctx, request.Request, query)
				if err != nil {
					return err
				}
				request.ConversationID = view.ConversationID
				key := business.RuntimeKey(view.Scope, request.ConversationID)
				turns, messages, conversation := ownedRuntimeDocuments(view, key)
				if request.TurnID != "" && operation != "snapshot" && operation != "turns" && operation != "events" {
					turns, err = loadOwnedRuntimeTarget(ctx, services.OwnedBusiness, request.Request, request.TurnID, view, turns)
					if err != nil {
						return err
					}
				}
				manager := business.RuntimeManager()
				runtime := manager.RuntimeSnapshot(key)
				if operation == "snapshot" || operation == "events" {
					runtime, err = manager.RuntimeSnapshotCurrent(ctx, key)
					if err != nil {
						return err
					}
				}
				approvals := ownedRuntimeApprovals(services, view.Scope, request.ConversationID, turns)
				data := gin.H{"executionScope": view.Scope, "version": conversationstream.ProtocolVersion, "revision": len(turns), "conversation": conversation, "messages": messages, "turns": turns, "lastEventSequence": runtime.LastEventSequence, "activeTurn": runtime.ActiveTurn, "approvals": approvals, "messageHistory": gin.H{"hasMore": view.Snapshot.NextCursors["message"] != "", "nextCursor": view.Snapshot.NextCursors["message"]}, "turnHistory": gin.H{"hasMore": view.Snapshot.NextCursors["checkpoint"] != "", "nextCursor": view.Snapshot.NextCursors["checkpoint"]}}
				switch operation {
				case "snapshot":
					queued, err := services.OwnedBusiness.ResumeConversationQueuedChats(ctx, request.Request)
					if err != nil {
						return err
					}
					data["queuedRequests"] = queued
					c.JSON(200, gin.H{"code": 200, "data": data})
					return nil
				case "turns":
					c.JSON(200, gin.H{"code": 200, "data": gin.H{"executionScope": view.Scope, "items": turns, "hasMore": view.Snapshot.NextCursors["checkpoint"] != "", "nextCursor": view.Snapshot.NextCursors["checkpoint"]}})
					return nil
				case "events":
					return streamOwnedRuntime(c, services, request, key, view.Scope)
				case "retry":
					return startOwnedRetry(c, services, request, view, turns)
				}
				var target map[string]any
				for _, turn := range turns {
					if turn["id"] == request.TurnID {
						target = turn
						break
					}
				}
				if target == nil {
					return errors.New("目标执行不存在或不属于当前会话")
				}
				switch operation {
				case "interrupt":
					if !manager.Interrupt(key, request.TurnID) {
						return errors.New("目标执行当前不可中断")
					}
					data = gin.H{"interrupted": true, "turnId": request.TurnID, "requestId": target["requestId"], "executionScope": view.Scope}
				case "steer":
					if strings.TrimSpace(request.Content) == "" || !manager.Steer(key, request.TurnID, request.Content) {
						return errors.New("目标执行当前不可干预")
					}
					if err := publishOwnedControl(ctx, key, target, "turn.steered", "running", gin.H{"content": request.Content}); err != nil {
						return err
					}
					data = gin.H{"accepted": true, "turnId": request.TurnID, "executionScope": view.Scope}
				case "approval":
					found := false
					for _, approval := range approvals {
						if approval["id"] == request.ApprovalID && approval["requestId"] == target["requestId"] {
							found = true
							break
						}
					}
					if !found {
						return errors.New("审批不存在或不属于目标执行")
					}
					if err := services.KernelContainer.ApprovalBroker.Resolve(request.ApprovalID, request.Approved); err != nil {
						return err
					}
					data = gin.H{"approved": request.Approved, "id": request.ApprovalID, "turnId": request.TurnID, "executionScope": view.Scope}
				}
				c.JSON(200, gin.H{"code": 200, "data": data})
				return nil
			})
			if err != nil && !c.Writer.Written() {
				c.JSON(409, gin.H{"message": err.Error()})
			}
		})
	}
}

func publishOwnedControl(ctx context.Context, key string, target map[string]any, event, status string, payload map[string]any) error {
	request, _ := target["requestId"].(string)
	turn, _ := target["id"].(string)
	execution, _ := target["executionId"].(string)
	_, err := business.RuntimeManager().Publish(ctx, conversationstream.AgentUIEvent{ConversationID: key, RequestID: request, TurnID: turn, ExecutionID: execution, Type: event, Status: status, Payload: payload}, false)
	return err
}

func loadOwnedRuntimeTarget(ctx context.Context, engine *business.Engine, request business.Request, turnID string, view business.QueryResult, turns []map[string]any) ([]map[string]any, error) {
	for {
		for _, turn := range turns {
			if turn["id"] == turnID {
				return turns, nil
			}
		}
		cursor := view.Snapshot.NextCursors["checkpoint"]
		if cursor == "" {
			return nil, errors.New("目标执行不存在或不属于当前会话")
		}
		var err error
		view, err = engine.Query(ctx, request, coordination.DataQuery{ConversationID: view.ConversationID, Management: true, RuntimeTurns: true, ResourceKind: "checkpoint", Cursor: cursor, Limit: 128})
		if err != nil {
			return nil, err
		}
		if view.Snapshot.NextCursors["checkpoint"] == cursor {
			return nil, coordination.ErrRequestConflict
		}
		turns, _, _ = ownedRuntimeDocuments(view, business.RuntimeKey(view.Scope, view.ConversationID))
	}
}

func ownedRuntimeDocuments(view business.QueryResult, key string) ([]map[string]any, []map[string]any, map[string]any) {
	turns, messages := make([]map[string]any, 0), make([]map[string]any, 0)
	conversation := map[string]any{"id": view.ConversationID}
	for _, resource := range view.Snapshot.Resources {
		if resource.Deleted {
			continue
		}
		var document map[string]any
		if json.Unmarshal(resource.Body, &document) != nil {
			continue
		}
		if document["conversationId"] != view.ConversationID && resource.Kind != "conversation" {
			continue
		}
		switch resource.Kind {
		case "conversation":
			if resource.ID == view.ConversationID {
				conversation = document
			}
		case "message":
			document["sourceOwnerId"], document["sourceScope"], document["origin"], document["readOnly"] = resource.OwnerID, view.Scope, resource.Origin, resource.ReadOnly
			if resource.Origin == "legacy" {
				document["sourceRevision"] = nil
			} else {
				document["sourceRevision"] = resource.Revision
			}
			messages = append(messages, document)
		case "checkpoint":
			if !strings.HasPrefix(resource.ID, "turn/") {
				continue
			}
			var scope coordination.ExecutionScope
			if raw, err := json.Marshal(document["scope"]); err == nil {
				_ = json.Unmarshal(raw, &scope)
			}
			if response, ok := document["response"].(map[string]any); ok {
				raw, _ := json.Marshal(response["executionScope"])
				_ = json.Unmarshal(raw, &scope)
			}
			status, _ := document["status"].(string)
			if status == "running" && !business.RuntimeManager().HasExecution(key, scope.TurnID) {
				status = "interrupted"
			}
			id := strings.TrimPrefix(resource.ID, "turn/")
			var runtime conversationstream.ActiveTurnState
			raw, _ := json.Marshal(document["runtime"])
			_ = json.Unmarshal(raw, &runtime)
			items := ownedRuntimeItems(runtime, view.ConversationID, status)
			turns = append(turns, map[string]any{"id": scope.TurnID, "conversationId": view.ConversationID, "characterId": view.Scope.RoleID, "requestId": id, "executionId": scope.ExecutionID, "userMessageId": id + "/user", "status": status, "createdAt": resource.UpdatedAt, "items": items})
		}
	}
	sort.SliceStable(messages, func(i, j int) bool {
		return fmt.Sprint(messages[i]["createdAt"]) < fmt.Sprint(messages[j]["createdAt"])
	})
	sort.SliceStable(turns, func(i, j int) bool { return fmt.Sprint(turns[i]["createdAt"]) < fmt.Sprint(turns[j]["createdAt"]) })
	for _, turn := range turns {
		created, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(turn["createdAt"]))
		turn["sequence"] = created.UnixMilli()
	}
	return turns, messages, conversation
}

func ownedRuntimeApprovals(services *AppServices, scope coordination.ExecutionScope, conversation string, turns []map[string]any) []map[string]any {
	result := make([]map[string]any, 0)
	if services.KernelContainer == nil || services.KernelContainer.ApprovalBroker == nil {
		return result
	}
	for _, approval := range services.KernelContainer.ApprovalBroker.List(scope.SpaceID, conversation) {
		if approval.RuntimeKey != business.RuntimeKey(scope, conversation) {
			continue
		}
		for _, turn := range turns {
			if approval.RequestID == turn["requestId"] && approval.RequestID != "" && approval.TurnID == turn["id"] {
				raw, _ := json.Marshal(approval)
				var item map[string]any
				_ = json.Unmarshal(raw, &item)
				item["turnId"] = turn["id"]
				result = append(result, item)
				break
			}
		}
	}
	return result
}

func streamOwnedRuntime(c *gin.Context, services *AppServices, request ownedRuntimeRequest, key string, scope coordination.ExecutionScope) error {
	live, replay, covered, unsubscribe, err := business.RuntimeManager().SubscribeCurrent(c.Request.Context(), key, uuid.NewString(), request.AfterSequence)
	if err != nil {
		return err
	}
	defer unsubscribe()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	write := func(event conversationstream.AgentUIEvent) error {
		if event.ConversationID != key {
			return coordination.ErrWrongOwner
		}
		if err := services.DeviceMesh.Coordination.Validate(c.Request.Context(), scope); err != nil {
			return err
		}
		event.ConversationID = request.ConversationID
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		var envelope map[string]any
		_ = json.Unmarshal(raw, &envelope)
		envelope["executionScope"] = scope
		raw, err = json.Marshal(envelope)
		if err != nil {
			return err
		}
		resync, err := writeOwnedRuntimeSequencedFrame(c.Writer, request.AfterSequence, event.EventSequence, raw)
		c.Writer.Flush()
		if resync && err == nil {
			return errOwnedRuntimeSnapshotRequired
		}
		if err == nil {
			request.AfterSequence = event.EventSequence
		}
		return err
	}
	if !covered {
		if err := writeOwnedRuntimeSnapshotRequired(c.Writer); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}
	for _, event := range replay {
		if err := write(event); err != nil {
			if errors.Is(err, errOwnedRuntimeSnapshotRequired) {
				return nil
			}
			return err
		}
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return c.Request.Context().Err()
		case event, ok := <-live:
			if !ok {
				latest, err := business.RuntimeManager().LatestSequence(c.Request.Context(), key)
				if err != nil {
					return err
				}
				if ownedRuntimeDurableCursorAdvanced(request.AfterSequence, latest) {
					if err := writeOwnedRuntimeSnapshotRequired(c.Writer); err != nil {
						return err
					}
					c.Writer.Flush()
				}
				return nil
			}
			if err := services.DeviceMesh.Coordination.Validate(c.Request.Context(), scope); err != nil {
				return err
			}
			if err := write(event); err != nil {
				if errors.Is(err, errOwnedRuntimeSnapshotRequired) {
					return nil
				}
				return err
			}
		case <-ticker.C:
			if err := services.DeviceMesh.Coordination.Validate(c.Request.Context(), scope); err != nil {
				return err
			}
			latest, err := business.RuntimeManager().LatestSequence(c.Request.Context(), key)
			if err != nil {
				return err
			}
			if ownedRuntimeDurableCursorAdvanced(request.AfterSequence, latest) {
				if err := writeOwnedRuntimeSnapshotRequired(c.Writer); err != nil {
					return err
				}
				c.Writer.Flush()
				return nil
			}
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return err
			}
			c.Writer.Flush()
		}
	}
}

func startOwnedRetry(c *gin.Context, services *AppServices, request ownedRuntimeRequest, view business.QueryResult, turns []map[string]any) error {
	var source map[string]any
	for _, turn := range turns {
		if turn["id"] == request.TurnID {
			source = turn
			break
		}
	}
	if source == nil || source["status"] != "failed" && source["status"] != "interrupted" {
		return errors.New("当前执行不可重试")
	}
	old, _ := source["requestId"].(string)
	inputView, err := services.OwnedBusiness.Query(c.Request.Context(), request.Request, coordination.DataQuery{ConversationID: request.ConversationID, Management: true, ResourceKind: "message", ResourceID: old + "/user", Limit: 1})
	if err != nil {
		return err
	}
	var input struct {
		Content     string                `json:"content"`
		Attachments []business.Attachment `json:"attachments"`
	}
	found := false
	for _, resource := range inputView.Snapshot.Resources {
		if resource.Kind == "message" && resource.ID == old+"/user" && json.Unmarshal(resource.Body, &input) == nil {
			found = true
		}
	}
	if !found || strings.TrimSpace(input.Content) == "" {
		return errors.New("原始输入不存在，无法重试")
	}
	current := request.Request
	current.RequestID, current.Message, current.Attachments = uuid.NewString(), input.Content, input.Attachments
	current.RetryRequestID = old
	queued, err := services.OwnedBusiness.QueueChat(c.Request.Context(), view.Scope, current)
	if err != nil {
		return err
	}
	c.JSON(200, gin.H{"code": 200, "data": queued})
	return nil
}

func ownedToolResultRequest(raw json.RawMessage) string {
	var result struct {
		Scope coordination.ExecutionScope `json:"executionScope"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return ""
	}
	return result.Scope.RequestID
}

func ownedRuntimeItems(runtime conversationstream.ActiveTurnState, conversation, status string) []map[string]any {
	items := make([]map[string]any, 0, len(runtime.Blocks))
	for _, block := range runtime.Blocks {
		blockStatus := block.Status
		if status == "completed" || status == "failed" || status == "interrupted" {
			if blockStatus != "completed" && blockStatus != "failed" && blockStatus != "interrupted" {
				blockStatus = status
			}
		}
		item := map[string]any{"id": block.BlockID, "turnId": runtime.TurnID, "conversationId": conversation, "sequence": block.BlockSequence, "type": block.Type, "status": blockStatus, "revision": block.Revision, "callId": block.CallID, "content": block.Content}
		for source, target := range map[string]string{"arguments": "argumentsJson", "result": "resultJson", "toolName": "toolName", "errorCode": "errorCode", "durationMs": "durationMs"} {
			if value, ok := block.Payload[source]; ok {
				item[target] = value
			}
		}
		items = append(items, item)
	}
	return items
}
