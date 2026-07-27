package extension

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/canary"
)

type CanaryAPI struct {
	runtime *Runtime
}

func NewCanaryAPI(runtime *Runtime) *CanaryAPI {
	return &CanaryAPI{runtime: runtime}
}

func (api *CanaryAPI) container(c *gin.Context) *kernel.Container {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return nil
	}
	return api.runtime.Kernel.Container()
}

func (api *CanaryAPI) RegisterRoutes(group *gin.RouterGroup) {
	canaryGroup := group.Group("/canary")
	canaryGroup.GET("/policies", api.listPolicies)
	canaryGroup.GET("/policies/:policyId", api.getPolicy)
	canaryGroup.POST("/policies", api.createPolicy)
	canaryGroup.GET("/states", api.listStates)
	canaryGroup.GET("/states/:canaryId", api.getCanaryState)
	canaryGroup.POST("/states", api.createCanaryState)
	canaryGroup.POST("/states/:canaryId/advance", api.advanceStage)
	canaryGroup.POST("/states/:canaryId/pause", api.pauseCanary)
	canaryGroup.POST("/states/:canaryId/resume", api.resumeCanary)
	canaryGroup.POST("/states/:canaryId/abort", api.abortCanary)
	canaryGroup.POST("/states/:canaryId/commit", api.commitCanary)
	canaryGroup.GET("/metrics", api.listMetrics)
	canaryGroup.POST("/metrics", api.recordMetric)
	canaryGroup.GET("/health/:extensionId", api.getHealth)
	canaryGroup.GET("/routes/:extensionId", api.getRoute)
}

func (api *CanaryAPI) listPolicies(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary repository unavailable"})
		return
	}
	extensionID := c.Query("extensionId")
	if extensionID == "" {
		c.JSON(http.StatusOK, gin.H{"items": []canary.CanaryPolicy{}, "total": 0})
		return
	}
	policy, err := container.CanaryRepository.GetPolicyByExtension(c.Request.Context(), extensionID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"items": []canary.CanaryPolicy{}, "total": 0})
		return
	}
	items := []canary.CanaryPolicy{*policy}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (api *CanaryAPI) getPolicy(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary repository unavailable"})
		return
	}
	policyID := c.Param("policyId")
	policy, err := container.CanaryRepository.GetPolicy(c.Request.Context(), policyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": policy})
}

func (api *CanaryAPI) createPolicy(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary repository unavailable"})
		return
	}
	var policy canary.CanaryPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if policy.PolicyID == "" {
		policy.PolicyID = uuid.New().String()
	}
	if err := container.CanaryRepository.SavePolicy(c.Request.Context(), policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": policy})
}

func (api *CanaryAPI) listStates(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary repository unavailable"})
		return
	}
	extensionID := c.Query("extensionId")
	if extensionID == "" {
		c.JSON(http.StatusOK, gin.H{"items": []canary.CanaryState{}, "total": 0})
		return
	}
	state, err := container.CanaryRepository.GetCanaryStateByExtension(c.Request.Context(), extensionID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"items": []canary.CanaryState{}, "total": 0})
		return
	}
	items := []canary.CanaryState{*state}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (api *CanaryAPI) getCanaryState(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary repository unavailable"})
		return
	}
	canaryID := c.Param("canaryId")
	state, err := container.CanaryRepository.GetCanaryState(c.Request.Context(), canaryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": state})
}

type canaryCreateRequest struct {
	State  canary.CanaryState  `json:"state"`
	Policy canary.CanaryPolicy `json:"policy"`
}

func (api *CanaryAPI) createCanaryState(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil || container.CanaryStageManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary service unavailable"})
		return
	}
	var req canaryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Policy.PolicyID == "" {
		req.Policy.PolicyID = uuid.New().String()
	}
	if req.State.CanaryID == "" {
		req.State.CanaryID = uuid.New().String()
	}
	req.State.PolicyID = req.Policy.PolicyID
	if req.State.ExtensionID == "" {
		req.State.ExtensionID = req.Policy.ExtensionID
	}
	ctx := c.Request.Context()
	if err := container.CanaryRepository.SavePolicy(ctx, req.Policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save policy: " + err.Error()})
		return
	}
	if err := container.CanaryStageManager.StartCanary(ctx, &req.State, &req.Policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "start canary: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": req.State})
}

func (api *CanaryAPI) advanceStage(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil || container.CanaryStageManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary service unavailable"})
		return
	}
	if container.CanaryHealthCollector == nil || container.CanaryHealthEvaluator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary health service unavailable"})
		return
	}
	canaryID := c.Param("canaryId")
	ctx := c.Request.Context()
	state, err := container.CanaryRepository.GetCanaryState(ctx, canaryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	policy, err := container.CanaryRepository.GetPolicy(ctx, state.PolicyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found: " + err.Error()})
		return
	}
	now := time.Now().UTC()
	elapsed := now.Sub(state.StartedAt)
	currentMetrics, err := container.CanaryHealthCollector.GetMetrics(ctx, state.ExtensionID, state.NewGeneration, state.StartedAt, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "collect current metrics: " + err.Error()})
		return
	}
	baseline, err := container.CanaryHealthCollector.CollectBaseline(ctx, state.ExtensionID, state.OldGeneration, elapsed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "collect baseline: " + err.Error()})
		return
	}
	current := container.CanaryHealthCollector.AggregateMetrics(ctx, currentMetrics)
	healthEval := container.CanaryHealthEvaluator.Evaluate(ctx, &policy.HealthPolicy, current, baseline)
	if healthEval.ShouldAbort {
		if err := container.CanaryStageManager.AbortCanary(ctx, state, healthEval.AbortReason); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "abort canary: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"item": state, "aborted": true, "reason": healthEval.AbortReason, "evaluation": healthEval})
		return
	}
	observations := len(currentMetrics)
	result, err := container.CanaryStageManager.AdvanceStage(ctx, state, policy, healthEval, observations, elapsed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": state, "advance": result, "evaluation": healthEval})
}

func (api *CanaryAPI) pauseCanary(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil || container.CanaryStageManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary service unavailable"})
		return
	}
	canaryID := c.Param("canaryId")
	ctx := c.Request.Context()
	state, err := container.CanaryRepository.GetCanaryState(ctx, canaryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := container.CanaryStageManager.PauseCanary(ctx, state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": state})
}

func (api *CanaryAPI) resumeCanary(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil || container.CanaryStageManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary service unavailable"})
		return
	}
	canaryID := c.Param("canaryId")
	ctx := c.Request.Context()
	state, err := container.CanaryRepository.GetCanaryState(ctx, canaryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := container.CanaryStageManager.ResumeCanary(ctx, state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": state})
}

func (api *CanaryAPI) abortCanary(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil || container.CanaryStageManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary service unavailable"})
		return
	}
	canaryID := c.Param("canaryId")
	ctx := c.Request.Context()
	state, err := container.CanaryRepository.GetCanaryState(ctx, canaryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Reason == "" {
		body.Reason = "manual_abort"
	}
	if err := container.CanaryStageManager.AbortCanary(ctx, state, body.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": state})
}

func (api *CanaryAPI) commitCanary(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil || container.CanaryStageManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary service unavailable"})
		return
	}
	canaryID := c.Param("canaryId")
	ctx := c.Request.Context()
	state, err := container.CanaryRepository.GetCanaryState(ctx, canaryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := container.CanaryStageManager.CommitCanary(ctx, state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": state})
}

func (api *CanaryAPI) listMetrics(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary repository unavailable"})
		return
	}
	extensionID := c.Query("extensionId")
	if extensionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId required"})
		return
	}
	generationStr := c.Query("generation")
	var generation int64
	if generationStr != "" {
		g, err := strconv.ParseInt(generationStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid generation"})
			return
		}
		generation = g
	}
	now := time.Now().UTC()
	windowStart := now.Add(-24 * time.Hour)
	windowEnd := now
	if startStr := c.Query("startTime"); startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid startTime"})
			return
		}
		windowStart = t
	}
	if endStr := c.Query("endTime"); endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid endTime"})
			return
		}
		windowEnd = t
	}
	metrics, err := container.CanaryRepository.ListMetrics(c.Request.Context(), extensionID, generation, windowStart, windowEnd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if metrics == nil {
		metrics = []canary.CanaryMetric{}
	}
	c.JSON(http.StatusOK, gin.H{"items": metrics, "total": len(metrics)})
}

func (api *CanaryAPI) recordMetric(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary repository unavailable"})
		return
	}
	var metric canary.CanaryMetric
	if err := c.ShouldBindJSON(&metric); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	ctx := c.Request.Context()
	if container.CanaryHealthCollector != nil {
		if err := container.CanaryHealthCollector.RecordMetric(ctx, metric); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "record metric: " + err.Error()})
			return
		}
	}
	if err := container.CanaryRepository.SaveMetric(ctx, metric); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": metric})
}

func (api *CanaryAPI) getHealth(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryHealthCollector == nil || container.CanaryHealthEvaluator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary health service unavailable"})
		return
	}
	extensionID := c.Param("extensionId")
	generationStr := c.Query("generation")
	if generationStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "generation required"})
		return
	}
	generation, err := strconv.ParseInt(generationStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid generation"})
		return
	}
	baselineWindowStr := c.Query("baselineWindow")
	if baselineWindowStr == "" {
		baselineWindowStr = "1h"
	}
	baselineWindow, err := time.ParseDuration(baselineWindowStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid baselineWindow"})
		return
	}
	ctx := c.Request.Context()
	now := time.Now().UTC()
	windowStart := now.Add(-baselineWindow)
	currentMetrics, err := container.CanaryHealthCollector.GetMetrics(ctx, extensionID, generation, windowStart, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	baseline, err := container.CanaryHealthCollector.CollectBaseline(ctx, extensionID, generation, baselineWindow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	current := container.CanaryHealthCollector.AggregateMetrics(ctx, currentMetrics)
	var healthPolicy *canary.CanaryHealthPolicy
	if container.CanaryRepository != nil {
		if policy, perr := container.CanaryRepository.GetPolicyByExtension(ctx, extensionID); perr == nil && policy != nil {
			healthPolicy = &policy.HealthPolicy
		}
	}
	eval := container.CanaryHealthEvaluator.Evaluate(ctx, healthPolicy, current, baseline)
	c.JSON(http.StatusOK, gin.H{"item": eval, "current": current, "baseline": baseline})
}

func (api *CanaryAPI) getRoute(c *gin.Context) {
	container := api.container(c)
	if container == nil || container.CanaryGenerationRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "canary router unavailable"})
		return
	}
	extensionID := c.Param("extensionId")
	cohortType := c.Query("cohortType")
	cohortID := c.Query("cohortId")
	if cohortType == "" || cohortID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cohortType and cohortId required"})
		return
	}
	route, err := container.CanaryGenerationRouter.GetPersistedRoute(c.Request.Context(), extensionID, cohortType, cohortID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if route == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": route})
}
