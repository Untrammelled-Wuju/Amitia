// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
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

	userID := voiceHandlerUserID(c)
	if userID == "" {
		util.ErrorResponse(c, http.StatusForbidden, "缺少有效的用户身份", nil)
		return
	}
	conversationCharacterID, err := requireRealtimeConversationOwner(req.ConversationID, userID)
	if err != nil {
		util.ErrorResponse(c, http.StatusNotFound, "会话不存在或无权访问", nil)
		return
	}
	if req.CharacterID == "" {
		req.CharacterID = conversationCharacterID
	} else {
		if conversationCharacterID != "" && conversationCharacterID != req.CharacterID {
			util.ErrorResponse(c, http.StatusBadRequest, "角色与会话不匹配", nil)
			return
		}
		if err := requireRealtimeCharacterOwner(req.CharacterID, userID); err != nil {
			util.ErrorResponse(c, http.StatusNotFound, "角色不存在或无权访问", nil)
			return
		}
	}
	voiceReq := VoiceSessionRequest{
		ConversationID: req.ConversationID,
		CharacterID:    req.CharacterID,
		UserID:         userID,
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

	sess, ok := h.requireOwnedSession(c, sessionID)
	if !ok {
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

	if _, ok := h.requireOwnedSession(c, sessionID); !ok {
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

	if _, ok := h.requireOwnedSession(c, sessionID); !ok {
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

	if _, ok := h.requireOwnedSession(c, sessionID); !ok {
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

	if _, ok := h.requireOwnedSession(c, sessionID); !ok {
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

	if _, ok := h.requireOwnedSession(c, sessionID); !ok {
		return
	}

	if err := h.service.DisarmWake(c.Request.Context(), sessionID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "关闭唤醒失败", err.Error())
		return
	}

	util.SuccessMsgResponse(c, "已关闭唤醒", gin.H{"sessionId": sessionID})
}

func (h *VoiceHandler) PublishASRFinal(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		util.ErrorResponse(c, http.StatusBadRequest, "会话ID不能为空", nil)
		return
	}
	if _, ok := h.requireOwnedSession(c, sessionID); !ok {
		return
	}
	var req struct {
		Transcript string `json:"transcript"`
		EventID    string `json:"eventId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "请求参数错误", err.Error())
		return
	}
	if err := h.service.PublishASRFinal(c.Request.Context(), sessionID, req.Transcript, req.EventID); err != nil {
		util.ErrorResponse(c, http.StatusBadRequest, "发布 ASR Final 失败", err.Error())
		return
	}
	util.SuccessMsgResponse(c, "ASR Final 已发布", gin.H{"sessionId": sessionID})
}

func (h *VoiceHandler) ListSessions(c *gin.Context) {
	userID := voiceHandlerUserID(c)
	if userID == "" {
		util.ErrorResponse(c, http.StatusForbidden, "缺少有效的用户身份", nil)
		return
	}
	all := h.service.ListActiveSessions()
	sessions := make([]*ContinuousVoiceSession, 0, len(all))
	for _, sess := range all {
		if sess != nil && realtimeOwnerMatches(sess.UserID, userID) {
			sessions = append(sessions, sess)
		}
	}
	util.SuccessResponse(c, gin.H{"sessions": sessions, "total": len(sessions)})
}

func (h *VoiceHandler) GetStatus(c *gin.Context) {
	userID := voiceHandlerUserID(c)
	if userID == "" {
		util.ErrorResponse(c, http.StatusForbidden, "缺少有效的用户身份", nil)
		return
	}
	status := h.service.Status()
	status.ActiveSessions = 0
	status.WakeArmedSessions = 0
	for _, sess := range h.service.ListActiveSessions() {
		if sess == nil || !realtimeOwnerMatches(sess.UserID, userID) {
			continue
		}
		status.ActiveSessions++
		if sess.WakeArmed {
			status.WakeArmedSessions++
		}
	}
	util.SuccessResponse(c, status)
}

func voiceHandlerUserID(c *gin.Context) string {
	if actor := security.GetActor(c); actor != nil && actor.UserID != "" {
		return realtimeEffectiveUserID(actor.UserID.String())
	}
	return realtimeEffectiveUserID("")
}

func (h *VoiceHandler) requireOwnedSession(c *gin.Context, sessionID string) (*ContinuousVoiceSession, bool) {
	userID := voiceHandlerUserID(c)
	if userID == "" {
		util.ErrorResponse(c, http.StatusForbidden, "缺少有效的用户身份", nil)
		return nil, false
	}
	sess, err := h.service.GetSession(sessionID)
	if err != nil || sess == nil || !realtimeOwnerMatches(sess.UserID, userID) {
		util.ErrorResponse(c, http.StatusNotFound, "会话不存在或无权访问", nil)
		return nil, false
	}
	return sess, true
}
