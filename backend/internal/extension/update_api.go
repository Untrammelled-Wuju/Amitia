package extension

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
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
	migrations := group.Group("/migrations")
	migrations.GET("", api.listMigrations)
	migrations.GET("/:migrationId", api.getMigration)
	migrations.POST("/plan", api.planMigration)
	migrations.POST("/execute", api.executeMigration)
	migrations.GET("/operations/:operationId", api.getMigrationOperation)

	rollbacks := group.Group("/rollbacks")
	rollbacks.GET("", api.listRollbacks)
	rollbacks.GET("/:rollbackId", api.getRollback)
	rollbacks.GET("/:rollbackId/steps", api.listRollbackSteps)
	rollbacks.POST("/:rollbackId/execute", api.requirePackageRecovery)
	rollbacks.POST("/:rollbackId/recover", api.requirePackageRecovery)

	recovery := group.Group("/recovery")
	recovery.GET("/scan", api.scanRecovery)
	recovery.POST("/execute", api.requirePackageRecovery)

	journal := group.Group("/journal")
	journal.GET("/:operationId", api.listJournal)
}

func (api *UpdateAPI) listMigrations(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.MigrationRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "migration repository unavailable"})
		return
	}
	extensionID := c.Query("extensionId")
	defs, err := container.MigrationRepository.ListMigrationDefinitions(c.Request.Context(), extensionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": defs, "total": len(defs)})
}

func (api *UpdateAPI) getMigration(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.MigrationRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "migration repository unavailable"})
		return
	}
	migrationID := c.Param("migrationId")
	if migrationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "migrationId is required"})
		return
	}
	def, err := container.MigrationRepository.GetMigrationDefinition(c.Request.Context(), migrationID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": def})
}

func (api *UpdateAPI) planMigration(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.MigrationPlanner == nil || container.MigrationRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "migration planner unavailable"})
		return
	}
	var body struct {
		ExtensionID        string `json:"extensionId"`
		FromVersion        string `json:"fromVersion"`
		ToVersion          string `json:"toVersion"`
		FromDefinitionHash string `json:"fromDefinitionHash"`
		ToDefinitionHash   string `json:"toDefinitionHash"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if body.ExtensionID == "" || body.FromVersion == "" || body.ToVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId, fromVersion and toVersion are required"})
		return
	}
	defs, err := container.MigrationRepository.ListMigrationDefinitions(c.Request.Context(), body.ExtensionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	input := migration.MigrationPlanInput{
		ExtensionID:         body.ExtensionID,
		FromVersion:         body.FromVersion,
		ToVersion:           body.ToVersion,
		FromDefinitionHash:  body.FromDefinitionHash,
		ToDefinitionHash:    body.ToDefinitionHash,
		AvailableMigrations: defs,
	}
	output, err := container.MigrationPlanner.PlanMigration(input)
	if err != nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "no path") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": output})
}

func (api *UpdateAPI) executeMigration(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{"code": "MIGRATION_PACKAGE_CONTEXT_REQUIRED", "error": "独立迁移缺少经过验证的扩展包和更新事务上下文，请通过扩展包更新预览、确认并执行迁移"})
}

func (api *UpdateAPI) requirePackageRecovery(c *gin.Context) {
	c.JSON(http.StatusConflict, gin.H{"code": "PACKAGE_RECOVERY_CONTEXT_REQUIRED", "error": "旧回滚或恢复操作缺少扩展包快照与真实资源恢复上下文，请通过扩展包版本回滚流程执行"})
}

func (api *UpdateAPI) getMigrationOperation(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.MigrationExecutor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "migration executor unavailable"})
		return
	}
	operationID := c.Param("operationId")
	if operationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "operationId is required"})
		return
	}
	op, err := container.MigrationExecutor.GetMigrationStatus(c.Request.Context(), operationID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": op})
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
