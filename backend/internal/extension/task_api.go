package extension

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type TaskAPI struct {
	runtime *Runtime
}

func NewTaskAPI(runtime *Runtime) *TaskAPI {
	return &TaskAPI{runtime: runtime}
}

func (api *TaskAPI) RegisterRoutes(group *gin.RouterGroup) {
	tasks := group.Group("/tasks")
	tasks.GET("", api.listTasks)
	tasks.POST("", api.enqueueTask)
	tasks.GET("/:taskRunId", api.getTask)
	tasks.POST("/:taskRunId/cancel", api.cancelTask)
	tasks.POST("/:taskRunId/pause", api.pauseTask)
	tasks.POST("/:taskRunId/resume", api.resumeTask)
	tasks.POST("/:taskRunId/retry", api.retryTask)
	tasks.POST("/:taskRunId/recover", api.recoverTask)
	tasks.GET("/:taskRunId/progress", api.getProgress)
	tasks.GET("/:taskRunId/result", api.getResult)
	tasks.GET("/:taskRunId/checkpoint", api.getCheckpoint)

	defs := group.Group("/task-definitions")
	defs.GET("", api.listTaskDefinitions)
	defs.POST("", api.createTaskDefinition)
	defs.GET("/:defId", api.getTaskDefinition)
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
	if limitStr := c.Query("limit"); limitStr != "" {
		if n := parseIntSafe(limitStr); n > 0 {
			filter.Limit = n
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if n := parseIntSafe(offsetStr); n >= 0 {
			filter.Offset = n
		}
	}
	runs, err := svc.ListTaskRuns(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if runs == nil {
		runs = []*task_runtime.TaskRun{}
	}
	c.JSON(http.StatusOK, gin.H{"items": runs, "total": len(runs)})
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
	result, err := svc.Enqueue(c.Request.Context(), req, def)
	if err != nil {
		writeTaskError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
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
	c.JSON(http.StatusOK, run)
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
	c.JSON(http.StatusOK, gin.H{"taskRunId": taskRunID, "status": "cancelling"})
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
	c.JSON(http.StatusOK, gin.H{"taskRunId": taskRunID, "status": "paused"})
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
	c.JSON(http.StatusOK, gin.H{"taskRunId": taskRunID, "status": "running"})
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
	c.JSON(http.StatusCreated, run)
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
	c.JSON(http.StatusOK, run)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if prog == nil {
		c.JSON(http.StatusOK, gin.H{"taskRunId": taskRunID})
		return
	}
	c.JSON(http.StatusOK, prog)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if result == nil {
		c.JSON(http.StatusOK, gin.H{"taskRunId": taskRunID})
		return
	}
	c.JSON(http.StatusOK, result)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cp == nil {
		c.JSON(http.StatusOK, gin.H{"taskRunId": taskRunID})
		return
	}
	c.JSON(http.StatusOK, cp)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	if te, ok := err.(*task_runtime.TaskError); ok {
		c.JSON(task_runtime.HTTPStatusForErrorCode(te.Code), gin.H{
			"error":   string(te.Code),
			"message": te.Message,
		})
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
