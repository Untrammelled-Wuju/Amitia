package reminder

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.service.List(owner(c))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询提醒失败", nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) Create(c *gin.Context) {
	var request CreateReminderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "标题和提醒时间不能为空", nil)
		return
	}
	item, err := h.service.Create(&request, owner(c))
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "提醒创建成功", item)
}

func (h *Handler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	item, err := h.service.Update(id, owner(c), updates)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "提醒更新成功", item)
}

func (h *Handler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.Delete(id, owner(c)); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "提醒已删除", nil)
}

func (h *Handler) Toggle(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.service.Toggle(id, owner(c))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "操作失败", nil)
		return
	}
	util.SuccessMsgResponse(c, "状态已切换", item)
}

func (h *Handler) Test(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.service.Find(id, owner(c))
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "提醒不存在", nil)
		return
	}
	util.SuccessResponse(c, h.service.Test(item))
}

func (h *Handler) Trigger(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.service.Find(id, owner(c))
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "提醒不存在", nil)
		return
	}
	messageID, conversationID, err := h.service.TriggerNow(c.Request.Context(), item)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "提醒未发送: "+err.Error(), nil)
		return
	}
	h.service.advance(item)
	util.SuccessResponse(c, gin.H{
		"id":             id,
		"triggered":      true,
		"title":          item.Title,
		"conversationId": conversationID,
		"messageId":      messageID,
	})
}

func (h *Handler) Status(c *gin.Context) {
	status, err := h.service.Status(owner(c))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询状态失败", nil)
		return
	}
	util.SuccessResponse(c, status)
}

func (h *Handler) History(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	state := c.Query("state")
	items, total, err := h.service.History(owner(c), page, pageSize, state)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询触发历史失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"items": items, "total": total})
}

func (h *Handler) Prospective(c *gin.Context) {
	util.SuccessResponse(c, []interface{}{})
}

func (h *Handler) QueueSummary(c *gin.Context) {
	summary, err := h.service.QueueSummary(owner(c))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询队列状态失败", nil)
		return
	}
	util.SuccessResponse(c, summary)
}

func (h *Handler) GetCleanupConfig(c *gin.Context) {
	util.SuccessResponse(c, gin.H{"cleanupDays": h.service.GetCleanupDays()})
}

func (h *Handler) SetCleanupConfig(c *gin.Context) {
	var request struct {
		CleanupDays string `json:"cleanupDays"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	if err := h.service.SetCleanupDays(request.CleanupDays); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已更新", gin.H{"cleanupDays": h.service.GetCleanupDays()})
}

func (h *Handler) ClearBackpressure(c *gin.Context) {
	util.SuccessMsgResponse(c, "背压标记已清除", nil)
}

func owner(c *gin.Context) string {
	value := requestidentity.ResolveGin(c, "")
	if value == "" {
		return "default"
	}
	return value
}
