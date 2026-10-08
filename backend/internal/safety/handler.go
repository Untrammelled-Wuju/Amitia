package safety

import (
	"encoding/json"
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
	value, err := h.settingValue(c, "safety_bdi_config")
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取配置失败", nil)
		return
	}
	if value == "" || value == "{}" {
		util.SuccessResponse(c, &BdiConfig{
			HardConstraints: []HardConstraint{},
			SoftPreferences: []SoftPreference{},
			CopingStrategy: &CopingStrategy{
				Selected:     "active",
				Alternatives: []string{},
			},
			EmotionExpression: &EmotionExpression{
				DisplayMode:       "show",
				InternalIntensity: 5,
				DisplayIntensity:  5,
			},
		})
		return
	}
	var cfg BdiConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		util.ErrorResponse(c, response.InternalError, "配置解析失败", nil)
		return
	}
	if cfg.HardConstraints == nil {
		cfg.HardConstraints = []HardConstraint{}
	}
	if cfg.SoftPreferences == nil {
		cfg.SoftPreferences = []SoftPreference{}
	}
	util.SuccessResponse(c, &cfg)
}

func (h *Handler) PutBdiConfig(c *gin.Context) {
	var body BdiConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	data, err := json.Marshal(body)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "序列化失败", nil)
		return
	}
	if err := h.saveSetting(c, "safety_bdi_config", string(data)); err != nil {
		util.ErrorResponse(c, response.InternalError, "保存配置失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "BDI 配置已保存", nil)
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
	value, err := h.settingValue(c, "safety_config")
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取配置失败", nil)
		return
	}
	if value == "" || value == "{}" {
		util.SuccessResponse(c, defaultSafetyConfig())
		return
	}
	var cfg SafetyConfig
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		util.ErrorResponse(c, response.InternalError, "配置解析失败", nil)
		return
	}
	util.SuccessResponse(c, &cfg)
}

func (h *Handler) PutConfig(c *gin.Context) {
	var body SafetyConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	data, err := json.Marshal(body)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "序列化失败", nil)
		return
	}
	if err := h.saveSetting(c, "safety_config", string(data)); err != nil {
		util.ErrorResponse(c, response.InternalError, "保存配置失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "安全配置已保存", nil)
}

func (h *Handler) settingValue(c *gin.Context, key string) (string, error) {
	var value string
	err := configwrite.Transaction(h.db.WithContext(c.Request.Context()), func(tx *gorm.DB) error {
		var err error
		var row struct{ Value string }
		err = tx.Table("app_settings").Select("value").Where("key = ? AND deleted_at IS NULL", key).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = nil
		}
		value = row.Value
		return err
	})
	return value, err
}

func (h *Handler) saveSetting(c *gin.Context, key, value string) error {
	return configwrite.Transaction(h.db.WithContext(c.Request.Context()), func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO app_settings (key, value, revision, updated_at) VALUES (?, ?, 1, CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = excluded.value, revision = app_settings.revision + 1, deleted_at = NULL, updated_at = excluded.updated_at", key, value).Error
	})
}
