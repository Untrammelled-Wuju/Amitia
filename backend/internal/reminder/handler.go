package reminder

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

type Handler struct {
	service       *Service
	localDeviceID string
}

func NewHandler(service *Service, localDeviceID string) *Handler {
	return &Handler{service: service, localDeviceID: localDeviceID}
}

func (h *Handler) caller(c *gin.Context) (context.Context, Caller, error) {
	raw, exists := c.Get("actorContext")
	actor, ok := raw.(*auth.ActorContext)
	if !exists || !ok || actor == nil || actor.SpaceID == "" {
		return c.Request.Context(), Caller{}, ErrForbidden
	}
	device := actor.DeviceID.String()
	if device == "" && actor.IsLocalTrusted && actor.PrincipalType == auth.PrincipalLocalUI {
		device = h.localDeviceID
	}
	if device == "" {
		return c.Request.Context(), Caller{}, ErrForbidden
	}
	return auth.WithActor(c.Request.Context(), actor), Caller{SpaceID: actor.SpaceID.String(), DeviceID: device}, nil
}

func fail(c *gin.Context, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, ErrUnavailable):
		status = http.StatusServiceUnavailable
	case errors.Is(err, schedule.ErrGenerationMismatch), errors.Is(err, schedule.ErrIdempotencyConflict):
		status = http.StatusConflict
	case errors.Is(err, schedule.ErrScheduleNotFound):
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{"code": status, "msg": err.Error(), "data": nil})
}

func success(c *gin.Context, value any) {
	c.JSON(200, gin.H{"code": 200, "msg": "操作成功", "data": value})
}

func (h *Handler) List(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	items, err := h.service.List(ctx, caller, reminderViewKind(c))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, items)
}

func reminderViewKind(c *gin.Context) string {
	kind := c.Query("kind")
	if kind == "" {
		return UserReminder
	}
	return kind
}

func (h *Handler) Find(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	item, err := h.service.Find(ctx, caller, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) Create(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	var request CreateReminderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, errors.New("任务参数无效"))
		return
	}
	item, err := h.service.Create(ctx, caller, request, "manual")
	if err != nil {
		fail(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) Update(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	var payload struct {
		CreateReminderRequest
		Generation int64 `json:"generation"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		fail(c, errors.New("任务参数无效"))
		return
	}
	item, err := h.service.Update(ctx, caller, c.Param("id"), payload.Generation, payload.CreateReminderRequest)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) Change(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	var payload struct {
		Generation int64  `json:"generation"`
		Action     string `json:"action"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		fail(c, errors.New("操作须提供任务版本"))
		return
	}
	if c.Request.Method == "DELETE" {
		payload.Action = "cancel"
	}
	item, err := h.service.Change(ctx, caller, c.Param("id"), payload.Generation, payload.Action)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) Toggle(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	var payload struct {
		Generation int64 `json:"generation"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		fail(c, errors.New("操作须提供任务版本"))
		return
	}
	item, err := h.service.Find(ctx, caller, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	action := "enable"
	if item.Enabled == 1 {
		action = "disable"
	} else if item.Paused {
		action = "resume"
	}
	item, err = h.service.Change(ctx, caller, item.ID, payload.Generation, action)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, item)
}

func (h *Handler) Test(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	result, err := h.service.Preview(ctx, caller, c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, result)
}

func (h *Handler) Trigger(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	var payload struct {
		Generation int64 `json:"generation"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		fail(c, errors.New("操作须提供任务版本"))
		return
	}
	trigger, err := h.service.TriggerNow(ctx, caller, c.Param("id"), payload.Generation)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"id": c.Param("id"), "accepted": true, "triggered": false, "triggerId": trigger.TriggerID, "state": trigger.Status})
}

func (h *Handler) Status(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	status, err := h.service.Status(ctx, caller, reminderViewKind(c))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, status)
}

func (h *Handler) History(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, err := h.service.History(ctx, caller, reminderViewKind(c), c.Query("id"), page, size, c.Query("state"))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"items": items, "total": total})
}

func (h *Handler) Prospective(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	items, err := h.service.List(ctx, caller, reminderViewKind(c))
	if err != nil {
		fail(c, err)
		return
	}
	pending := []Reminder{}
	for _, item := range items {
		if item.Enabled == 1 && item.RemindAt != "" && (c.Query("characterId") == "" || c.Query("characterId") == item.CharacterID) {
			pending = append(pending, item)
		}
	}
	success(c, pending)
}

func (h *Handler) Channels(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	if !h.service.Ready() || h.service.deps.Channels == nil {
		fail(c, ErrUnavailable)
		return
	}
	channels, err := h.service.deps.Channels(ctx, caller)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, channels)
}

func (h *Handler) QueueSummary(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	result, err := h.service.QueueSummary(ctx, caller, reminderViewKind(c))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, result)
}

func (h *Handler) GetCleanupConfig(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	value, err := h.service.GetCleanupDays(ctx, caller)
	if err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"cleanupDays": value})
}

func (h *Handler) SetCleanupConfig(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	var payload struct {
		CleanupDays string `json:"cleanupDays"`
	}
	if json.NewDecoder(c.Request.Body).Decode(&payload) != nil {
		fail(c, errors.New("清理配置参数无效"))
		return
	}
	if err := h.service.SetCleanupDays(ctx, caller, payload.CleanupDays); err != nil {
		fail(c, err)
		return
	}
	success(c, gin.H{"cleanupDays": payload.CleanupDays})
}

func (h *Handler) ClearBackpressure(c *gin.Context) {
	ctx, caller, err := h.caller(c)
	if err != nil {
		fail(c, err)
		return
	}
	result, err := h.service.ClearBackpressure(ctx, caller, reminderViewKind(c))
	if err != nil {
		fail(c, err)
		return
	}
	success(c, result)
}
