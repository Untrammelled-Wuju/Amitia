package system

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

const timeoutSettingsKey = "operation_timeout_policy"

func (s *service) loadTimeoutSettings() {
	settings := timeoutpolicy.Default()
	if raw := s.getAppSetting(timeoutSettingsKey); raw != "" {
		var saved timeoutpolicy.Settings
		if json.Unmarshal([]byte(raw), &saved) == nil && timeoutpolicy.Validate(saved) == nil {
			settings = saved
		}
	}
	timeoutpolicy.Configure(settings)
}

func (s *service) UpdateTimeoutSettings(settings timeoutpolicy.Settings) error {
	return s.UpdateTimeoutSettingsContext(context.Background(), settings)
}

func (s *service) UpdateTimeoutSettingsContext(ctx context.Context, settings timeoutpolicy.Settings) error {
	if err := timeoutpolicy.Validate(settings); err != nil {
		return err
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return configwrite.TransactionAndApply(s.db.WithContext(ctx), func(tx *gorm.DB) error {
		_, err := NewSettingsStore(tx).Upsert(timeoutSettingsKey, string(raw))
		return err
	}, func() { timeoutpolicy.Configure(settings) })
}

func (h *Handler) TimeoutSettings(c *gin.Context) {
	settings, _ := timeoutpolicy.Current()
	util.SuccessResponse(c, settings)
}

func (h *Handler) UpdateTimeoutSettings(c *gin.Context) {
	var settings timeoutpolicy.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "超时设置格式错误"})
		return
	}
	if err := timeoutpolicy.Validate(settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
		return
	}
	svc, ok := h.service.(interface {
		UpdateTimeoutSettingsContext(context.Context, timeoutpolicy.Settings) error
	})
	if !ok {
		util.ErrorResponse(c, 409, "当前配置服务不支持可撤销的管理操作", nil)
		return
	}
	if err := svc.UpdateTimeoutSettingsContext(c.Request.Context(), settings); err != nil {
		util.ErrorResponse(c, 409, "保存超时设置失败", nil)
		return
	}
	c.Header("X-Amitia-Timeout-Disabled", strconv.FormatBool(settings.Disabled))
	c.Header("X-Amitia-Timeout-Seconds", strconv.Itoa(settings.Seconds))
	util.SuccessResponse(c, settings)
}
