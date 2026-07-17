package extension

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListPlugins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, err := h.service.ListPlugins(c.Request.Context(), page, pageSize)
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, items)
}

func (h *Handler) GetPlugin(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	item, err := h.service.GetPlugin(c.Request.Context(), scope, c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) EnablePlugin(c *gin.Context) {
	scope, ok := h.queryScope(c, false)
	if !ok {
		return
	}
	if err := h.service.EnablePlugin(c.Request.Context(), scope, c.Param("id")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) DisablePlugin(c *gin.Context) {
	scope, ok := h.queryScope(c, false)
	if !ok {
		return
	}
	if err := h.service.DisablePlugin(c.Request.Context(), scope, c.Param("id")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) ReloadPlugin(c *gin.Context) {
	if err := h.service.ReloadPlugin(c.Request.Context(), c.Param("id")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) GetPluginConfig(c *gin.Context) {
	scope, ok := h.queryScope(c, false)
	if !ok {
		return
	}
	config, err := h.service.GetPluginConfig(c.Request.Context(), scope, c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	var value any
	if json.Unmarshal(config, &value) != nil {
		value = map[string]any{}
	}
	success(c, value)
}

func (h *Handler) UpdatePluginConfig(c *gin.Context) {
	scope, ok := h.queryScope(c, false)
	if !ok {
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		h.problem(c, NewExtensionError(ErrPluginConfigInvalid, "Invalid plugin config request", err.Error(), false, err))
		return
	}
	if err := h.service.UpdatePluginConfig(c.Request.Context(), scope, c.Param("id"), raw); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) ResetPluginConfig(c *gin.Context) {
	scope, ok := h.queryScope(c, false)
	if !ok {
		return
	}
	if err := h.service.ResetPluginConfig(c.Request.Context(), scope, c.Param("id")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) GetPluginPermissions(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	items, err := h.service.GetPluginPermissions(c.Request.Context(), scope, c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, items)
}

func (h *Handler) UpdatePluginPermissions(c *gin.Context) {
	var body struct {
		Grants         []PermissionGrantInput `json:"grants"`
		CharacterID    string                 `json:"characterId"`
		ConversationID string                 `json:"conversationId"`
		Channel        string                 `json:"channel"`
		SessionID      string                 `json:"sessionId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid plugin permission request", err.Error(), false, err))
		return
	}
	scope := h.baseScope(c)
	scope.CharacterID, scope.ConversationID, scope.Channel, scope.SessionID = strings.TrimSpace(body.CharacterID), strings.TrimSpace(body.ConversationID), strings.TrimSpace(body.Channel), strings.TrimSpace(body.SessionID)
	if err := h.service.UpdatePluginPermissions(c.Request.Context(), scope, c.Param("id"), body.Grants); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) GetPluginHealth(c *gin.Context) {
	item, err := h.service.GetPluginHealth(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) ResetPluginCircuit(c *gin.Context) {
	if err := h.service.ResetPluginCircuit(c.Request.Context(), c.Param("id")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) GetPluginState(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	items, err := h.service.GetPluginStates(c.Request.Context(), scope, c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, items)
}

func (h *Handler) GetPluginSurface(c *gin.Context) {
	item, err := h.service.GetPluginSurface(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	var value any
	if json.Unmarshal(item, &value) != nil {
		value = map[string]any{}
	}
	success(c, value)
}

func (h *Handler) GetPluginSchedules(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	items, err := h.service.GetPluginSchedules(c.Request.Context(), scope, c.Param("id"))
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, items)
}

func (h *Handler) PausePluginSchedule(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	if err := h.service.SetPluginScheduleEnabled(c.Request.Context(), scope, c.Param("id"), c.Param("scheduleId"), false); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) ResumePluginSchedule(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	if err := h.service.SetPluginScheduleEnabled(c.Request.Context(), scope, c.Param("id"), c.Param("scheduleId"), true); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) GetPluginEvents(c *gin.Context) {
	h.listPluginEvents(c, c.Query("status"))
}

func (h *Handler) GetPluginDeadLetters(c *gin.Context) { h.listPluginEvents(c, "dead_letter") }

func (h *Handler) listPluginEvents(c *gin.Context, status string) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, err := h.service.GetPluginEvents(c.Request.Context(), scope, c.Param("id"), status, page, pageSize)
	if err != nil {
		h.problem(c, err)
		return
	}
	success(c, items)
}

func (h *Handler) RetryPluginEvent(c *gin.Context) {
	scope, ok := h.queryScope(c, true)
	if !ok {
		return
	}
	if err := h.service.RetryPluginEvent(c.Request.Context(), scope, c.Param("id"), c.Param("eventId")); err != nil {
		h.problem(c, err)
		return
	}
	success(c, nil)
}

func (h *Handler) ExecutePluginSurfaceAction(c *gin.Context) {
	var body struct {
		Input          json.RawMessage `json:"input"`
		CharacterID    string          `json:"characterId"`
		ConversationID string          `json:"conversationId"`
		Channel        string          `json:"channel"`
		SessionID      string          `json:"sessionId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "Invalid plugin action request", err.Error(), false, err))
		return
	}
	if strings.TrimSpace(body.CharacterID) == "" {
		h.problem(c, NewExtensionError(ErrSkillInputInvalid, "characterId is required", "", false, nil))
		return
	}
	scope := h.baseScope(c)
	scope.CharacterID, scope.ConversationID, scope.Channel, scope.SessionID = strings.TrimSpace(body.CharacterID), strings.TrimSpace(body.ConversationID), strings.TrimSpace(body.Channel), strings.TrimSpace(body.SessionID)
	result, err := h.service.ExecutePluginSurfaceAction(c.Request.Context(), scope, c.Param("id"), c.Param("actionId"), body.Input)
	if err != nil {
		h.problemWithResult(c, err, result)
		return
	}
	success(c, result)
}
