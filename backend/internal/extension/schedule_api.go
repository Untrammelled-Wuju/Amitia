package extension

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

type ScheduleAPI struct {
	runtime *Runtime
}

func NewScheduleAPI(runtime *Runtime) *ScheduleAPI {
	return &ScheduleAPI{runtime: runtime}
}

func (api *ScheduleAPI) service(c *gin.Context) *schedule.ScheduleService {
	if api.runtime == nil || api.runtime.Kernel == nil {
		return nil
	}
	container := api.runtime.Kernel.Container()
	if container == nil {
		return nil
	}
	return container.ScheduleService
}

func parseExpectedGeneration(c *gin.Context, svc *schedule.ScheduleService, scheduleID string) (int64, error) {
	if raw := c.Query("expectedGeneration"); raw != "" {
		var gen int64
		if _, err := fmt.Sscanf(raw, "%d", &gen); err != nil {
			return 0, fmt.Errorf("invalid expectedGeneration: %w", err)
		}
		return gen, nil
	}
	if svc == nil || scheduleID == "" {
		return 0, nil
	}
	state, err := svc.GetScheduleState(c.Request.Context(), scheduleID)
	if err != nil || state == nil {
		return 0, schedule.ErrScheduleNotFound
	}
	return state.Generation, nil
}

func (api *ScheduleAPI) RegisterRoutes(group *gin.RouterGroup) {
	schedules := group.Group("/schedules")
	schedules.GET("", api.listSchedules)
	schedules.POST("", api.installSchedule)
	schedules.GET("/quarantines", api.listQuarantines)
	schedules.GET("/:scheduleId", api.getSchedule)
	schedules.PUT("/:scheduleId", api.updateSchedule)
	schedules.DELETE("/:scheduleId", api.uninstallSchedule)
	schedules.POST("/:scheduleId/enable", api.enableSchedule)
	schedules.POST("/:scheduleId/disable", api.disableSchedule)
	schedules.POST("/:scheduleId/pause", api.pauseSchedule)
	schedules.POST("/:scheduleId/resume", api.resumeSchedule)
	schedules.POST("/:scheduleId/run-now", api.runNow)
	schedules.POST("/:scheduleId/skip-next", api.skipNext)
	schedules.POST("/:scheduleId/recalculate", api.recalculate)
	schedules.GET("/:scheduleId/triggers", api.listTriggers)
	schedules.GET("/:scheduleId/runs", api.listRuns)
	schedules.GET("/:scheduleId/misfires", api.listMisfires)
	schedules.GET("/:scheduleId/circuit", api.getCircuit)
	schedules.POST("/:scheduleId/circuit/reset", api.resetCircuit)
}

func (api *ScheduleAPI) listSchedules(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	extensionID := c.Query("extensionId")
	var (
		defs []*schedule.ScheduleContributionDefinition
		err  error
	)
	if extensionID != "" {
		defs, err = svc.ListSchedules(c.Request.Context(), extensionID)
	} else {
		defs, err = svc.ListAllSchedules(c.Request.Context())
	}
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	type scheduleListItem struct {
		Definition *schedule.ScheduleContributionDefinition `json:"definition"`
		State      *schedule.ScheduleState                  `json:"state"`
	}
	items := make([]scheduleListItem, 0, len(defs))
	for _, def := range defs {
		state, stateErr := svc.GetScheduleState(c.Request.Context(), def.ScheduleID)
		if stateErr != nil || state == nil {
			state = &schedule.ScheduleState{
				ScheduleID: def.ScheduleID,
				Status:     schedule.DefinitionStatusCreated,
			}
		}
		items = append(items, scheduleListItem{Definition: def, State: state})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (api *ScheduleAPI) getSchedule(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	def, state, err := svc.GetSchedule(c.Request.Context(), scheduleID)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"definition": def, "state": state})
}

func (api *ScheduleAPI) installSchedule(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	var def schedule.ScheduleContributionDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if def.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	if def.ExtensionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "extensionId required"})
		return
	}
	if err := svc.InstallDefinition(c.Request.Context(), &def); err != nil {
		writeScheduleError(c, err)
		return
	}
	state, _ := svc.GetScheduleState(c.Request.Context(), def.ScheduleID)
	c.JSON(http.StatusCreated, gin.H{"definition": def, "state": state})
}

func (api *ScheduleAPI) updateSchedule(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	var def schedule.ScheduleContributionDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	def.ScheduleID = scheduleID
	expectedGen, err := parseExpectedGeneration(c, svc, scheduleID)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if err := svc.Update(c.Request.Context(), scheduleID, expectedGen, &def); err != nil {
		writeScheduleError(c, err)
		return
	}
	state, _ := svc.GetScheduleState(c.Request.Context(), def.ScheduleID)
	c.JSON(http.StatusOK, gin.H{"definition": def, "state": state})
}

func (api *ScheduleAPI) uninstallSchedule(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	if err := svc.Uninstall(c.Request.Context(), scheduleID); err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduleId": scheduleID, "status": "uninstalled"})
}

func (api *ScheduleAPI) enableSchedule(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	expectedGen, err := parseExpectedGeneration(c, svc, scheduleID)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if err := svc.Enable(c.Request.Context(), scheduleID, expectedGen); err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduleId": scheduleID, "status": "enabled"})
}

func (api *ScheduleAPI) disableSchedule(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	expectedGen, err := parseExpectedGeneration(c, svc, scheduleID)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if err := svc.Disable(c.Request.Context(), scheduleID, expectedGen); err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduleId": scheduleID, "status": "disabled"})
}

func (api *ScheduleAPI) pauseSchedule(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	expectedGen, err := parseExpectedGeneration(c, svc, scheduleID)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if err := svc.Pause(c.Request.Context(), scheduleID, expectedGen); err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduleId": scheduleID, "status": "paused"})
}

func (api *ScheduleAPI) resumeSchedule(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	expectedGen, err := parseExpectedGeneration(c, svc, scheduleID)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if err := svc.Resume(c.Request.Context(), scheduleID, expectedGen); err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduleId": scheduleID, "status": "resumed"})
}

func (api *ScheduleAPI) runNow(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	trigger, err := svc.RunNow(c.Request.Context(), scheduleID)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, trigger)
}

func (api *ScheduleAPI) skipNext(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	if err := svc.SkipNext(c.Request.Context(), scheduleID); err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduleId": scheduleID, "status": "skipped"})
}

func (api *ScheduleAPI) recalculate(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	if err := svc.Recalculate(c.Request.Context(), scheduleID); err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduleId": scheduleID, "status": "recalculated"})
}

func (api *ScheduleAPI) listTriggers(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	limit := parseScheduleLimit(c)
	items, err := svc.GetTriggers(c.Request.Context(), scheduleID, limit)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if items == nil {
		items = []*schedule.ScheduleTriggerRecord{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (api *ScheduleAPI) listRuns(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	limit := parseScheduleLimit(c)
	items, err := svc.GetRuns(c.Request.Context(), scheduleID, limit)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if items == nil {
		items = []*schedule.ScheduleRunRecord{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (api *ScheduleAPI) listMisfires(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	limit := parseScheduleLimit(c)
	items, err := svc.GetMisfires(c.Request.Context(), scheduleID, limit)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if items == nil {
		items = []*schedule.ScheduleMisfireRecord{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (api *ScheduleAPI) getCircuit(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	record, err := svc.GetCircuit(c.Request.Context(), scheduleID)
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if record == nil {
		c.JSON(http.StatusOK, gin.H{
			"scheduleId":       scheduleID,
			"state":            schedule.CircuitStateClosed,
			"consecutiveFails": 0,
			"totalFails":       0,
			"totalSuccess":     0,
		})
		return
	}
	c.JSON(http.StatusOK, record)
}

func (api *ScheduleAPI) resetCircuit(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	scheduleID := c.Param("scheduleId")
	if err := svc.ResetCircuit(c.Request.Context(), scheduleID); err != nil {
		writeScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"scheduleId": scheduleID, "status": "circuit_reset"})
}

func (api *ScheduleAPI) listQuarantines(c *gin.Context) {
	svc := api.service(c)
	if svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "schedule service unavailable"})
		return
	}
	items, err := svc.GetQuarantines(c.Request.Context())
	if err != nil {
		writeScheduleError(c, err)
		return
	}
	if items == nil {
		items = []*schedule.ScheduleQuarantineRecord{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func parseScheduleLimit(c *gin.Context) int {
	if l := c.Query("limit"); l != "" {
		if n := parseIntSafe(l); n > 0 {
			return n
		}
	}
	return 50
}

func writeScheduleError(c *gin.Context, err error) {
	if se, ok := err.(*schedule.ScheduleError); ok {
		c.JSON(scheduleHTTPStatusForCode(se.Code), gin.H{
			"error":   se.Code,
			"message": se.Message,
		})
		return
	}
	switch {
	case errors.Is(err, schedule.ErrScheduleNotFound), errors.Is(err, schedule.ErrTriggerNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	case errors.Is(err, schedule.ErrInvalidStateTransition):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case errors.Is(err, schedule.ErrCircuitOpen):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	case errors.Is(err, schedule.ErrPermissionDenied), errors.Is(err, schedule.ErrScopeDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	case errors.Is(err, schedule.ErrScheduleQuarantined):
		c.JSON(http.StatusLocked, gin.H{"error": err.Error()})
		return
	case errors.Is(err, schedule.ErrLeaseAcquisitionFailed), errors.Is(err, schedule.ErrIdempotencyConflict), errors.Is(err, schedule.ErrOverlapForbidden):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case errors.Is(err, schedule.ErrScheduleNotEnabled), errors.Is(err, schedule.ErrSchedulePaused), errors.Is(err, schedule.ErrScheduleExpired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func scheduleHTTPStatusForCode(code string) int {
	switch code {
	case schedule.ErrCodeInvalidStateTransition:
		return http.StatusConflict
	case schedule.ErrCodeCircuitOpen:
		return http.StatusServiceUnavailable
	case schedule.ErrCodePermissionDenied, schedule.ErrCodeScopeDenied:
		return http.StatusForbidden
	case schedule.ErrCodeQuarantined:
		return http.StatusLocked
	case schedule.ErrCodeLeaseFailed, schedule.ErrCodeIdempotencyConflict, schedule.ErrCodeOverlapForbidden:
		return http.StatusConflict
	case schedule.ErrCodeTargetNotFound:
		return http.StatusNotFound
	default:
		return http.StatusBadRequest
	}
}
