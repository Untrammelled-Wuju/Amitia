package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

type TaskAPI struct {
	runtime *Runtime
}

func NewTaskAPI(runtime *Runtime) *TaskAPI {
	return &TaskAPI{runtime: runtime}
}

func (api *TaskAPI) RegisterRoutes(group *gin.RouterGroup) {
	tasks := group.Group("/tasks")
	tasks.Use(api.authorizeTaskRequest())
	tasks.GET("", api.listTasks)
	tasks.POST("", security.SharedCoreAdminOnly(), api.enqueueTask)
	tasks.GET("/:taskRunId", api.getTask)
	tasks.POST("/:taskRunId/cancel", api.cancelTask)
	tasks.POST("/:taskRunId/pause", api.pauseTask)
	tasks.POST("/:taskRunId/resume", api.resumeTask)
	tasks.POST("/:taskRunId/retry", api.retryTask)
	tasks.POST("/:taskRunId/recover", api.recoverTask)
	tasks.GET("/:taskRunId/progress", api.getProgress)
	tasks.GET("/:taskRunId/result", api.getResult)
	tasks.GET("/:taskRunId/result/artifact", api.downloadResultArtifact)
	tasks.GET("/:taskRunId/artifacts/:artifactId", api.downloadArtifact)
	tasks.GET("/:taskRunId/checkpoint", api.getCheckpoint)

	defs := group.Group("/task-definitions")
	defs.GET("", api.listTaskDefinitions)
	defs.POST("", security.SharedCoreAdminOnly(), api.authorizeTaskRequest(), api.createTaskDefinition)
	defs.GET("/:defId", api.getTaskDefinition)
}

func (api *TaskAPI) authorizeTaskRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		service := api.service(c)
		if service == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
			return
		}
		open := service.OpenPublicTaskRequest
		if c.Request.Method == http.MethodGet {
			open = service.OpenPublicTaskRead
		}
		ctx, finish, err := open(c.Request.Context(), c.Param("taskRunId"))
		if err != nil {
			writeTaskError(c, err)
			c.Abort()
			return
		}
		defer finish()
		if c.Request.Method == http.MethodGet && c.Param("taskRunId") != "" && !validateTaskReadIntent(c, ctx) {
			return
		}
		if c.Request.Method != http.MethodGet && c.Param("taskRunId") != "" {
			if !validateTaskControlIntent(c, ctx) {
				return
			}
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func validateTaskReadIntent(c *gin.Context, ctx context.Context) bool {
	encoded := c.Query("expectedExecutionScope")
	if encoded == "" {
		return true
	}
	var expected coordination.ExecutionScope
	if len(encoded) > 16<<10 || json.Unmarshal([]byte(encoded), &expected) != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "任务读取范围参数无效"})
		return false
	}
	actual, owned := coordination.FromContext(ctx)
	if !owned || !taskScopeEqual(expected, actual) {
		writeTaskError(c, coordination.ErrScopeExpired)
		c.Abort()
		return false
	}
	return true
}

func validateTaskControlIntent(c *gin.Context, ctx context.Context) bool {
	actor, _ := auth.FromContext(ctx)
	var expected struct {
		Scope *coordination.ExecutionScope `json:"expectedExecutionScope"`
	}
	payload, readErr := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10))
	c.Request.Body = io.NopCloser(bytes.NewReader(payload))
	if readErr != nil || len(payload) > 0 && json.Unmarshal(payload, &expected) != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "任务控制范围参数无效"})
		return false
	}
	actual, owned := coordination.FromContext(ctx)
	if expected.Scope == nil && actor != nil && actor.PrincipalType == auth.PrincipalTrustedDevice || expected.Scope != nil && (!owned || !taskScopeEqual(*expected.Scope, actual)) {
		writeTaskError(c, coordination.ErrScopeExpired)
		c.Abort()
		return false
	}
	return true
}

func taskScopeEqual(expected, actual coordination.ExecutionScope) bool {
	expected.RequestID, expected.TurnID, expected.ExecutionID = actual.RequestID, actual.TurnID, actual.ExecutionID
	return expected == actual
}

func taskAuthorityPayload(ctx context.Context, payload any, readOnly bool) (map[string]json.RawMessage, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	result := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	if scope, ok := coordination.FromContext(ctx); ok {
		result["executionScope"], _ = json.Marshal(scope)
		result["ownerId"], _ = json.Marshal(scope.ResourceOwnerID)
		result["readOnly"], _ = json.Marshal(readOnly)
		management := scope
		if current, _, historical := coordination.TaskReadAuthority(ctx); historical {
			management = current
		}
		result["managementExecutionScope"], _ = json.Marshal(management)
	}
	return result, nil
}

func (api *TaskAPI) writeScopedTask(c *gin.Context, status int, payload any) {
	readOnly := false
	if c.Request.Method == http.MethodGet && c.Param("taskRunId") != "" {
		_, finish, err := api.service(c).OpenPublicTaskRequest(c.Request.Context(), c.Param("taskRunId"))
		readOnly = err != nil
		if finish != nil {
			finish()
		}
	}
	result, err := taskAuthorityPayload(c.Request.Context(), payload, readOnly)
	if err != nil {
		writeTaskError(c, err)
		return
	}
	c.JSON(status, result)
}

func (api *TaskAPI) service(c *gin.Context) *task_runtime.TaskRuntimeService {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return nil
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		return nil
	}
	return container.TaskRuntimeService
}

func (api *TaskAPI) listTasks(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	filter := task_runtime.ListTasksFilter{
		ExtensionID: c.Query("extensionId"),
		Status:      c.Query("status"),
	}
	actor, authenticated := auth.FromContext(c.Request.Context())
	if !authenticated || actor == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if !actor.HasPermission(auth.PermSystemAdmin) {
		filter.ScopedSpaceID, filter.ScopedDeviceID = actor.SpaceID.String(), actor.DeviceID.String()
	}
	explicitLimit := false
	if limitStr := c.Query("limit"); limitStr != "" {
		if n := parseIntSafe(limitStr); n > 0 {
			filter.Limit = n
			explicitLimit = true
		}
	}
	explicitOffset := false
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if n := parseIntSafe(offsetStr); n >= 0 {
			filter.Offset = n
			explicitOffset = true
		}
	}
	// The extension task list UI historically uses page/pageSize, while the
	// kernel task center uses limit/offset. Accept both contracts at the shared
	// endpoint so pagination cannot silently degrade on one client. Explicit
	// limit/offset parameters take precedence when both forms are supplied.
	if !explicitLimit {
		if pageSize := parseIntSafe(c.Query("pageSize")); pageSize > 0 {
			filter.Limit = pageSize
			if !explicitOffset {
				page := parseIntSafe(c.Query("page"))
				if page < 1 {
					page = 1
				}
				filter.Offset = (page - 1) * pageSize
			}
		}
	}

	total := 0
	if filter.Limit > 0 || filter.Offset > 0 {
		allRuns, countErr := svc.ListTaskRuns(c.Request.Context(), task_runtime.ListTasksFilter{
			ScopedSpaceID:  filter.ScopedSpaceID,
			ScopedDeviceID: filter.ScopedDeviceID,
			ExtensionID:    filter.ExtensionID,
			Status:         filter.Status,
		})
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": countErr.Error()})
			return
		}
		total = len(allRuns)
	}
	runs, err := svc.ListTaskRuns(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if runs == nil {
		runs = []*task_runtime.TaskRun{}
	}
	if filter.Limit == 0 && filter.Offset == 0 {
		total = len(runs)
	}
	items := make([]map[string]json.RawMessage, 0, len(runs))
	for _, run := range runs {
		if run != nil {
			readCtx, finish, accessErr := svc.OpenPublicTaskRead(c.Request.Context(), run.TaskRunID)
			if accessErr == nil {
				_, controlFinish, controlErr := svc.OpenPublicTaskRequest(c.Request.Context(), run.TaskRunID)
				if controlFinish != nil {
					controlFinish()
				}
				item, encodeErr := taskAuthorityPayload(readCtx, run, controlErr != nil)
				if encodeErr != nil {
					finish()
					writeTaskError(c, encodeErr)
					return
				}
				progress, progressErr := svc.GetProgress(readCtx, run.TaskRunID)
				if progressErr == nil {
					item["progress"], _ = json.Marshal(progress)
				}
				items = append(items, item)
				finish()
			}
		}
	}
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		writeTaskError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (api *TaskAPI) enqueueTask(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	var req task_runtime.EnqueueTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.TaskDefinitionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskDefinitionId required"})
		return
	}
	if req.OperationID == "" {
		req.OperationID = "op-" + uuid.NewString()
	}
	def, err := svc.GetTaskDefinition(c.Request.Context(), req.TaskDefinitionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task definition invalid: " + err.Error()})
		return
	}
	if err := api.resolvePublicExecutionTarget(c, &req, def); err != nil {
		writeTaskError(c, err)
		return
	}
	result, err := svc.Enqueue(c.Request.Context(), req, def)
	if err != nil {
		writeTaskError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (api *TaskAPI) resolvePublicExecutionTarget(c *gin.Context, req *task_runtime.EnqueueTaskRequest, def *task_runtime.TaskDefinition) error {
	if api == nil || api.runtime == nil || req == nil || def == nil {
		return task_runtime.NewTaskError(task_runtime.ErrTaskExecutionTargetInvalid, "task runtime target resolver unavailable")
	}

	// A public caller may select a device, but it must not change an explicit
	// placement declared by the task definition. Trusted coordinators (for
	// example workflow execution) use the internal enqueue path and retain the
	// ability to bind a pre-resolved target.
	if req.ExecutionPlacement != "" && def.ExecutionPlacement != "" && req.ExecutionPlacement != def.ExecutionPlacement {
		return task_runtime.NewTaskError(task_runtime.ErrTaskExecutionPlacementInvalid, "public enqueue placement conflicts with task definition")
	}
	placement, err := task_runtime.ResolveRequestedPlacement(req.ExecutionPlacement, def.ExecutionPlacement)
	if err != nil {
		return err
	}

	// A cloud task is local to the Cloud Core that accepted this authenticated
	// request. Do not silently execute a cloud-only definition inside a local
	// full runtime; cloud-to-cloud dispatch is intentionally not exposed by this
	// public API.
	if placement == task_runtime.TaskExecutionPlacementCloud {
		container := api.runtime.Kernel.Container()
		if container == nil || container.RuntimeProfile != runtimeprofile.ProfileCloudCore {
			return task_runtime.NewTaskError(task_runtime.ErrRemoteTaskExecutorUnavailable, "cloud task must be enqueued on Cloud Core")
		}
		req.ExecutionPlacement = task_runtime.TaskExecutionPlacementLocal
		req.TrustedExecutionTarget = nil
		return nil
	}
	if placement != task_runtime.TaskExecutionPlacementDevice {
		return nil
	}

	spaceID := strings.TrimSpace(workflowSpaceID(c))
	if spaceID == "" {
		return task_runtime.NewTaskError(task_runtime.ErrTaskDeviceBindingInvalid, "authenticated user is required for device execution")
	}
	control := api.runtime.WorkflowDeviceControl
	if control == nil {
		return task_runtime.NewTaskError(task_runtime.ErrRemoteTaskExecutorUnavailable, "device control plane unavailable")
	}
	devices, err := control.ListDevices(c.Request.Context(), spaceID)
	if err != nil {
		return task_runtime.NewTaskError(task_runtime.ErrRemoteTaskExecutorUnavailable, "list devices: "+err.Error())
	}

	requestedDeviceID := strings.TrimSpace(req.DeviceID)
	if requestedDeviceID == "" {
		requestedDeviceID = strings.TrimSpace(c.GetHeader("X-Amitia-Target-Device-ID"))
	}
	if requestedDeviceID == "" {
		requestedDeviceID = strings.TrimSpace(c.GetHeader("X-Amitia-Device-ID"))
	}

	online := make([]WorkflowDeviceDescriptor, 0, len(devices))
	for _, item := range devices {
		if item.Online && strings.TrimSpace(item.DeviceID) != "" && strings.TrimSpace(item.RuntimeID) != "" {
			online = append(online, item)
		}
	}
	sort.Slice(online, func(i, j int) bool {
		if online[i].LastSeenAt.Equal(online[j].LastSeenAt) {
			return online[i].DeviceID < online[j].DeviceID
		}
		return online[i].LastSeenAt.After(online[j].LastSeenAt)
	})

	if len(online) == 0 {
		return task_runtime.NewTaskError(task_runtime.ErrRemoteTaskExecutorUnavailable, "no online device is available for device task execution")
	}

	container := api.runtime.Kernel.Container()
	if container == nil || container.CapabilityProviders == nil {
		return task_runtime.NewTaskError(task_runtime.ErrTaskProviderBindingInvalid, "provider registry unavailable")
	}
	instances := container.CapabilityProviders.ListInstancesByPlacement(capability.ProviderPlacementDevice)

	// Resolve only against an online device owned by the authenticated user and
	// a provider instance that belongs to the task definition's extension/module.
	// This prevents an older client (without deviceId) from selecting the most
	// recently seen device when that device cannot actually execute the task.
	type providerBinding struct {
		descriptor *WorkflowDeviceDescriptor
		provider   *capability.CapabilityProviderInstance
	}
	providerByDevice := make(map[string]providerBinding, len(online))
	for _, instance := range instances {
		if instance == nil || !instance.IsExecutable() || string(instance.SpaceID) != spaceID {
			continue
		}
		if strings.TrimSpace(def.ExtensionID) != "" && strings.TrimSpace(instance.ExtensionID) != strings.TrimSpace(def.ExtensionID) {
			continue
		}
		if strings.TrimSpace(def.ModuleID) != "" && strings.TrimSpace(instance.ModuleID) != strings.TrimSpace(def.ModuleID) {
			continue
		}
		deviceID := strings.TrimSpace(string(instance.DeviceID))
		runtimeID := strings.TrimSpace(string(instance.RuntimeID))
		if deviceID == "" || runtimeID == "" {
			continue
		}
		var descriptor *WorkflowDeviceDescriptor
		for i := range online {
			if online[i].DeviceID == deviceID && online[i].RuntimeID == runtimeID {
				descriptor = &online[i]
				break
			}
		}
		if descriptor == nil {
			continue
		}

		// A device can briefly expose more than one runtime generation while a
		// reconnect is converging. Keep the newest online runtime, then use a
		// stable provider-ID tie break. The trusted target below must take its
		// RuntimeID from the same binding as the ProviderInstance; otherwise a
		// device-only map can accidentally create an impossible mixed target.
		current, exists := providerByDevice[deviceID]
		if !exists || descriptor.LastSeenAt.After(current.descriptor.LastSeenAt) ||
			(descriptor.LastSeenAt.Equal(current.descriptor.LastSeenAt) && instance.ID < current.provider.ID) {
			providerByDevice[deviceID] = providerBinding{descriptor: descriptor, provider: instance}
		}
	}

	var selected providerBinding
	if requestedDeviceID != "" {
		deviceOnline := false
		for i := range online {
			if online[i].DeviceID == requestedDeviceID {
				deviceOnline = true
				break
			}
		}
		if !deviceOnline {
			return task_runtime.NewTaskError(task_runtime.ErrTaskDeviceBindingInvalid, "requested device is not online or is not owned by the authenticated user")
		}
		var ok bool
		selected, ok = providerByDevice[requestedDeviceID]
		if !ok {
			return task_runtime.NewTaskError(task_runtime.ErrTaskProviderBindingInvalid, "selected device has no executable provider instance for this task definition")
		}
	} else {
		for i := range online {
			binding, ok := providerByDevice[online[i].DeviceID]
			if ok && binding.descriptor.RuntimeID == online[i].RuntimeID {
				selected = binding
				break
			}
		}
		if selected.provider == nil {
			return task_runtime.NewTaskError(task_runtime.ErrTaskProviderBindingInvalid, "no online device has an executable provider instance for this task definition")
		}
	}
	providerInstance := selected.provider
	descriptor := selected.descriptor

	req.ExecutionPlacement = task_runtime.TaskExecutionPlacementDevice
	req.TrustedExecutionTarget = &task_runtime.TrustedExecutionTargetRequest{
		Placement: task_runtime.TaskExecutionPlacementDevice,
		Target: task_runtime.TaskExecutionTarget{
			ProviderID:         providerInstance.ProviderID,
			ProviderInstanceID: providerInstance.ID,
			SpaceID:            runtimeidentity.SpaceID(spaceID),
			DeviceID:           runtimeidentity.DeviceID(descriptor.DeviceID),
			RuntimeID:          runtimeidentity.RuntimeID(descriptor.RuntimeID),
			RuntimeInstanceID:  providerInstance.RuntimeInstanceID,
		},
		ResolvedBy: "task_api_server",
	}
	return nil
}

func (api *TaskAPI) getTask(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	run, err := svc.GetTaskRun(c.Request.Context(), taskRunID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	api.writeScopedTask(c, http.StatusOK, run)
}

func (api *TaskAPI) cancelTask(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Reason == "" {
		body.Reason = "user_requested"
	}
	if err := svc.Cancel(c.Request.Context(), taskRunID, body.Reason); err != nil {
		writeTaskError(c, err)
		return
	}
	api.writeScopedTask(c, http.StatusOK, gin.H{"taskRunId": taskRunID, "status": "cancelling"})
}

func (api *TaskAPI) pauseTask(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	var body struct {
		Reason     string `json:"reason"`
		Generation int64  `json:"generation"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Reason == "" {
		body.Reason = "user_requested"
	}
	if err := svc.PauseTask(c.Request.Context(), task_runtime.PauseTaskRequest{TaskRunID: taskRunID, Reason: body.Reason, Generation: body.Generation}); err != nil {
		writeTaskError(c, err)
		return
	}
	api.writeScopedTask(c, http.StatusOK, gin.H{"taskRunId": taskRunID, "status": "paused"})
}

func (api *TaskAPI) resumeTask(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	var body struct {
		Generation int64  `json:"generation"`
		ResumeKind string `json:"resumeKind"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := svc.ResumeTask(c.Request.Context(), task_runtime.ResumeTaskRequest{TaskRunID: taskRunID, Generation: body.Generation, ResumeKind: body.ResumeKind}); err != nil {
		writeTaskError(c, err)
		return
	}
	api.writeScopedTask(c, http.StatusOK, gin.H{"taskRunId": taskRunID, "status": "queued"})
}

func (api *TaskAPI) retryTask(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	run, err := svc.Retry(c.Request.Context(), taskRunID)
	if err != nil {
		writeTaskError(c, err)
		return
	}
	api.writeScopedTask(c, http.StatusCreated, run)
}

func (api *TaskAPI) recoverTask(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	run, err := svc.Recover(c.Request.Context(), taskRunID)
	if err != nil {
		writeTaskError(c, err)
		return
	}
	api.writeScopedTask(c, http.StatusOK, run)
}

func (api *TaskAPI) getProgress(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	prog, err := svc.GetProgress(c.Request.Context(), taskRunID)
	if err != nil {
		writeTaskError(c, err)
		return
	}
	if prog == nil {
		api.writeScopedTask(c, http.StatusOK, gin.H{"taskRunId": taskRunID})
		return
	}
	api.writeScopedTask(c, http.StatusOK, prog)
}

func (api *TaskAPI) getResult(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	result, err := svc.GetResult(c.Request.Context(), taskRunID)
	if err != nil {
		writeTaskError(c, err)
		return
	}
	if result == nil {
		api.writeScopedTask(c, http.StatusOK, gin.H{"taskRunId": taskRunID})
		return
	}
	api.writeScopedTask(c, http.StatusOK, result)
}

func (api *TaskAPI) getCheckpoint(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	taskRunID := c.Param("taskRunId")
	cp, err := svc.GetLatestCheckpoint(c.Request.Context(), taskRunID)
	if err != nil {
		writeTaskError(c, err)
		return
	}
	if cp == nil {
		api.writeScopedTask(c, http.StatusOK, gin.H{"taskRunId": taskRunID})
		return
	}
	api.writeScopedTask(c, http.StatusOK, cp)
}

func (api *TaskAPI) downloadResultArtifact(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	download, err := svc.DownloadTaskResultArtifact(c.Request.Context(), c.Param("taskRunId"), c.Query("artifactId"))
	if err != nil {
		writeTaskError(c, err)
		return
	}
	writeTaskArtifact(c, download)
}

func (api *TaskAPI) downloadArtifact(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	download, err := svc.DownloadTaskArtifact(c.Request.Context(), c.Param("taskRunId"), c.Param("artifactId"))
	if err != nil {
		writeTaskError(c, err)
		return
	}
	writeTaskArtifact(c, download)
}

func writeTaskArtifact(c *gin.Context, download *task_runtime.TaskArtifactDownload) {
	if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
		writeTaskError(c, err)
		return
	}
	name := download.Name
	if name == "" || strings.ContainsAny(name, "/\\") || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		name = "task-artifact"
	}
	contentType := download.MimeType
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'")
	c.Header("ETag", "\""+download.Hash+"\"")
	c.Data(http.StatusOK, contentType, download.Content)
}

func (api *TaskAPI) listTaskDefinitions(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	extensionID := c.Query("extensionId")
	defs, err := svc.ListTaskDefinitions(c.Request.Context(), extensionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if defs == nil {
		defs = []*task_runtime.TaskDefinition{}
	}
	c.JSON(http.StatusOK, gin.H{"items": defs, "total": len(defs)})
}

func (api *TaskAPI) createTaskDefinition(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	var def task_runtime.TaskDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid definition: " + err.Error()})
		return
	}
	if def.TaskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskId required"})
		return
	}
	if def.ExtensionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId required"})
		return
	}
	if def.Entry == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "entry required"})
		return
	}
	if def.RuntimeType == "" {
		def.RuntimeType = "task_javascript"
	}
	if err := svc.PutTaskDefinition(c.Request.Context(), &def); err != nil {
		writeTaskError(c, err)
		return
	}
	c.JSON(http.StatusCreated, def)
}

func (api *TaskAPI) getTaskDefinition(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runtime unavailable"})
		return
	}
	defID := c.Param("defId")
	def, err := svc.GetTaskDefinition(c.Request.Context(), defID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, def)
}

func writeTaskError(c *gin.Context, err error) {
	var te *task_runtime.TaskError
	if errors.As(err, &te) {
		c.JSON(task_runtime.HTTPStatusForErrorCode(te.Code), gin.H{
			"error":   string(te.Code),
			"message": te.Message,
		})
		return
	}
	status := http.StatusInternalServerError
	code := "task_runtime_error"
	switch {
	case errors.Is(err, coordination.ErrScopeExpired), errors.Is(err, coordination.ErrRevision), errors.Is(err, coordination.ErrResourceVersion), errors.Is(err, coordination.ErrRequestConflict), errors.Is(err, coordination.ErrAuthorityUnconfirmed):
		status, code = http.StatusConflict, "task_state_changed"
	case errors.Is(err, coordination.ErrWrongOwner), errors.Is(err, coordination.ErrCapabilityGrant):
		status, code = http.StatusForbidden, "task_scope_denied"
	case errors.Is(err, coordination.ErrRoleRequired), errors.Is(err, coordination.ErrRoleSelection):
		status, code = http.StatusConflict, "task_role_required"
	case errors.Is(err, coordination.ErrPendingLimit):
		status, code = http.StatusRequestEntityTooLarge, "task_resource_limit"
	}
	if status != http.StatusInternalServerError {
		c.JSON(status, gin.H{"error": code, "message": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func parseIntSafe(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return -1
		}
		n = n*10 + int(ch-'0')
	}
	return n
}
