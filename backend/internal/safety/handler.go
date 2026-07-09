package safety

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) GetBdiConfig(c *gin.Context) {
	var value string
	h.db.Raw("SELECT value FROM app_settings WHERE key = 'safety_bdi_config' LIMIT 1").Row().Scan(&value)
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
	h.db.Exec("INSERT OR REPLACE INTO app_settings (key, value, updated_at) VALUES ('safety_bdi_config', ?, datetime('now', 'localtime'))", string(data))
	util.SuccessMsgResponse(c, "BDI 配置已保存", nil)
}

func (h *Handler) GetAuditLogs(c *gin.Context) {
	var logs []AuditLog
	h.db.Table("audit_logs").Order("time DESC").Limit(50).Find(&logs)
	if logs == nil {
		logs = []AuditLog{}
	}
	util.SuccessResponse(c, logs)
}
