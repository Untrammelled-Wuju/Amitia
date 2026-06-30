// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type Handler struct {
	service Service
}

func NewHandler(srv Service) *Handler {
	return &Handler{service: srv}
}

func (h *Handler) Create(c *gin.Context) {
	msgID := c.Param("id")
	var req CreateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效的反馈类型", nil)
		return
	}
	fb, err := h.service.Create(msgID, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "反馈已提交", fb)
}

func (h *Handler) GetByMessage(c *gin.Context) {
	msgID := c.Param("id")
	items, err := h.service.GetByMessage(msgID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) Stats(c *gin.Context) {
	stats := h.service.GetStats()
	util.SuccessResponse(c, stats)
}

func (h *Handler) Recent(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, err := h.service.GetRecent(limit)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.service.Delete(id); err != nil {
		util.ErrorResponse(c, response.NotFound, "反馈不存在", nil)
		return
	}
	util.SuccessResponse(c, nil)
}
