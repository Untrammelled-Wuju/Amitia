package safety

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/securityaudit"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

func (h *Handler) GetBdiConfig(c *gin.Context) {
	scope, err := ScopeFromContextWithCoordinator(c.Request.Context(), h.coordination)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	policy, err := NewRepository(h.db).Policy(c.Request.Context(), scope)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取配置失败", nil)
		return
	}
	policy.BDI.Revision = policy.Revision
	util.SuccessResponse(c, &policy.BDI)
}

func (h *Handler) PutBdiConfig(c *gin.Context) {
	var body BdiConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	scope, err := ScopeFromContextWithCoordinator(c.Request.Context(), h.coordination)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	policy, err := NewRepository(h.db).SaveBDI(c.Request.Context(), scope, body)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(409, gin.H{"code": 409, "msg": "BDI 安全策略已被其他操作更新，请重新加载"})
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存配置失败", nil)
		return
	}
	policy.BDI.Revision = policy.Revision
	util.SuccessResponse(c, &policy.BDI)
}

func (h *Handler) GetAuditLogs(c *gin.Context) {
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil || strings.TrimSpace(string(actor.SpaceID)) == "" {
		util.ErrorResponse(c, response.InternalError, "审计查询缺少可信空间身份", nil)
		return
	}
	scope, scoped := coordination.FromContext(c.Request.Context())
	if (actor.PrincipalType == auth.PrincipalTrustedDevice && !scoped) || (scoped && scope.SpaceID != string(actor.SpaceID)) {
		util.ErrorResponse(c, response.InternalError, "审计空间与当前授权不一致", nil)
		return
	}
	logs := make([]map[string]string, 0)
	err := configwrite.Transaction(h.db.WithContext(c.Request.Context()), func(tx *gorm.DB) error {
		var events []securityaudit.AuditEvent
		if err := tx.Where("space_id = ?", string(actor.SpaceID)).Order("occurred_at DESC, event_id DESC").Limit(50).Find(&events).Error; err != nil {
			return err
		}
		for _, event := range events {
			logs = append(logs, map[string]string{"id": event.EventID, "time": event.OccurredAt, "ruleId": event.ReasonCode, "action": event.EventType})
		}
		return nil
	})
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取审计记录失败", nil)
		return
	}
	util.SuccessResponse(c, logs)
}

func defaultSafetyConfig() *SafetyConfig {
	return &SafetyConfig{
		PreventEmotionalBlackmail:        true,
		PreventExclusiveDependency:       true,
		PreventRealityIsolation:          true,
		PreventPunitiveExpression:        true,
		PreventPretendingHuman:           true,
		PreventSensitiveProactiveMention: true,
		RestrictAdultContent:             true,
		NegativeEmotionCap:               5,
		IntimacyExpressionCap:            7,
		ViolationAction:                  "block",
		AuditLogRetentionDays:            30,
	}
}

func (h *Handler) GetConfig(c *gin.Context) {
	scope, err := ScopeFromContextWithCoordinator(c.Request.Context(), h.coordination)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	policy, err := NewRepository(h.db).Policy(c.Request.Context(), scope)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取配置失败", nil)
		return
	}
	policy.Config.Revision = policy.Revision
	util.SuccessResponse(c, &policy.Config)
}

func (h *Handler) PutConfig(c *gin.Context) {
	var body SafetyConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	scope, err := ScopeFromContextWithCoordinator(c.Request.Context(), h.coordination)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	policy, err := NewRepository(h.db).SaveConfig(c.Request.Context(), scope, body)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(409, gin.H{"code": 409, "msg": "安全配置已被其他操作更新，请重新加载"})
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存配置失败", nil)
		return
	}
	policy.Config.Revision = policy.Revision
	util.SuccessResponse(c, &policy.Config)
}
