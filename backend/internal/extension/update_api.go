package extension

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
	"github.com/u-ai/backend/internal/extension/kernel/update"
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
	rollbacks.POST("/:rollbackId/execute", api.executeRollback)
	rollbacks.POST("/:rollbackId/recover", api.recoverRollback)

	recovery := group.Group("/recovery")
	recovery.GET("/scan", api.scanRecovery)
	recovery.POST("/execute", api.executeRecovery)

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
	container := api.service(c)
	if container == nil || container.MigrationExecutor == nil || container.MigrationRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "migration executor unavailable"})
		return
	}
	var body struct {
		ExtensionID        string `json:"extensionId"`
		FromVersion        string `json:"fromVersion"`
		ToVersion          string `json:"toVersion"`
		FromDefinitionHash string `json:"fromDefinitionHash"`
		ToDefinitionHash   string `json:"toDefinitionHash"`
		Handler            struct {
			Type  string `json:"type"`
			Entry string `json:"entry"`
		} `json:"handler"`
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
	handler := func(ctx context.Context, step migration.MigrationPathStep, def *migration.MigrationDefinition, checkpoint *migration.MigrationCheckpoint) (json.RawMessage, error) {
		return nil, nil
	}
	op, err := container.MigrationExecutor.PlanAndExecute(c.Request.Context(), input, handler)
	if err != nil && op == nil {
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "confirm") || strings.Contains(err.Error(), "no path") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": op})
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

func (api *UpdateAPI) executeRollback(c *gin.Context) {
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
	if err := container.RollbackExecutorV2.Execute(c.Request.Context(), plan); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, _ := container.RollbackExecutorV2.GetRollback(c.Request.Context(), rollbackID)
	c.JSON(http.StatusOK, gin.H{"item": updated})
}

func (api *UpdateAPI) recoverRollback(c *gin.Context) {
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
	if err := container.RollbackExecutorV2.RecoverRollback(c.Request.Context(), rollbackID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	plan, _ := container.RollbackExecutorV2.GetRollback(c.Request.Context(), rollbackID)
	c.JSON(http.StatusOK, gin.H{"item": plan})
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

func (api *UpdateAPI) executeRecovery(c *gin.Context) {
	container := api.service(c)
	if container == nil || container.UpdateRecoveryManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "recovery manager unavailable"})
		return
	}
	var body struct {
		OperationID string `json:"operationId"`
		Strategy    string `json:"strategy"`
		Detail      string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if body.OperationID == "" || body.Strategy == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "operationId and strategy are required"})
		return
	}
	action := update.RecoveryAction{
		OperationID: body.OperationID,
		Strategy:    body.Strategy,
		Detail:      body.Detail,
	}
	if err := container.UpdateRecoveryManager.ExecuteRecovery(c.Request.Context(), action); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "recovery executed", "operationId": body.OperationID, "strategy": body.Strategy})
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
