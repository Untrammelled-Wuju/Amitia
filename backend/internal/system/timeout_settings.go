package system

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"github.com/u-ai/backend/pkg/util"
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
	if err := timeoutpolicy.Validate(settings); err != nil {
		return err
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if _, err = NewSettingsStore(s.db).Upsert(timeoutSettingsKey, string(raw)); err != nil {
		return err
	}
	timeoutpolicy.Configure(settings)
	return nil
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
	if err := h.service.UpdateTimeoutSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "保存超时设置失败"})
		return
	}
	c.Header("X-Amitia-Timeout-Disabled", strconv.FormatBool(settings.Disabled))
	c.Header("X-Amitia-Timeout-Seconds", strconv.Itoa(settings.Seconds))
	util.SuccessResponse(c, settings)
}
