package extension

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	g.GET("/:deviceId/workflows", api.listDeviceWorkflows)
	g.GET("/:deviceId/workflows/:workflowId", api.getDeviceWorkflow)
	g.POST("/:deviceId/workflows", api.createDeviceWorkflow)
	g.PUT("/:deviceId/workflows/:workflowId", api.updateDeviceWorkflow)
	g.DELETE("/:deviceId/workflows/:workflowId", api.deleteDeviceWorkflow)
	g.POST("/:deviceId/workflows/:workflowId/enable", func(c *gin.Context) { api.setDeviceWorkflowEnabled(c, true) })
	g.POST("/:deviceId/workflows/:workflowId/disable", func(c *gin.Context) { api.setDeviceWorkflowEnabled(c, false) })
	g.POST("/:deviceId/workflows/:workflowId/run", api.runDeviceWorkflow)
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
		Input json.RawMessage `json:"input"`
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
	result, ok := api.invokeDeviceWorkflow(c, WorkflowMeshRun, map[string]any{"workflowId": c.Param("workflowId"), "input": body.Input})
	if !ok {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}
