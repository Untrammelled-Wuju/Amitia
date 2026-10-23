package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) StorageInfo(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageInfo())
}

func (h *Handler) StorageMigrations(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageMigrations())
}

func (h *Handler) StorageMigrationsCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.CheckStorageMigrations())
}
