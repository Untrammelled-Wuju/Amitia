// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	applog "github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type webChatSendRequest struct {
	ConversationID  string  `json:"conversationId"`
	Content         string  `json:"content"`
	Message         string  `json:"message"`
	CharacterID     string  `json:"characterId"`
	UserID          string  `json:"userId"`
	PeerID          string  `json:"peerId"`
	RequestID       string  `json:"requestId"`
	SessionID       string  `json:"sessionId"`
	Source          string  `json:"source"`
	ClientMessageID string  `json:"clientMessageId"`
	MessageID       string  `json:"messageId"`
	VoiceMessage    bool    `json:"voiceMessage"`
	AudioUrl        string  `json:"audioUrl"`
	AudioDuration   float64 `json:"audioDuration"`
	ImageUrl        string  `json:"imageUrl"`
	VideoUrl        string  `json:"videoUrl"`
}

func (h *Handler) WebChatCreateConv(c *gin.Context) {
	var body struct {
		Title       string `json:"title"`
		CharacterID string `json:"characterId"`
		Channel     string `json:"channel"`
		Source      string `json:"source"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	if body.Title == "" {
		body.Title = "新对话"
		if body.CharacterID != "" {
			var charName string
			h.db.Table("characters").Select("name").Where("id = ?", body.CharacterID).Limit(1).Row().Scan(&charName)
			if charName != "" {
				body.Title = charName
			}
		}
	}
	if body.Channel == "" {
		body.Channel = "web"
	}
	if body.Source == "" {
		body.Source = "web"
	}
	if body.CharacterID != "" {
		var existingConvID string
		h.db.Table("characters").Select("conversation_id").Where("id = ?", body.CharacterID).Limit(1).Row().Scan(&existingConvID)
		if existingConvID != "" {
			var conv struct {
				ID, Title, Channel, Source, CharacterID string
			}
			h.db.Table("conversations").Select("id, title, channel, source, character_id").Where("id = ?", existingConvID).Limit(1).Row().Scan(&conv.ID, &conv.Title, &conv.Channel, &conv.Source, &conv.CharacterID)
			if conv.ID != "" {
				util.SuccessResponse(c, gin.H{"id": conv.ID, "title": conv.Title, "channel": conv.Channel, "source": conv.Source, "characterId": conv.CharacterID})
				return
			}
		}
	}
	if body.Channel == "wechat" || body.Channel == "qq" {
		var existing []map[string]interface{}
		h.db.Table("conversations").Where("channel = ? AND character_id = ?", body.Channel, body.CharacterID).Limit(1).Find(&existing)
		if len(existing) > 0 {
			util.SuccessResponse(c, gin.H{"id": existing[0]["id"], "title": existing[0]["title"], "channel": body.Channel, "source": existing[0]["source"], "characterId": existing[0]["character_id"]})
			return
		}
	}
	convID := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	h.db.Exec("INSERT INTO conversations (id, title, character_id, channel, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", convID, body.Title, body.CharacterID, body.Channel, body.Source, now, now)
	h.db.Exec("UPDATE characters SET conversation_id = ?, updated_at = ? WHERE id = ? AND (conversation_id IS NULL OR conversation_id = '')", convID, now, body.CharacterID)
	util.SuccessResponse(c, gin.H{"id": convID, "title": body.Title, "channel": body.Channel, "source": body.Source, "characterId": body.CharacterID})
}

func (h *Handler) WebChatDeleteConv(c *gin.Context) {
	id := c.Param("id")
	h.db.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
	h.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) WebChatUpdateConv(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		CharacterID string `json:"characterId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	if body.CharacterID != "" {
		h.db.Exec("UPDATE conversations SET character_id = ?, updated_at = ? WHERE id = ?", body.CharacterID, time.Now(), id)
	}
	util.SuccessResponse(c, gin.H{"updated": true, "id": id})
}

func (h *Handler) WebChatDeleteConvMessages(c *gin.Context) {
	id := c.Param("id")
	h.db.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) WebChatRegenerate(c *gin.Context) {
	convID := c.Param("id")
	if convID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	type lastMsg struct {
		Role    string
		Content string
	}
	var msg lastMsg
	if err := h.db.Table("messages").Select("role, content").Where("conversation_id = ?", convID).Order("created_at DESC").Limit(1).Row().Scan(&msg.Role, &msg.Content); err != nil || msg.Role != "user" {
		util.ErrorResponse(c, response.DataNotFound, "没有可重新生成的消息", nil)
		return
	}
	h.db.Exec("DELETE FROM messages WHERE id = (SELECT id FROM messages WHERE conversation_id = ? AND role = 'assistant' ORDER BY created_at DESC LIMIT 1)", convID)
	h.WebChatSend(c)
}

func (h *Handler) WebChatReplyTimingForce(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"forced": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingHold(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"held": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingResume(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"resumed": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingStatus(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"id": c.Param("id"), "status": "idle"})
}

func (h *Handler) WebChatMessageStatus(c *gin.Context) {
	msgID := c.Param("id")
	if msgID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少消息ID", nil)
		return
	}

	var msg struct {
		ID             string `gorm:"column:id"`
		ConversationID string `gorm:"column:conversation_id"`
		Status         string `gorm:"column:status"`
		RequestID      string `gorm:"column:request_id"`
		Role           string `gorm:"column:role"`
		CreatedAt      string `gorm:"column:created_at"`
		UpdatedAt      string `gorm:"column:updated_at"`
	}
	if err := h.db.Table("messages").Select("id, conversation_id, status, request_id, role, created_at, updated_at").Where("id = ?", msgID).Take(&msg).Error; err != nil {
		util.ErrorResponse(c, response.DataNotFound, "消息不存在", nil)
		return
	}

	result := gin.H{"id": msgID, "status": msg.Status, "role": msg.Role, "conversationId": msg.ConversationID, "createdAt": msg.CreatedAt, "updatedAt": msg.UpdatedAt}

	if msg.RequestID != "" {
		var interactionStatus string
		if scanErr := h.db.Table("interaction_records").Select("status").Where("id = ?", msg.RequestID).Limit(1).Row().Scan(&interactionStatus); scanErr == nil && interactionStatus != "" {
			result["interactionStatus"] = interactionStatus
			if msg.Status == "processing" && strings.Contains(interactionStatus, "committed") {
				result["status"] = "completed"
			}
		}
	}

	util.SuccessResponse(c, result)
}

func (h *Handler) WebChatSend(c *gin.Context) {
	var body webChatSendRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	msgContent := body.Content
	if msgContent == "" {
		msgContent = body.Message
	}
	if msgContent == "" {
		util.ErrorResponse(c, response.InvalidParams, "消息不能为空", nil)
		return
	}

	convID := body.ConversationID
	if convID == "" {
		convID = "web-" + uuid.New().String()[:8]
	}
	requestID := resolveRequestID(c, body.RequestID, body.ClientMessageID, body.MessageID)
	sessionID := resolveRequestBackedValue(c, body.SessionID, "X-Session-ID", "sessionId", "session_id")
	if sessionID == "" {
		sessionID = convID
	}
	userID := resolveRequestBackedValue(c, body.UserID, "X-User-ID", "userId", "user_id")
	peerID := resolveRequestBackedValue(c, body.PeerID, "X-Peer-ID", "peerId", "peer_id")
	source := resolveSource(c, body.Source, "web")
	c.Header("X-Request-ID", requestID)
	c.Header("X-Session-ID", sessionID)
	c.Header("X-Source", source)

	applog.Info(fmt.Sprintf("[Webhook] ImageUrl=%s VideoUrl=%s", body.ImageUrl[:min(len(body.ImageUrl), 60)], body.VideoUrl[:min(len(body.VideoUrl), 60)]))
	chat.GetBuffer().AnalyzeImage(convID, body.ImageUrl)
	chat.GetBuffer().AnalyzeVideo(convID, body.VideoUrl)

	bufferedMsgs, bufErr := chat.GetBuffer().Buffer(convID, msgContent)
	if bufErr != nil {
		util.SuccessResponse(c, gin.H{"status": "queued", "conversationId": convID, "requestId": requestID, "sessionId": sessionID})
		return
	}

	mergedContent := strings.Join(bufferedMsgs, "\n")
	imageCtx := chat.GetBuffer().GetImageContexts(convID)
	applog.Info(fmt.Sprintf("[Webhook] imageCtx len=%d content=%s", len(imageCtx), imageCtx[:min(len(imageCtx), 200)]))
	chat.GetBuffer().ClearImageContexts(convID)

	characterID := body.CharacterID
	if characterID == "" && body.ConversationID != "" {
		var dbCharID string
		if scanErr := h.db.Table("conversations").Select("character_id").Where("id = ?", body.ConversationID).Limit(1).Row().Scan(&dbCharID); scanErr == nil && strings.TrimSpace(dbCharID) != "" {
			characterID = dbCharID
		}
	}

	orchResult, err := h.unifiedEntry.Handle(c.Request.Context(), &interaction.UnifiedEntryRequest{
		ConversationID: convID, Channel: "web", Source: source,
		UserID: userID, PeerID: peerID, RequestID: requestID, SessionID: sessionID,
		CharacterID: characterID, Message: mergedContent,
		AudioUrl: body.AudioUrl, AudioDuration: body.AudioDuration,
		VoiceMessage: body.VoiceMessage,
		ImageUrl:     body.ImageUrl,
		VideoUrl:     body.VideoUrl,
		ImageContext: imageCtx,
	})
	if errors.Is(err, interaction.ErrOrchestratorProcessing) {
		util.SuccessResponse(c, gin.H{"status": "processing", "requestId": requestID, "sessionId": sessionID, "userId": userID, "source": source})
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"conversationId": orchResult.Response.ConversationID, "reply": orchResult.Response.Reply, "messageIds": orchResult.Response.MessageIDs, "characterName": orchResult.Response.CharacterName, "requestId": requestID, "sessionId": sessionID, "userId": userID, "source": source})
}

func (h *Handler) WebChatFromImport(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, map[string]interface{}{"imported": true, "conversationId": ""})
}

func resolveRequestID(c *gin.Context, candidates ...string) string {
	candidates = append(candidates, c.GetHeader("X-Request-ID"), c.GetHeader("X-Idempotency-Key"), c.Query("requestId"), c.Query("request_id"), c.Query("idempotencyKey"))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return uuid.New().String()
}

func resolveHeaderBackedValue(c *gin.Context, bodyValue string, headerName string) string {
	return resolveRequestBackedValue(c, bodyValue, headerName)
}

func resolveRequestBackedValue(c *gin.Context, bodyValue string, headerName string, queryNames ...string) string {
	bodyValue = strings.TrimSpace(bodyValue)
	if bodyValue != "" {
		return bodyValue
	}
	headerValue := strings.TrimSpace(c.GetHeader(headerName))
	if headerValue != "" {
		return headerValue
	}
	for _, queryName := range queryNames {
		queryValue := strings.TrimSpace(c.Query(queryName))
		if queryValue != "" {
			return queryValue
		}
	}
	return ""
}

func resolveSource(c *gin.Context, bodyValue string, fallback string) string {
	source := resolveRequestBackedValue(c, bodyValue, "X-Source", "source")
	if source != "" {
		return source
	}
	return strings.TrimSpace(fallback)
}
