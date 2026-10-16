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
}

func registerMeshChatRuntimeRouter(mesh *gin.RouterGroup, services *AppServices, coreID string) {
	for _, operation := range []string{"snapshot", "turns", "events", "interrupt", "steer", "retry", "approval"} {
		operation := operation
		mesh.POST("/chat-runtime/"+operation, func(c *gin.Context) {
			actor := security.GetActor(c)
			if actor == nil || actor.DeviceID == "" {
				c.AbortWithStatus(401)
				return
			}
			var request ownedRuntimeRequest
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
			if c.ShouldBindJSON(&request) != nil || request.ExpectedScope == nil || request.ConversationID == "" || len(request.ConversationID) > 512 || len(request.Content) > 128<<10 || request.AfterSequence < 0 {
				c.JSON(400, gin.H{"message": "聊天运行控制参数无效"})
				return
			}
			request.SpaceID, request.DeviceID, request.CoreID = actor.SpaceID.String(), actor.DeviceID.String(), coreID
			if request.ExpectedScope.SpaceID != request.SpaceID || request.ExpectedScope.InitiatorDeviceID != request.DeviceID || request.ExpectedScope.TargetDeviceID != request.TargetDeviceID || request.ExpectedScope.CoreID != coreID || request.ExpectedScope.RoleID != request.RoleID {
				c.JSON(409, gin.H{"message": "聊天数据归属已变化，请刷新"})
				return
			}
			err := services.OwnedBusiness.WithExecutionScope(c.Request.Context(), *request.ExpectedScope, func(ctx context.Context) error {
				view, err := services.OwnedBusiness.Query(ctx, request.Request, coordination.DataQuery{ConversationID: request.ConversationID, Management: true, RuntimeTurns: true, Limit: 128})
				if err != nil {
					return err
				}
				request.ConversationID = view.ConversationID
				key := business.RuntimeKey(view.Scope, request.ConversationID)
				turns, messages, conversation := ownedRuntimeDocuments(view, key)
				manager := business.RuntimeManager()
				runtime := manager.RuntimeSnapshot(key)
				approvals := ownedRuntimeApprovals(services, view.Scope, request.ConversationID, turns)
				data := gin.H{"executionScope": view.Scope, "version": conversationstream.ProtocolVersion, "revision": len(turns), "conversation": conversation, "messages": messages, "turns": turns, "lastEventSequence": runtime.LastEventSequence, "activeTurn": runtime.ActiveTurn, "approvals": approvals, "messageHistory": gin.H{"hasMore": view.Snapshot.NextCursors["message"] != "", "nextCursor": view.Snapshot.NextCursors["message"]}, "turnHistory": gin.H{"hasMore": view.Snapshot.NextCursors["checkpoint"] != "", "nextCursor": view.Snapshot.NextCursors["checkpoint"]}}
				switch operation {
				case "snapshot", "turns":
					c.JSON(200, gin.H{"code": 200, "data": data})
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
			document["sourceOwnerId"], document["sourceScope"], document["sourceRevision"] = resource.OwnerID, view.Scope, resource.Revision
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
			turns = append(turns, map[string]any{"id": scope.TurnID, "conversationId": view.ConversationID, "characterId": view.Scope.RoleID, "requestId": id, "executionId": scope.ExecutionID, "userMessageId": id + "/user", "status": status, "createdAt": resource.UpdatedAt, "items": []any{}})
		}
	}
	for _, raw := range view.Snapshot.LegacyMessages {
		var item map[string]any
		if json.Unmarshal(raw, &item) == nil {
			messages = append(messages, item)
		}
	}
	sort.SliceStable(messages, func(i, j int) bool {
		return fmt.Sprint(messages[i]["createdAt"]) < fmt.Sprint(messages[j]["createdAt"])
	})
	sort.SliceStable(turns, func(i, j int) bool { return fmt.Sprint(turns[i]["createdAt"]) < fmt.Sprint(turns[j]["createdAt"]) })
	for index, turn := range turns {
		turn["sequence"] = index + 1
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
			if approval.RequestID == turn["requestId"] && approval.RequestID != "" {
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
	live, replay, covered, unsubscribe := business.RuntimeManager().Subscribe(key, uuid.NewString(), request.AfterSequence)
	defer unsubscribe()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	write := func(event conversationstream.AgentUIEvent) error {
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
		_, err = fmt.Fprintf(c.Writer, "event: agent_event\ndata: %s\n\n", raw)
		c.Writer.Flush()
		return err
	}
	if !covered {
		if _, err := fmt.Fprint(c.Writer, "event: snapshot_required\ndata: {}\n\n"); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}
	for _, event := range replay {
		if err := write(event); err != nil {
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
				return nil
			}
			if err := services.DeviceMesh.Coordination.Validate(c.Request.Context(), scope); err != nil {
				return err
			}
			if err := write(event); err != nil {
				return err
			}
		case <-ticker.C:
			if err := services.DeviceMesh.Coordination.Validate(c.Request.Context(), scope); err != nil {
				return err
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
	var input struct {
		Content     string                `json:"content"`
		Attachments []business.Attachment `json:"attachments"`
	}
	found := false
	for _, resource := range view.Snapshot.Resources {
		if resource.Kind == "tool-result" {
			return errors.New("执行存在已保存的能力结果，请先核对结果，再发送新的指令，系统不会自动重放副作用")
		}
		if resource.Kind == "message" && resource.ID == old+"/user" && json.Unmarshal(resource.Body, &input) == nil {
			found = true
		}
	}
	if !found || strings.TrimSpace(input.Content) == "" {
		return errors.New("原始输入不存在，无法重试")
	}
	current := request.Request
	current.RequestID, current.Message, current.Attachments = uuid.NewString(), input.Content, input.Attachments
	ctx := context.WithoutCancel(c.Request.Context())
	go func() { _, _ = services.OwnedBusiness.Run(ctx, current) }()
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"conversationId": current.ConversationID, "requestId": current.RequestID, "status": "queued", "executionScope": view.Scope}})
	return nil
}
