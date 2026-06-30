// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package profile

import (
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

func (h *Handler) List(c *gin.Context) {
	var q ProfileListQuery
	c.ShouldBindQuery(&q)
	resp, err := h.service.List(q)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少必要参数", nil)
		return
	}
	p, err := h.service.Create(&req)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "画像创建成功", p)
}

func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	p, err := h.service.Update(id, &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "画像更新成功", p)
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(id); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "删除成功", nil)
}

func (h *Handler) GetByUserID(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		util.ErrorResponse(c, response.InvalidParams, "userId不能为空", nil)
		return
	}
	profiles, err := h.service.GetByUserID(userID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, profiles)
}

func (h *Handler) Extract(c *gin.Context) {
	var body struct {
		UserID         string              `json:"userId"`
		ConversationID string              `json:"conversationId"`
		Messages       []map[string]string `json:"messages"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	if err := h.service.ExtractFromConversation(body.UserID, body.ConversationID, body.Messages); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "画像提取完成", nil)
}

func (h *Handler) SystemPrompt(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		userID = "default"
	}
	prompt := h.service.ToSystemPrompt(userID)
	util.SuccessResponse(c, map[string]string{"prompt": prompt})
}
