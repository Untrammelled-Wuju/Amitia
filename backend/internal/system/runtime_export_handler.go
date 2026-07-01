package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) RuntimeExportSnapshot(c *gin.Context) {
	export := mindruntime.ExportRuntimeSnapshot()
	util.SuccessResponse(c, export)
}
