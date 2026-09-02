package extension

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type WorkflowDeviceDescriptor struct {
	DeviceID   string    `json:"deviceId"`
	RuntimeID  string    `json:"runtimeId,omitempty"`
	Label      string    `json:"label,omitempty"`
	Platform   string    `json:"platform,omitempty"`
	Online     bool      `json:"online"`
	LastSeenAt time.Time `json:"lastSeenAt,omitempty"`
}

type WorkflowDeviceControlPlane interface {
	ListDevices(ctx context.Context, userID string) ([]WorkflowDeviceDescriptor, error)
	Invoke(ctx context.Context, userID, deviceID, operation string, input json.RawMessage) (json.RawMessage, error)
}

func (r *Runtime) AttachWorkflowDeviceControl(control WorkflowDeviceControlPlane) {
	if r == nil {
		return
	}
	r.WorkflowDeviceControl = control
	if r.Kernel != nil && r.Kernel.Container() != nil && r.Kernel.Container().WorkflowExecutor != nil {
		r.Kernel.Container().WorkflowExecutor.SetRemoteWorkflowRunner(workflowDeviceRemoteRunner{control: control})
	}
}

func (api *WorkflowAPI) registerDeviceWorkflowRoutes(group *gin.RouterGroup) {
	g := group.Group("/workflow-devices")
	g.GET("", api.listWorkflowDevices)
	g.GET("/:deviceId/trigger-capabilities", api.getDeviceWorkflowTriggerCapabilities)
	g.GET("/:deviceId/runtime-health", api.getDeviceWorkflowRuntimeHealth)
	g.GET("/:deviceId/trigger-app-catalog", api.getDeviceWorkflowTriggerAppCatalog)
	g.GET("/:deviceId/trigger-wake-configs", api.getDeviceWorkflowTriggerWakeConfigs)
	g.POST("/:deviceId/trigger-wake-configs", api.createDeviceWorkflowTriggerWakeConfig)
	g.POST("/:deviceId/trigger-secrets/tasker", api.createDeviceTaskerTriggerSecret)
	g.GET("/:deviceId/workflows", api.listDeviceWorkflows)
	g.GET("/:deviceId/workflows/catalog", api.getDeviceWorkflowCatalog)
	g.GET("/:deviceId/workflows/:workflowId", api.getDeviceWorkflow)
	g.POST("/:deviceId/workflows", api.createDeviceWorkflow)
	g.PUT("/:deviceId/workflows/:workflowId", api.updateDeviceWorkflow)
	g.DELETE("/:deviceId/workflows/:workflowId", api.deleteDeviceWorkflow)
	g.POST("/:deviceId/workflows/:workflowId/enable", func(c *gin.Context) { api.setDeviceWorkflowEnabled(c, true) })
	g.POST("/:deviceId/workflows/:workflowId/disable", func(c *gin.Context) { api.setDeviceWorkflowEnabled(c, false) })
	g.POST("/:deviceId/workflows/:workflowId/run", api.runDeviceWorkflow)
	g.GET("/:deviceId/workflows/:workflowId/runs", api.listDeviceWorkflowRuns)
	g.GET("/:deviceId/runs/:runId", api.getDeviceWorkflowRun)
	g.GET("/:deviceId/runs/:runId/steps", api.getDeviceWorkflowRunSteps)
	g.GET("/:deviceId/runs/:runId/attempts", api.getDeviceWorkflowRunAttempts)
	g.GET("/:deviceId/runs/:runId/checkpoints", api.getDeviceWorkflowRunCheckpoints)
	g.GET("/:deviceId/runs/:runId/logs", api.getDeviceWorkflowRunLogs)
	g.POST("/:deviceId/runs/:runId/pause", api.pauseDeviceWorkflowRun)
	g.POST("/:deviceId/runs/:runId/resume", api.resumeDeviceWorkflowRun)
	g.POST("/:deviceId/runs/:runId/confirm", api.confirmDeviceWorkflowRun)
	g.POST("/:deviceId/runs/:runId/cancel", api.cancelDeviceWorkflowRun)
	g.POST("/:deviceId/runs/:runId/recover", api.recoverDeviceWorkflowRun)
	g.POST("/:deviceId/runs/:runId/rerun", api.rerunDeviceWorkflowRun)
}

func (api *WorkflowAPI) workflowDeviceControl(c *gin.Context) (WorkflowDeviceControlPlane, bool) {
	if api == nil || api.runtime == nil || api.runtime.WorkflowDeviceControl == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow device control plane unavailable"})
		return nil, false
	}
	if strings.TrimSpace(workflowUserID(c)) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "workflow user is required"})
		return nil, false
	}
	return api.runtime.WorkflowDeviceControl, true
}

func (api *WorkflowAPI) listWorkflowDevices(c *gin.Context) {
	control, ok := api.workflowDeviceControl(c)
	if !ok {
		return
	}
	items, err := control.ListDevices(c.Request.Context(), workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (api *WorkflowAPI) invokeDeviceWorkflow(c *gin.Context, operation string, payload any) (json.RawMessage, bool) {
	control, ok := api.workflowDeviceControl(c)
	if !ok {
		return nil, false
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, false
	}
	result, err := control.Invoke(c.Request.Context(), workflowUserID(c), strings.TrimSpace(c.Param("deviceId")), operation, raw)
	if err != nil {
		status := http.StatusBadGateway
		message := err.Error()
		if strings.Contains(strings.ToLower(message), "offline") || strings.Contains(strings.ToLower(message), "not found") {
			status = http.StatusServiceUnavailable
		}
		if strings.Contains(message, "WORKFLOW_REVISION_CONFLICT") {
			status = http.StatusConflict
			message = "WORKFLOW_REVISION_CONFLICT"
		}
		c.JSON(status, gin.H{"error": message})
		return nil, false
	}
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	return result, true
}

func (api *WorkflowAPI) getDeviceWorkflowRuntimeHealth(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshAndroidRuntimeHealth, map[string]any{})
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowTriggerCapabilities(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshTriggerCapabilities, map[string]any{})
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowTriggerAppCatalog(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshTriggerAppCatalog, map[string]any{})
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowTriggerWakeConfigs(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshTriggerWakeConfigs, map[string]any{})
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) createDeviceWorkflowTriggerWakeConfig(c *gin.Context) {
	var request workflowWakeConfigCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wake config: " + err.Error()})
		return
	}
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshCreateWakeConfig, request)
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusCreated, "application/json", result)
}

func (api *WorkflowAPI) createDeviceTaskerTriggerSecret(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshCreateTriggerSecret, map[string]any{"kind": "tasker"})
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusCreated, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowCatalog(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshToolCatalog, map[string]any{})
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) listDeviceWorkflows(c *gin.Context) {
	control, ok := api.workflowDeviceControl(c)
	if !ok {
		return
	}
	userID := workflowUserID(c)
	deviceID := strings.TrimSpace(c.Param("deviceId"))
	result, err := control.Invoke(c.Request.Context(), userID, deviceID, WorkflowMeshCatalog, json.RawMessage(`{}`))
	if err != nil {
		// A device catalog is a metadata mirror only. It may be shown while the
		// device is offline, but mutating/running operations still fail rather
		// than pretending the remote device accepted the request.
		if api.runtime != nil && api.runtime.Kernel != nil && api.runtime.Kernel.Container() != nil {
			if repo := api.runtime.Kernel.Container().WorkflowDeviceCatalogRepo; repo != nil {
				cached, cacheErr := repo.ListDevice(c.Request.Context(), userID, deviceID)
				if cacheErr == nil && len(cached) > 0 {
					c.JSON(http.StatusOK, gin.H{"items": cached, "total": len(cached), "cached": true, "offline": true})
					return
				}
			}
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "offline": true})
		return
	}
	if len(result) == 0 {
		result = json.RawMessage(`{"items":[],"total":0}`)
	}
	api.cacheDeviceWorkflowCatalog(c.Request.Context(), userID, deviceID, result)
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) cacheDeviceWorkflowCatalog(ctx context.Context, userID, deviceID string, raw json.RawMessage) {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil {
		return
	}
	repo := api.runtime.Kernel.Container().WorkflowDeviceCatalogRepo
	if repo == nil {
		return
	}
	var envelope struct {
		Items []workflowAPIResponse `json:"items"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return
	}
	now := time.Now().UTC()
	items := make([]sqlite.WorkflowDeviceCatalogItem, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		updatedAt := item.Installation.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}
		items = append(items, sqlite.WorkflowDeviceCatalogItem{
			OwnerUserID:  userID,
			DeviceID:     deviceID,
			WorkflowID:   item.ID,
			Name:         item.Name,
			Description:  item.Description,
			InputSchema:  item.InputSchema,
			OutputSchema: item.OutputSchema,
			Version:      item.Version,
			Enabled:      item.Installation.Enabled,
			UpdatedAt:    updatedAt,
			LastSeen:     now,
		})
	}
	_ = repo.ReplaceDevice(ctx, userID, deviceID, items)
}

func (api *WorkflowAPI) getDeviceWorkflow(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshGet, map[string]any{"workflowId": c.Param("workflowId")})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) emitRemoteWorkflowInstallationEvent(ctx context.Context, typeID, userID, deviceID string, raw json.RawMessage) {
	var envelope struct {
		Installation *workflow.WorkflowInstallation `json:"installation"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Installation == nil {
		return
	}
	inst := *envelope.Installation
	inst.OwnerUserID = strings.TrimSpace(userID)
	inst.Location = workflow.WorkflowLocationLocal
	inst.HostDeviceID = strings.TrimSpace(deviceID)
	api.emitWorkflowInstallationEvent(ctx, typeID, &inst)
}

func (api *WorkflowAPI) createDeviceWorkflow(c *gin.Context) {
	var def json.RawMessage
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 || !json.Valid(raw) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid workflow definition is required"})
		return
	}
	def = raw
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshUpsert, map[string]any{"definition": def})
	if !ok {
		return
	}
	api.emitRemoteWorkflowInstallationEvent(c.Request.Context(), "workflow.installation.created", workflowUserID(c), c.Param("deviceId"), result)
	c.Data(http.StatusCreated, "application/json", result)
}

func (api *WorkflowAPI) updateDeviceWorkflow(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 || !json.Valid(raw) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid workflow definition is required"})
		return
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	object["id"] = c.Param("workflowId")
	expected, err := api.expectedRevision(c, 1)
	if err != nil && c.Query("expectedRevision") != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if c.Query("expectedRevision") == "" && c.GetHeader("If-Match") == "" {
		expected = 0
	}
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshUpsert, map[string]any{"definition": object, "expectedRevision": expected})
	if !ok {
		return
	}
	api.emitRemoteWorkflowInstallationEvent(c.Request.Context(), "workflow.installation.updated", workflowUserID(c), c.Param("deviceId"), result)
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) deleteDeviceWorkflow(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshDelete, map[string]any{"workflowId": c.Param("workflowId")})
	if !ok {
		return
	}
	api.emitRemoteWorkflowInstallationEvent(c.Request.Context(), "workflow.installation.deleted", workflowUserID(c), c.Param("deviceId"), result)
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) setDeviceWorkflowEnabled(c *gin.Context, enabled bool) {
	expected := int64(0)
	if q := strings.TrimSpace(c.Query("expectedRevision")); q != "" {
		var err error
		expected, err = api.expectedRevision(c, 1)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshSetEnabled, map[string]any{"workflowId": c.Param("workflowId"), "enabled": enabled, "expectedRevision": expected})
	if !ok {
		return
	}
	eventType := "workflow.installation.disabled"
	if enabled {
		eventType = "workflow.installation.enabled"
	}
	api.emitRemoteWorkflowInstallationEvent(c.Request.Context(), eventType, workflowUserID(c), c.Param("deviceId"), result)
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) runDeviceWorkflow(c *gin.Context) {
	var body struct {
		Input               json.RawMessage         `json:"input"`
		Wait                bool                    `json:"wait"`
		Mode                workflow.ExecutionMode  `json:"mode"`
		Mocks               []workflow.MockBehavior `json:"mocks"`
		ApprovedSideEffects []string                `json:"approvedSideEffects"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, context.Canceled) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if len(body.Input) == 0 {
		body.Input = json.RawMessage(`{}`)
	}
	operation := WorkflowMeshRunStart
	if body.Wait {
		operation = WorkflowMeshRun
	}
	result, ok := api.invokeDeviceWorkflow(c, operation, map[string]any{
		"workflowId": c.Param("workflowId"), "input": body.Input,
		"options": workflow.ExecutionOptions{Mode: body.Mode, Mocks: body.Mocks, ApprovedSideEffects: body.ApprovedSideEffects},
	})
	if !ok {
		return
	}
	status := http.StatusAccepted
	if body.Wait {
		status = http.StatusOK
	}
	c.Data(status, "application/json", result)
}

func (api *WorkflowAPI) resolveRemoteWorkflowRun(ctx context.Context, userID, runID, operation string, payload map[string]any) (json.RawMessage, string, error) {
	if api == nil || api.runtime == nil || api.runtime.WorkflowDeviceControl == nil {
		return nil, "", errors.New("workflow device control plane unavailable")
	}
	userID = strings.TrimSpace(userID)
	runID = strings.TrimSpace(runID)
	if userID == "" || runID == "" {
		return nil, "", errors.New("workflow run owner resolution requires userId and runId")
	}
	devices, err := api.runtime.WorkflowDeviceControl.ListDevices(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["runId"] = runID
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	var lastErr error
	for _, device := range devices {
		deviceID := strings.TrimSpace(device.DeviceID)
		if deviceID == "" || !device.Online {
			continue
		}
		result, invokeErr := api.runtime.WorkflowDeviceControl.Invoke(ctx, userID, deviceID, operation, raw)
		if invokeErr == nil {
			if len(result) == 0 {
				result = json.RawMessage(`{}`)
			}
			return result, deviceID, nil
		}
		lastErr = invokeErr
	}
	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", errors.New("workflow run not found on online devices")
}

func (api *WorkflowAPI) listDeviceWorkflowRuns(c *gin.Context) {
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 200 {
		limit = 200
	}
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunList, map[string]any{
		"workflowId": c.Param("workflowId"),
		"limit":      limit,
		"offset":     offset,
	})
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowRun(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunGet, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowRunSteps(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunSteps, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowRunAttempts(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunAttempts, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowRunCheckpoints(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunCheckpoints, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getDeviceWorkflowRunLogs(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunLogs, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) pauseDeviceWorkflowRun(c *gin.Context) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunPause, map[string]any{"runId": c.Param("runId"), "reason": body.Reason})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) resumeDeviceWorkflowRun(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunResume, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) confirmDeviceWorkflowRun(c *gin.Context) {
	var body struct {
		NodeIDs []string `json:"nodeIds"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunConfirm, map[string]any{"runId": c.Param("runId"), "nodeIds": body.NodeIDs})
	if !ok {
		return
	}
	c.Data(http.StatusAccepted, "application/json", result)
}

func (api *WorkflowAPI) cancelDeviceWorkflowRun(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunCancel, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) recoverDeviceWorkflowRun(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunRecover, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Data(http.StatusAccepted, "application/json", result)
}

func (api *WorkflowAPI) rerunDeviceWorkflowRun(c *gin.Context) {
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRunRerun, map[string]any{"runId": c.Param("runId")})
	if !ok {
		return
	}
	c.Data(http.StatusAccepted, "application/json", result)
}
