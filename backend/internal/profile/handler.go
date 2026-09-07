// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package profile

import (
	"fmt"
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
	var q ProfileListQuery
	c.ShouldBindQuery(&q)
	q.UserID = requestidentity.ResolveGin(c, "")
	resp, err := h.service.List(q)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	userID := requestidentity.ResolveGin(c, "")
	scoped, ok := h.service.(profileUserScopedService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "profile service does not support authenticated ownership", nil)
		return
	}
	p, err := scoped.CreateForUser(&req, userID)
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
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	p, err := h.updateForUser(id, requestidentity.ResolveGin(c, ""), &req)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "画像更新成功", p)
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.deleteForUser(id, requestidentity.ResolveGin(c, "")); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "删除成功", nil)
}

func (h *Handler) GetByUserID(c *gin.Context) {
	userID := requestidentity.ResolveGin(c, "")
	profiles, err := h.service.GetByUserID(userID, c.Query("characterId"))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, profiles)
}

func (h *Handler) Extract(c *gin.Context) {
	var body struct {
		UserID         string              `json:"userId"`
		CharacterID    string              `json:"characterId"`
		ConversationID string              `json:"conversationId"`
		Messages       []map[string]string `json:"messages"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	if err := h.service.ExtractFromConversation(requestidentity.ResolveGin(c, ""), body.ConversationID, body.Messages, body.CharacterID); err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "画像提取完成", nil)
}

func (h *Handler) SystemPrompt(c *gin.Context) {
	userID := requestidentity.ResolveGin(c, "")
	prompt := h.service.ToSystemPrompt(userID, c.Query("characterId"))
	util.SuccessResponse(c, map[string]string{"prompt": prompt})
}

type profileUserScopedService interface {
	CreateForUser(req *CreateProfileRequest, userID string) (*UserProfile, error)
	UpdateForUser(id, userID string, req *UpdateProfileRequest) (*UserProfile, error)
	DeleteForUser(id, userID string) error
}

func (h *Handler) updateForUser(id, userID string, req *UpdateProfileRequest) (*UserProfile, error) {
	scoped, ok := h.service.(profileUserScopedService)
	if !ok {
		return nil, fmt.Errorf("profile service does not support authenticated ownership")
	}
	return scoped.UpdateForUser(id, userID, req)
}

func (h *Handler) deleteForUser(id, userID string) error {
	scoped, ok := h.service.(profileUserScopedService)
	if !ok {
		return fmt.Errorf("profile service does not support authenticated ownership")
	}
	return scoped.DeleteForUser(id, userID)
}
