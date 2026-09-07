package proactive

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"strconv"
	"time"

	"gorm.io/gorm"
)

func (h *Handler) ListRules(c *gin.Context) {
	characterID := c.Query("characterId")
	var rules []map[string]interface{}
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rules, err = scoped.ListRulesForUser(characterID, requestidentity.ResolveGin(c, ""))
	} else {
		rules, err = h.service.ListRules(characterID)
	}
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
	var rule *ProactiveRule
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rule, err = scoped.CreateRuleForUser(&req, requestidentity.ResolveGin(c, ""))
	} else {
		rule, err = h.service.CreateRule(&req)
	}
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
	var rule *ProactiveRule
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rule, err = scoped.UpdateRuleForUser(id, updates, requestidentity.ResolveGin(c, ""))
	} else {
		rule, err = h.service.UpdateRule(id, updates)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "规则更新成功", rule)
}

func (h *Handler) DeleteRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		err = scoped.DeleteRuleForUser(id, requestidentity.ResolveGin(c, ""))
	} else {
		err = h.service.DeleteRule(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "规则已删除", nil)
}

func (h *Handler) ToggleRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule *ProactiveRule
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rule, err = scoped.ToggleRuleForUser(id, requestidentity.ResolveGin(c, ""))
	} else {
		rule, err = h.service.ToggleRule(id)
	}
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "操作失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "状态已切换", rule)
}

func (h *Handler) Status(c *gin.Context) {
	characterID := c.Query("characterId")
	var rules []map[string]interface{}
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rules, _ = scoped.ListRulesForUser(characterID, requestidentity.ResolveGin(c, ""))
	} else {
		rules, _ = h.service.ListRules(characterID)
	}
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
	userID := requestidentity.ResolveGin(c, "")
	var rule *ProactiveRule
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rule, err = scoped.FindRuleForUser(id, userID)
	} else {
		var fallback ProactiveRule
		err = h.db.First(&fallback, id).Error
		rule = &fallback
	}
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "规则不存在", nil)
		return
	}
	character, ok := resolveProactiveCharacterForUser(h.db, userID, rule.CharacterID, rule.ConversationID)
	if !ok {
		util.ErrorResponse(c, response.OperationFailed, "规则未绑定有效角色", nil)
		return
	}

	channel := rule.Channel
	if channel == "" {
		channel = "web"
	}
	convID := resolveProactiveConversationForUser(h.db, userID, rule.ConversationID, character.ID, channel, false)
	if convID == "" {
		convID = resolveProactiveConversationForUser(h.db, userID, "", character.ID, channel, false)
	}
	prompt := rule.PromptTemplate
	if prompt == "" {
		prompt = "发一条自然的主动消息。"
	}

	content, err := h.dispatchContent(c.Request.Context(), userID, character.ID, convID, channel, prompt)
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
	userID := requestidentity.ResolveGin(c, "")
	var rule *ProactiveRule
	var err error
	if scoped, ok := h.service.(scopedProactiveService); ok {
		rule, err = scoped.FindRuleForUser(id, userID)
	} else {
		var fallback ProactiveRule
		err = h.db.First(&fallback, id).Error
		rule = &fallback
	}
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "规则不存在", nil)
		return
	}
	character, ok := resolveProactiveCharacterForUser(h.db, userID, rule.CharacterID, rule.ConversationID)
	if !ok {
		util.ErrorResponse(c, response.OperationFailed, "规则未绑定有效角色", nil)
		return
	}

	channel := rule.Channel
	if channel == "" {
		channel = "web"
	}
	convID := resolveProactiveConversationForUser(h.db, userID, rule.ConversationID, character.ID, channel, false)
	if convID == "" {
		util.ErrorResponse(c, response.OperationFailed, "无可用对话", nil)
		return
	}
	prompt := rule.PromptTemplate
	if prompt == "" {
		prompt = "发一条自然的主动消息。"
	}

	content, err := h.dispatchContent(c.Request.Context(), userID, character.ID, convID, channel, prompt)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "AI生成失败："+err.Error(), nil)
		return
	}

	now := time.Now()
	proactiveOwnerQuery(h.db.Table("proactive_rules").Where("id = ?", rule.ID), userID).Updates(map[string]interface{}{"sent_count_today": gorm.Expr("sent_count_today + 1"), "last_sent_at": now, "updated_at": now})
	util.SuccessResponse(c, gin.H{"id": rule.ID, "triggered": true, "messageContent": content, "channel": channel})
}

func (h *Handler) dispatchContent(ctx context.Context, userID, characterID, convID, channel, prompt string) (string, error) {
	if h.compSvc == nil {
		return "", fmt.Errorf("主动消息统一派发未配置")
	}
	requestID := fmt.Sprintf("proactive-handler-%d", time.Now().UnixNano())
	return h.compSvc.DispatchProactiveMessage(ctx, userID, characterID, convID, channel, prompt, requestID)
}
