package system

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) commitAdministratorAction(c *gin.Context, action func() error) bool {
	if err := coordination.CommitCurrent(c.Request.Context(), action); err != nil {
		util.ErrorResponse(c, 409, err.Error(), nil)
		return false
	}
	return true
}

func (h *Handler) administratorAction(c *gin.Context, operation, id string) {
	svc, ok := h.service.(interface {
		AdministratorActionContext(context.Context, string, string) (map[string]interface{}, error)
	})
	if !ok {
		util.ErrorResponse(c, 409, "当前服务不支持可撤销的管理操作", nil)
		return
	}
	result, err := svc.AdministratorActionContext(c.Request.Context(), operation, id)
	if err != nil {
		util.ErrorResponse(c, 409, err.Error(), result)
		return
	}
	util.SuccessResponse(c, result)
}
