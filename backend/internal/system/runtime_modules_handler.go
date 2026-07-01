package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) RuntimeModulesHealth(c *gin.Context) {
	results := mindruntime.RunAllModuleChecks()
	allHealthy := true
	for _, r := range results {
		if !r.Healthy {
			allHealthy = false
			break
		}
	}
	util.SuccessResponse(c, map[string]interface{}{
		"healthy": allHealthy,
		"modules": results,
	})
}
