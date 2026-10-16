// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
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

func (h *Handler) Settings(c *gin.Context) {
	util.SuccessResponse(c, h.service.Settings())
}

func (h *Handler) SaveSettings(c *gin.Context) {
	var body struct {
		UploadURL string `json:"uploadUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "参数无效", nil)
		return
	}
	settings, err := h.service.SaveUploadURL(body.UploadURL)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "上报地址已更新", settings)
}

func (h *Handler) Preview(c *gin.Context) {
	var body TicketInput
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "参数无效", nil)
		return
	}
	applyActorDevice(&body, c)
	result, err := h.service.Preview(&body)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) Create(c *gin.Context) {
	var body TicketInput
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "反馈内容无效", nil)
		return
	}
	applyActorDevice(&body, c)
	ticket, err := h.service.Create(requestidentity.ResolveGin(c), &body)
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
		return
	}
	util.SuccessMsgResponse(c, "反馈已提交", ticket)
}

func (h *Handler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.service.List(requestidentity.ResolveGin(c), limit)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"tickets": items})
}

func (h *Handler) Retry(c *gin.Context) {
	ticket, err := h.service.Retry(requestidentity.ResolveGin(c), c.Param("id"))
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, ticket)
}

func applyActorDevice(body *TicketInput, c *gin.Context) {
	actor := security.GetActor(c)
	if actor == nil {
		return
	}
	if deviceID := strings.TrimSpace(string(actor.DeviceID)); deviceID != "" {
		body.DeviceID = deviceID
	}
}
