// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agentpermission"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/modelerror"
	"github.com/u-ai/backend/internal/requestidentity"
	applog "github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

var webChatRequestLocks [128]sync.Mutex

func webChatRequestLock(spaceID, requestID string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(spaceID)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.TrimSpace(requestID)))
	return &webChatRequestLocks[int(h.Sum32())%len(webChatRequestLocks)]
}

type webChatSendRequest struct {
	ConversationID    string  `json:"conversationId"`
	ProjectID         string  `json:"projectId"`
	WorkspaceID       string  `json:"workspaceId"`
	WorkspaceDeviceID string  `json:"workspaceDeviceId"`
	Content           string  `json:"content"`
	CharacterID       string  `json:"characterId"`
	PeerID            string  `json:"peerId"`
	RequestID         string  `json:"requestId"`
	SessionID         string  `json:"sessionId"`
	Source            string  `json:"source"`
	ClientMessageID   string  `json:"clientMessageId"`
	DeviceTimezone    string  `json:"deviceTimezone"`
	VoiceMessage      bool    `json:"voiceMessage"`
	AudioUrl          string  `json:"audioUrl"`
	AudioDuration     float64 `json:"audioDuration"`
	ImageUrl          string  `json:"imageUrl"`
	VideoUrl          string  `json:"videoUrl"`
	ReplyToMessageID  *string `json:"replyToMessageId,omitempty"`
	ModelConfigID     int     `json:"modelConfigId"`
	ReasoningEffort   string  `json:"reasoningEffort"`
	ReasoningEnabled  *bool   `json:"reasoningEnabled"`
	PermissionMode    string  `json:"permissionMode"`
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

func (h *Handler) WebChatGetConv(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	var conversation chat.Conversation
	if err := h.webChatOwnedConversationQuery(webChatSpaceID(c)).Where("id = ?", id).First(&conversation).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	util.SuccessResponse(c, conversation)
}

func (h *Handler) WebChatListAssistantTurns(c *gin.Context) {
	convID := strings.TrimSpace(c.Param("id"))
	if convID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	if _, err := h.requireWebChatConversation(convID, webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	before, _ := strconv.ParseInt(strings.TrimSpace(c.Query("before")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "50")))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := h.db.WithContext(c.Request.Context()).Model(&chat.AssistantTurn{}).Where("conversation_id = ?", convID)
	if before > 0 {
		query = query.Where("sequence < ?", before)
	}
	turns := make([]chat.AssistantTurn, 0)
	if err := query.Order("sequence DESC").Limit(limit + 1).Find(&turns).Error; err != nil {
		util.ErrorResponse(c, response.OperationFailed, "读取发言轮次失败", nil)
		return
	}
	hasMore := len(turns) > limit
	if hasMore {
		turns = turns[:limit]
	}
	for left, right := 0, len(turns)-1; left < right; left, right = left+1, right-1 {
		turns[left], turns[right] = turns[right], turns[left]
	}
	for index := range turns {
		turns[index].Items = make([]chat.AssistantTurnItem, 0)
		if err := h.db.WithContext(c.Request.Context()).Where("turn_id = ?", turns[index].ID).Order("sequence ASC").Find(&turns[index].Items).Error; err != nil {
			util.ErrorResponse(c, response.OperationFailed, "读取发言内容块失败", nil)
			return
		}
	}
	nextBefore := int64(0)
	if len(turns) > 0 {
		nextBefore = turns[0].Sequence
	}
	util.SuccessResponse(c, gin.H{"items": turns, "nextBefore": nextBefore, "hasMore": hasMore})
}

func (h *Handler) WebChatGetMessages(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	beforeSequence, _ := strconv.ParseInt(strings.TrimSpace(c.Query("beforeSequence")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "50")))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
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
	msgs, hasMore, err := scoped.GetMessagesBeforeForSpace(id, webChatSpaceID(c), beforeSequence, limit)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "查询失败", nil)
		return
	}
	nextBefore := int64(0)
	if len(msgs) > 0 {
		nextBefore = msgs[0].Sequence
	}
	util.SuccessResponse(c, gin.H{"items": msgs, "nextBefore": nextBefore, "hasMore": hasMore})
}

func (h *Handler) WebChatCreateRealtimeConversation(c *gin.Context) {
	var body struct {
		ProjectID string `json:"projectId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	scoped, ok := h.chatSvc.(webChatScopedService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide user-scoped operations", nil)
		return
	}
	conv, err := scoped.CreateConversationForSpace(&chat.CreateConversationRequest{
		ProjectID: strings.TrimSpace(body.ProjectID),
		Title:     "新对话",
		Channel:   "web",
		Source:    "realtime_call",
	}, webChatSpaceID(c))
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
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
		Title            *string `json:"title"`
		ProjectID        *string `json:"projectId"`
		Pinned           *bool   `json:"pinned"`
		Archived         *bool   `json:"archived"`
		ModelConfigID    *int    `json:"modelConfigId"`
		ReasoningEffort  *string `json:"reasoningEffort"`
		ReasoningEnabled *bool   `json:"reasoningEnabled"`
		PermissionMode   *string `json:"permissionMode"`
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
	conversationUpdates := map[string]interface{}{}
	if body.ModelConfigID != nil {
		if *body.ModelConfigID > 0 {
			var exists int64
			if err := h.db.Table("model_configs").Where("id = ?", *body.ModelConfigID).Count(&exists).Error; err != nil || exists == 0 {
				util.ErrorResponse(c, response.InvalidParams, "模型配置不存在", nil)
				return
			}
		}
		conversationUpdates["model_config_id"] = *body.ModelConfigID
	}
	if body.ReasoningEffort != nil {
		effort := chat.NormalizeReasoningEffort(*body.ReasoningEffort)
		if effort == "" {
			util.ErrorResponse(c, response.InvalidParams, "思考强度无效", nil)
			return
		}
		conversationUpdates["reasoning_effort"] = effort
	}
	if body.ReasoningEnabled != nil {
		value := 0
		if *body.ReasoningEnabled {
			value = 1
		}
		conversationUpdates["reasoning_enabled"] = value
	}
	if body.PermissionMode != nil {
		mode := strings.TrimSpace(*body.PermissionMode)
		if !agentpermission.Valid(mode) {
			util.ErrorResponse(c, response.InvalidParams, "权限模式无效", nil)
			return
		}
		conversationUpdates["permission_mode"] = agentpermission.Normalize(mode)
	}
	if len(conversationUpdates) > 0 {
		if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ?", id).Updates(conversationUpdates).Error; err != nil {
			util.ErrorResponse(c, response.OperationFailed, "更新模型设置失败", nil)
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

func (h *Handler) WebChatSubmitMessage(c *gin.Context) {
	var body webChatSendRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	msgContent := strings.TrimSpace(body.Content)
	if msgContent == "" && strings.TrimSpace(body.ImageUrl) == "" && strings.TrimSpace(body.AudioUrl) == "" && strings.TrimSpace(body.VideoUrl) == "" {
		util.ErrorResponse(c, response.InvalidParams, "消息不能为空", nil)
		return
	}

	requestID := strings.TrimSpace(body.RequestID)
	if requestID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少 requestId", nil)
		return
	}
	spaceID := requestidentity.ResolveGin(c)
	requestLock := webChatRequestLock(spaceID, requestID)
	requestLock.Lock()
	defer requestLock.Unlock()

	convID := strings.TrimSpace(body.ConversationID)
	if convID == "" {
		existingConversationID, lookupErr := h.findWebChatConversationByRequest(spaceID, requestID)
		if lookupErr != nil {
			util.ErrorResponse(c, response.InternalError, "请求幂等检查失败", nil)
			return
		}
		if existingConversationID != "" {
			convID = existingConversationID
		} else {
			convID = "web-" + uuid.New().String()[:8]
		}
	}
	clientMessageID := strings.TrimSpace(body.ClientMessageID)
	if clientMessageID == "" {
		clientMessageID = requestID
	}
	sessionID := resolveRequestBackedValue(c, body.SessionID, "X-Session-ID", "sessionId", "session_id")
	if sessionID == "" {
		sessionID = convID
	}
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

	userMsg, queuedTurn, createdTurn, err := h.persistQueuedWebChatMessage(body, convID, characterID, source, requestID, msgContent, spaceID, replyToRole, replyToExcerpt)
	if err != nil {
		applog.Error(fmt.Sprintf("[WebChatSubmitMessage] persist user message failed: %v", err))
		util.ErrorResponse(c, response.InternalError, "消息存储失败", nil)
		return
	}
	msgID := userMsg.ID
	if !createdTurn {
		c.Header("X-Request-ID", requestID)
		util.SuccessResponse(c, gin.H{"conversationId": convID, "userMessageId": msgID, "clientMessageId": clientMessageID, "requestId": requestID, "turnId": queuedTurn.ID, "executionId": queuedTurn.ExecutionID, "status": queuedTurn.Status})
		return
	}
	workspaceBinding := h.workspaceBindingForRequest(convID, body, spaceID)

	if _, err := conversationstream.DefaultManager().Publish(c.Request.Context(), conversationstream.AgentUIEvent{
		ConversationID: convID,
		RequestID:      requestID,
		ExecutionID:    queuedTurn.ExecutionID,
		TurnID:         queuedTurn.ID,
		TurnSequence:   queuedTurn.Sequence,
		Type:           "turn.queued",
		Status:         "queued",
	}, true); err != nil {
		applog.Error(fmt.Sprintf("[WebChatSubmitMessage] publish queued turn failed: %v", err))
		h.finalizeWebChatTurnRuntime(queuedTurn, msgID, "failed", "event_persist_failed", true, "Turn 事件持久化失败")
		util.ErrorResponse(c, response.InternalError, "消息运行时初始化失败", nil)
		return
	}

	c.Header("X-Request-ID", requestID)
	genCtx, genCancel, executionStarted := conversationstream.DefaultManager().BeginExecution(convID, queuedTurn.ID)
	if !executionStarted {
		h.finalizeWebChatTurnRuntime(queuedTurn, msgID, "failed", "conversation_busy", true, "当前会话已有正在执行的 Turn，请使用中途干预或先停止当前 Turn")
		util.SuccessResponse(c, gin.H{
			"conversationId":  convID,
			"userMessageId":   msgID,
			"clientMessageId": clientMessageID,
			"requestId":       requestID,
			"turnId":          queuedTurn.ID,
			"executionId":     queuedTurn.ExecutionID,
			"status":          "failed",
			"errorCode":       "conversation_busy",
		})
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				applog.Error(fmt.Sprintf("[WebChatSubmitMessage] panic recovered: %v\n%s", r, debug.Stack()))
				h.finalizeWebChatTurnRuntime(queuedTurn, msgID, "failed", "runtime_panic", true, "Agent 运行时异常中止")
			}
		}()
		imageContext, visionError := chat.AnalyzeImageContext(spaceID, body.ImageUrl)
		if visionError != "" {
			h.publishModelError(modelerror.Event{ModelType: "vision", ConversationID: convID, RequestID: requestID, Channel: "web", RawError: visionError})
		}
		videoContext, videoError := chat.AnalyzeVideoContext(spaceID, body.VideoUrl)
		if videoError != "" {
			h.publishModelError(modelerror.Event{ModelType: "vision", ConversationID: convID, RequestID: requestID, Channel: "web", RawError: videoError})
		}
		mediaContexts := make([]string, 0, 2)
		if imageContext != "" {
			mediaContexts = append(mediaContexts, imageContext)
		}
		if videoContext != "" {
			mediaContexts = append(mediaContexts, videoContext)
		}
		imageCtx := strings.Join(mediaContexts, "\n")

		defer genCancel()
		defer conversationstream.DefaultManager().ClearExecution(convID, queuedTurn.ID)

		if genCtx.Err() != nil {
			applog.Info(fmt.Sprintf("[WebChatSubmitMessage] generation cancelled before LLM call for %s", convID))
			h.finalizeWebChatTurnRuntime(queuedTurn, msgID, "interrupted", "interrupted", false, "已停止生成")
			return
		}

		orchResult, err := h.handleUnifiedEntryWithWorkspace(genCtx, &interaction.UnifiedEntryRequest{
			ConversationID: convID, Channel: "web", Source: source,
			SpaceID: spaceID, PeerID: peerID, RequestID: requestID, SessionID: sessionID,
			DeviceTimezone: deviceTimezone,
			CharacterID:    characterID, Message: msgContent,
			AudioUrl: body.AudioUrl, AudioDuration: body.AudioDuration,
			VoiceMessage:     body.VoiceMessage,
			ImageUrl:         body.ImageUrl,
			VideoUrl:         body.VideoUrl,
			ImageContext:     imageCtx,
			ReplyToMessageID: body.ReplyToMessageID,
			ModelConfigID:    body.ModelConfigID,
			ReasoningEffort:  body.ReasoningEffort,
			ReasoningEnabled: body.ReasoningEnabled,
			PermissionMode:   body.PermissionMode,
			TurnID:           queuedTurn.ID,
			ExecutionID:      queuedTurn.ExecutionID,
		}, workspaceBinding)
		if err != nil {
			applog.Warn(fmt.Sprintf("[WebChatSubmitMessage] generation failed: %v", err))
			if genCtx.Err() != nil || errors.Is(err, context.Canceled) {
				h.finalizeWebChatTurnRuntime(queuedTurn, msgID, "interrupted", "interrupted", false, "已停止生成")
			} else {
				h.finalizeWebChatTurnRuntime(queuedTurn, msgID, "failed", "generation_failed", true, "Agent 执行失败")
			}
		} else if orchResult != nil && orchResult.Response != nil {
			applog.Info(fmt.Sprintf("[WebChatSubmitMessage] generation completed for %s, assistant count=%d", convID, len(orchResult.Response.MessageIDs)))
		}
	}()

	util.SuccessResponse(c, gin.H{
		"conversationId":  convID,
		"userMessageId":   msgID,
		"clientMessageId": clientMessageID,
		"requestId":       requestID,
		"turnId":          queuedTurn.ID,
		"executionId":     queuedTurn.ExecutionID,
		"status":          "queued",
	})
}

func (h *Handler) persistQueuedWebChatMessage(body webChatSendRequest, convID, characterID, source, requestID, msgContent, spaceID string, replyToRole, replyToExcerpt *string) (*chat.Message, *chat.AssistantTurn, bool, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	msg := &chat.Message{ID: uuid.New().String(), ConversationID: convID, CharacterID: characterID, Role: "user", Content: msgContent, MsgType: "text", Source: source, Status: "queued", AudioUrl: body.AudioUrl, AudioDuration: body.AudioDuration, ImageUrl: body.ImageUrl, VideoUrl: body.VideoUrl, RequestID: requestID, ReplyToMessageID: body.ReplyToMessageID, ReplyToRole: replyToRole, ReplyToExcerpt: replyToExcerpt, CreatedAt: now, UpdatedAt: now}
	turn := &chat.AssistantTurn{}
	createdTurn := false
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
			projectID := strings.TrimSpace(body.ProjectID)
			workspaceID := ""
			workspaceDeviceID := ""
			if projectID != "" {
				var project chat.Project
				projectQuery := webChatOwnerQuery(tx.Model(&chat.Project{}).Where("id = ?", projectID), spaceID)
				if err := projectQuery.First(&project).Error; err != nil {
					return err
				}
				var mount struct {
					Enabled int
				}
				if err := tx.Table("workspace_mounts").Select("enabled").Where("id = ?", project.WorkspaceID).Take(&mount).Error; err != nil {
					return err
				}
				if mount.Enabled == 0 {
					return errors.New("project workspace is disabled")
				}
			} else if candidate := strings.TrimSpace(body.WorkspaceID); candidate != "" {
				var mount struct {
					ID      string
					Enabled int
				}
				if err := tx.Table("workspace_mounts").Select("id, enabled").Where("id = ?", candidate).Take(&mount).Error; err != nil {
					return err
				}
				if mount.Enabled == 0 {
					return errors.New("workspace is disabled")
				}
				workspaceID = mount.ID
				workspaceDeviceID = strings.TrimSpace(body.WorkspaceDeviceID)
			}
			conv := &chat.Conversation{ID: convID, SpaceID: requestidentity.NormalizeSpaceID(spaceID), ProjectID: projectID, WorkspaceID: workspaceID, WorkspaceDeviceID: workspaceDeviceID, Title: msgContent, Channel: "web", Source: source, CreatedAt: now, UpdatedAt: now}
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
		} else if err := tx.Create(msg).Error; err != nil {
			return err
		}
		var existingTurn chat.AssistantTurn
		turnResult := tx.Where("conversation_id = ? AND request_id = ?", convID, requestID).Order("sequence ASC").Limit(1).Find(&existingTurn)
		if turnResult.Error != nil {
			return turnResult.Error
		}
		if turnResult.RowsAffected > 0 {
			turn = &existingTurn
			return nil
		}
		var turnSequence int64
		if err := tx.Model(&chat.AssistantTurn{}).Where("conversation_id = ?", convID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&turnSequence).Error; err != nil {
			return err
		}
		turn = &chat.AssistantTurn{ID: uuid.New().String(), ConversationID: convID, CharacterID: characterID, UserMessageID: msg.ID, RequestID: requestID, ExecutionID: uuid.New().String(), Sequence: turnSequence, Status: "queued", CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(turn).Error; err != nil {
			return err
		}
		createdTurn = true
		return nil
	})
	if err != nil {
		return nil, nil, false, err
	}
	return msg, turn, createdTurn, nil
}

func (h *Handler) finalizeWebChatTurnRuntime(turn *chat.AssistantTurn, userMessageID, status, errorCode string, retryable bool, userMessage string) {
	if h == nil || h.db == nil || turn == nil || strings.TrimSpace(turn.ID) == "" {
		return
	}
	status = strings.TrimSpace(status)
	if status != "failed" && status != "interrupted" {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updated := false
	err := h.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&chat.AssistantTurn{}).
			Where("id = ? AND status NOT IN ?", turn.ID, []string{"completed", "failed", "interrupted"}).
			Updates(map[string]any{"status": status, "updated_at": now, "completed_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		updated = true
		return tx.Model(&chat.AssistantTurnItem{}).
			Where("turn_id = ? AND status NOT IN ?", turn.ID, []string{"completed", "failed", "interrupted"}).
			Updates(map[string]any{"status": status, "revision": gorm.Expr("revision + 1"), "updated_at": now}).Error
	})
	if err != nil || !updated {
		return
	}
	if strings.TrimSpace(userMessageID) != "" {
		_ = h.db.Model(&chat.Message{}).Where("id = ?", userMessageID).Updates(map[string]any{"status": "sent", "updated_at": now}).Error
	}
	eventType := "turn.failed"
	if status == "interrupted" {
		eventType = "turn.interrupted"
	}
	payload := map[string]any{
		"errorCode":          strings.TrimSpace(errorCode),
		"errorType":          "runtime",
		"retryable":          retryable,
		"userMessage":        strings.TrimSpace(userMessage),
		"recoveryCheckpoint": true,
	}
	_, _ = conversationstream.DefaultManager().Publish(context.Background(), conversationstream.AgentUIEvent{
		ConversationID: turn.ConversationID,
		RequestID:      turn.RequestID,
		ExecutionID:    turn.ExecutionID,
		TurnID:         turn.ID,
		TurnSequence:   turn.Sequence,
		Type:           eventType,
		Status:         status,
		Payload:        payload,
	}, true)
}

func (h *Handler) publishModelError(event modelerror.Event) {
	if h == nil || h.db == nil || strings.TrimSpace(event.ConversationID) == "" || strings.TrimSpace(event.RawError) == "" {
		return
	}
	var turn chat.AssistantTurn
	query := h.db.Model(&chat.AssistantTurn{}).Where("conversation_id = ?", strings.TrimSpace(event.ConversationID))
	if requestID := strings.TrimSpace(event.RequestID); requestID != "" {
		query = query.Where("request_id = ?", requestID)
	}
	if err := query.Order("sequence DESC").First(&turn).Error; err != nil {
		return
	}
	modelType := strings.TrimSpace(event.ModelType)
	if modelType == "" {
		modelType = "model"
	}
	_ = chat.PersistAssistantTurnError(
		context.Background(), h.db, turn, modelType+"_model_error", modelType,
		modelType+" 模型调用失败", event.RawError, modelType, true,
	)
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
