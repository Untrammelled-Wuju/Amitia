package system

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

type administratorSettingsService interface {
	AdministratorSettingsContext(context.Context, string, map[string]interface{}) (map[string]interface{}, error)
}

func (h *Handler) updateAdministratorSettings(c *gin.Context, operation string, bindBody bool) {
	var body map[string]interface{}
	if bindBody {
		if err := c.ShouldBindJSON(&body); err != nil {
			util.ErrorResponse(c, 400, err.Error(), nil)
			return
		}
	}
	svc, ok := h.service.(administratorSettingsService)
	if !ok {
		util.ErrorResponse(c, 409, "当前配置服务不支持可撤销的管理操作", nil)
		return
	}
	result, err := svc.AdministratorSettingsContext(c.Request.Context(), operation, body)
	if err != nil {
		util.ErrorResponse(c, 409, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}
