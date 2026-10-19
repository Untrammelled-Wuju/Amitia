package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func ownedChatAdminTool(name string) bool {
	switch name {
	case "start_chat_service", "agent_status", "create_new_chat", "list_chats", "find_chat", "switch_chat", "update_chat_title", "delete_chat", "send_message_to_ai", "get_chat_messages", "get_chat_messages_range", "list_character_cards":
		return true
	default:
		return false
	}
}

func (c *serverAgentAdminController) executeOwnedChatAdmin(ctx context.Context, name string, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	scope, ok := coordination.FromContext(ctx)
	if !ok || c.owned == nil || scope.SpaceID != invocation.SpaceID || scope.RoleID != invocation.CharacterID {
		return nil, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	if name == "list_character_cards" {
		if c.ownedData == nil {
			return nil, errors.New("角色Owner端口尚未就绪")
		}
		roles, err := c.ownedData.Roles(ctx, scope)
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(roles))
		for _, role := range roles {
			items = append(items, map[string]any{"id": role.ID, "name": role.Name, "revision": role.Revision})
		}
		return jsonResult(map[string]any{"characters": items, "count": len(items)})
	}
	var payload struct {
		coordination.ChatManagementRequest
		Message     string `json:"message"`
		CharacterID string `json:"characterId"`
		Query       string `json:"query"`
		Limit       int    `json:"limit"`
	}
	if err := parseAdminInput(input, &payload); err != nil {
		return nil, err
	}
	if payload.CharacterID != "" && payload.CharacterID != scope.RoleID {
		return nil, coordination.ErrWrongOwner
	}
	request := business.Request{SpaceID: scope.SpaceID, DeviceID: scope.InitiatorDeviceID, TargetDeviceID: scope.TargetDeviceID, CoreID: scope.CoreID, RoleID: scope.RoleID, ExpectedScope: &scope}
	operation := payload.ChatManagementRequest
	operation.Limit = payload.Limit
	operation.ConversationOwnerID = scope.ResourceOwnerID
	if operation.RequestID == "" {
		operation.RequestID = invocation.InvocationID
	}
	if operation.RequestID == "" {
		return nil, errors.New("聊天工具缺少稳定调用编号")
	}
	switch name {
	case "send_message_to_ai":
		if strings.TrimSpace(payload.Message) == "" {
			return nil, errors.New("消息不能为空")
		}
		request.ConversationID = c.resolveConversationID(invocation, operation.ConversationID)
		request.RequestID, request.Message = operation.RequestID, payload.Message
		if request.ConversationID == "" {
			request.ConversationID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(scope.ResourceOwnerID+"\x00"+operation.RequestID)).String()
		}
		response, err := c.owned.QueueChat(business.WithIngress(ctx, business.Ingress{Channel: "web", Source: "agent_tool", SessionID: invocation.InvocationID, IsInternal: true}), scope, request)
		if err != nil {
			return nil, err
		}
		c.setSelectedConversation(invocation, response.ConversationID)
		return jsonResult(map[string]any{"response": response})
	case "start_chat_service", "agent_status":
		operation.Operation = "stats"
	case "create_new_chat":
		operation.Operation = "conversation/create"
	case "list_chats", "find_chat":
		operation.Operation = "conversations"
		if name == "find_chat" {
			operation.Keyword, operation.PageSize = payload.Query, payload.Limit
		}
	case "switch_chat":
		operation.Operation = "conversation/read"
	case "update_chat_title":
		operation.Operation = "conversation/rename"
	case "delete_chat":
		operation.Operation = "conversation/delete"
	case "get_chat_messages":
		operation.Operation = "search"
	case "get_chat_messages_range":
		operation.Operation = "message-range"
	}
	if operation.Operation != "conversation/create" && operation.Operation != "conversations" && operation.Operation != "stats" {
		operation.ConversationID = c.resolveConversationID(invocation, operation.ConversationID)
		if operation.ConversationID == "" {
			return nil, errors.New("会话编号不能为空")
		}
	}
	if name == "get_chat_messages_range" {
		operation.PageSize = 128
	}
	result, err := c.owned.ManageChat(ctx, request, operation)
	if err != nil {
		return nil, err
	}
	if name == "create_new_chat" || name == "switch_chat" {
		if id, ok := result["id"].(string); ok {
			c.setSelectedConversation(invocation, id)
		}
	}
	if name == "delete_chat" {
		c.clearSelectedConversation(invocation, operation.ConversationID)
	}
	if name == "start_chat_service" || name == "agent_status" {
		return jsonResult(map[string]any{"ready": true, "stats": result, "selectedConversationId": c.selectedConversation(invocation)})
	}
	return jsonResult(result)
}
