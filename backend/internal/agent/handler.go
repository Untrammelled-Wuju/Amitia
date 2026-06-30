// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package agent

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

func (h *Handler) Test(c *gin.Context) {
	var body struct {
		CharacterID string `json:"characterId"`
		Message     string `json:"message"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	result, err := h.service.Test(body.CharacterID, body.Message)
	if err != nil {
		util.ErrorResponse(c, response.BusinessError, "AI 调用失败: "+err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) Webhook(c *gin.Context) {
	var body struct {
		Channel        string `json:"channel"`
		AccountID      string `json:"accountId"`
		ConversationID string `json:"conversationId"`
		SenderID       string `json:"senderId"`
		MessageID      string `json:"messageId"`
		Text           string `json:"text"`
		VoiceMessage   bool   `json:"voiceMessage"`
		MsgType        string `json:"type"`
		ImageUrl       string `json:"imageUrl"`
		VideoUrl       string `json:"videoUrl"`
		AudioBase64    string `json:"audioBase64"`
		SkipTiming     bool   `json:"skipTiming"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	result, err := h.service.Webhook(body.Channel, body.SenderID, body.ConversationID, body.Text, body.VoiceMessage, body.ImageUrl, body.VideoUrl, body.AudioBase64, body.SkipTiming)
	if err != nil {
		util.ErrorResponse(c, response.BusinessError, "AI 调用失败: "+err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}

func (h *Handler) ContextPreview(c *gin.Context) {
	convID := c.Query("conversationId")
	if convID == "" {
		util.ErrorResponse(c, response.InvalidParams, "conversationId 不能为空", nil)
		return
	}
	result, err := h.service.ContextPreview(convID)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, result)
}
