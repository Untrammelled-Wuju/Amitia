// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	kernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

const ProactiveCommandToolID = "com.amitia/proactive/command"

type PluginToolExecutor interface {
	ExecuteTool(ctx context.Context, toolID string, input json.RawMessage, scope PluginExecutionScope, externalCallID string, idempotencyKey string) (PluginToolResult, bool)
}

type PluginExecutionScope struct {
	UserID         string
	CharacterID    string
	ConversationID string
	Channel        string
	SessionID      string
	TraceID        string
	RequestID      string
	ToolCallID     string
}

type PluginToolResult struct {
	Status      string
	Output      json.RawMessage
	VisibleText string
	ErrorCode   string
	ErrorText   string
}

type KernelPluginToolExecutor struct {
	Facade *kernel.ToolFacade
}

func (e KernelPluginToolExecutor) ExecuteTool(ctx context.Context, toolID string, input json.RawMessage, scope PluginExecutionScope, externalCallID string, idempotencyKey string) (PluginToolResult, bool) {
	if e.Facade == nil {
		return PluginToolResult{Status: "failed", ErrorCode: "KERNEL_UNAVAILABLE", ErrorText: "extension kernel unavailable"}, false
	}
	result, ok := e.Facade.ExecuteTool(ctx, kernelCapabilityID(toolID), input, kernel.LegacyScope{
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Channel:        scope.Channel,
		SessionID:      scope.SessionID,
		TraceID:        scope.TraceID,
		RequestID:      scope.RequestID,
		ToolCallID:     scope.ToolCallID,
	}, externalCallID, idempotencyKey)
	out := PluginToolResult{
		Status:      result.Status,
		Output:      result.Output,
		VisibleText: result.VisibleText,
	}
	if result.Error != nil {
		out.ErrorCode = result.Error.Code
		out.ErrorText = result.Error.Message
	}
	return out, ok
}

type PluginHandler struct {
	executor PluginToolExecutor
	db       *gorm.DB
}

func NewPluginHandler(executor PluginToolExecutor, db *gorm.DB) *PluginHandler {
	return &PluginHandler{executor: executor, db: db}
}

func (h *PluginHandler) ListRules(c *gin.Context) {
	h.command(c, "rules.list", nil)
}

func (h *PluginHandler) CreateRule(c *gin.Context) {
	payload, ok := readPayload(c)
	if !ok {
		return
	}
	h.command(c, "rules.create", payload)
}

func (h *PluginHandler) UpdateRule(c *gin.Context) {
	payload, ok := readPayload(c)
	if !ok {
		return
	}
	payload["id"] = parseID(c.Param("id"))
	h.command(c, "rules.update", payload)
}

func (h *PluginHandler) DeleteRule(c *gin.Context) {
	h.command(c, "rules.delete", gin.H{"id": parseID(c.Param("id"))})
}

func (h *PluginHandler) ToggleRule(c *gin.Context) {
	payload, _ := readPayload(c)
	if payload == nil {
		payload = gin.H{}
	}
	payload["id"] = parseID(c.Param("id"))
	h.command(c, "rules.toggle", payload)
}

func (h *PluginHandler) Status(c *gin.Context) {
	h.command(c, "status", nil)
}

func (h *PluginHandler) TestRule(c *gin.Context) {
	h.command(c, "rules.test", gin.H{"id": parseID(c.Param("id"))})
}

func (h *PluginHandler) TriggerRule(c *gin.Context) {
	h.command(c, "rules.trigger", gin.H{"id": parseID(c.Param("id"))})
}

func (h *PluginHandler) ResetPresets(c *gin.Context) {
	payload, _ := readPayload(c)
	if payload == nil {
		payload = gin.H{}
	}
	h.command(c, "presets.reset", payload)
}

func (h *PluginHandler) RuleMessages(c *gin.Context) {
	h.hostRuleMessages(c)
}

func (h *PluginHandler) ListTriggerHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	h.command(c, "history.list", gin.H{
		"page":     page,
		"pageSize": pageSize,
		"state":    strings.TrimSpace(c.Query("state")),
	})
}

func (h *PluginHandler) QueueSummary(c *gin.Context) {
	h.command(c, "queue.summary", nil)
}

func (h *PluginHandler) GetSettings(c *gin.Context) {
	h.command(c, "settings.get", nil)
}

func (h *PluginHandler) UpdateSettings(c *gin.Context) {
	payload, ok := readPayload(c)
	if !ok {
		return
	}
	h.command(c, "settings.update", payload)
}

func (h *PluginHandler) command(c *gin.Context, action string, payload map[string]any) {
	if h == nil || h.executor == nil {
		util.ErrorResponse(c, 503, "主动消息插件运行时不可用", nil)
		return
	}
	scope := pluginScope(c)
	if payload == nil {
		payload = map[string]any{}
	} else {
		payload = clonePayload(payload)
	}
	payload["scope"] = map[string]any{
		"userId":         scope.UserID,
		"characterId":    scope.CharacterID,
		"conversationId": scope.ConversationID,
		"channel":        scope.Channel,
	}
	input, err := json.Marshal(map[string]any{"action": action, "payload": payload})
	if err != nil {
		util.ErrorResponse(c, 400, "主动消息请求编码失败", nil)
		return
	}
	result, ok := h.executor.ExecuteTool(
		c.Request.Context(),
		ProactiveCommandToolID,
		input,
		scope,
		"",
		"",
	)
	if !ok || result.ErrorCode != "" || strings.EqualFold(result.Status, "failed") {
		message := result.ErrorText
		if message == "" {
			message = result.VisibleText
		}
		if message == "" {
			message = "主动消息插件执行失败"
		}
		util.ErrorResponse(c, 500, message, nil)
		return
	}
	data, err := decodePluginOutput(result)
	if err != nil {
		util.ErrorResponse(c, 500, "主动消息插件返回数据无效", nil)
		return
	}
	util.SuccessResponse(c, data)
}

func (h *PluginHandler) hostRuleMessages(c *gin.Context) {
	if h == nil || h.db == nil {
		util.ErrorResponse(c, 503, "宿主消息服务不可用", nil)
		return
	}
	id := parseID(c.Param("id"))
	if id <= 0 {
		util.ErrorResponse(c, 400, "规则不存在", nil)
		return
	}
	userID := requestidentity.ResolveGin(c, "")
	type hostMessage struct {
		ID             string `json:"id"`
		RequestID      string `json:"requestId"`
		Content        string `json:"content"`
		Role           string `json:"role"`
		Status         string `json:"status"`
		Channel        string `json:"channel"`
		ConversationID string `json:"conversationId"`
		CreatedAt      string `json:"createdAt"`
	}
	items := make([]hostMessage, 0)
	prefix := fmt.Sprintf("proactive:%d:%%", id)
	err := h.db.Table("messages AS m").
		Select("m.id, m.request_id, m.content, m.role, m.status, c.channel, m.conversation_id, m.created_at").
		Joins("JOIN conversations AS c ON c.id = m.conversation_id").
		Where("c.user_id = ? AND m.role = ? AND m.source = ? AND m.request_id LIKE ?", userID, "assistant", "proactive", prefix).
		Order("m.created_at DESC, m.sequence DESC").
		Limit(100).
		Scan(&items).Error
	if err != nil {
		util.ErrorResponse(c, 500, "查询主动消息失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": len(items)})
}

func pluginScope(c *gin.Context) PluginExecutionScope {
	return PluginExecutionScope{
		UserID:         requestidentity.ResolveGin(c, ""),
		CharacterID:    strings.TrimSpace(c.Query("characterId")),
		ConversationID: strings.TrimSpace(c.Query("conversationId")),
		Channel:        strings.TrimSpace(c.Query("channel")),
		TraceID:        c.GetString("request_id"),
		RequestID:      c.GetString("request_id"),
	}
}

func readPayload(c *gin.Context) (map[string]any, bool) {
	payload := map[string]any{}
	if c.Request.Body == nil {
		return payload, true
	}
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&payload); err != nil && err.Error() != "EOF" {
		util.ErrorResponse(c, 400, "请求参数格式错误", nil)
		return nil, false
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, true
}

func decodePluginOutput(result PluginToolResult) (any, error) {
	raw := result.Output
	if len(raw) == 0 {
		raw = json.RawMessage(result.VisibleText)
	}
	if len(raw) == 0 {
		return map[string]any{"ok": true}, nil
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func parseID(raw string) int {
	id, _ := strconv.Atoi(strings.TrimSpace(raw))
	return id
}

func clonePayload(payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		out[key] = value
	}
	return out
}

func kernelCapabilityID(value string) capability.CapabilityID {
	return capability.CapabilityID(strings.TrimSpace(value))
}
