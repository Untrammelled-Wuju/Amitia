// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/util"
)

type VoiceHandler struct {
	service Service
}

func NewVoiceHandler(service Service) *VoiceHandler {
	return &VoiceHandler{service: service}
}

type CreateSessionRequest struct {
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	Mode           string `json:"mode"`
	Platform       string `json:"platform"`
	ProfileID      string `json:"profileId"`
	UserID         string `json:"userId"`
}

func (h *VoiceHandler) CreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "请求参数错误", err.Error())
		return
	}

	voiceReq := VoiceSessionRequest{
		ConversationID: req.ConversationID,
		CharacterID:    req.CharacterID,
		UserID:         req.UserID,
		ProfileID:      req.ProfileID,
	}

		if req.Mode != "" {
		voiceReq.Mode = ContinuousVoiceSessionMode(req.Mode)
	} else {
		voiceReq.Mode = ContinuousVoiceSessionModePushToTalk
	}

	if req.Platform != "" {
		voiceReq.Platform = Platform(req.Platform)
	} else {
		voiceReq.Platform = PlatformWeb
	}

	sess, err := h.service.CreateSession(c.Request.Context(), voiceReq)
	if err != nil {
		util.ErrorResponse(c, http.StatusInternalServerError, "创建语音会话失败", err.Error())
		return
	}

	util.SuccessResponse(c, sess)
}

func (h *VoiceHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "会话ID不能为空", nil)
		return
	}

	sess, err := h.service.GetSession(sessionID)
	if err != nil {
		util.ErrorResponse(c, http.StatusNotFound, "会话不存在", err.Error())
		return
	}

	util.SuccessResponse(c, sess)
}

func (h *VoiceHandler) StartSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "会话ID不能为空", nil)
		return
	}

	if err := h.service.StartSession(c.Request.Context(), sessionID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "启动会话失败", err.Error())
		return
	}

	util.SuccessMsgResponse(c, "会话已启动", gin.H{"sessionId": sessionID})
}

func (h *VoiceHandler) StopSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "会话ID不能为空", nil)
		return
	}

	if err := h.service.StopSession(c.Request.Context(), sessionID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "停止会话失败", err.Error())
		return
	}

	util.SuccessMsgResponse(c, "会话已停止", gin.H{"sessionId": sessionID})
}

func (h *VoiceHandler) InterruptSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "会话ID不能为空", nil)
		return
	}

	if err := h.service.InterruptSession(c.Request.Context(), sessionID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "打断会话失败", err.Error())
		return
	}

	util.SuccessMsgResponse(c, "已打断", gin.H{"sessionId": sessionID})
}

func (h *VoiceHandler) ArmWake(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "会话ID不能为空", nil)
		return
	}

	if err := h.service.ArmWake(c.Request.Context(), sessionID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "启动唤醒失败", err.Error())
		return
	}

	util.SuccessMsgResponse(c, "已启动唤醒", gin.H{"sessionId": sessionID})
}

func (h *VoiceHandler) DisarmWake(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "会话ID不能为空", nil)
		return
	}

	if err := h.service.DisarmWake(c.Request.Context(), sessionID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "关闭唤醒失败", err.Error())
		return
	}

	util.SuccessMsgResponse(c, "已关闭唤醒", gin.H{"sessionId": sessionID})
}

func (h *VoiceHandler) ListSessions(c *gin.Context) {
	sessions := h.service.ListActiveSessions()
	util.SuccessResponse(c, gin.H{"sessions": sessions, "total": len(sessions)})
}

func (h *VoiceHandler) GetStatus(c *gin.Context) {
	status := h.service.Status()
	util.SuccessResponse(c, status)
}

