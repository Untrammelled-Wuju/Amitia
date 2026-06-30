// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
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

func (h *Handler) GetSleepSetting(c *gin.Context) {
	characterID := c.Query("characterId")
	util.SuccessResponse(c, h.service.GetSleepSetting(characterID))
}
func (h *Handler) UpdateSleepSetting(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	characterID := c.Query("characterId")
	util.SuccessResponse(c, h.service.UpdateSleepSetting(body, characterID))
}
func (h *Handler) GetSchedule(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSchedule(c.Query("date"), c.Query("characterId")))
}
func (h *Handler) GetScheduleConflicts(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetScheduleConflicts(c.Query("date"), c.Query("characterId")))
}
func (h *Handler) GetScheduleToday(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetScheduleToday(c.Query("characterId")))
}
func (h *Handler) GetStateLife(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStateLife(c.Query("characterId")))
}
func (h *Handler) GetState(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetState(c.Query("characterId")))
}
func (h *Handler) GetTimelineToday(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetTimelineToday(c.Query("characterId")))
}

func (h *Handler) ListFixedEvents(c *gin.Context) {
	util.SuccessResponse(c, h.service.ListFixedEvents(c.Query("date"), c.Query("characterId")))
}
func (h *Handler) CreateFixedEvent(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	characterID := c.Query("characterId")
	util.SuccessMsgResponse(c, "事件已创建", h.service.CreateFixedEvent(body, characterID))
}
func (h *Handler) UpdateFixedEvent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	characterID := c.Query("characterId")
	util.SuccessMsgResponse(c, "事件已更新", h.service.UpdateFixedEvent(id, body, characterID))
}
func (h *Handler) DeleteFixedEvent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	characterID := c.Query("characterId")
	if !h.service.DeleteFixedEvent(id, characterID) {
		util.ErrorResponse(c, response.NotFound, "事件不存在", nil)
		return
	}
	util.SuccessMsgResponse(c, "事件已删除", nil)
}
func (h *Handler) ToggleFixedEventEnabled(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	util.SuccessResponse(c, h.service.ToggleFixedEventEnabled(id))
}

func (h *Handler) ListSpecialEvents(c *gin.Context) {
	util.SuccessResponse(c, h.service.ListSpecialEvents(c.Query("characterId")))
}
func (h *Handler) CreateSpecialEvent(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	characterID := c.Query("characterId")
	util.SuccessMsgResponse(c, "事件已创建", h.service.CreateSpecialEvent(body, characterID))
}
func (h *Handler) UpdateSpecialEvent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	characterID := c.Query("characterId")
	util.SuccessMsgResponse(c, "事件已更新", h.service.UpdateSpecialEvent(id, body, characterID))
}
func (h *Handler) DeleteSpecialEvent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	characterID := c.Query("characterId")
	if !h.service.DeleteSpecialEvent(id, characterID) {
		util.ErrorResponse(c, response.NotFound, "事件不存在", nil)
		return
	}
	util.SuccessMsgResponse(c, "事件已删除", nil)
}
func (h *Handler) ToggleSpecialEventEnabled(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	util.SuccessResponse(c, h.service.ToggleSpecialEventEnabled(id))
}

func (h *Handler) ListClassAdjustments(c *gin.Context) {
	util.SuccessResponse(c, h.service.ListClassAdjustments(c.Query("characterId")))
}
func (h *Handler) CreateClassAdjustment(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	characterID := c.Query("characterId")
	util.SuccessMsgResponse(c, "调课已创建", h.service.CreateClassAdjustment(body, characterID))
}
func (h *Handler) UpdateClassAdjustment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	characterID := c.Query("characterId")
	util.SuccessMsgResponse(c, "调课已更新", h.service.UpdateClassAdjustment(id, body, characterID))
}
func (h *Handler) DeleteClassAdjustment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	characterID := c.Query("characterId")
	if !h.service.DeleteClassAdjustment(id, characterID) {
		util.ErrorResponse(c, response.NotFound, "调课不存在", nil)
		return
	}
	util.SuccessMsgResponse(c, "调课已删除", nil)
}
func (h *Handler) GetEffectiveClasses(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetEffectiveClasses(c.Query("date"), c.Query("characterId")))
}

func (h *Handler) GetLifestyleTendency(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLifestyleTendency(c.Query("characterId")))
}
func (h *Handler) UpdateLifestyleTendency(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateLifestyleTendency(body, c.Query("characterId")))
}
func (h *Handler) ResetLifestyleTendency(c *gin.Context) {
	util.SuccessResponse(c, h.service.ResetLifestyleTendency(c.Query("characterId")))
}

func (h *Handler) GetWorkProfile(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWorkProfile(c.Query("characterId")))
}
func (h *Handler) UpdateWorkProfile(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateWorkProfile(body, c.Query("characterId")))
}

func (h *Handler) GetActiveMessageSetting(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetActiveMessageSetting(c.Query("characterId")))
}
func (h *Handler) UpdateActiveMessageSetting(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateActiveMessageSetting(body, c.Query("characterId")))
}
func (h *Handler) GetActiveMessageTasksToday(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetActiveMessageTasksToday(c.Query("characterId")))
}
func (h *Handler) RegenerateActiveMessageTasks(c *gin.Context) {
	util.SuccessResponse(c, h.service.RegenerateActiveMessageTasks(c.Query("characterId")))
}
func (h *Handler) RunActiveMessageTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	characterID := c.Query("characterId")
	util.SuccessResponse(c, h.service.RunActiveMessageTask(id, characterID))
}
func (h *Handler) CancelActiveMessageTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	characterID := c.Query("characterId")
	util.SuccessResponse(c, h.service.CancelActiveMessageTask(id, characterID))
}

func (h *Handler) ListDelayedReplies(c *gin.Context) {
	util.SuccessResponse(c, h.service.ListDelayedReplies(c.Query("characterId")))
}
func (h *Handler) CancelDelayedReply(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	util.SuccessResponse(c, h.service.CancelDelayedReply(id, c.Query("characterId")))
}
func (h *Handler) ProcessDelayedReplies(c *gin.Context) {
	util.SuccessResponse(c, h.service.ProcessDelayedReplies(c.Query("characterId")))
}

func (h *Handler) GetDebugOverview(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetDebugOverview(c.Query("characterId")))
}
func (h *Handler) RegenerateAllDebug(c *gin.Context) {
	util.SuccessResponse(c, h.service.RegenerateAllDebug(c.Query("characterId")))
}
func (h *Handler) ProcessActiveMessagesDebug(c *gin.Context) {
	util.SuccessResponse(c, h.service.ProcessActiveMessagesDebug(c.Query("characterId")))
}
func (h *Handler) ProcessDelayedRepliesDebug(c *gin.Context) {
	util.SuccessResponse(c, h.service.ProcessDelayedRepliesDebug(c.Query("characterId")))
}

func (h *Handler) GetRuleLogs(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRuleLogs(c.Query("characterId")))
}
func (h *Handler) RegenerateSchedule(c *gin.Context) {
	util.SuccessResponse(c, h.service.RegenerateSchedule(c.Query("characterId")))
}
func (h *Handler) RegenerateTimeline(c *gin.Context) {
	util.SuccessResponse(c, h.service.RegenerateTimeline(c.Query("characterId")))
}

func (h *Handler) TriggerDailyRegeneration(c *gin.Context) {
	util.SuccessResponse(c, h.service.TriggerDailyRegeneration(c.Query("characterId")))
}
