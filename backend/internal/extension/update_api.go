package extension

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel"
)

type UpdateAPI struct {
	runtime *Runtime
}

func NewUpdateAPI(runtime *Runtime) *UpdateAPI {
	return &UpdateAPI{runtime: runtime}
}

func (api *UpdateAPI) service(c *gin.Context) *kernel.Container {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return nil
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		return nil
	}
	return container
}

func (api *UpdateAPI) RegisterRoutes(group *gin.RouterGroup) {
	rollbacks := group.Group("/rollbacks")
	rollbacks.GET("", api.listRollbacks)
	rollbacks.GET("/:rollbackId", api.getRollback)
	rollbacks.GET("/:rollbackId/steps", api.listRollbackSteps)

	recovery := group.Group("/recovery")
	recovery.GET("/scan", api.scanRecovery)

	journal := group.Group("/journal")
	journal.GET("/:operationId", api.listJournal)
}

func (api *UpdateAPI) listRollbacks(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.RollbackRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "rollback repository unavailable"})
		return
	}
	extensionID := c.Query("extensionId")
	plans, err := container.RollbackRepository.ListRollbackPlans(c.Request.Context(), extensionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": plans, "total": len(plans)})
}

func (api *UpdateAPI) getRollback(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.RollbackExecutorV2 == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "rollback executor unavailable"})
		return
	}
	rollbackID := c.Param("rollbackId")
	if rollbackID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rollbackId is required"})
		return
	}
	plan, err := container.RollbackExecutorV2.GetRollback(c.Request.Context(), rollbackID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": plan})
}

func (api *UpdateAPI) listRollbackSteps(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.RollbackRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "rollback repository unavailable"})
		return
	}
	rollbackID := c.Param("rollbackId")
	if rollbackID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rollbackId is required"})
		return
	}
	steps, err := container.RollbackRepository.ListRollbackSteps(c.Request.Context(), rollbackID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": steps, "total": len(steps)})
}

func (api *UpdateAPI) scanRecovery(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.UpdateRecoveryManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "recovery manager unavailable"})
		return
	}
	actions, err := container.UpdateRecoveryManager.ScanOnStartup(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": actions, "total": len(actions)})
}

func (api *UpdateAPI) listJournal(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.JournalManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "journal manager unavailable"})
		return
	}
	operationID := c.Param("operationId")
	if operationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "operationId is required"})
		return
	}
	entries, err := container.JournalManager.ListEntries(c.Request.Context(), operationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": entries, "total": len(entries)})
}
