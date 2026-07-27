package extension

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type TrustedServiceAPI struct {
	runtime *Runtime
}

func NewTrustedServiceAPI(runtime *Runtime) *TrustedServiceAPI {
	return &TrustedServiceAPI{runtime: runtime}
}

func (api *TrustedServiceAPI) RegisterRoutes(group *gin.RouterGroup) {
	svc := group.Group("/services")
	svc.GET("", api.listServices)
	svc.POST("", api.registerService)
	svc.GET("/:serviceId", api.getService)
	svc.DELETE("/:serviceId", api.unregisterService)
	svc.POST("/:serviceId/start", api.startService)
	svc.POST("/:serviceId/stop", api.stopService)
	svc.GET("/:serviceId/status", api.getServiceStatus)
	svc.GET("/:serviceId/health", api.healthCheck)
	svc.POST("/:serviceId/invoke", api.invokeService)
	svc.GET("/quarantine/list", api.listQuarantined)
	svc.POST("/quarantine/:serviceId/release", api.releaseQuarantine)
}

func (api *TrustedServiceAPI) getSupervisor() (*trusted_service.ProcessSupervisor, bool) {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return nil, false
	}
	container := api.runtime.Kernel.Container()
	if container == nil || container.TrustedServiceSupervisor == nil {
		return nil, false
	}
	return container.TrustedServiceSupervisor, true
}

func (api *TrustedServiceAPI) listServices(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	instances := supervisor.List()
	results := make([]gin.H, 0, len(instances))
	for _, inst := range instances {
		results = append(results, api.instanceToJSON(inst))
	}
	c.JSON(http.StatusOK, gin.H{"services": results, "total": len(results)})
}

func (api *TrustedServiceAPI) registerService(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	var def trusted_service.ServiceRuntimeDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid definition: " + err.Error()})
		return
	}
	if def.ServiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service_id required"})
		return
	}
	if def.TrustLevel == "" {
		def.TrustLevel = string(trusted_service.TrustLevelTrusted)
	}
	if def.Network.LoopbackOnly == false && !def.Network.AllowInbound {
		def.Network.LoopbackOnly = true
	}
	if def.HealthCheck.Type == "" {
		def.HealthCheck.Type = "process"
	}
	if def.HealthCheck.Interval <= 0 {
		def.HealthCheck.Interval = 30 * time.Second
	}
	if def.HealthCheck.Timeout <= 0 {
		def.HealthCheck.Timeout = 5 * time.Second
	}
	if def.HealthCheck.GracePeriod <= 0 {
		def.HealthCheck.GracePeriod = 10 * time.Second
	}
	if def.HealthCheck.MaxConsecutiveFails <= 0 {
		def.HealthCheck.MaxConsecutiveFails = 3
	}
	if def.Recovery.MaxRestarts <= 0 {
		def.Recovery.MaxRestarts = 3
	}
	if def.Recovery.RestartDelay <= 0 {
		def.Recovery.RestartDelay = 2 * time.Second
	}
	if def.Recovery.BackoffMultiplier <= 0 {
		def.Recovery.BackoffMultiplier = 2
	}
	if def.Recovery.MaxRestartDelay <= 0 {
		def.Recovery.MaxRestartDelay = 60 * time.Second
	}
	if def.Shutdown.GracePeriod <= 0 {
		def.Shutdown.GracePeriod = 5 * time.Second
	}
	if def.Shutdown.KillTimeout <= 0 {
		def.Shutdown.KillTimeout = 10 * time.Second
	}
	def.Shutdown.CleanupChildren = true
	def.Shutdown.RemoveTempDir = true

	if err := supervisor.Register(&def); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, def)
}

func (api *TrustedServiceAPI) getService(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	serviceID := c.Param("serviceId")
	inst, err := supervisor.Get(serviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, api.instanceToJSON(inst))
}

func (api *TrustedServiceAPI) unregisterService(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	serviceID := c.Param("serviceId")
	if err := supervisor.Unregister(serviceID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"service_id": serviceID, "status": "unregistered"})
}

func (api *TrustedServiceAPI) startService(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	serviceID := c.Param("serviceId")
	var body struct {
		InstanceID   string            `json:"instance_id"`
		Generation   int64             `json:"generation"`
		BasePath     string            `json:"base_path"`
		WorkingDir   string            `json:"working_dir"`
		SessionToken string            `json:"session_token"`
		SecretLease  string            `json:"secret_lease"`
		LogLevel     string            `json:"log_level"`
		Args         map[string]string `json:"args"`
		TrustLevel   string            `json:"trust_level"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Generation <= 0 {
		body.Generation = 1
	}
	publisherTrust := trusted_service.TrustLevelOfficial
	if body.TrustLevel != "" {
		publisherTrust = trusted_service.TrustLevel(body.TrustLevel)
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	result, err := supervisor.Start(ctx, trusted_service.StartRequest{
		ServiceID:      serviceID,
		InstanceID:     body.InstanceID,
		Generation:     body.Generation,
		PublisherTrust: publisherTrust,
		BasePath:       body.BasePath,
		WorkingDir:     body.WorkingDir,
		SessionToken:   body.SessionToken,
		SecretLease:    body.SecretLease,
		LogLevel:       body.LogLevel,
		Args:           body.Args,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "service_id": serviceID})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"instance_id": result.InstanceID,
		"pid":         result.PID,
		"state":       string(result.State),
		"started_at":  result.StartedAt,
		"generation":  result.Generation,
	})
}

func (api *TrustedServiceAPI) stopService(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	serviceID := c.Param("serviceId")
	var body struct {
		Reason string `json:"reason"`
		Force  bool   `json:"force"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Reason == "" {
		body.Reason = "manual"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result, err := supervisor.Stop(ctx, trusted_service.StopRequest{
		ServiceID: serviceID,
		Reason:    body.Reason,
		Force:     body.Force,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"service_id":  result.ServiceID,
		"state":       string(result.State),
		"stopped_at":  result.StoppedAt,
	})
}

func (api *TrustedServiceAPI) getServiceStatus(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	serviceID := c.Param("serviceId")
	inst, err := supervisor.Get(serviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, api.instanceToJSON(inst))
}

func (api *TrustedServiceAPI) healthCheck(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	serviceID := c.Param("serviceId")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := supervisor.HealthCheck(ctx, serviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"service_id": serviceID,
		"status":     result.Status,
		"details":    result.Details,
	})
}

func (api *TrustedServiceAPI) invokeService(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	serviceID := c.Param("serviceId")
	var body struct {
		Operation string          `json:"operation"`
		Input     json.RawMessage `json:"input"`
		Timeout   string          `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if body.Operation == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "operation required"})
		return
	}
	timeout := 30 * time.Second
	if body.Timeout != "" {
		if d, err := time.ParseDuration(body.Timeout); err == nil {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout+5*time.Second)
	defer cancel()
	result, err := supervisor.Invoke(ctx, serviceID, body.Operation, body.Input, timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"service_id": serviceID,
		"operation":  body.Operation,
		"output":     json.RawMessage(result.Output),
	})
}

func (api *TrustedServiceAPI) listQuarantined(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	qm := supervisor.QuarantineManager()
	records := qm.ListActive()
	results := make([]gin.H, 0, len(records))
	for _, r := range records {
		results = append(results, gin.H{
			"service_id":     r.ServiceID,
			"instance_id":    r.InstanceID,
			"reason":         string(r.Reason),
			"detail":         r.Detail,
			"evidence":       r.Evidence,
			"quarantined_at": r.QuarantinedAt,
		})
	}
	history := qm.History()
	historyResults := make([]gin.H, 0, len(history))
	for _, r := range history {
		entry := gin.H{
			"service_id":     r.ServiceID,
			"reason":         string(r.Reason),
			"quarantined_at": r.QuarantinedAt,
		}
		if r.ReleasedAt != nil {
			entry["released_at"] = r.ReleasedAt
			entry["release_reason"] = r.ReleaseReason
		}
		historyResults = append(historyResults, entry)
	}
	c.JSON(http.StatusOK, gin.H{
		"active":  results,
		"history": historyResults,
	})
}

func (api *TrustedServiceAPI) releaseQuarantine(c *gin.Context) {
	supervisor, ok := api.getSupervisor()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trusted service runtime unavailable"})
		return
	}
	serviceID := c.Param("serviceId")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Reason == "" {
		body.Reason = "manual_release"
	}
	qm := supervisor.QuarantineManager()
	if err := qm.Release(serviceID, "api", body.Reason); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"service_id": serviceID,
		"status":     "released",
	})
}

func (api *TrustedServiceAPI) instanceToJSON(inst *trusted_service.ServiceInstance) gin.H {
	h := gin.H{
		"instance_id":   inst.InstanceID,
		"service_id":    inst.ServiceID,
		"state":         string(inst.State_()),
		"pid":           inst.PID,
		"generation":    inst.Generation,
		"restart_count": inst.RestartCount,
		"health_fails":  inst.HealthFails,
		"working_dir":   inst.WorkingDir,
		"stdio_conn":    inst.StdioConn,
	}
	if inst.StartedAt != nil {
		h["started_at"] = inst.StartedAt
	}
	if inst.StoppedAt != nil {
		h["stopped_at"] = inst.StoppedAt
	}
	if inst.LastHealthAt != nil {
		h["last_health_at"] = inst.LastHealthAt
	}
	if inst.Definition != nil {
		h["name"] = inst.Definition.Name
		h["publisher"] = inst.Definition.Publisher
		h["trust_level"] = inst.Definition.TrustLevel
		h["protocol"] = inst.Definition.Protocol
		h["health_check_type"] = inst.Definition.HealthCheck.Type
	}
	if inst.Executable != nil {
		h["platform"] = string(inst.Executable.Platform)
		h["executable_path"] = inst.Executable.Path
	}
	return h
}
