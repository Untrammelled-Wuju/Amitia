// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/modelerror"
	"github.com/u-ai/backend/internal/requestidentity"
	applog "github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type webChatSendRequest struct {
	ConversationID   string  `json:"conversationId"`
	ProjectID        string  `json:"projectId"`
	Content          string  `json:"content"`
	Message          string  `json:"message"`
	CharacterID      string  `json:"characterId"`
	SpaceID          string  `json:"spaceId"`
	PeerID           string  `json:"peerId"`
	RequestID        string  `json:"requestId"`
	SessionID        string  `json:"sessionId"`
	Source           string  `json:"source"`
	ClientMessageID  string  `json:"clientMessageId"`
	MessageID        string  `json:"messageId"`
	DeviceTimezone   string  `json:"deviceTimezone"`
	VoiceMessage     bool    `json:"voiceMessage"`
	AudioUrl         string  `json:"audioUrl"`
	AudioDuration    float64 `json:"audioDuration"`
	ImageUrl         string  `json:"imageUrl"`
	VideoUrl         string  `json:"videoUrl"`
	ReplyToMessageID *string `json:"replyToMessageId,omitempty"`
}

func (h *Handler) WebChatListConversations(c *gin.Context) {
	q := chat.ConversationQuery{}
	c.ShouldBindQuery(&q)
	scoped, ok := h.chatSvc.(webChatScopedService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide user-scoped operations", nil)
		return
	}
	resp, err := scoped.ListConversationsForSpace(q, webChatSpaceID(c))
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	util.SuccessResponse(c, resp)
}

func (h *Handler) WebChatGetMessages(c *gin.Context) {
	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if h.channelAccess != nil {
		conversation, err := h.requireWebChatConversation(id, webChatSpaceID(c))
		if err != nil {
			util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
			return
		}
		if conversation.Channel != "" && conversation.Channel != "web" && !h.channelAccess.Has(conversation.Channel) {
			util.ErrorResponse(c, response.NotFound, "渠道消息不可用", nil)
			return
		}
	}
	scoped, ok := h.chatSvc.(webChatScopedService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide user-scoped operations", nil)
		return
	}
	msgs, total, err := scoped.GetMessagesForSpace(id, webChatSpaceID(c), page, pageSize)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	util.SuccessResponse(c, gin.H{"items": msgs, "total": total, "page": page, "pageSize": pageSize, "totalPages": totalPages})
}

func (h *Handler) WebChatCreateConv(c *gin.Context) {
	var body struct {
		Title     string `json:"title"`
		ProjectID string `json:"projectId"`
		Channel   string `json:"channel"`
		Source    string `json:"source"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	spaceID := webChatSpaceID(c)
	if body.Title == "" {
		body.Title = "新对话"
	}
	if body.Channel == "" {
		body.Channel = "web"
	}
	if body.Source == "" {
		body.Source = "web"
	}
	if body.Channel != "web" {
		scoped, ok := h.chatSvc.(webChatScopedService)
		if !ok {
			util.ErrorResponse(c, response.InternalError, "chat service does not provide user-scoped operations", nil)
			return
		}
		existingChannelConv, err := scoped.EnsureChannelConversationForSpace(body.Channel, spaceID)
		if err == nil && existingChannelConv != nil {
			util.SuccessResponse(c, gin.H{"id": existingChannelConv.ID, "title": existingChannelConv.Title, "channel": existingChannelConv.Channel, "source": existingChannelConv.Source, "projectId": existingChannelConv.ProjectID})
			return
		}
	}
	request := &chat.CreateConversationRequest{ProjectID: body.ProjectID, Title: body.Title, Channel: body.Channel, Source: body.Source}
	scoped, ok := h.chatSvc.(webChatScopedService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide user-scoped operations", nil)
		return
	}
	conv, err := scoped.CreateConversationForSpace(request, spaceID)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"id": conv.ID, "title": conv.Title, "channel": conv.Channel, "source": conv.Source, "projectId": conv.ProjectID})
}

func (h *Handler) WebChatDeleteConv(c *gin.Context) {
	id := c.Param("id")
	scoped, ok := h.chatSvc.(webChatScopedService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide user-scoped operations", nil)
		return
	}
	_, err := scoped.DeleteConversationForSpace(id, webChatSpaceID(c))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "删除失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) WebChatUpdateConv(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Title     *string `json:"title"`
		ProjectID *string `json:"projectId"`
		Pinned    *bool   `json:"pinned"`
		Archived  *bool   `json:"archived"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatConversation(id, spaceID); err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	if body.ProjectID != nil {
		scoped, ok := h.chatSvc.(webChatProjectService)
		if !ok {
			util.ErrorResponse(c, response.InternalError, "chat service does not provide project operations", nil)
			return
		}
		if _, err := scoped.MoveConversationToProjectForSpace(id, strings.TrimSpace(*body.ProjectID), spaceID); err != nil {
			util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
			return
		}
	}
	if body.Title != nil {
		title := strings.TrimSpace(*body.Title)
		if title == "" {
			util.ErrorResponse(c, response.InvalidParams, "会话标题不能为空", nil)
			return
		}
		if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ?", id).Update("title", title).Error; err != nil {
			util.ErrorResponse(c, response.OperationFailed, "重命名失败", nil)
			return
		}
	}
	if body.Pinned != nil || body.Archived != nil {
		scoped, ok := h.chatSvc.(webChatProjectService)
		if !ok {
			util.ErrorResponse(c, response.InternalError, "chat service does not provide project operations", nil)
			return
		}
		if _, err := scoped.UpdateConversationSidebarStateForSpace(id, body.Pinned, body.Archived, spaceID); err != nil {
			util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
			return
		}
	}
	util.SuccessResponse(c, gin.H{"updated": true, "id": id})
}

func (h *Handler) WebChatDeleteConvMessages(c *gin.Context) {
	id := c.Param("id")
	scoped, ok := h.chatSvc.(webChatScopedService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide user-scoped operations", nil)
		return
	}
	err := scoped.DeleteMessagesForSpace(id, webChatSpaceID(c))
	if err != nil {
		applog.Error(fmt.Sprintf("[WebChatDeleteConvMessages] clear messages failed: conversation=%s err=%v", id, err))
		util.ErrorResponse(c, response.OperationFailed, "清空失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) WebChatUpdateMessage(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少消息ID", nil)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		util.ErrorResponse(c, response.InvalidParams, "消息内容不能为空", nil)
		return
	}
	scoped, ok := h.chatSvc.(webChatMessageEditService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide message edit operations", nil)
		return
	}
	msg, err := scoped.UpdateMessageForSpace(id, webChatSpaceID(c), content)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, msg)
}

func (h *Handler) WebChatRegenerate(c *gin.Context) {
	convID := strings.TrimSpace(c.Param("id"))
	if convID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	spaceID := webChatSpaceID(c)
	_, ownerErr := h.requireWebChatConversation(convID, spaceID)
	if ownerErr != nil {
		util.ErrorResponse(c, response.DataNotFound, "会话不存在", nil)
		return
	}

	var userMsg chat.Message
	if err := h.db.Where("conversation_id = ? AND role = ?", convID, "user").Order("sequence DESC").First(&userMsg).Error; err != nil {
		util.ErrorResponse(c, response.DataNotFound, "没有可重新生成的消息", nil)
		return
	}

	var previousAssistants []chat.Message
	if err := h.db.Where("conversation_id = ? AND role = ? AND sequence > ?", convID, "assistant", userMsg.Sequence).Order("sequence ASC").Find(&previousAssistants).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取上一条回复失败", nil)
		return
	}
	if len(previousAssistants) == 0 {
		util.ErrorResponse(c, response.DataNotFound, "没有可重新生成的回复", nil)
		return
	}

	characterID := strings.TrimSpace(userMsg.CharacterID)
	if characterID == "" {
		h.webChatCharacterQuery(spaceID).Select("id").Where("is_active = 1").Limit(1).Row().Scan(&characterID)
	}
	if characterID == "" {
		util.ErrorResponse(c, response.OperationFailed, "请先创建并启用角色", nil)
		return
	}

	requestID := "regenerate-" + uuid.New().String()
	oldRequestID := userMsg.RequestID
	oldStatus := userMsg.Status
	oldUpdatedAt := userMsg.UpdatedAt
	assistantIDs := make([]string, 0, len(previousAssistants))
	assistantInclude := make(map[string]int, len(previousAssistants))
	for _, msg := range previousAssistants {
		assistantIDs = append(assistantIDs, msg.ID)
		assistantInclude[msg.ID] = msg.IncludeInCtx
	}

	restorePreviousState := func() {
		_ = h.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&chat.Message{}).Where("id = ?", userMsg.ID).Updates(map[string]interface{}{
				"request_id": oldRequestID,
				"status":     oldStatus,
				"updated_at": oldUpdatedAt,
			}).Error; err != nil {
				return err
			}
			for id, include := range assistantInclude {
				if err := tx.Model(&chat.Message{}).Where("id = ?", id).Update("include_in_context", include).Error; err != nil {
					return err
				}
			}
			return tx.Where("conversation_id = ? AND request_id = ? AND role = ?", convID, requestID, "assistant").Delete(&chat.Message{}).Error
		})
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&chat.Message{}).Where("id = ?", userMsg.ID).Updates(map[string]interface{}{
			"request_id": requestID,
			"status":     "sent",
			"updated_at": time.Now().Format("2006-01-02 15:04:05"),
		}).Error; err != nil {
			return err
		}
		if len(assistantIDs) > 0 {
			return tx.Model(&chat.Message{}).Where("id IN ?", assistantIDs).Update("include_in_context", 0).Error
		}
		return nil
	}).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "准备重新生成失败", nil)
		return
	}

	imageContext := ""
	if strings.TrimSpace(userMsg.ImageUrl) != "" {
		if visionError := chat.GetBuffer().AnalyzeImage(convID, spaceID, userMsg.ImageUrl); visionError != "" {
			h.publishModelError(modelerror.Event{ModelType: "vision", ConversationID: convID, RequestID: requestID, Channel: "web", RawError: visionError})
		}
		imageContext = chat.GetBuffer().GetImageContexts(convID)
		chat.GetBuffer().ClearImageContexts(convID)
	}
	if strings.TrimSpace(userMsg.VideoUrl) != "" {
		chat.GetBuffer().AnalyzeVideo(convID, spaceID, userMsg.VideoUrl)
	}

	source := strings.TrimSpace(userMsg.Source)
	if source == "" {
		source = "web"
	}
	orchResult, err := h.handleUnifiedEntryWithWorkspace(c.Request.Context(), &interaction.UnifiedEntryRequest{
		ConversationID:   convID,
		Channel:          "web",
		Source:           source,
		RequestID:        requestID,
		SessionID:        convID,
		SpaceID:          spaceID,
		CharacterID:      characterID,
		Message:          userMsg.Content,
		AudioUrl:         userMsg.AudioUrl,
		AudioDuration:    userMsg.AudioDuration,
		VoiceMessage:     strings.TrimSpace(userMsg.AudioUrl) != "",
		ImageUrl:         userMsg.ImageUrl,
		VideoUrl:         userMsg.VideoUrl,
		ImageContext:     imageContext,
		ReplyToMessageID: userMsg.ReplyToMessageID,
	}, h.workspaceBindingForRequest(convID, webChatSendRequest{}, spaceID))
	if err != nil || orchResult == nil || orchResult.Response == nil {
		restorePreviousState()
		if errors.Is(err, interaction.ErrOrchestratorProcessing) {
			util.ErrorResponse(c, response.OperationFailed, "重新生成仍在处理中，请稍后重试", nil)
			return
		}
		if err != nil {
			h.publishTextModelError(convID, requestID, "web", err)
			util.ErrorResponse(c, response.InternalError, err.Error(), nil)
			return
		}
		util.ErrorResponse(c, response.InternalError, "重新生成失败", nil)
		return
	}

	if len(assistantIDs) > 0 {
		if err := h.db.Where("id IN ?", assistantIDs).Delete(&chat.Message{}).Error; err != nil {
			applog.Error(fmt.Sprintf("[WebChatRegenerate] delete previous assistant messages failed: %v", err))
			restorePreviousState()
			util.ErrorResponse(c, response.InternalError, "替换旧回复失败，已恢复原回复，请重试", nil)
			return
		}
	}

	var generatedMessages []chat.Message
	if len(orchResult.Response.MessageIDs) > 0 {
		_ = h.db.Where("id IN ?", orchResult.Response.MessageIDs).Order("sequence ASC").Find(&generatedMessages).Error
	}
	util.SuccessResponse(c, gin.H{
		"conversationId":    orchResult.Response.ConversationID,
		"reply":             orchResult.Response.Reply,
		"messageIds":        orchResult.Response.MessageIDs,
		"assistantMessages": generatedMessages,
		"requestId":         requestID,
	})
}

func (h *Handler) WebChatReplyTimingForce(c *gin.Context) {
	if _, err := h.requireWebChatConversation(c.Param("id"), webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "对话不存在", nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"forced": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingHold(c *gin.Context) {
	if _, err := h.requireWebChatConversation(c.Param("id"), webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "对话不存在", nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"held": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingResume(c *gin.Context) {
	if _, err := h.requireWebChatConversation(c.Param("id"), webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "对话不存在", nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"resumed": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingStatus(c *gin.Context) {
	if _, err := h.requireWebChatConversation(c.Param("id"), webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "对话不存在", nil)
		return
	}
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
	if err := h.webChatOwnedMessageQuery(webChatSpaceID(c)).Select("messages.id, messages.conversation_id, messages.status, messages.request_id, messages.role, messages.created_at, messages.updated_at").Where("messages.id = ?", msgID).Take(&msg).Error; err != nil {
		util.ErrorResponse(c, response.DataNotFound, "消息不存在", nil)
		return
	}

	result := gin.H{"id": msgID, "status": msg.Status, "role": msg.Role, "conversationId": msg.ConversationID, "createdAt": msg.CreatedAt, "updatedAt": msg.UpdatedAt}

	if msg.RequestID != "" {
		var interactionStatus string
		if scanErr := h.db.Table("interaction_records").Select("status").Where("space_id = ? AND request_id = ?", requestidentity.NormalizeSpaceID(webChatSpaceID(c)), msg.RequestID).Limit(1).Row().Scan(&interactionStatus); scanErr == nil && interactionStatus != "" {
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
	spaceID := requestidentity.ResolveGin(c)
	if err := h.requireWebChatConversationOrAbsent(convID, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "对话不存在", nil)
		return
	}
	if err := h.requireWebChatCharacter(body.CharacterID, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "角色不存在", nil)
		return
	}
	peerID := resolveRequestBackedValue(c, body.PeerID, "X-Peer-ID", "peerId", "peer_id")
	source := resolveSource(c, body.Source, "web")
	deviceTimezone := strings.TrimSpace(body.DeviceTimezone)
	if deviceTimezone == "" {
		deviceTimezone = strings.TrimSpace(c.GetHeader("X-Device-Timezone"))
	}
	c.Header("X-Request-ID", requestID)
	c.Header("X-Session-ID", sessionID)
	c.Header("X-Source", source)

	applog.Info(fmt.Sprintf("[Webhook] ImageUrl=%s VideoUrl=%s", body.ImageUrl[:min(len(body.ImageUrl), 60)], body.VideoUrl[:min(len(body.VideoUrl), 60)]))
	visionError := chat.GetBuffer().AnalyzeImage(convID, spaceID, body.ImageUrl)
	if visionError != "" {
		h.publishModelError(modelerror.Event{ModelType: "vision", ConversationID: convID, RequestID: requestID, Channel: "web", RawError: visionError})
	}
	chat.GetBuffer().AnalyzeVideo(convID, spaceID, body.VideoUrl)

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
	if characterID == "" {
		h.webChatCharacterQuery(spaceID).Select("id").Where("is_active = 1").Limit(1).Row().Scan(&characterID)
	}
	if characterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "请先创建并启用角色", nil)
		return
	}

	workspaceBinding := h.workspaceBindingForRequest(convID, body, spaceID)
	orchResult, err := h.handleUnifiedEntryWithWorkspace(c.Request.Context(), &interaction.UnifiedEntryRequest{
		ConversationID: convID, Channel: "web", Source: source,
		SpaceID: spaceID, PeerID: peerID, RequestID: requestID, SessionID: sessionID,
		DeviceTimezone: deviceTimezone,
		CharacterID:    characterID, Message: mergedContent,
		AudioUrl: body.AudioUrl, AudioDuration: body.AudioDuration,
		VoiceMessage:     body.VoiceMessage,
		ImageUrl:         body.ImageUrl,
		VideoUrl:         body.VideoUrl,
		ImageContext:     imageCtx,
		ReplyToMessageID: body.ReplyToMessageID,
	}, workspaceBinding)
	if errors.Is(err, interaction.ErrOrchestratorProcessing) {
		util.SuccessResponse(c, gin.H{"status": "processing", "requestId": requestID, "sessionId": sessionID, "spaceId": spaceID, "source": source})
		return
	}
	if err != nil {
		h.publishTextModelError(convID, requestID, "web", err)
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"conversationId": orchResult.Response.ConversationID, "reply": orchResult.Response.Reply, "reasoning": orchResult.Response.Reasoning, "messageIds": orchResult.Response.MessageIDs, "characterName": orchResult.Response.CharacterName, "requestId": requestID, "sessionId": sessionID, "spaceId": spaceID, "source": source})
}

func (h *Handler) WebChatSubmitMessage(c *gin.Context) {
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
	clientMessageID := strings.TrimSpace(body.ClientMessageID)
	if clientMessageID == "" {
		clientMessageID = requestID
	}
	sessionID := resolveRequestBackedValue(c, body.SessionID, "X-Session-ID", "sessionId", "session_id")
	if sessionID == "" {
		sessionID = convID
	}
	spaceID := requestidentity.ResolveGin(c)
	if err := h.requireWebChatConversationOrAbsent(convID, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "对话不存在", nil)
		return
	}
	if err := h.requireWebChatCharacter(body.CharacterID, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "角色不存在", nil)
		return
	}
	peerID := resolveRequestBackedValue(c, body.PeerID, "X-Peer-ID", "peerId", "peer_id")
	source := resolveSource(c, body.Source, "web")
	deviceTimezone := strings.TrimSpace(body.DeviceTimezone)
	if deviceTimezone == "" {
		deviceTimezone = strings.TrimSpace(c.GetHeader("X-Device-Timezone"))
	}

	characterID := body.CharacterID
	if characterID == "" {
		h.webChatCharacterQuery(spaceID).Select("id").Where("is_active = 1").Limit(1).Row().Scan(&characterID)
	}
	if characterID == "" {
		util.ErrorResponse(c, response.InvalidParams, "请先创建并启用角色", nil)
		return
	}

	var replyToRole *string
	var replyToExcerpt *string
	if body.ReplyToMessageID != nil && *body.ReplyToMessageID != "" {
		var targetMsg chat.Message
		if err := h.webChatOwnedMessageQuery(spaceID).Where("messages.id = ? AND messages.conversation_id = ?", *body.ReplyToMessageID, convID).First(&targetMsg).Error; err == nil {
			role := targetMsg.Role
			excerpt := chat.BuildMessageExcerpt(&targetMsg)
			replyToRole = &role
			replyToExcerpt = &excerpt
		}
	}

	userMsg, err := h.persistQueuedWebChatMessage(body, convID, characterID, source, requestID, msgContent, spaceID, replyToRole, replyToExcerpt)
	if err != nil {
		applog.Error(fmt.Sprintf("[WebChatSubmitMessage] persist user message failed: %v", err))
		util.ErrorResponse(c, response.InternalError, "消息存储失败", nil)
		return
	}
	msgID := userMsg.ID
	// The queued-message transaction has created the conversation at this point,
	// so persist the workspace binding synchronously before generation starts.
	// This makes the first turn durable even if the client disconnects immediately
	// after receiving the submit acknowledgement.
	workspaceBinding := h.workspaceBindingForRequest(convID, body, spaceID)

	c.Header("X-Request-ID", requestID)
	genID := chat.GetGenerationQueue().StartCollection(convID)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				applog.Error(fmt.Sprintf("[WebChatSubmitMessage] panic recovered: %v\n%s", r, debug.Stack()))
				h.db.Exec("UPDATE messages SET status = 'failed', updated_at = ? WHERE id = ?", time.Now().Format("2006-01-02 15:04:05"), msgID)
			}
		}()
		visionError := chat.GetBuffer().AnalyzeImage(convID, spaceID, body.ImageUrl)
		if visionError != "" {
			h.publishModelError(modelerror.Event{ModelType: "vision", ConversationID: convID, RequestID: requestID, Channel: "web", RawError: visionError})
		}
		chat.GetBuffer().AnalyzeVideo(convID, spaceID, body.VideoUrl)

		bufferedMsgs, bufErr := chat.GetBuffer().Buffer(convID, msgContent)
		if bufErr != nil {
			applog.Info(fmt.Sprintf("[WebChatSubmitMessage] buffer aborted for %s", convID))
			return
		}

		mergedContent := strings.Join(bufferedMsgs, "\n")
		imageCtx := chat.GetBuffer().GetImageContexts(convID)
		chat.GetBuffer().ClearImageContexts(convID)

		genCtx, genCancel, err := chat.GetGenerationQueue().AcquireSlot(context.Background(), convID, genID)
		if err != nil {
			applog.Info(fmt.Sprintf("[WebChatSubmitMessage] generation slot cancelled for %s: %v", convID, err))
			return
		}
		defer genCancel()
		defer func() {
			chat.GetGenerationQueue().FinishProcessing(convID)
		}()

		if genCtx.Err() != nil {
			applog.Info(fmt.Sprintf("[WebChatSubmitMessage] generation cancelled before LLM call for %s", convID))
			return
		}

		orchResult, err := h.handleUnifiedEntryWithWorkspace(genCtx, &interaction.UnifiedEntryRequest{
			ConversationID: convID, Channel: "web", Source: source,
			SpaceID: spaceID, PeerID: peerID, RequestID: requestID, SessionID: sessionID,
			DeviceTimezone: deviceTimezone,
			CharacterID:    characterID, Message: mergedContent,
			AudioUrl: body.AudioUrl, AudioDuration: body.AudioDuration,
			VoiceMessage:     body.VoiceMessage,
			ImageUrl:         body.ImageUrl,
			VideoUrl:         body.VideoUrl,
			ImageContext:     imageCtx,
			ReplyToMessageID: body.ReplyToMessageID,
		}, workspaceBinding)
		if err != nil {
			applog.Warn(fmt.Sprintf("[WebChatSubmitMessage] generation failed: %v", err))
			h.publishTextModelError(convID, requestID, "web", err)
			h.db.Exec("UPDATE messages SET status = 'failed', updated_at = ? WHERE id = ?", time.Now().Format("2006-01-02 15:04:05"), msgID)
		} else if orchResult != nil && orchResult.Response != nil {
			applog.Info(fmt.Sprintf("[WebChatSubmitMessage] generation completed for %s, assistant count=%d", convID, len(orchResult.Response.MessageIDs)))
		}
	}()

	util.SuccessResponse(c, gin.H{
		"conversationId":  convID,
		"userMessageId":   msgID,
		"clientMessageId": clientMessageID,
		"requestId":       requestID,
		"status":          "queued",
		"mergeWindowMs":   config.AppCfg.Chat.MergeWindowMs,
	})
}

func (h *Handler) persistQueuedWebChatMessage(body webChatSendRequest, convID, characterID, source, requestID, msgContent, spaceID string, replyToRole, replyToExcerpt *string) (*chat.Message, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	msg := &chat.Message{
		ID:               uuid.New().String(),
		ConversationID:   convID,
		CharacterID:      characterID,
		Role:             "user",
		Content:          msgContent,
		MsgType:          "text",
		Source:           source,
		Status:           "queued",
		AudioUrl:         body.AudioUrl,
		AudioDuration:    body.AudioDuration,
		ImageUrl:         body.ImageUrl,
		VideoUrl:         body.VideoUrl,
		RequestID:        requestID,
		ReplyToMessageID: body.ReplyToMessageID,
		ReplyToRole:      replyToRole,
		ReplyToExcerpt:   replyToExcerpt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var existingConv chat.Conversation
		lookup := webChatOwnerQuery(tx.Model(&chat.Conversation{}).Where("id = ? AND deleted_at IS NULL", convID), spaceID).Limit(1).Find(&existingConv)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected == 0 {
			var foreignCount int64
			if err := tx.Model(&chat.Conversation{}).Where("id = ? AND deleted_at IS NULL", convID).Count(&foreignCount).Error; err != nil {
				return err
			}
			if foreignCount > 0 {
				return gorm.ErrRecordNotFound
			}
			conv := &chat.Conversation{ID: convID, SpaceID: requestidentity.NormalizeSpaceID(spaceID), ProjectID: strings.TrimSpace(body.ProjectID), Title: msgContent, Channel: "web", Source: source, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(conv).Error; err != nil {
				return err
			}
		} else if err := webChatOwnerQuery(tx.Model(&chat.Conversation{}).Where("id = ?", convID), spaceID).Update("updated_at", now).Error; err != nil {
			return err
		}
		var existing chat.Message
		result := tx.Where("conversation_id = ? AND request_id = ? AND role = ?", convID, requestID, "user").Order("sequence ASC").Limit(1).Find(&existing)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			msg = &existing
			return nil
		}
		return tx.Create(msg).Error
	})
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (h *Handler) publishModelError(event modelerror.Event) {
	if strings.TrimSpace(event.ConversationID) == "" || strings.TrimSpace(event.RawError) == "" {
		return
	}
	var userMessage struct {
		ID       string
		Sequence int64
	}
	requestID := strings.TrimSpace(event.RequestID)
	if h.db != nil && requestID != "" {
		h.db.Model(&chat.Message{}).
			Select("id, sequence").
			Where("conversation_id = ? AND request_id = ? AND role = ?", event.ConversationID, requestID, "user").
			Order("sequence ASC").
			Limit(1).
			Scan(&userMessage)
	}
	labels := map[string]string{
		"vision": "图片识别模型",
		"text":   "文本模型",
		"voice":  "语音模型",
		"vector": "向量模型",
	}
	label := labels[event.ModelType]
	if label == "" {
		label = "模型"
	}
	channel := strings.TrimSpace(event.Channel)
	if channel == "" {
		channel = "web"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	content := label + "调用失败\n\n原始错误：" + event.RawError
	GetMessageEventBus().Publish(MessageEvent{
		Type:           EventMessageCreated,
		ConversationID: event.ConversationID,
		MessageID:      event.ModelType + "-error-" + uuid.New().String(),
		Channel:        channel,
		Direction:      "outbound",
		Role:           "assistant",
		Content:        content,
		CreatedAt:      now,
		Status:         "failed",
		Data: map[string]interface{}{
			"messageType":         event.ModelType + "_error",
			"modelType":           event.ModelType,
			"rawError":            event.RawError,
			"requestId":           requestID,
			"userMessageId":       userMessage.ID,
			"userMessageSequence": userMessage.Sequence,
		},
	})
}

func (h *Handler) publishTextModelError(convID, requestID, channel string, err error) {
	var textModelError *chat.TextModelCallError
	if errors.As(err, &textModelError) {
		h.publishModelError(modelerror.Event{ModelType: "text", ConversationID: convID, RequestID: requestID, Channel: channel, RawError: textModelError.RawError})
	}
}

func (h *Handler) WebChatGenerationStatus(c *gin.Context) {
	convID := c.Param("id")
	if convID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	if _, err := h.requireWebChatConversation(convID, webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "会话不存在", nil)
		return
	}
	util.SuccessResponse(c, gin.H{
		"conversationId": convID,
		"status":         chat.GetGenerationQueue().GetStatus(convID),
	})
}

func (h *Handler) WebChatCancelGeneration(c *gin.Context) {
	convID := c.Param("id")
	if convID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	if _, err := h.requireWebChatConversation(convID, webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "会话不存在", nil)
		return
	}
	chat.GetGenerationQueue().Cancel(convID)
	util.SuccessResponse(c, gin.H{"cancelled": true, "conversationId": convID})
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
