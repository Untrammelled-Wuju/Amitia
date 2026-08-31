package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

const (
	userWorkflowSource         = "user"
	userWorkflowSchedulePrefix = "userwf-sched-"
)

type WorkflowAPI struct {
	runtime *Runtime
}

func NewWorkflowAPI(runtime *Runtime) *WorkflowAPI { return &WorkflowAPI{runtime: runtime} }

func (api *WorkflowAPI) RegisterRoutes(group *gin.RouterGroup) {
	g := group.Group("/workflows")
	g.GET("", api.list)
	g.POST("", api.create)
	g.GET("/catalog", api.catalog)
	g.POST("/validate", api.validate)
	g.POST("/ai/generate", api.aiGenerate)
	g.POST("/events/:eventType", api.dispatchEvent)
	g.GET("/:id", api.get)
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
	runs.POST("/:runId/cancel", api.cancelRun)
	runs.POST("/:runId/pause", api.pauseRun)
	runs.POST("/:runId/resume", api.resumeRun)
}

func (api *WorkflowAPI) kernelContainer() (*workflow.WorkflowRegistry, *workflow.WorkflowExecutor, error) {
	if api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil {
		return nil, nil, errors.New("workflow kernel unavailable")
	}
	c := api.runtime.Kernel.Container()
	if c.WorkflowRegistry == nil || c.WorkflowExecutor == nil || c.WorkflowDefRepo == nil || c.WorkflowExecRepo == nil {
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
	return strings.TrimSpace(fmt.Sprint(def.Metadata["ownerUserId"])) == userID
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
		switch trigger.Type {
		case "manual":
		case "event":
			if strings.TrimSpace(trigger.EventType) == "" {
				return fmt.Errorf("trigger %s eventType is required", id)
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

func (api *WorkflowAPI) list(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	userID := workflowUserID(c)
	items := make([]workflow.WorkflowDefinition, 0)
	for _, def := range registry.List(workflow.WorkflowFilter{}) {
		if workflowOwnedBy(def, userID) {
			if def.SchemaVersion != workflow.UserWorkflowSchemaVersion && len(def.Edges) == 0 {
				def.Edges = workflow.DeriveEdges(def.Nodes)
			}
			items = append(items, def)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	total := len(items)
	limit, offset := parsePagination(c)
	if offset >= total {
		items = []workflow.WorkflowDefinition{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		items = items[offset:end]
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (api *WorkflowAPI) catalog(c *gin.Context) {
	if _, _, err := api.kernelContainer(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	kc := api.runtime.Kernel.Container()
	if kc.ToolRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tool registry unavailable"})
		return
	}
	userID := workflowUserID(c)
	items := make([]gin.H, 0)
	for _, def := range kc.ToolRegistry.List(c.Request.Context(), capability.ToolFilter{Enabled: boolPtrWorkflow(true)}) {
		if def.Source == capability.ToolSourceWorkflow && def.Metadata != nil {
			if flag, ok := def.Metadata["userWorkflow"].(bool); ok && flag {
				owner := strings.TrimSpace(fmt.Sprint(def.Metadata["ownerUserId"]))
				if owner == "" || owner != userID {
					continue
				}
			}
		}
		items = append(items, gin.H{
			"id":           def.ID,
			"modelName":    def.ModelName,
			"name":         def.Name,
			"description":  def.Description,
			"source":       def.Source,
			"inputSchema":  json.RawMessage(def.InputSchema),
			"outputSchema": json.RawMessage(def.OutputSchema),
			"runtime":      def.Runtime,
		})
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
	def, err = prepareUserWorkflow(def, workflowUserID(c), "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, exists := registry.Get(def.ID); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "workflow id already exists"})
		return
	}
	if err := registry.Upsert(def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.syncTriggers(c.Request.Context(), workflow.WorkflowDefinition{}, def, workflowUserID(c)); err != nil {
		_ = api.syncTriggers(c.Request.Context(), def, workflow.WorkflowDefinition{}, workflowUserID(c))
		_ = registry.Unregister(def.ID)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, def)
}

func (api *WorkflowAPI) validate(c *gin.Context) {
	var def workflow.WorkflowDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
		return
	}
	prepared, err := prepareUserWorkflow(def, workflowUserID(c), def.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
		return
	}
	compiled, err := workflow.NewCompiler().Compile(prepared, workflow.DefaultCompileOptions())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "topologicalOrder": compiled.TopologicalOrder, "entryNodes": compiled.EntryNodes, "exitNodes": compiled.ExitNodes, "definitionHash": prepared.DefinitionHash})
}

func (api *WorkflowAPI) owned(c *gin.Context) (workflow.WorkflowDefinition, bool) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return workflow.WorkflowDefinition{}, false
	}
	def, ok := registry.Get(c.Param("id"))
	if !ok || !workflowOwnedBy(def, workflowUserID(c)) {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return workflow.WorkflowDefinition{}, false
	}
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
	c.JSON(http.StatusOK, def)
}

func (api *WorkflowAPI) update(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	old, ok := api.owned(c)
	if !ok {
		return
	}
	var def workflow.WorkflowDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow: " + err.Error()})
		return
	}
	def, err = prepareUserWorkflow(def, workflowUserID(c), old.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := registry.Upsert(def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.syncTriggers(c.Request.Context(), old, def, workflowUserID(c)); err != nil {
		_ = api.syncTriggers(c.Request.Context(), def, old, workflowUserID(c))
		_ = registry.Upsert(old)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, def)
}

func (api *WorkflowAPI) patch(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	old, ok := api.owned(c)
	if !ok {
		return
	}
	var patch map[string]json.RawMessage
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow patch: " + err.Error()})
		return
	}
	baseRaw, err := json.Marshal(old)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	immutable := map[string]struct{}{
		"id": {}, "extensionId": {}, "moduleId": {}, "source": {}, "definitionHash": {},
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
	def, err = prepareUserWorkflow(def, workflowUserID(c), old.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := registry.Upsert(def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.syncTriggers(c.Request.Context(), old, def, workflowUserID(c)); err != nil {
		_ = api.syncTriggers(c.Request.Context(), def, old, workflowUserID(c))
		_ = registry.Upsert(old)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, def)
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
	clone, err = prepareUserWorkflow(clone, workflowUserID(c), "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := registry.Upsert(clone); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := api.syncTriggers(c.Request.Context(), workflow.WorkflowDefinition{}, clone, workflowUserID(c)); err != nil {
		_ = api.syncTriggers(c.Request.Context(), clone, workflow.WorkflowDefinition{}, workflowUserID(c))
		_ = registry.Unregister(clone.ID)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, clone)
}

func (api *WorkflowAPI) delete(c *gin.Context) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	old, ok := api.owned(c)
	if !ok {
		return
	}
	if err := api.syncTriggers(c.Request.Context(), old, workflow.WorkflowDefinition{}, workflowUserID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := registry.Unregister(old.ID); err != nil {
		_ = api.syncTriggers(c.Request.Context(), workflow.WorkflowDefinition{}, old, workflowUserID(c))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := api.runtime.Kernel.Container().WorkflowExecRepo.DeleteByWorkflow(c.Request.Context(), old.ID); err != nil {
		_ = registry.Upsert(old)
		_ = api.syncTriggers(c.Request.Context(), workflow.WorkflowDefinition{}, old, workflowUserID(c))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true, "id": old.ID})
}

func (api *WorkflowAPI) setEnabled(c *gin.Context, enabled bool) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	old, ok := api.owned(c)
	if !ok {
		return
	}
	def := old
	def.Enabled = enabled
	if err := registry.Upsert(def); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := api.syncTriggers(c.Request.Context(), old, def, workflowUserID(c)); err != nil {
		_ = api.syncTriggers(c.Request.Context(), def, old, workflowUserID(c))
		_ = registry.Upsert(old)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": def.ID, "enabled": enabled})
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
		Input json.RawMessage `json:"input"`
		Wait  bool            `json:"wait"`
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
	executionID := "wf-run-" + uuid.NewString()
	req := workflow.ExecuteRequest{WorkflowID: def.ID, Input: body.Input, Context: workflow.ExecutionContext{UserID: workflowUserID(c), RootID: executionID, InvocationID: executionID, OperationID: "wf-op-" + uuid.NewString(), TraceID: "trace-" + uuid.NewString()}}
	if body.Wait {
		result, err := executor.Execute(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "executionId": executionID})
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}
	go func() { _, _ = executor.Execute(context.Background(), req) }()
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "executionId": executionID, "workflowId": def.ID, "status": workflow.RunStatusRunning})
}

func parsePagination(c *gin.Context) (int, int) {
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

func (api *WorkflowAPI) listRuns(c *gin.Context) {
	if _, ok := api.owned(c); !ok {
		return
	}
	kc := api.runtime.Kernel.Container()
	limit, offset := parsePagination(c)
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
	kc := api.runtime.Kernel.Container()
	run, err := kc.WorkflowExecRepo.Get(c.Request.Context(), c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}
	def, ok := kc.WorkflowRegistry.Get(run.WorkflowID)
	if !ok || !workflowOwnedBy(def, workflowUserID(c)) {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return
	}
	steps, err := kc.WorkflowExecRepo.ListStepRuns(c.Request.Context(), run.ExecutionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"run": run, "stepRuns": steps, "workflow": def})
}

func (api *WorkflowAPI) runOwned(c *gin.Context) (*workflow.WorkflowRun, *workflow.WorkflowExecutor, bool) {
	_, executor, err := api.kernelContainer()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return nil, nil, false
	}
	kc := api.runtime.Kernel.Container()
	run, err := kc.WorkflowExecRepo.Get(c.Request.Context(), c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return nil, nil, false
	}
	def, ok := kc.WorkflowRegistry.Get(run.WorkflowID)
	if !ok || !workflowOwnedBy(def, workflowUserID(c)) {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow run not found"})
		return nil, nil, false
	}
	return run, executor, true
}

func (api *WorkflowAPI) cancelRun(c *gin.Context) {
	_, executor, ok := api.runOwned(c)
	if !ok {
		return
	}
	cancelled, err := executor.CancelRun(c.Request.Context(), c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cancelled": cancelled})
}
func (api *WorkflowAPI) pauseRun(c *gin.Context) {
	_, executor, ok := api.runOwned(c)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	run, err := executor.Pause(c.Request.Context(), c.Param("runId"), body.Reason)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}
func (api *WorkflowAPI) resumeRun(c *gin.Context) {
	_, executor, ok := api.runOwned(c)
	if !ok {
		return
	}
	run, err := executor.Resume(c.Request.Context(), c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
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
	payload, err := c.GetRawData()
	if err != nil || len(payload) == 0 {
		payload = []byte(`{}`)
	}
	if !json.Valid(payload) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event payload must be valid JSON"})
		return
	}
	if err := kc.WorkflowTriggerManager.HandleEvent(c.Request.Context(), "user:"+workflowUserID(c)+":"+c.Param("eventType"), payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "eventType": c.Param("eventType")})
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
		if err := kc.WorkflowDefRepo.SaveTrigger(ctx, workflow.TriggerBinding{BindingID: "userwf:" + newDef.ID + ":" + trigger.ID, Type: workflow.TriggerTypeEvent, EventType: "user:" + userID + ":" + eventType, WorkflowID: newDef.ID, Input: trigger.Input, Enabled: true}); err != nil {
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
	return &schedule.ScheduleContributionDefinition{ContributionID: scheduleIDFor(def.ID, trigger.ID), ExtensionID: "", ScheduleID: scheduleIDFor(def.ID, trigger.ID), Name: def.Name + " / " + trigger.ID, Description: "Creative Workshop workflow trigger", Trigger: td, Target: schedule.ScheduleTargetDefinition{Type: schedule.TargetTypeWorkflow, TargetID: def.ID, InputTemplate: input, IdempotencyMode: schedule.IdempotencyModeIdempotent}, Timezone: tz, EnabledByDefault: true, ExecutionOwner: schedule.ExecutionOwnerBackend}, nil
}
