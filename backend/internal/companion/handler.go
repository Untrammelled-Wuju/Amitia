package companion

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

func NewHandler(srv Service) *Handler { return &Handler{service: srv} }

func (h *Handler) GetSleepSetting(c *gin.Context) { util.SuccessResponse(c, h.service.GetSleepSetting()) }
func (h *Handler) UpdateSleepSetting(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body); util.SuccessResponse(c, h.service.UpdateSleepSetting(body))
}
func (h *Handler) GetSchedule(c *gin.Context) { util.SuccessResponse(c, h.service.GetSchedule(c.Query("date"))) }
func (h *Handler) GetScheduleConflicts(c *gin.Context) { util.SuccessResponse(c, h.service.GetScheduleConflicts(c.Query("date"))) }
func (h *Handler) GetScheduleToday(c *gin.Context) { util.SuccessResponse(c, h.service.GetScheduleToday()) }
func (h *Handler) GetStateLife(c *gin.Context) { util.SuccessResponse(c, h.service.GetStateLife()) }
func (h *Handler) GetState(c *gin.Context) { util.SuccessResponse(c, h.service.GetState()) }
func (h *Handler) GetTimelineToday(c *gin.Context) { util.SuccessResponse(c, h.service.GetTimelineToday()) }

func (h *Handler) ListFixedEvents(c *gin.Context) { util.SuccessResponse(c, h.service.ListFixedEvents(c.Query("date"))) }
func (h *Handler) CreateFixedEvent(c *gin.Context) { var body map[string]interface{}; c.ShouldBindJSON(&body); util.SuccessMsgResponse(c, "事件已创建", h.service.CreateFixedEvent(body)) }
func (h *Handler) UpdateFixedEvent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id")); var body map[string]interface{}; c.ShouldBindJSON(&body)
	util.SuccessMsgResponse(c, "事件已更新", h.service.UpdateFixedEvent(id, body))
}
func (h *Handler) DeleteFixedEvent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !h.service.DeleteFixedEvent(id) { util.ErrorResponse(c, response.NotFound, "事件不存在", nil); return }
	util.SuccessMsgResponse(c, "事件已删除", nil)
}
func (h *Handler) ToggleFixedEventEnabled(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id")); util.SuccessResponse(c, h.service.ToggleFixedEventEnabled(id))
}

func (h *Handler) ListSpecialEvents(c *gin.Context) { util.SuccessResponse(c, h.service.ListSpecialEvents()) }
func (h *Handler) CreateSpecialEvent(c *gin.Context) { var body map[string]interface{}; c.ShouldBindJSON(&body); util.SuccessMsgResponse(c, "事件已创建", h.service.CreateSpecialEvent(body)) }
func (h *Handler) UpdateSpecialEvent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id")); var body map[string]interface{}; c.ShouldBindJSON(&body)
	util.SuccessMsgResponse(c, "事件已更新", h.service.UpdateSpecialEvent(id, body))
}
func (h *Handler) DeleteSpecialEvent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !h.service.DeleteSpecialEvent(id) { util.ErrorResponse(c, response.NotFound, "事件不存在", nil); return }
	util.SuccessMsgResponse(c, "事件已删除", nil)
}
func (h *Handler) ToggleSpecialEventEnabled(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id")); util.SuccessResponse(c, h.service.ToggleSpecialEventEnabled(id))
}

func (h *Handler) ListClassAdjustments(c *gin.Context) { util.SuccessResponse(c, h.service.ListClassAdjustments()) }
func (h *Handler) CreateClassAdjustment(c *gin.Context) { var body map[string]interface{}; c.ShouldBindJSON(&body); util.SuccessMsgResponse(c, "调课已创建", h.service.CreateClassAdjustment(body)) }
func (h *Handler) UpdateClassAdjustment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id")); var body map[string]interface{}; c.ShouldBindJSON(&body)
	util.SuccessMsgResponse(c, "调课已更新", h.service.UpdateClassAdjustment(id, body))
}
func (h *Handler) DeleteClassAdjustment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if !h.service.DeleteClassAdjustment(id) { util.ErrorResponse(c, response.NotFound, "调课不存在", nil); return }
	util.SuccessMsgResponse(c, "调课已删除", nil)
}
func (h *Handler) GetEffectiveClasses(c *gin.Context) { util.SuccessResponse(c, h.service.GetEffectiveClasses(c.Query("date"))) }

func (h *Handler) GetLifestyleTendency(c *gin.Context) { util.SuccessResponse(c, h.service.GetLifestyleTendency()) }
func (h *Handler) UpdateLifestyleTendency(c *gin.Context) { var body map[string]interface{}; c.ShouldBindJSON(&body); util.SuccessResponse(c, h.service.UpdateLifestyleTendency(body)) }
func (h *Handler) ResetLifestyleTendency(c *gin.Context) { util.SuccessResponse(c, h.service.ResetLifestyleTendency()) }

func (h *Handler) GetWorkProfile(c *gin.Context) { util.SuccessResponse(c, h.service.GetWorkProfile()) }
func (h *Handler) UpdateWorkProfile(c *gin.Context) { var body map[string]interface{}; c.ShouldBindJSON(&body); util.SuccessResponse(c, h.service.UpdateWorkProfile(body)) }

func (h *Handler) GetActiveMessageSetting(c *gin.Context) { util.SuccessResponse(c, h.service.GetActiveMessageSetting()) }
func (h *Handler) UpdateActiveMessageSetting(c *gin.Context) { var body map[string]interface{}; c.ShouldBindJSON(&body); util.SuccessResponse(c, h.service.UpdateActiveMessageSetting(body)) }
func (h *Handler) GetActiveMessageTasksToday(c *gin.Context) { util.SuccessResponse(c, h.service.GetActiveMessageTasksToday()) }
func (h *Handler) RegenerateActiveMessageTasks(c *gin.Context) { util.SuccessResponse(c, h.service.RegenerateActiveMessageTasks()) }
func (h *Handler) RunActiveMessageTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id")); util.SuccessResponse(c, h.service.RunActiveMessageTask(id))
}
func (h *Handler) CancelActiveMessageTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id")); util.SuccessResponse(c, h.service.CancelActiveMessageTask(id))
}

func (h *Handler) ListDelayedReplies(c *gin.Context) { util.SuccessResponse(c, h.service.ListDelayedReplies()) }
func (h *Handler) CancelDelayedReply(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id")); util.SuccessResponse(c, h.service.CancelDelayedReply(id))
}
func (h *Handler) ProcessDelayedReplies(c *gin.Context) { util.SuccessResponse(c, h.service.ProcessDelayedReplies()) }

func (h *Handler) GetDebugOverview(c *gin.Context) { util.SuccessResponse(c, h.service.GetDebugOverview()) }
func (h *Handler) RegenerateAllDebug(c *gin.Context) { util.SuccessResponse(c, h.service.RegenerateAllDebug()) }
func (h *Handler) ProcessActiveMessagesDebug(c *gin.Context) { util.SuccessResponse(c, h.service.ProcessActiveMessagesDebug()) }
func (h *Handler) ProcessDelayedRepliesDebug(c *gin.Context) { util.SuccessResponse(c, h.service.ProcessDelayedRepliesDebug()) }

func (h *Handler) GetRuleLogs(c *gin.Context) { util.SuccessResponse(c, h.service.GetRuleLogs()) }
func (h *Handler) RegenerateSchedule(c *gin.Context) { util.SuccessResponse(c, h.service.RegenerateSchedule()) }
func (h *Handler) RegenerateTimeline(c *gin.Context) { util.SuccessResponse(c, h.service.RegenerateTimeline()) }
