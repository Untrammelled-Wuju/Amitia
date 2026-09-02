package extension

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	workflowdb "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/middleware/security"
)

const (
	userWorkflowSource         = "user"
	userWorkflowSchedulePrefix = "userwf-sched-"
)

type WorkflowAPI struct {
	runtime  *Runtime
	location workflow.WorkflowLocation
}

func NewWorkflowAPI(runtime *Runtime) *WorkflowAPI {
	return &WorkflowAPI{runtime: runtime}
}

func NewWorkflowAPIForLocation(runtime *Runtime, location workflow.WorkflowLocation) *WorkflowAPI {
	return &WorkflowAPI{runtime: runtime, location: location}
}

func (api *WorkflowAPI) RegisterRoutes(group *gin.RouterGroup) {
	api.registerDeviceWorkflowRoutes(group)
	api.registerWorkflowRoutes(group)
}

func (api *WorkflowAPI) registerWorkflowRoutes(group *gin.RouterGroup) {
	api.registerWorkflowManagementRoutes(group)
	api.registerWorkflowEventRoute(group)
	api.registerWorkflowRuntimeProducerRoutes(group)
}

// registerWorkflowManagementRoutes exposes user-facing Workflow CRUD, Builder,
// catalog and run-control endpoints. Device Agent callers using the root local
// token must not receive this surface; RegisterDeviceExecutionWorkflowRoutes
// mounts it behind Desktop Session authentication only.
func (api *WorkflowAPI) registerWorkflowManagementRoutes(group *gin.RouterGroup) {
	g := group.Group("/workflows")
	g.GET("", api.list)
	g.POST("", api.create)
	g.GET("/catalog", api.catalog)
	g.GET("/reliability-metrics", api.reliabilityMetrics)
	g.GET("/templates", api.listTemplates)
	g.POST("/templates/:templateId/instantiate", api.instantiateTemplate)
	g.DELETE("/templates/:templateId", api.deleteTemplate)
	g.POST("/import", api.importWorkflow)
	g.POST("/validate", api.validate)
	g.POST("/:id/preflight", api.preflight)
	g.POST("/ai/generate", api.aiGenerate)
	g.GET("/sync-events", api.workflowSyncEvents)
	g.GET("/sync-conflicts", api.workflowSyncConflicts)
	g.GET("/trigger-capabilities", api.getTriggerCapabilities)
	g.GET("/trigger-app-catalog", api.getTriggerAppCatalog)
	g.GET("/trigger-wake-configs", api.getTriggerWakeConfigs)
	g.POST("/trigger-wake-configs", api.createTriggerWakeConfig)
	g.POST("/trigger-secrets/tasker", api.createTaskerTriggerSecret)
	g.GET("/:id", api.get)
	g.GET("/:id/analysis", api.analysis)
	g.GET("/:id/stats", api.stats)
	g.GET("/:id/export", api.exportWorkflow)
	g.POST("/:id/templates", api.saveTemplate)
	g.GET("/:id/revisions", api.listRevisions)
	g.POST("/:id/revisions", api.createRevision)
	g.POST("/:id/revisions/:revisionId/publish", api.publishRevision)
	g.POST("/:id/revisions/:revisionId/archive", api.archiveRevision)
	g.POST("/:id/revisions/:revisionId/rollback", api.rollbackRevision)
	g.POST("/:id/ai/edit", api.aiEdit)
	g.POST("/:id/ai/repair", api.aiRepair)
	g.POST("/:id/ai/explain", api.aiExplain)
	g.PUT("/:id", api.update)
	g.PATCH("/:id", api.patch)
	g.POST("/:id/duplicate", api.duplicate)
	g.DELETE("/:id", api.delete)
	g.POST("/:id/enable", api.enable)
	g.POST("/:id/disable", api.disable)
	g.POST("/:id/run", api.run)
	g.GET("/:id/runs", api.listRuns)

	runs := group.Group("/workflow-runs")
	runs.GET("/:runId", api.getRun)
	runs.GET("/:runId/logs", api.getRunLogs)
	runs.POST("/:runId/cancel", api.cancelRun)
	runs.POST("/:runId/pause", api.pauseRun)
	runs.POST("/:runId/resume", api.resumeRun)
	runs.POST("/:runId/confirm", api.confirmRun)
	runs.POST("/:runId/rerun", api.rerunRun)
	runs.POST("/:runId/recover", api.recoverRun)
}

// registerWorkflowEventRoute is intentionally isolated because local device
// producers need this one endpoint through the root local token while Builder
// traffic uses a Desktop Session. The handler still applies structured-event
// validation, rate limiting and account/device scoping.
func (api *WorkflowAPI) registerWorkflowEventRoute(group *gin.RouterGroup) {
	group.POST("/workflows/events/:eventType", api.dispatchEvent)
}

// registerWorkflowRuntimeProducerRoutes is the narrow device-runtime write
// surface. These endpoints accept capability/catalog telemetry and wake PCM;
// they are mounted root-local-token-only on Device Agent/local hosts.
func (api *WorkflowAPI) registerWorkflowRuntimeProducerRoutes(group *gin.RouterGroup) {
	g := group.Group("/workflows")
	g.POST("/trigger-capabilities/status", api.updateTriggerCapabilityStatus)
	g.POST("/trigger-app-catalog/status", api.updateTriggerAppCatalog)
	g.GET("/wake-runtime/status", api.getWakeRuntimeStatus)
	g.POST("/wake-runtime/audio", api.ingestWakeRuntimeAudio)
	g.POST("/wake-runtime/device-status", api.updateWakeRuntimeDeviceStatus)
	g.GET("/android-runtime-health/status", api.getAndroidRuntimeHealth)
	g.POST("/android-runtime-health/status", api.updateAndroidRuntimeHealth)
}

func (api *WorkflowAPI) getWakeRuntimeStatus(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow wake runtime unavailable"})
		return
	}
	if !requireWorkflowRuntimeReporter(c) {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, api.runtime.workflowWakeStatus(c.Request.Context(), true))
}

func (api *WorkflowAPI) updateWakeRuntimeDeviceStatus(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow wake runtime unavailable"})
		return
	}
	if !requireWorkflowRuntimeReporter(c) {
		return
	}
	var request struct {
		State  string `json:"state"`
		Reason string `json:"reason"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wake device status"})
		return
	}
	if err := api.runtime.updateWorkflowWakeDeviceStatus(request.State, request.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *WorkflowAPI) ingestWakeRuntimeAudio(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow wake runtime unavailable"})
		return
	}
	if !requireWorkflowRuntimeReporter(c) {
		return
	}
	if contentType := strings.ToLower(strings.TrimSpace(strings.Split(c.GetHeader("Content-Type"), ";")[0])); contentType != "application/octet-stream" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "wake audio requires application/octet-stream PCM16"})
		return
	}
	reader := http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	pcm, err := io.ReadAll(reader)
	if len(pcm) > 0 {
		defer clear(pcm)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wake audio payload"})
		return
	}
	if len(pcm) == 0 || len(pcm)%2 != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wake audio must contain non-empty PCM16 samples"})
		return
	}
	sequence := uint64(0)
	if raw := strings.TrimSpace(c.GetHeader("X-Amitia-Audio-Sequence")); raw != "" {
		parsed, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wake audio sequence"})
			return
		}
		sequence = parsed
	}
	capturedAt := time.Now().UTC()
	if raw := strings.TrimSpace(c.GetHeader("X-Amitia-Captured-At-Ms")); raw != "" {
		millis, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wake audio timestamp"})
			return
		}
		candidate := time.UnixMilli(millis).UTC()
		now := time.Now().UTC()
		if candidate.After(now.Add(-5*time.Minute)) && candidate.Before(now.Add(5*time.Minute)) {
			capturedAt = candidate
		}
	}
	deviceID := strings.TrimSpace(c.GetHeader("X-Amitia-Device-ID"))
	if len(deviceID) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device id is too long"})
		return
	}
	if err := api.runtime.processWorkflowWakeAudio(c.Request.Context(), pcm, deviceID, sequence, capturedAt); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *WorkflowAPI) getTriggerCapabilities(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow trigger capability endpoint unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": api.workflowTriggerCapabilitiesSnapshot(c.Request.Context())})
}

func (api *WorkflowAPI) getTriggerAppCatalog(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow trigger app catalog unavailable"})
		return
	}
	items, updatedAt := api.runtime.WorkflowTriggerAppCatalog()
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"items": items, "updatedAt": updatedAt})
}

func (api *WorkflowAPI) getTriggerWakeConfigs(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow trigger wake config catalog unavailable"})
		return
	}
	items, err := api.runtime.workflowWakeConfigCatalog(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (api *WorkflowAPI) createTriggerWakeConfig(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow trigger wake config endpoint unavailable"})
		return
	}
	var request workflowWakeConfigCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wake config: " + err.Error()})
		return
	}
	item, err := api.runtime.createWorkflowWakeConfig(c.Request.Context(), request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, item)
}

func requireWorkflowRuntimeReporter(c *gin.Context) bool {
	actor := security.GetActor(c)
	if actor == nil || !actor.IsLocalTrusted || actor.AuthMethod != security.AuthMethodLocalToken {
		c.JSON(http.StatusForbidden, gin.H{"error": "device runtime local token required"})
		return false
	}
	return true
}

func (api *WorkflowAPI) updateTriggerAppCatalog(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow trigger app catalog unavailable"})
		return
	}
	if !requireWorkflowRuntimeReporter(c) {
		return
	}
	var request struct {
		Items []WorkflowTriggerAppCatalogItem `json:"items"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 512*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid trigger app catalog"})
		return
	}
	if len(request.Items) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trigger app catalog exceeds 2000 entries"})
		return
	}
	seen := make(map[string]struct{}, len(request.Items))
	filtered := make([]WorkflowTriggerAppCatalogItem, 0, len(request.Items))
	for _, item := range request.Items {
		item.PackageName = strings.TrimSpace(item.PackageName)
		item.Label = strings.TrimSpace(item.Label)
		if item.PackageName == "" || len(item.PackageName) > 255 || len(item.Label) > 256 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid trigger app catalog entry"})
			return
		}
		if _, exists := seen[item.PackageName]; exists {
			continue
		}
		seen[item.PackageName] = struct{}{}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Label == filtered[j].Label {
			return filtered[i].PackageName < filtered[j].PackageName
		}
		return filtered[i].Label < filtered[j].Label
	})
	api.runtime.SetWorkflowTriggerAppCatalog(filtered)
	c.JSON(http.StatusOK, gin.H{"updated": len(filtered)})
}

func (api *WorkflowAPI) createTaskerTriggerSecret(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow trigger secret endpoint unavailable"})
		return
	}
	value, err := api.newTaskerTriggerSecret(c.Request.Context(), workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, value)
}

func (api *WorkflowAPI) newTaskerTriggerSecret(ctx context.Context, userID string) (map[string]string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("workflow user is required")
	}
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil || api.runtime.Kernel.Container().ExecutionKernel == nil || api.runtime.Kernel.Container().ExecutionKernel.SecretBroker == nil {
		return nil, errors.New("workflow trigger secret broker unavailable")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	encoded := make([]byte, base64.RawURLEncoding.EncodedLen(len(raw)))
	base64.RawURLEncoding.Encode(encoded, raw)
	for i := range raw {
		raw[i] = 0
	}
	ref, err := api.runtime.Kernel.Container().ExecutionKernel.SecretBroker.Store(ctx, workflow.TriggerSecretNamespace(userID), encoded)
	if err != nil {
		for i := range encoded {
			encoded[i] = 0
		}
		return nil, err
	}
	secretValue := string(encoded)
	for i := range encoded {
		encoded[i] = 0
	}
	return map[string]string{"secretRef": ref.String(), "secret": secretValue}, nil
}

func (api *WorkflowAPI) updateTriggerCapabilityStatus(c *gin.Context) {
	if api == nil || api.runtime == nil || api.effectiveLocation() != workflow.WorkflowLocationLocal {
		c.JSON(http.StatusNotFound, gin.H{"error": "local workflow trigger capability endpoint unavailable"})
		return
	}
	if !requireWorkflowRuntimeReporter(c) {
		return
	}
	var request struct {
		Items []WorkflowTriggerCapabilityStatus `json:"items"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid trigger capability status"})
		return
	}
	allowed := map[string]struct{}{
		"workflow.trigger.android_intent.v1": {},
		"workflow.trigger.tasker.v1":         {},
		"workflow.trigger.voice_wake.v1":     {},
		"workflow.trigger.voice_phrase.v1":   {},
		"workflow.trigger.app_foreground.v1": {},
		"workflow.trigger.notification.v1":   {},
		"workflow.trigger.system_event.v1":   {},
		"workflow.trigger.network.v1":        {},
		"workflow.trigger.bluetooth.v1":      {},
		"workflow.trigger.location.v1":       {},
	}
	filtered := make([]WorkflowTriggerCapabilityStatus, 0, len(request.Items))
	for _, item := range request.Items {
		item.ID = strings.TrimSpace(item.ID)
		if _, ok := allowed[item.ID]; !ok {
			continue
		}
		if len(item.Permission) > 256 || len(item.Reason) > 512 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "trigger capability status exceeds limits"})
			return
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no supported trigger capability status supplied"})
		return
	}
	api.runtime.SetWorkflowTriggerCapabilityStatuses(filtered)
	c.JSON(http.StatusOK, gin.H{"updated": len(filtered)})
}

func (api *WorkflowAPI) workflowSyncConflicts(c *gin.Context) {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil || api.runtime.Kernel.Container().WorkflowDefRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow sync repository unavailable"})
		return
	}
	userID := strings.TrimSpace(workflowUserID(c))
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "workflow user is required"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		if parsed > 500 {
			parsed = 500
		}
		limit = parsed
	}
	items, err := api.runtime.Kernel.Container().WorkflowDefRepo.ListWorkflowSyncConflicts(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []workflowdb.WorkflowSyncConflictRecord{}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (api *WorkflowAPI) workflowSyncEvents(c *gin.Context) {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow kernel unavailable"})
		return
	}
	service := api.runtime.Kernel.Container().EventService
	if service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow durable event service unavailable"})
		return
	}
	userID := strings.TrimSpace(workflowUserID(c))
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "workflow user is required"})
		return
	}

	afterRaw := strings.TrimSpace(c.Query("afterCursor"))
	if afterRaw == "" {
		cursor, err := service.LatestWorkflowSyncCursor(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"cursor": cursor, "items": []any{}})
		return
	}
	afterCursor, err := strconv.ParseInt(afterRaw, 10, 64)
	if err != nil || afterCursor < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "afterCursor must be a non-negative integer"})
		return
	}
	limit := 200
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		if parsed > 1000 {
			parsed = 1000
		}
		limit = parsed
	}
	records, err := service.ListWorkflowSyncEventsAfterCursor(c.Request.Context(), userID, afterCursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type item struct {
		Cursor           int64           `json:"cursor"`
		EventID          string          `json:"eventId"`
		Type             string          `json:"type"`
		AggregateID      string          `json:"aggregateId,omitempty"`
		AggregateVersion *int64          `json:"aggregateVersion,omitempty"`
		OccurredAt       time.Time       `json:"occurredAt"`
		Payload          json.RawMessage `json:"payload"`
	}
	items := make([]item, 0, len(records))
	cursor := afterCursor
	for _, record := range records {
		if record.Cursor > cursor {
			cursor = record.Cursor
		}
		items = append(items, item{
			Cursor:           record.Cursor,
			EventID:          record.EventID,
			Type:             string(record.EventTypeID),
			AggregateID:      record.AggregateID,
			AggregateVersion: record.AggregateVersion,
			OccurredAt:       record.OccurredAt,
			Payload:          record.Payload,
		})
	}
	c.JSON(http.StatusOK, gin.H{"cursor": cursor, "items": items})
}

func (api *WorkflowAPI) kernelContainer() (*workflow.WorkflowRegistry, *workflow.WorkflowExecutor, error) {
	if api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil {
		return nil, nil, errors.New("workflow kernel unavailable")
	}
	c := api.runtime.Kernel.Container()
	if c.WorkflowRegistry == nil || c.WorkflowExecutor == nil || c.WorkflowDefRepo == nil || c.WorkflowInstallationRepo == nil || c.WorkflowExecRepo == nil {
		return nil, nil, errors.New("workflow services unavailable")
	}
	return c.WorkflowRegistry, c.WorkflowExecutor, nil
}

func workflowUserID(c *gin.Context) string {
	if value, ok := c.Get(authenticatedUserKey); ok {
		return fmt.Sprint(value)
	}
	return ""
}

func workflowOwnedBy(def workflow.WorkflowDefinition, userID string) bool {
	if userID == "" || def.Source != userWorkflowSource || def.Metadata == nil {
		return false
	}
	owner, ok := def.Metadata["ownerUserId"]
	return ok && owner != nil && strings.TrimSpace(fmt.Sprint(owner)) == userID
}

func prepareUserWorkflow(def workflow.WorkflowDefinition, userID string, existingID string) (workflow.WorkflowDefinition, error) {
	if existingID != "" {
		def.ID = existingID
	}
	if def.ID == "" {
		def.ID = "wf-" + uuid.NewString()
	}
	def.ExtensionID = ""
	def.ModuleID = ""
	def.Source = userWorkflowSource
	if def.Metadata == nil {
		def.Metadata = map[string]any{}
	}
	def.Metadata["ownerUserId"] = userID
	def.Metadata["editor"] = "creative-workshop"
	if def.SchemaVersion != workflow.UserWorkflowSchemaVersion && len(def.Edges) == 0 {
		def.Edges = workflow.DeriveEdges(def.Nodes)
	}
	def.SchemaVersion = workflow.UserWorkflowSchemaVersion
	if def.Version == "" {
		def.Version = "1.0.0"
	}
	if def.Limits.MaxSteps == 0 {
		def.Limits.MaxSteps = 128
	}
	if def.Limits.MaxConcurrency == 0 {
		def.Limits.MaxConcurrency = 4
	}
	if def.Limits.MaxExecutionDurationMS == 0 {
		def.Limits.MaxExecutionDurationMS = 30 * 60 * 1000
	}
	if def.Limits.MaxStepDurationMS == 0 {
		def.Limits.MaxStepDurationMS = 5 * 60 * 1000
	}
	if len(def.Triggers) == 0 {
		def.Triggers = []workflow.WorkflowTriggerDefinition{{ID: "manual", Type: "manual", Enabled: true}}
	}
	for i := range def.Triggers {
		def.Triggers[i].Type = strings.ToLower(strings.TrimSpace(def.Triggers[i].Type))
		def.Triggers[i].EventType = strings.TrimSpace(def.Triggers[i].EventType)
		if def.Triggers[i].Type == "" {
			def.Triggers[i].Type = "manual"
		}
		if def.Triggers[i].ID == "" {
			def.Triggers[i].ID = fmt.Sprintf("trigger-%d", i+1)
		}
	}
	normalized, err := workflow.NormalizeDefinition(def)
	if err != nil {
		return def, err
	}
	if err := validateUserWorkflowTriggers(normalized, userID); err != nil {
		return def, err
	}
	if _, err := workflow.NewCompiler().Compile(normalized, workflow.DefaultCompileOptions()); err != nil {
		return def, err
	}
	return normalized, nil
}

func validateUserWorkflowTriggers(def workflow.WorkflowDefinition, userID string) error {
	seen := make(map[string]struct{}, len(def.Triggers))
	for _, trigger := range def.Triggers {
		id := strings.TrimSpace(trigger.ID)
		if id == "" {
			return errors.New("workflow trigger id is required")
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("duplicate workflow trigger id %q", id)
		}
		seen[id] = struct{}{}
		if len(trigger.Input) > 0 && !json.Valid(trigger.Input) {
			return fmt.Errorf("trigger %s input must be valid JSON", id)
		}
		if len(trigger.Config) > 0 && !json.Valid(trigger.Config) {
			return fmt.Errorf("trigger %s config must be valid JSON", id)
		}
		switch trigger.Type {
		case "manual":
		case "event":
			if strings.TrimSpace(trigger.EventType) == "" {
				return fmt.Errorf("trigger %s eventType is required", id)
			}
			if err := validateWorkflowEventTriggerConfig(trigger, userID); err != nil {
				return fmt.Errorf("trigger %s: %w", id, err)
			}
		case "schedule", "cron", "interval", "one_shot":
			if _, err := buildWorkflowSchedule(def, trigger, userID); err != nil {
				return err
			}
		default:
			return fmt.Errorf("trigger %s has unsupported type %q", id, trigger.Type)
		}
	}
	return nil
}

func (api *WorkflowAPI) reliabilityMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, workflow.DefaultWorkflowReliabilityMetrics.Snapshot())
}

func (api *WorkflowAPI) list(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	userID := workflowUserID(c)
	items := make([]workflowAPIResponse, 0)
	for _, def := range registry.List(workflow.WorkflowFilter{}) {
		if !workflowOwnedBy(def, userID) {
			continue
		}
		inst, installErr := api.installationFor(c.Request.Context(), def, userID)
		if installErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": installErr.Error()})
			return
		}
		if def.SchemaVersion != workflow.UserWorkflowSchemaVersion && len(def.Edges) == 0 {
			def.Edges = workflow.DeriveEdges(def.Nodes)
		}
		items = append(items, workflowResponse(def, inst))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	total := len(items)
	limit, offset := parseWorkflowPagination(c)
	if offset >= total {
		items = []workflowAPIResponse{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		items = items[offset:end]
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "limit": limit, "offset": offset, "location": api.effectiveLocation()})
}

func (api *WorkflowAPI) workflowToolCatalogSnapshot(ctx context.Context, userID string) ([]map[string]any, error) {
	if _, _, err := api.kernelContainer(); err != nil {
		return nil, err
	}
	kc := api.runtime.Kernel.Container()
	if kc.ToolRegistry == nil {
		return nil, errors.New("tool registry unavailable")
	}
	items := make([]map[string]any, 0)
	for _, def := range kc.ToolRegistry.List(ctx, capability.ToolFilter{Enabled: boolPtrWorkflow(true)}) {
		if def.Source == capability.ToolSourceWorkflow && def.Metadata != nil {
			if flag, ok := def.Metadata["userWorkflow"].(bool); ok && flag {
				owner := strings.TrimSpace(fmt.Sprint(def.Metadata["ownerUserId"]))
				if owner == "" || owner != strings.TrimSpace(userID) {
					continue
				}
			}
		}
		items = append(items, map[string]any{
			"id":             def.ID,
			"modelName":      def.ModelName,
			"name":           def.Name,
			"description":    def.Description,
			"source":         def.Source,
			"inputSchema":    json.RawMessage(def.InputSchema),
			"outputSchema":   json.RawMessage(def.OutputSchema),
			"runtime":        def.Runtime,
			"permissions":    def.Permissions,
			"riskLevel":      def.RiskLevel,
			"sideEffect":     def.SideEffect,
			"hasSideEffects": def.HasSideEffects,
			"idempotent":     def.Idempotent,
			"retryable":      def.Retryable,
			"timeoutMs":      def.TimeoutMS,
			"metadata":       workflowCatalogSafeMetadata(def.Metadata),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.TrimSpace(fmt.Sprint(items[i]["name"]))
		right := strings.TrimSpace(fmt.Sprint(items[j]["name"]))
		if left == right {
			return fmt.Sprint(items[i]["id"]) < fmt.Sprint(items[j]["id"])
		}
		return left < right
	})
	return items, nil
}

func workflowCatalogSafeMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	// Catalog metadata is user-facing. Do not mirror arbitrary provider/plugin
	// metadata because it may contain internal paths or configuration details.
	// Only expose keys used for discovery/grouping and UX hints.
	allowed := []string{"androidNativeOperation", "bridgeProtocol", "category", "platform", "tags", "icon", "deprecated"}
	out := make(map[string]any, len(allowed))
	for _, key := range allowed {
		if value, ok := metadata[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (api *WorkflowAPI) catalog(c *gin.Context) {
	items, err := api.workflowToolCatalogSnapshot(c.Request.Context(), workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func boolPtrWorkflow(value bool) *bool { return &value }

func (api *WorkflowAPI) create(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	var def workflow.WorkflowDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow: " + err.Error()})
		return
	}
	def, err = api.prepareValidatedUserWorkflow(def, workflowUserID(c), "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.validateExecutionTargets(def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, exists := registry.Get(def.ID); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "workflow id already exists"})
		return
	}
	userID := workflowUserID(c)
	if err := api.registerNewUserWorkflow(c.Request.Context(), registry, def, userID, "初始版本"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inst, err := api.installationFor(c.Request.Context(), def, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, workflowResponse(def, inst))
}

func (api *WorkflowAPI) validate(c *gin.Context) {
	var def workflow.WorkflowDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
		return
	}
	prepared, err := api.prepareValidatedUserWorkflow(def, workflowUserID(c), def.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
		return
	}
	if err := api.validateExecutionTargets(prepared); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
		return
	}
	compiled, err := workflow.NewCompiler().Compile(prepared, workflow.DefaultCompileOptions())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
		return
	}
	report := api.preflightDefinition(c.Request.Context(), prepared, workflowUserID(c))
	c.JSON(http.StatusOK, gin.H{"valid": report.Runnable, "topologicalOrder": compiled.TopologicalOrder, "entryNodes": compiled.EntryNodes, "exitNodes": compiled.ExitNodes, "definitionHash": prepared.DefinitionHash, "preflight": report})
}

func (api *WorkflowAPI) preflight(c *gin.Context) {
	def, ok := api.owned(c)
	if !ok {
		return
	}
	report := api.preflightDefinition(c.Request.Context(), def, workflowUserID(c))
	status := http.StatusOK
	if !report.Runnable {
		status = http.StatusConflict
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(status, report)
}

func (api *WorkflowAPI) owned(c *gin.Context) (workflow.WorkflowDefinition, bool) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return workflow.WorkflowDefinition{}, false
	}
	userID := workflowUserID(c)
	def, ok := registry.Get(c.Param("id"))
	if !ok || !workflowOwnedBy(def, userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return workflow.WorkflowDefinition{}, false
	}
	inst, err := api.installationFor(c.Request.Context(), def, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workflow installation not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return workflow.WorkflowDefinition{}, false
	}
	def = applyInstallation(def, *inst)
	if def.SchemaVersion != workflow.UserWorkflowSchemaVersion && len(def.Edges) == 0 {
		def.Edges = workflow.DeriveEdges(def.Nodes)
	}
	return def, true
}

func (api *WorkflowAPI) get(c *gin.Context) {
	def, ok := api.owned(c)
	if !ok {
		return
	}
	inst, err := api.installationFor(c.Request.Context(), def, workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, workflowResponse(def, inst))
}

func (api *WorkflowAPI) update(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	userID := workflowUserID(c)
	unlock := api.lockWorkflowMutation(userID, c.Param("id"))
	defer unlock()
	old, ok := api.owned(c)
	if !ok {
		return
	}
	currentInst, err := api.installationFor(c.Request.Context(), old, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	expectedRevision, err := api.expectedRevision(c, currentInst.Revision)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := requireWorkflowRevision(expectedRevision, currentInst.Revision); err != nil {
		writeWorkflowRevisionConflict(c)
		return
	}
	var def workflow.WorkflowDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow: " + err.Error()})
		return
	}
	def, err = api.prepareValidatedUserWorkflow(def, userID, old.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.validateExecutionTargets(def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if old.DefinitionHash != def.DefinitionHash {
		if _, err := api.runtime.Kernel.Container().WorkflowDefRepo.SaveRevision(c.Request.Context(), userID, old, "保存前自动快照"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if err := registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rollback := func() {
		_ = api.syncTriggers(c.Request.Context(), def, old, userID)
		_ = registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), old)
	}
	if err := api.syncTriggers(c.Request.Context(), old, def, userID); err != nil {
		rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedInst, err := api.updateInstallationCAS(c.Request.Context(), def, userID, currentInst, expectedRevision)
	if err != nil {
		rollback()
		if isWorkflowRevisionConflict(err) {
			writeWorkflowRevisionConflict(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, workflowResponse(def, updatedInst))
}

func (api *WorkflowAPI) patch(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	userID := workflowUserID(c)
	unlock := api.lockWorkflowMutation(userID, c.Param("id"))
	defer unlock()
	old, ok := api.owned(c)
	if !ok {
		return
	}
	currentInst, err := api.installationFor(c.Request.Context(), old, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	expectedRevision, err := api.expectedRevision(c, currentInst.Revision)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := requireWorkflowRevision(expectedRevision, currentInst.Revision); err != nil {
		writeWorkflowRevisionConflict(c)
		return
	}
	var patch map[string]json.RawMessage
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow patch: " + err.Error()})
		return
	}
	delete(patch, "expectedRevision")
	baseRaw, err := json.Marshal(old)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	immutable := map[string]struct{}{
		"id": {}, "extensionId": {}, "moduleId": {}, "source": {}, "definitionHash": {}, "installation": {},
	}
	for key := range immutable {
		delete(patch, key)
	}
	patchRaw, err := json.Marshal(patch)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mergedRaw, err := applyJSONMergePatch(baseRaw, patchRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow patch: " + err.Error()})
		return
	}
	var def workflow.WorkflowDefinition
	if err := json.Unmarshal(mergedRaw, &def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow patch: " + err.Error()})
		return
	}
	def, err = api.prepareValidatedUserWorkflow(def, userID, old.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.validateExecutionTargets(def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if old.DefinitionHash != def.DefinitionHash {
		if _, err := api.runtime.Kernel.Container().WorkflowDefRepo.SaveRevision(c.Request.Context(), userID, old, "保存前自动快照"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if err := registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rollback := func() {
		_ = api.syncTriggers(c.Request.Context(), def, old, userID)
		_ = registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), old)
	}
	if err := api.syncTriggers(c.Request.Context(), old, def, userID); err != nil {
		rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedInst, err := api.updateInstallationCAS(c.Request.Context(), def, userID, currentInst, expectedRevision)
	if err != nil {
		rollback()
		if isWorkflowRevisionConflict(err) {
			writeWorkflowRevisionConflict(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, workflowResponse(def, updatedInst))
}

func applyJSONMergePatch(base, patch []byte) ([]byte, error) {
	var patchValue any
	if err := json.Unmarshal(patch, &patchValue); err != nil {
		return nil, err
	}
	patchObject, ok := patchValue.(map[string]any)
	if !ok {
		return json.Marshal(patchValue)
	}
	var baseValue any
	if err := json.Unmarshal(base, &baseValue); err != nil {
		return nil, err
	}
	baseObject, ok := baseValue.(map[string]any)
	if !ok {
		baseObject = map[string]any{}
	}
	var merge func(map[string]any, map[string]any) map[string]any
	merge = func(target, delta map[string]any) map[string]any {
		for key, value := range delta {
			if value == nil {
				delete(target, key)
				continue
			}
			childPatch, childIsObject := value.(map[string]any)
			if !childIsObject {
				target[key] = value
				continue
			}
			childTarget, childIsObject := target[key].(map[string]any)
			if !childIsObject {
				childTarget = map[string]any{}
			}
			target[key] = merge(childTarget, childPatch)
		}
		return target
	}
	return json.Marshal(merge(baseObject, patchObject))
}

func (api *WorkflowAPI) duplicate(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	old, ok := api.owned(c)
	if !ok {
		return
	}
	clone := old
	clone.ID = ""
	clone.DefinitionHash = ""
	clone.Name = strings.TrimSpace(old.Name) + " 副本"
	if old.Metadata != nil {
		clone.Metadata = make(map[string]any, len(old.Metadata))
		for key, value := range old.Metadata {
			clone.Metadata[key] = value
		}
	}
	clone, err = api.prepareValidatedUserWorkflow(clone, workflowUserID(c), "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.registerNewUserWorkflow(c.Request.Context(), registry, clone, workflowUserID(c), "复制创建"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inst, err := api.installationFor(c.Request.Context(), clone, workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, workflowResponse(clone, inst))
}

func (api *WorkflowAPI) registerNewUserWorkflow(ctx context.Context, registry *workflow.WorkflowRegistry, def workflow.WorkflowDefinition, userID, revisionNote string) error {
	if err := api.validateExecutionTargets(def); err != nil {
		return err
	}
	if err := registry.UpsertContext(api.workflowDefinitionMutationContext(ctx), def); err != nil {
		return err
	}
	rollback := func() {
		_ = api.syncTriggers(ctx, def, workflow.WorkflowDefinition{}, userID)
		_ = registry.UnregisterContext(api.workflowDefinitionMutationContext(ctx), def.ID)
	}
	if err := api.syncTriggers(ctx, workflow.WorkflowDefinition{}, def, userID); err != nil {
		rollback()
		return err
	}
	if _, err := api.runtime.Kernel.Container().WorkflowDefRepo.SaveRevision(ctx, userID, def, revisionNote); err != nil {
		rollback()
		return err
	}
	created, err := api.runtime.Kernel.Container().WorkflowInstallationRepo.Create(ctx, workflow.WorkflowInstallation{
		WorkflowID:      def.ID,
		OwnerUserID:     userID,
		Location:        api.effectiveLocation(),
		Enabled:         def.Enabled,
		Triggers:        def.Triggers,
		CallableByAgent: def.CallableByAgent,
		AgentTool:       def.AgentTool,
	})
	if err != nil {
		rollback()
		return err
	}
	api.emitWorkflowInstallationEvent(ctx, "workflow.installation.created", created)
	return nil
}

func portableWorkflowDefinition(def workflow.WorkflowDefinition) (workflow.WorkflowDefinition, error) {
	clone, err := workflow.CloneDefinition(def)
	if err != nil {
		return workflow.WorkflowDefinition{}, err
	}
	clone.ID = ""
	clone.ExtensionID = ""
	clone.ModuleID = ""
	clone.Source = ""
	clone.DefinitionHash = ""
	if clone.Metadata != nil {
		delete(clone.Metadata, "ownerUserId")
		delete(clone.Metadata, "editor")
	}
	return clone, nil
}

func (api *WorkflowAPI) exportWorkflow(c *gin.Context) {
	def, ok := api.owned(c)
	if !ok {
		return
	}
	portable, err := portableWorkflowDefinition(def)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(def.Name)
	if name == "" {
		name = "workflow"
	}
	name = strings.NewReplacer("/", "-", "\\", "-", "\"", "", "\r", "", "\n", "").Replace(name)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.workflow.json"`, name))
	c.JSON(http.StatusOK, workflow.WorkflowExportEnvelope{Format: "amitia-workflow", FormatVersion: 1, ExportedAt: time.Now().UTC(), Workflow: portable})
}

func (api *WorkflowAPI) importWorkflow(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 || !json.Valid(raw) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow import must be valid JSON"})
		return
	}
	var header struct {
		Format        string `json:"format"`
		FormatVersion int    `json:"formatVersion"`
	}
	_ = json.Unmarshal(raw, &header)
	var def workflow.WorkflowDefinition
	if strings.TrimSpace(header.Format) != "" {
		if header.Format != "amitia-workflow" || header.FormatVersion != 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported workflow export format"})
			return
		}
		var envelope workflow.WorkflowExportEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow import: " + err.Error()})
			return
		}
		def = envelope.Workflow
	} else if err := json.Unmarshal(raw, &def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow import: " + err.Error()})
		return
	}
	def.ID = ""
	def.ExtensionID = ""
	def.ModuleID = ""
	def.Source = ""
	def.DefinitionHash = ""
	def.Enabled = false
	def.CallableByAgent = false
	if def.Metadata != nil {
		delete(def.Metadata, "ownerUserId")
		delete(def.Metadata, "editor")
	}
	for i := range def.Triggers {
		def.Triggers[i].Enabled = false
	}
	def, err = api.prepareValidatedUserWorkflow(def, workflowUserID(c), "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.registerNewUserWorkflow(c.Request.Context(), registry, def, workflowUserID(c), "导入创建"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inst, err := api.installationFor(c.Request.Context(), def, workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, workflowResponse(def, inst))
}

func (api *WorkflowAPI) saveTemplate(c *gin.Context) {
	def, ok := api.owned(c)
	if !ok {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = c.ShouldBindJSON(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = def.Name
	}
	description := strings.TrimSpace(body.Description)
	if description == "" {
		description = def.Description
	}
	portable, err := portableWorkflowDefinition(def)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	item, err := api.runtime.Kernel.Container().WorkflowDefRepo.SaveTemplate(c.Request.Context(), workflowUserID(c), name, description, portable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (api *WorkflowAPI) listTemplates(c *gin.Context) {
	if _, _, err := api.kernelContainer(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	items, err := api.runtime.Kernel.Container().WorkflowDefRepo.ListTemplates(c.Request.Context(), workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (api *WorkflowAPI) instantiateTemplate(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	item, err := api.runtime.Kernel.Container().WorkflowDefRepo.GetTemplate(c.Request.Context(), workflowUserID(c), c.Param("templateId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow template not found"})
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&body)
	def, err := workflow.CloneDefinition(item.Definition)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	def.ID = ""
	def.DefinitionHash = ""
	def.Enabled = false
	def.CallableByAgent = false
	if strings.TrimSpace(body.Name) != "" {
		def.Name = strings.TrimSpace(body.Name)
	} else {
		def.Name = item.Name
	}
	for i := range def.Triggers {
		def.Triggers[i].Enabled = false
	}
	def, err = api.prepareValidatedUserWorkflow(def, workflowUserID(c), "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.registerNewUserWorkflow(c.Request.Context(), registry, def, workflowUserID(c), "从本地模板创建"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inst, err := api.installationFor(c.Request.Context(), def, workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, workflowResponse(def, inst))
}

func (api *WorkflowAPI) deleteTemplate(c *gin.Context) {
	if _, _, err := api.kernelContainer(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	if err := api.runtime.Kernel.Container().WorkflowDefRepo.DeleteTemplate(c.Request.Context(), workflowUserID(c), c.Param("templateId")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow template not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (api *WorkflowAPI) listRevisions(c *gin.Context) {
	def, ok := api.owned(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	userID := workflowUserID(c)
	kc := api.runtime.Kernel.Container()
	items, err := kc.WorkflowDefRepo.ListRevisions(c.Request.Context(), userID, c.Param("id"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	inst, _ := api.installationFor(c.Request.Context(), def, userID)
	runs, _, _ := kc.WorkflowExecRepo.ListRuns(c.Request.Context(), def.ID, "", 200, 0)
	for i := range items {
		item := &items[i]
		item.Current = item.DefinitionHash == def.DefinitionHash
		item.Installed = item.Current && inst != nil
		for _, run := range runs {
			if run.Status.IsTerminal() {
				continue
			}
			if (run.Context.RevisionID != "" && run.Context.RevisionID == item.RevisionID) ||
				(run.Context.RevisionID == "" && run.Context.DefinitionHash != "" && run.Context.DefinitionHash == item.DefinitionHash) {
				item.Running = true
				break
			}
		}
		switch {
		case item.Running:
			item.EffectiveState = "running"
		case item.Installed:
			item.EffectiveState = "installed"
		default:
			item.EffectiveState = string(item.State)
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (api *WorkflowAPI) createRevision(c *gin.Context) {
	def, ok := api.owned(c)
	if !ok {
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&body)
	item, err := api.runtime.Kernel.Container().WorkflowDefRepo.SaveDraftRevision(c.Request.Context(), workflowUserID(c), def, body.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (api *WorkflowAPI) publishRevision(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	userID := workflowUserID(c)
	workflowID := c.Param("id")
	unlock := api.lockWorkflowMutation(userID, workflowID)
	defer unlock()

	current, ok := api.owned(c)
	if !ok {
		return
	}
	inst, err := api.installationFor(c.Request.Context(), current, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	expectedRevision, err := api.expectedRevision(c, inst.Revision)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := requireWorkflowRevision(expectedRevision, inst.Revision); err != nil {
		writeWorkflowRevisionConflict(c)
		return
	}

	revision, err := api.runtime.Kernel.Container().WorkflowDefRepo.GetRevision(c.Request.Context(), userID, workflowID, c.Param("revisionId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow revision not found"})
		return
	}
	previousState := revision.State
	previousPublishedAt := revision.PublishedAt
	previousArchivedAt := revision.ArchivedAt
	published, err := api.runtime.Kernel.Container().WorkflowDefRepo.PublishRevision(c.Request.Context(), userID, workflowID, revision.RevisionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	restoreLifecycle := func() {
		if previousState == published.State && previousPublishedAt == published.PublishedAt && previousArchivedAt == published.ArchivedAt {
			return
		}
		_, _ = api.runtime.Kernel.Container().WorkflowDefRepo.RestoreRevisionLifecycle(
			context.Background(), userID, workflowID, revision.RevisionID,
			previousState, previousPublishedAt, previousArchivedAt,
		)
	}
	target, err := workflow.CloneDefinition(revision.Definition)
	if err != nil {
		restoreLifecycle()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	target, err = api.prepareValidatedUserWorkflow(target, userID, current.ID)
	if err != nil {
		restoreLifecycle()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.validateExecutionTargets(target); err != nil {
		restoreLifecycle()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedInst := inst
	if current.DefinitionHash != target.DefinitionHash {
		if _, err := api.runtime.Kernel.Container().WorkflowDefRepo.SaveRevision(c.Request.Context(), userID, current, "发布新 Revision 前自动快照"); err != nil {
			restoreLifecycle()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), target); err != nil {
			restoreLifecycle()
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rollback := func() {
			_ = api.syncTriggers(c.Request.Context(), target, current, userID)
			_ = registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), current)
			restoreLifecycle()
		}
		if err := api.syncTriggers(c.Request.Context(), current, target, userID); err != nil {
			rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		updatedInst, err = api.updateInstallationCAS(c.Request.Context(), target, userID, inst, expectedRevision)
		if err != nil {
			rollback()
			if isWorkflowRevisionConflict(err) {
				writeWorkflowRevisionConflict(c)
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"revision": published,
		"workflow": workflowResponse(target, updatedInst),
	})
}

func (api *WorkflowAPI) archiveRevision(c *gin.Context) {
	userID := workflowUserID(c)
	workflowID := c.Param("id")
	unlock := api.lockWorkflowMutation(userID, workflowID)
	defer unlock()
	current, ok := api.owned(c)
	if !ok {
		return
	}
	repo := api.runtime.Kernel.Container().WorkflowDefRepo
	revision, err := repo.GetRevision(c.Request.Context(), userID, workflowID, c.Param("revisionId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow revision not found"})
		return
	}
	if revision.DefinitionHash == current.DefinitionHash {
		c.JSON(http.StatusConflict, gin.H{"error": "active workflow revision cannot be archived; publish or roll back another revision first"})
		return
	}
	archived, err := repo.ArchiveRevision(c.Request.Context(), userID, workflowID, revision.RevisionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, archived)
}

func (api *WorkflowAPI) rollbackRevision(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	userID := workflowUserID(c)
	unlock := api.lockWorkflowMutation(userID, c.Param("id"))
	defer unlock()
	current, ok := api.owned(c)
	if !ok {
		return
	}
	inst, err := api.installationFor(c.Request.Context(), current, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	expectedRevision, err := api.expectedRevision(c, inst.Revision)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := requireWorkflowRevision(expectedRevision, inst.Revision); err != nil {
		writeWorkflowRevisionConflict(c)
		return
	}
	revision, err := api.runtime.Kernel.Container().WorkflowDefRepo.GetRevision(c.Request.Context(), userID, current.ID, c.Param("revisionId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow revision not found"})
		return
	}
	target, err := workflow.CloneDefinition(revision.Definition)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	target, err = api.prepareValidatedUserWorkflow(target, userID, current.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.validateExecutionTargets(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if current.DefinitionHash == target.DefinitionHash {
		c.JSON(http.StatusOK, workflowResponse(target, inst))
		return
	}
	if _, err := api.runtime.Kernel.Container().WorkflowDefRepo.SaveRevision(c.Request.Context(), userID, current, "回滚前自动快照"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rollback := func() {
		_ = api.syncTriggers(c.Request.Context(), target, current, userID)
		_ = registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), current)
	}
	if err := api.syncTriggers(c.Request.Context(), current, target, userID); err != nil {
		rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedInst, err := api.updateInstallationCAS(c.Request.Context(), target, userID, inst, expectedRevision)
	if err != nil {
		rollback()
		if isWorkflowRevisionConflict(err) {
			writeWorkflowRevisionConflict(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, workflowResponse(target, updatedInst))
}

func (api *WorkflowAPI) delete(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	userID := workflowUserID(c)
	unlock := api.lockWorkflowMutation(userID, c.Param("id"))
	defer unlock()
	old, ok := api.owned(c)
	if !ok {
		return
	}
	inst, err := api.installationFor(c.Request.Context(), old, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := api.syncTriggers(c.Request.Context(), old, workflow.WorkflowDefinition{}, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := registry.UnregisterContext(api.workflowDefinitionMutationContext(c.Request.Context()), old.ID); err != nil {
		_ = api.syncTriggers(c.Request.Context(), workflow.WorkflowDefinition{}, old, userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	api.emitWorkflowInstallationEvent(c.Request.Context(), "workflow.installation.deleted", inst)
	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": old.ID})
}

func (api *WorkflowAPI) setEnabled(c *gin.Context, enabled bool) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	userID := workflowUserID(c)
	unlock := api.lockWorkflowMutation(userID, c.Param("id"))
	defer unlock()
	old, ok := api.owned(c)
	if !ok {
		return
	}
	currentInst, err := api.installationFor(c.Request.Context(), old, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	expectedRevision, err := api.expectedRevision(c, currentInst.Revision)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := requireWorkflowRevision(expectedRevision, currentInst.Revision); err != nil {
		writeWorkflowRevisionConflict(c)
		return
	}
	def := old
	def.Enabled = enabled
	if err := registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), def); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := api.syncTriggers(c.Request.Context(), old, def, userID); err != nil {
		_ = api.syncTriggers(c.Request.Context(), def, old, userID)
		_ = registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), old)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedInst, err := api.updateInstallationCAS(c.Request.Context(), def, userID, currentInst, expectedRevision)
	if err != nil {
		_ = api.syncTriggers(c.Request.Context(), def, old, userID)
		_ = registry.UpsertContext(api.workflowDefinitionMutationContext(c.Request.Context()), old)
		if isWorkflowRevisionConflict(err) {
			writeWorkflowRevisionConflict(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	eventType := "workflow.installation.disabled"
	if enabled {
		eventType = "workflow.installation.enabled"
	}
	api.emitWorkflowInstallationEvent(c.Request.Context(), eventType, updatedInst)
	c.JSON(http.StatusOK, gin.H{"id": def.ID, "enabled": enabled, "installation": updatedInst})
}
func (api *WorkflowAPI) enable(c *gin.Context)  { api.setEnabled(c, true) }
func (api *WorkflowAPI) disable(c *gin.Context) { api.setEnabled(c, false) }

func (api *WorkflowAPI) run(c *gin.Context) {
	_, executor, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	def, ok := api.owned(c)
	if !ok {
		return
	}
	if !def.Enabled {
		c.JSON(http.StatusConflict, gin.H{"error": "workflow is disabled"})
		return
	}
	var body struct {
		Input               json.RawMessage         `json:"input"`
		Wait                bool                    `json:"wait"`
		Mode                workflow.ExecutionMode  `json:"mode"`
		Mocks               []workflow.MockBehavior `json:"mocks"`
		ApprovedSideEffects []string                `json:"approvedSideEffects"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if len(body.Input) == 0 {
		body.Input = json.RawMessage(`{}`)
	}
	opts, err := (workflow.ExecutionOptions{Mode: body.Mode, Mocks: body.Mocks, ApprovedSideEffects: body.ApprovedSideEffects}).Normalize()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	report := api.preflightDefinition(c.Request.Context(), def, workflowUserID(c))
	if err := workflowPreflightBlockedError(report); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "WORKFLOW_PREFLIGHT_BLOCKED", "code": "WORKFLOW_PREFLIGHT_BLOCKED", "detail": err.Error(), "preflight": report})
		return
	}
	inst, err := api.installationFor(c.Request.Context(), def, workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	publishedRevision, err := api.runtime.Kernel.Container().WorkflowDefRepo.EnsurePublishedRevision(c.Request.Context(), workflowUserID(c), def, "运行时发布绑定")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	executionID := "wf-run-" + uuid.NewString()
	req := workflow.ExecuteRequest{WorkflowID: def.ID, Input: body.Input, Context: workflow.ExecutionContext{UserID: workflowUserID(c), WorkflowID: def.ID, InstallationID: inst.InstallationID, RevisionID: publishedRevision.RevisionID, RootID: executionID, InvocationID: executionID, OperationID: "wf-op-" + uuid.NewString(), TraceID: "trace-" + uuid.NewString()}, Options: opts}
	mustReturnInitialResult := body.Wait || opts.IsDryRun() || (opts.Mode == workflow.ExecutionModeControlled && len(opts.MissingControlledApprovals(def.Nodes)) > 0)
	if mustReturnInitialResult {
		result, err := executor.Execute(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "executionId": executionID})
			return
		}
		status := http.StatusOK
		if result.Status == workflow.RunStatusWaitingConfirmation {
			status = http.StatusAccepted
		}
		c.JSON(status, result)
		return
	}
	go func() { _, _ = executor.Execute(context.Background(), req) }()
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "executionId": executionID, "workflowId": def.ID, "status": workflow.RunStatusRunning, "executionMode": opts.Mode})
}

func parseWorkflowPagination(c *gin.Context) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (api *WorkflowAPI) analysis(c *gin.Context) {
	def, ok := api.owned(c)
	if !ok {
		return
	}
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, analyzeWorkflowRisk(def, registry, workflowUserID(c)))
}

func (api *WorkflowAPI) stats(c *gin.Context) {
	if _, ok := api.owned(c); !ok {
		return
	}
	stats, err := api.runtime.Kernel.Container().WorkflowExecRepo.GetStats(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (api *WorkflowAPI) listRuns(c *gin.Context) {
	if _, ok := api.owned(c); !ok {
		return
	}
	kc := api.runtime.Kernel.Container()
	limit, offset := parseWorkflowPagination(c)
	items, total, err := kc.WorkflowExecRepo.ListRuns(c.Request.Context(), c.Param("id"), workflow.RunStatus(c.Query("status")), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (api *WorkflowAPI) getRun(c *gin.Context) {
	if _, _, err := api.kernelContainer(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	userID := workflowUserID(c)
	runID := strings.TrimSpace(c.Param("runId"))
	kc := api.runtime.Kernel.Container()
	if run, _, localErr := api.localRunOwned(ctx, userID, runID); localErr == nil && run != nil {
		steps, listErr := kc.WorkflowExecRepo.ListStepRuns(ctx, run.ExecutionID)
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": listErr.Error()})
			return
		}
		attempts, listErr := kc.WorkflowExecRepo.ListStepAttempts(ctx, run.ExecutionID)
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": listErr.Error()})
			return
		}
		compensations, listErr := kc.WorkflowExecRepo.ListCompensations(ctx, run.ExecutionID)
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": listErr.Error()})
			return
		}
		checkpoints := []workflow.Checkpoint{}
		if store := kc.WorkflowExecutor.CheckpointStore(); store != nil {
			checkpoints, listErr = store.List(ctx, run.ExecutionID)
			if listErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": listErr.Error()})
				return
			}
		}
		def, ok := kc.WorkflowRegistry.Get(run.WorkflowID)
		if !ok && len(run.Context.DefinitionSnapshot) > 0 {
			_ = json.Unmarshal(run.Context.DefinitionSnapshot, &def)
		}
		c.JSON(http.StatusOK, gin.H{
			"run":                   run,
			"classifiedError":       classifyWorkflowRunError(run, steps),
			"stepRuns":              steps,
			"attempts":              attempts,
			"trace":                 workflow.BuildDistributedTrace(run, steps, attempts),
			"checkpoints":           checkpoints,
			"compensations":         compensations,
			"workflow":              def,
			"requiredConfirmations": workflow.MissingControlledApprovalsForRun(run),
			"executionOwner": gin.H{
				"kind": "local",
			},
		})
		return
	}

	result, _, remoteErr := api.resolveRemoteWorkflowRun(ctx, userID, runID, WorkflowMeshRunGet, nil)
	if remoteErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) getRunLogs(c *gin.Context) {
	if _, _, err := api.kernelContainer(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	userID := workflowUserID(c)
	runID := strings.TrimSpace(c.Param("runId"))
	kc := api.runtime.Kernel.Container()
	if run, _, localErr := api.localRunOwned(ctx, userID, runID); localErr == nil && run != nil {
		steps, err := kc.WorkflowExecRepo.ListStepRuns(ctx, run.ExecutionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		attempts, err := kc.WorkflowExecRepo.ListStepAttempts(ctx, run.ExecutionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		compensations, err := kc.WorkflowExecRepo.ListCompensations(ctx, run.ExecutionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": workflow.BuildWorkflowRunLogs(run, steps, attempts, compensations)})
		return
	}
	result, _, remoteErr := api.resolveRemoteWorkflowRun(ctx, userID, runID, WorkflowMeshRunLogs, nil)
	if remoteErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) localRunOwned(ctx context.Context, userID, runID string) (*workflow.WorkflowRun, *workflow.WorkflowExecutor, error) {
	_, executor, err := api.kernelContainer()
	if err != nil {
		return nil, nil, err
	}
	kc := api.runtime.Kernel.Container()
	run, err := kc.WorkflowExecRepo.Get(ctx, strings.TrimSpace(runID))
	if err != nil || run == nil {
		return nil, nil, errors.New("workflow run not found")
	}
	ownerID := strings.TrimSpace(run.Context.UserID)
	requestedUserID := strings.TrimSpace(userID)
	if ownerID != "" {
		if requestedUserID == "" || ownerID != requestedUserID {
			return nil, nil, errors.New("workflow run not found")
		}
		return run, executor, nil
	}
	// Legacy runs created before the execution context persisted UserID must
	// still resolve through the current definition ownership metadata.
	def, ok := kc.WorkflowRegistry.Get(run.WorkflowID)
	if !ok || !workflowOwnedBy(def, requestedUserID) {
		return nil, nil, errors.New("workflow run not found")
	}
	return run, executor, nil
}

func (api *WorkflowAPI) runOwned(c *gin.Context) (*workflow.WorkflowRun, *workflow.WorkflowExecutor, bool) {
	run, executor, err := api.localRunOwned(c.Request.Context(), workflowUserID(c), c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return nil, nil, false
	}
	return run, executor, true
}

func (api *WorkflowAPI) cancelRun(c *gin.Context) {
	ctx := c.Request.Context()
	userID := workflowUserID(c)
	runID := c.Param("runId")
	if _, executor, err := api.localRunOwned(ctx, userID, runID); err == nil {
		cancelled, cancelErr := executor.CancelRun(ctx, runID)
		if cancelErr != nil {
			c.JSON(http.StatusConflict, gin.H{"error": cancelErr.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"cancelled": cancelled})
		return
	}
	result, _, err := api.resolveRemoteWorkflowRun(ctx, userID, runID, WorkflowMeshRunCancel, nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) pauseRun(c *gin.Context) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	ctx := c.Request.Context()
	userID := workflowUserID(c)
	runID := c.Param("runId")
	if _, executor, err := api.localRunOwned(ctx, userID, runID); err == nil {
		run, pauseErr := executor.Pause(ctx, runID, body.Reason)
		if pauseErr != nil {
			c.JSON(http.StatusConflict, gin.H{"error": pauseErr.Error()})
			return
		}
		c.JSON(http.StatusOK, run)
		return
	}
	result, _, err := api.resolveRemoteWorkflowRun(ctx, userID, runID, WorkflowMeshRunPause, map[string]any{"reason": body.Reason})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) resumeRun(c *gin.Context) {
	ctx := c.Request.Context()
	userID := workflowUserID(c)
	runID := c.Param("runId")
	if _, executor, err := api.localRunOwned(ctx, userID, runID); err == nil {
		run, resumeErr := executor.Resume(ctx, runID)
		if resumeErr != nil {
			c.JSON(http.StatusConflict, gin.H{"error": resumeErr.Error()})
			return
		}
		c.JSON(http.StatusOK, run)
		return
	}
	result, _, err := api.resolveRemoteWorkflowRun(ctx, userID, runID, WorkflowMeshRunResume, nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func (api *WorkflowAPI) confirmRun(c *gin.Context) {
	var body struct {
		NodeIDs []string `json:"nodeIds"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	ctx := c.Request.Context()
	userID := workflowUserID(c)
	runID := c.Param("runId")
	if _, executor, err := api.localRunOwned(ctx, userID, runID); err == nil {
		run, missing, confirmErr := executor.ConfirmControlledRun(ctx, runID, body.NodeIDs)
		if confirmErr != nil {
			c.JSON(http.StatusConflict, gin.H{"error": confirmErr.Error()})
			return
		}
		status := http.StatusAccepted
		if len(missing) > 0 {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"accepted": len(missing) == 0, "run": run, "missingConfirmations": missing})
		return
	}
	result, _, err := api.resolveRemoteWorkflowRun(ctx, userID, runID, WorkflowMeshRunConfirm, map[string]any{"nodeIds": body.NodeIDs})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}
	c.Data(http.StatusAccepted, "application/json", result)
}

func (api *WorkflowAPI) recoverRun(c *gin.Context) {
	ctx := c.Request.Context()
	userID := workflowUserID(c)
	runID := c.Param("runId")
	run, executor, localErr := api.localRunOwned(ctx, userID, runID)
	if localErr != nil {
		result, _, remoteErr := api.resolveRemoteWorkflowRun(ctx, userID, runID, WorkflowMeshRunRecover, nil)
		if remoteErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
			return
		}
		c.Data(http.StatusAccepted, "application/json", result)
		return
	}
	if run.Status != workflow.RunStatusFailed && run.Status != workflow.RunStatusCancelled {
		c.JSON(http.StatusConflict, gin.H{"error": "only failed or cancelled runs can recover from checkpoints"})
		return
	}
	if strings.TrimSpace(run.Context.DefinitionHash) == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "run predates safe checkpoint recovery; rerun the workflow instead"})
		return
	}
	if len(run.Context.DefinitionSnapshot) > 0 {
		var snapshot workflow.WorkflowDefinition
		if err := json.Unmarshal(run.Context.DefinitionSnapshot, &snapshot); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "run definition snapshot is invalid; checkpoint recovery is unsafe"})
			return
		}
		if snapshot.ID != run.WorkflowID || workflow.ComputeDefinitionHash(snapshot) != run.Context.DefinitionHash {
			c.JSON(http.StatusConflict, gin.H{"error": "run definition snapshot integrity check failed; checkpoint recovery is unsafe"})
			return
		}
	} else {
		// Legacy runs without immutable snapshots can only recover when the
		// currently installed definition still matches exactly.
		def, exists := api.runtime.Kernel.Container().WorkflowRegistry.Get(run.WorkflowID)
		if !exists || !workflowOwnedBy(def, userID) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
			return
		}
		if !def.Enabled {
			c.JSON(http.StatusConflict, gin.H{"error": "workflow is disabled"})
			return
		}
		if workflow.ComputeDefinitionHash(def) != run.Context.DefinitionHash {
			c.JSON(http.StatusConflict, gin.H{"error": "workflow definition changed since this legacy run; checkpoint recovery is unsafe, rerun instead"})
			return
		}
	}
	store := executor.CheckpointStore()
	if store == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "checkpoint store unavailable"})
		return
	}
	checkpoints, err := store.List(ctx, run.ExecutionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(checkpoints) == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "this run has no checkpoint to recover from"})
		return
	}
	execution := run.Context
	execution.UserID = userID
	execution.InvocationID = run.ExecutionID
	execution.Recovery = true
	execution.Generation = run.Generation + 1
	execution.OperationID = "wf-recover-" + uuid.NewString()
	execution.TraceID = "trace-" + uuid.NewString()
	req := workflow.ExecuteRequest{WorkflowID: run.WorkflowID, Input: run.Input, Context: execution}
	go func() { _, _ = executor.Execute(context.Background(), req) }()
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "executionId": run.ExecutionID, "workflowId": run.WorkflowID, "status": workflow.RunStatusRunning, "generation": execution.Generation, "checkpointCount": len(checkpoints)})
}

func (api *WorkflowAPI) rerunRun(c *gin.Context) {
	previous, executor, ok := api.runOwned(c)
	if !ok {
		return
	}
	if !previous.Status.IsTerminal() {
		c.JSON(http.StatusConflict, gin.H{"error": "workflow run must be terminal before rerun"})
		return
	}
	kc := api.runtime.Kernel.Container()
	def, exists := kc.WorkflowRegistry.Get(previous.WorkflowID)
	if !exists || !workflowOwnedBy(def, workflowUserID(c)) {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	if !def.Enabled {
		c.JSON(http.StatusConflict, gin.H{"error": "workflow is disabled"})
		return
	}
	var body struct {
		Wait bool `json:"wait"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	input := previous.Input
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	inst, err := api.installationFor(c.Request.Context(), def, workflowUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	executionID := "wf-run-" + uuid.NewString()
	opts := workflow.ExecutionOptionsForRerun(previous)
	req := workflow.ExecuteRequest{
		WorkflowID: def.ID,
		Input:      input,
		Context: workflow.ExecutionContext{
			UserID:         workflowUserID(c),
			WorkflowID:     def.ID,
			InstallationID: inst.InstallationID,
			RootID:         executionID,
			InvocationID:   executionID,
			OperationID:    "wf-op-" + uuid.NewString(),
			TraceID:        "trace-" + uuid.NewString(),
			IdempotencyKey: executionID,
		},
		Options: opts,
	}
	materializeInitialState := body.Wait || opts.Mode == workflow.ExecutionModeDryRun || (opts.Mode == workflow.ExecutionModeControlled && len(opts.MissingControlledApprovals(def.Nodes)) > 0)
	if materializeInitialState {
		result, err := executor.Execute(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "executionId": executionID})
			return
		}
		status := http.StatusOK
		if result.Status == workflow.RunStatusWaitingConfirmation {
			status = http.StatusAccepted
		}
		c.JSON(status, gin.H{
			"executionId":           result.ExecutionID,
			"workflowId":            result.WorkflowID,
			"status":                result.Status,
			"success":               result.Success,
			"output":                result.Output,
			"steps":                 result.Steps,
			"error":                 result.Error,
			"duration":              result.Duration,
			"executionMode":         result.ExecutionMode,
			"requiredConfirmations": result.RequiredConfirmations,
			"sourceExecutionId":     previous.ExecutionID,
		})
		return
	}
	go func() { _, _ = executor.Execute(context.Background(), req) }()
	c.JSON(http.StatusAccepted, gin.H{
		"accepted":          true,
		"executionId":       executionID,
		"workflowId":        def.ID,
		"status":            workflow.RunStatusRunning,
		"executionMode":     opts.Mode,
		"sourceExecutionId": previous.ExecutionID,
	})
}

func (api *WorkflowAPI) dispatchEvent(c *gin.Context) {
	if _, _, err := api.kernelContainer(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	kc := api.runtime.Kernel.Container()
	if kc.WorkflowTriggerManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow trigger manager unavailable"})
		return
	}
	if c.Request.ContentLength > 128*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "event payload exceeds 128 KiB"})
		return
	}
	payload, err := c.GetRawData()
	if err != nil || len(payload) == 0 {
		payload = []byte(`{}`)
	}
	if len(payload) > 128*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "event payload exceeds 128 KiB"})
		return
	}
	if !json.Valid(payload) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event payload must be valid JSON"})
		return
	}
	userID := workflowUserID(c)
	eventType := strings.TrimSpace(c.Param("eventType"))
	if isDeviceWorkflowEventType(eventType) && api.effectiveLocation() == workflow.WorkflowLocationLocal {
		allowed, retryAfter := api.runtime.AllowWorkflowTriggerIngress(userID, eventType)
		if !allowed {
			retrySeconds := int((retryAfter + time.Second - 1) / time.Second)
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(retrySeconds))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "workflow device event rate limit exceeded", "retryAfterMs": retryAfter.Milliseconds()})
			return
		}
	}
	qualifiedEventType := "user:" + userID + ":" + eventType
	var envelope struct {
		EventID    string          `json:"eventId"`
		Source     string          `json:"source"`
		OccurredAt time.Time       `json:"occurredAt"`
		Payload    json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil && strings.TrimSpace(envelope.EventID) != "" && len(envelope.Payload) > 0 {
		eventID := strings.TrimSpace(envelope.EventID)
		if len(eventID) > 200 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "eventId is too long"})
			return
		}
		source := strings.TrimSpace(envelope.Source)
		if len(source) > 128 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "event source is too long"})
			return
		}
		occurredAt := envelope.OccurredAt
		if isDeviceWorkflowEventType(eventType) && api.effectiveLocation() == workflow.WorkflowLocationLocal {
			now := time.Now().UTC()
			if occurredAt.IsZero() || occurredAt.Before(now.Add(-5*time.Minute)) || occurredAt.After(now.Add(5*time.Minute)) {
				occurredAt = now
			}
		}
		deviceID := ""
		if api.effectiveLocation() == workflow.WorkflowLocationLocal {
			deviceID = strings.TrimSpace(c.GetHeader("X-Amitia-Device-ID"))
			if len(deviceID) > 200 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "device id is too long"})
				return
			}
		}
		structuredEventType := qualifiedEventType
		structuredOwnerUserID := userID
		execution := workflow.ExecutionContext{UserID: userID, DeviceID: deviceID}
		if isDeviceWorkflowEventType(eventType) && api.effectiveLocation() == workflow.WorkflowLocationLocal {
			actor := security.GetActor(c)
			if actor != nil && actor.AuthMethod == security.AuthMethodLocalToken {
				// Root-token device producers are device-scoped rather than account-scoped.
				// Cloud-installed local workflows retain their cloud owner in each TriggerBinding;
				// the TriggerManager resolves that owner per binding so Android never has to know
				// or assert a cloud account ID. Desktop Sessions stay account-scoped.
				structuredEventType = eventType
				structuredOwnerUserID = ""
				execution.UserID = ""
			}
		}
		event := workflow.WorkflowTriggerEvent{
			EventID:     eventID,
			EventType:   structuredEventType,
			Source:      source,
			OwnerUserID: structuredOwnerUserID,
			DeviceID:    deviceID,
			OccurredAt:  occurredAt,
			Payload:     envelope.Payload,
		}
		if err := kc.WorkflowTriggerManager.HandleStructuredEvent(c.Request.Context(), event, execution); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"accepted": true, "eventId": eventID, "eventType": eventType})
		return
	}
	if isDeviceWorkflowEventType(eventType) && api.effectiveLocation() == workflow.WorkflowLocationLocal {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device workflow events require a structured event envelope"})
		return
	}
	if err := kc.WorkflowTriggerManager.HandleEvent(c.Request.Context(), qualifiedEventType, payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "eventType": eventType})
}

func validateWorkflowEventTriggerConfig(trigger workflow.WorkflowTriggerDefinition, userID string) error {
	eventType := strings.TrimSpace(trigger.EventType)
	if len(trigger.Config) == 0 {
		switch eventType {
		case "device.android.intent", "device.android.tasker", "voice.wake.detected", "voice.asr.final", "device.app.foreground":
			return errors.New("device event trigger config is required")
		default:
			return nil
		}
	}
	if len(trigger.Config) > 32*1024 {
		return errors.New("workflow trigger config exceeds 32 KiB")
	}
	switch eventType {
	case "device.android.intent":
		var cfg struct {
			Actions       []string `json:"actions"`
			Categories    []string `json:"categories"`
			DataSchemes   []string `json:"dataSchemes"`
			MimeTypes     []string `json:"mimeTypes"`
			DedupWindowMS int64    `json:"dedupWindowMs"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if len(cfg.Actions) == 0 {
			return errors.New("android intent trigger requires at least one action")
		}
		if err := validateTriggerStringList("android intent actions", cfg.Actions, 32, 512); err != nil {
			return err
		}
		if err := validateTriggerStringList("android intent categories", cfg.Categories, 32, 512); err != nil {
			return err
		}
		if err := validateTriggerStringList("android intent dataSchemes", cfg.DataSchemes, 16, 128); err != nil {
			return err
		}
		if err := validateTriggerStringList("android intent mimeTypes", cfg.MimeTypes, 32, 256); err != nil {
			return err
		}
		if cfg.DedupWindowMS < 0 || cfg.DedupWindowMS > 10*60*1000 {
			return errors.New("android intent dedupWindowMs must be between 0 and 600000")
		}
	case "device.android.tasker":
		var cfg struct {
			EventName        string   `json:"eventName"`
			SecretRef        string   `json:"secretRef"`
			AllowedVariables []string `json:"allowedVariables"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		cfg.EventName = strings.TrimSpace(cfg.EventName)
		if cfg.EventName == "" {
			return errors.New("tasker trigger eventName is required")
		}
		if len(cfg.EventName) > 128 {
			return errors.New("tasker trigger eventName exceeds 128 characters")
		}
		if len(strings.TrimSpace(cfg.SecretRef)) > 512 {
			return errors.New("tasker trigger secretRef is too long")
		}
		if !strings.HasPrefix(strings.TrimSpace(cfg.SecretRef), "secret://") {
			return errors.New("tasker trigger secretRef must use secret://")
		}
		if !workflow.TriggerSecretRefOwnedByUser(cfg.SecretRef, userID) {
			return errors.New("tasker trigger secretRef does not belong to the workflow owner")
		}
		if err := validateTriggerStringList("tasker allowedVariables", cfg.AllowedVariables, 32, 128); err != nil {
			return err
		}
	case "voice.wake.detected":
		var cfg struct {
			Mode         string `json:"mode"`
			WakeConfigID string `json:"wakeConfigId"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if cfg.Mode != "" && cfg.Mode != "wake" {
			return errors.New("voice wake trigger mode must be wake")
		}
		cfg.WakeConfigID = strings.TrimSpace(cfg.WakeConfigID)
		if cfg.WakeConfigID == "" {
			return errors.New("voice wake trigger wakeConfigId is required")
		}
		if len(cfg.WakeConfigID) > 200 {
			return errors.New("voice wake trigger wakeConfigId is too long")
		}
	case "voice.asr.final":
		var cfg struct {
			Mode      string   `json:"mode"`
			Phrases   []string `json:"phrases"`
			MatchMode string   `json:"matchMode"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if cfg.Mode != "" && cfg.Mode != "phrase" {
			return errors.New("voice phrase trigger mode must be phrase")
		}
		if len(cfg.Phrases) == 0 {
			return errors.New("voice phrase trigger requires phrases")
		}
		if err := validateTriggerStringList("voice phrases", cfg.Phrases, 32, 256); err != nil {
			return err
		}
		if cfg.MatchMode != "" && cfg.MatchMode != "exact" && cfg.MatchMode != "normalized" {
			return errors.New("voice phrase matchMode must be exact or normalized")
		}
	case "device.app.foreground":
		var cfg struct {
			Packages   []string `json:"packages"`
			CooldownMS int64    `json:"cooldownMs"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if len(cfg.Packages) == 0 {
			return errors.New("app foreground trigger requires packages")
		}
		if err := validateTriggerStringList("app foreground packages", cfg.Packages, 64, 255); err != nil {
			return err
		}
		if cfg.CooldownMS < 0 || cfg.CooldownMS > 24*60*60*1000 {
			return errors.New("app foreground cooldownMs must be between 0 and 86400000")
		}
	case "device.notification.posted", "device.notification.removed":
		var cfg struct {
			Packages      []string `json:"packages"`
			TitleContains string   `json:"titleContains"`
			TextContains  string   `json:"textContains"`
			ChannelIDs    []string `json:"channelIds"`
			Categories    []string `json:"categories"`
			Ongoing       *bool    `json:"ongoing"`
			Clearable     *bool    `json:"clearable"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if err := validateTriggerStringList("notification packages", cfg.Packages, 64, 255); err != nil {
			return err
		}
		if err := validateTriggerStringList("notification channelIds", cfg.ChannelIDs, 64, 255); err != nil {
			return err
		}
		if err := validateTriggerStringList("notification categories", cfg.Categories, 32, 128); err != nil {
			return err
		}
		if len(cfg.TitleContains) > 512 || len(cfg.TextContains) > 1024 {
			return errors.New("notification text filter exceeds limits")
		}
	case "device.power.battery_changed":
		var cfg struct {
			MinPercent *int  `json:"minPercent"`
			MaxPercent *int  `json:"maxPercent"`
			Charging   *bool `json:"charging"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if cfg.MinPercent != nil && (*cfg.MinPercent < 0 || *cfg.MinPercent > 100) {
			return errors.New("battery minPercent must be between 0 and 100")
		}
		if cfg.MaxPercent != nil && (*cfg.MaxPercent < 0 || *cfg.MaxPercent > 100) {
			return errors.New("battery maxPercent must be between 0 and 100")
		}
		if cfg.MinPercent != nil && cfg.MaxPercent != nil && *cfg.MinPercent > *cfg.MaxPercent {
			return errors.New("battery minPercent must not exceed maxPercent")
		}
	case "device.network.available", "device.network.lost", "device.network.changed":
		var cfg struct {
			Transports []string `json:"transports"`
			Validated  *bool    `json:"validated"`
			Metered    *bool    `json:"metered"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if err := validateTriggerStringList("network transports", cfg.Transports, 8, 32); err != nil {
			return err
		}
		for _, transport := range cfg.Transports {
			switch strings.ToLower(strings.TrimSpace(transport)) {
			case "wifi", "cellular", "ethernet", "vpn", "bluetooth", "other":
			default:
				return fmt.Errorf("unsupported network transport %q", transport)
			}
		}
	case "device.app.installed", "device.app.removed", "device.app.updated", "device.app.self_updated":
		var cfg struct {
			Packages []string `json:"packages"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if err := validateTriggerStringList("package event packages", cfg.Packages, 64, 255); err != nil {
			return err
		}
	case "device.ble.characteristic_changed":
		var cfg struct {
			SessionID          string `json:"sessionId"`
			Address            string `json:"address"`
			ServiceUUID        string `json:"serviceUuid"`
			CharacteristicUUID string `json:"characteristicUuid"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if len(cfg.SessionID) > 256 || len(cfg.Address) > 64 || len(cfg.ServiceUUID) > 128 || len(cfg.CharacteristicUUID) > 128 {
			return errors.New("BLE characteristic trigger filter exceeds limits")
		}
	case "device.location.geofence.enter", "device.location.geofence.exit":
		var cfg struct {
			FenceIDs []string `json:"fenceIds"`
		}
		if err := decodeWorkflowTriggerConfig(trigger.Config, &cfg); err != nil {
			return err
		}
		if err := validateTriggerStringList("geofence ids", cfg.FenceIDs, 64, 128); err != nil {
			return err
		}
	}
	return nil
}

func decodeWorkflowTriggerConfig(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid device trigger config: %w", err)
	}
	return nil
}

func validateTriggerStringList(name string, values []string, maxItems, maxLength int) error {
	if len(values) > maxItems {
		return fmt.Errorf("%s exceeds %d items", name, maxItems)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
		if len(value) > maxLength {
			return fmt.Errorf("%s contains a value longer than %d characters", name, maxLength)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%s contains duplicate value %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func isDeviceWorkflowEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "device.android.intent", "device.android.tasker", "voice.wake.detected", "voice.asr.final",
		"device.app.foreground", "device.notification.posted", "device.notification.removed", "device.power.battery_changed",
		"device.power.battery_low", "device.power.battery_okay", "device.power.connected", "device.power.disconnected",
		"device.screen.on", "device.screen.off", "device.user.present", "device.audio.headset_connected",
		"device.audio.headset_disconnected", "device.bluetooth.state_changed", "device.bluetooth.connected", "device.bluetooth.disconnected",
		"device.ble.characteristic_changed", "device.network.available", "device.network.lost", "device.network.changed",
		"device.wifi.enabled", "device.wifi.disabled", "device.wifi.state_changed", "device.wifi.connected",
		"device.wifi.disconnected", "device.system.boot_completed", "device.app.installed", "device.app.removed",
		"device.app.updated", "device.app.self_updated", "device.time.changed", "device.time.timezone_changed",
		"device.time.date_changed", "device.location.geofence.enter", "device.location.geofence.exit":
		return true
	default:
		return false
	}
}

func (api *WorkflowAPI) syncTriggers(ctx context.Context, oldDef, newDef workflow.WorkflowDefinition, userID string) error {
	kc := api.runtime.Kernel.Container()
	if kc == nil || kc.WorkflowDefRepo == nil {
		return errors.New("workflow trigger store unavailable")
	}
	if newDef.ID != "" && !newDef.Enabled {
		newDef.Triggers = nil
	}
	if oldDef.ID != "" {
		_ = kc.WorkflowDefRepo.DeleteTriggersByWorkflow(ctx, oldDef.ID)
	}
	for _, trigger := range newDef.Triggers {
		if !trigger.Enabled || trigger.Type != "event" {
			continue
		}
		eventType := strings.TrimSpace(trigger.EventType)
		if err := kc.WorkflowDefRepo.SaveTrigger(ctx, workflow.TriggerBinding{BindingID: "userwf:" + newDef.ID + ":" + trigger.ID, Type: workflow.TriggerTypeEvent, EventType: "user:" + userID + ":" + eventType, WorkflowID: newDef.ID, Config: trigger.Config, Input: trigger.Input, Enabled: true}); err != nil {
			return err
		}
	}
	hasSchedules := false
	for _, trigger := range append(append([]workflow.WorkflowTriggerDefinition{}, oldDef.Triggers...), newDef.Triggers...) {
		if trigger.Enabled && isScheduleTrigger(trigger.Type) {
			hasSchedules = true
			break
		}
	}
	if hasSchedules && kc.ScheduleService == nil {
		return errors.New("workflow schedule service unavailable")
	}
	if kc.ScheduleService != nil {
		for _, trigger := range oldDef.Triggers {
			if isScheduleTrigger(trigger.Type) {
				_ = kc.ScheduleService.Uninstall(ctx, scheduleIDFor(oldDef.ID, trigger.ID))
			}
		}
		for _, trigger := range newDef.Triggers {
			if !trigger.Enabled || !isScheduleTrigger(trigger.Type) {
				continue
			}
			def, err := buildWorkflowSchedule(newDef, trigger, userID)
			if err != nil {
				return err
			}
			if err := kc.ScheduleService.InstallDefinition(ctx, def); err != nil {
				return fmt.Errorf("install workflow schedule %s: %w", trigger.ID, err)
			}
		}
	}
	if kc.ToolRegistry != nil {
		if oldDef.ID != "" && oldDef.ID != newDef.ID {
			if err := kernelruntime.RemoveUserWorkflowAgentTool(ctx, kc.ToolRegistry, oldDef.ID); err != nil {
				return fmt.Errorf("remove workflow agent tool: %w", err)
			}
		}
		if newDef.ID != "" {
			if err := kernelruntime.SyncUserWorkflowAgentTool(ctx, kc.ToolRegistry, newDef); err != nil {
				return fmt.Errorf("sync workflow agent tool: %w", err)
			}
		}
	}
	return nil
}

func isScheduleTrigger(t string) bool {
	return t == "schedule" || t == "cron" || t == "interval" || t == "one_shot"
}
func scheduleIDFor(workflowID, triggerID string) string {
	return userWorkflowSchedulePrefix + workflowID + "-" + triggerID
}

func buildWorkflowSchedule(def workflow.WorkflowDefinition, trigger workflow.WorkflowTriggerDefinition, userID string) (*schedule.ScheduleContributionDefinition, error) {
	var cfg struct {
		Type            string `json:"type"`
		CronExpression  string `json:"cronExpression"`
		Seconds         bool   `json:"seconds"`
		IntervalSeconds int64  `json:"intervalSeconds"`
		RunAt           string `json:"runAt"`
		Timezone        string `json:"timezone"`
		MisfirePolicy   string `json:"misfirePolicy"`
		MaxCatchUp      int    `json:"maxCatchUp"`
		OverlapPolicy   string `json:"overlapPolicy"`
		DSTSpringPolicy string `json:"dstSpringPolicy"`
		DSTFallPolicy   string `json:"dstFallPolicy"`
	}
	raw := trigger.Config
	if len(trigger.Schedule) > 0 {
		raw = trigger.Schedule
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("trigger %s schedule config: %w", trigger.ID, err)
		}
	}
	kind := cfg.Type
	if kind == "" {
		kind = trigger.Type
	}
	if kind == "schedule" {
		kind = "cron"
	}
	var td schedule.ScheduleTriggerDefinition
	switch kind {
	case "cron":
		if strings.TrimSpace(cfg.CronExpression) == "" {
			return nil, fmt.Errorf("trigger %s cronExpression is required", trigger.ID)
		}
		td = schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeCron, Cron: &schedule.CronTriggerDefinition{Expression: cfg.CronExpression, Seconds: cfg.Seconds}}
	case "interval":
		if cfg.IntervalSeconds <= 0 {
			return nil, fmt.Errorf("trigger %s intervalSeconds must be > 0", trigger.ID)
		}
		td = schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeInterval, Interval: &schedule.IntervalTriggerDefinition{Interval: time.Duration(cfg.IntervalSeconds) * time.Second, AnchorAt: time.Now().UTC()}}
	case "one_shot":
		runAt, err := time.Parse(time.RFC3339, cfg.RunAt)
		if err != nil {
			return nil, fmt.Errorf("trigger %s runAt must be RFC3339", trigger.ID)
		}
		td = schedule.ScheduleTriggerDefinition{Type: schedule.TriggerTypeOneShot, OneShot: &schedule.OneShotTriggerDefinition{RunAt: runAt}}
	default:
		return nil, fmt.Errorf("trigger %s unsupported schedule type %s", trigger.ID, kind)
	}
	tz := cfg.Timezone
	if tz == "" {
		tz = "UTC"
	}
	input := trigger.Input
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	misfirePolicy := schedule.DefaultMisfirePolicy()
	if rawPolicy := strings.ToLower(strings.TrimSpace(cfg.MisfirePolicy)); rawPolicy != "" {
		if rawPolicy == "catch_up" {
			rawPolicy = string(schedule.MisfirePolicyCatchUpLimited)
		}
		switch schedule.MisfirePolicy(rawPolicy) {
		case schedule.MisfirePolicySkip, schedule.MisfirePolicyFireOnce, schedule.MisfirePolicyCatchUpLimited, schedule.MisfirePolicyRescheduleFromNow:
			misfirePolicy.Policy = schedule.MisfirePolicy(rawPolicy)
		default:
			return nil, fmt.Errorf("trigger %s unsupported misfirePolicy %s", trigger.ID, cfg.MisfirePolicy)
		}
	}
	if cfg.MaxCatchUp < 0 || cfg.MaxCatchUp > 1000 {
		return nil, fmt.Errorf("trigger %s maxCatchUp must be between 0 and 1000", trigger.ID)
	}
	if cfg.MaxCatchUp > 0 {
		misfirePolicy.MaxCatchUp = cfg.MaxCatchUp
	}
	overlapPolicy := schedule.DefaultOverlapPolicy()
	if rawOverlap := strings.ToLower(strings.TrimSpace(cfg.OverlapPolicy)); rawOverlap != "" {
		switch schedule.OverlapPolicy(rawOverlap) {
		case schedule.OverlapPolicyForbid, schedule.OverlapPolicyAllow, schedule.OverlapPolicyReplace, schedule.OverlapPolicyQueueOne, schedule.OverlapPolicySkipIfRunning:
			overlapPolicy.Policy = schedule.OverlapPolicy(rawOverlap)
		default:
			return nil, fmt.Errorf("trigger %s unsupported overlapPolicy %s", trigger.ID, cfg.OverlapPolicy)
		}
	}
	springPolicy := schedule.DefaultDSTSpringPolicy()
	if rawSpring := strings.ToLower(strings.TrimSpace(cfg.DSTSpringPolicy)); rawSpring != "" {
		switch schedule.DSTSpringPolicy(rawSpring) {
		case schedule.DSTSpringSkip, schedule.DSTSpringFireOnceAfterGap, schedule.DSTSpringNextValidTime:
			springPolicy = schedule.DSTSpringPolicy(rawSpring)
		default:
			return nil, fmt.Errorf("trigger %s unsupported dstSpringPolicy %s", trigger.ID, cfg.DSTSpringPolicy)
		}
	}
	fallPolicy := schedule.DefaultDSTFallPolicy()
	if rawFall := strings.ToLower(strings.TrimSpace(cfg.DSTFallPolicy)); rawFall != "" {
		switch schedule.DSTFallPolicy(rawFall) {
		case schedule.DSTFallFireOnceFirst, schedule.DSTFallFireOnceSecond, schedule.DSTFallFireTwice:
			fallPolicy = schedule.DSTFallPolicy(rawFall)
		default:
			return nil, fmt.Errorf("trigger %s unsupported dstFallPolicy %s", trigger.ID, cfg.DSTFallPolicy)
		}
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, fmt.Errorf("trigger %s invalid timezone %s: %w", trigger.ID, tz, err)
	}
	return &schedule.ScheduleContributionDefinition{
		ContributionID: scheduleIDFor(def.ID, trigger.ID), ExtensionID: "", ScheduleID: scheduleIDFor(def.ID, trigger.ID),
		Name: def.Name + " / " + trigger.ID, Description: "Creative Workshop workflow trigger", Trigger: td,
		Target:   schedule.ScheduleTargetDefinition{Type: schedule.TargetTypeWorkflow, TargetID: def.ID, InputTemplate: input, IdempotencyMode: schedule.IdempotencyModeIdempotent},
		Timezone: tz, EnabledByDefault: true, MisfirePolicy: misfirePolicy, OverlapPolicy: overlapPolicy,
		DSTSpringPolicy: springPolicy, DSTFallPolicy: fallPolicy, ExecutionOwner: schedule.ExecutionOwnerBackend,
	}, nil
}
