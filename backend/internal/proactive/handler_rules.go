package proactive

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"strconv"
	"time"
)

func (h *Handler) ListRules(c *gin.Context) {
	characterID := c.Query("characterId")
	rules, err := h.service.ListRules(characterID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, rules)
}

func (h *Handler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "名称不能为空", nil)
		return
	}
	rule, err := h.service.CreateRule(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "规则创建成功", rule)
}

func (h *Handler) UpdateRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	rule, err := h.service.UpdateRule(id, updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "规则更新成功", rule)
}

func (h *Handler) DeleteRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.DeleteRule(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "规则已删除", nil)
}

func (h *Handler) ToggleRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	rule, err := h.service.ToggleRule(id)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "操作失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "状态已切换", rule)
}

func (h *Handler) Status(c *gin.Context) {
	characterID := c.Query("characterId")
	rules, _ := h.service.ListRules(characterID)
	enabled := 0
	total := len(rules)
	for _, r := range rules {
		if v, ok := r["enabled"]; ok {
			switch n := v.(type) {
			case int:
				if n == 1 {
					enabled++
				}
			case int64:
				if n == 1 {
					enabled++
				}
			case float64:
				if int(n) == 1 {
					enabled++
				}
			}
		}
	}
	util.SuccessResponse(c, gin.H{"schedulerRunning": SchedulerRunning, "enabledRuleCount": enabled, "totalRuleCount": total})
}

func (h *Handler) TestRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule ProactiveRule
	if err := h.db.First(&rule, id).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "规则不存在", nil)
		return
	}
	character, ok := resolveProactiveCharacter(h.db, rule.CharacterID, rule.ConversationID)
	if !ok {
		util.ErrorResponse(c, response.OperationFailed, "规则未绑定有效角色", nil)
		return
	}

	channel := rule.Channel
	if channel == "" {
		channel = "web"
	}
	convID := resolveProactiveConversation(h.db, rule.ConversationID, character.ID, channel, false)
	if convID == "" {
		convID = resolveProactiveConversation(h.db, "", character.ID, channel, false)
	}
	prompt := rule.PromptTemplate
	if prompt == "" {
		prompt = "发一条自然的主动消息。"
	}

	content, err := h.dispatchContent(c.Request.Context(), character.ID, convID, channel, prompt)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "AI生成失败："+err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{
		"id":             rule.ID,
		"tested":         true,
		"ruleName":       rule.Name,
		"messageContent": content,
		"channel":        channel,
		"safetyCheck":    gin.H{"safe": true},
	})
}

func (h *Handler) TriggerRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule ProactiveRule
	if err := h.db.First(&rule, id).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "规则不存在", nil)
		return
	}
	character, ok := resolveProactiveCharacter(h.db, rule.CharacterID, rule.ConversationID)
	if !ok {
		util.ErrorResponse(c, response.OperationFailed, "规则未绑定有效角色", nil)
		return
	}

	channel := rule.Channel
	if channel == "" {
		channel = "web"
	}
	convID := resolveProactiveConversation(h.db, rule.ConversationID, character.ID, channel, false)
	if convID == "" {
		util.ErrorResponse(c, response.OperationFailed, "无可用对话", nil)
		return
	}
	prompt := rule.PromptTemplate
	if prompt == "" {
		prompt = "发一条自然的主动消息。"
	}

	content, err := h.dispatchContent(c.Request.Context(), character.ID, convID, channel, prompt)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "AI生成失败："+err.Error(), nil)
		return
	}

	now := time.Now()
	h.db.Exec("UPDATE proactive_rules SET sent_count_today=sent_count_today+1, last_sent_at=?, updated_at=? WHERE id=?", now, now, rule.ID)
	util.SuccessResponse(c, gin.H{"id": rule.ID, "triggered": true, "messageContent": content, "channel": channel})
}

func (h *Handler) dispatchContent(ctx context.Context, characterID, convID, channel, prompt string) (string, error) {
	if h.compSvc == nil {
		return "", fmt.Errorf("主动消息统一派发未配置")
	}
	requestID := fmt.Sprintf("proactive-handler-%d", time.Now().UnixNano())
	return h.compSvc.DispatchProactiveMessage(ctx, characterID, convID, channel, prompt, requestID)
}
