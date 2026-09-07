// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
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
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	fb, err := h.service.CreateForUser(msgID, requestidentity.ResolveGin(c, ""), &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "反馈已提交", fb)
}

func (h *Handler) GetByMessage(c *gin.Context) {
	msgID := c.Param("id")
	items, err := h.service.GetByMessageForUser(msgID, requestidentity.ResolveGin(c, ""))
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "反馈不存在", nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, stats)
}

func (h *Handler) Recent(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, err := h.service.GetRecent(limit)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, items)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.ErrorResponse(c, response.InvalidParams, "无效反馈 ID", nil)
		return
	}
	if err := h.service.DeleteForUser(id, requestidentity.ResolveGin(c, "")); err != nil {
		util.ErrorResponse(c, response.NotFound, "反馈不存在", nil)
		return
	}
	util.SuccessResponse(c, nil)
}
