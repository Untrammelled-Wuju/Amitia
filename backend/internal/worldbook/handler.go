// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
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

func (h *Handler) List(c *gin.Context) {
	var q WorldBookListQuery
	c.ShouldBindQuery(&q)
	resp, err := h.service.ListForUser(requestidentity.ResolveGin(c, ""), q)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateWorldBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	e, err := h.service.CreateForUser(requestidentity.ResolveGin(c, ""), &req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "创建成功", e)
}

func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req UpdateWorldBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	e, err := h.service.UpdateForUser(requestidentity.ResolveGin(c, ""), id, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "更新成功", e)
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteForUser(requestidentity.ResolveGin(c, ""), id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "删除成功", nil)
}

func (h *Handler) TestMatch(c *gin.Context) {
	var req TestMatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少必要参数", nil)
		return
	}
	resp, err := h.service.TestMatchForUser(requestidentity.ResolveGin(c, ""), req.Text)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) DeleteAll(c *gin.Context) {
	if err := h.service.DeleteAllForUser(requestidentity.ResolveGin(c, "")); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "已清空所有规则", nil)
}

func (h *Handler) SystemPrompt(c *gin.Context) {
	userMessage := c.Query("userMessage")
	prompt := h.service.ToSystemPromptForUser(requestidentity.ResolveGin(c, ""), c.Query("characterId"), userMessage, "")
	util.SuccessResponse(c, map[string]string{"prompt": prompt})
}
