package system

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type steerTurnRequest struct {
	Content string `json:"content"`
}

func (h *Handler) WebChatConversationSnapshot(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Param("id"))
	if conversationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	conversation, err := h.requireWebChatConversation(conversationID, webChatSpaceID(c))
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	ctx := c.Request.Context()
	turns := make([]chat.AssistantTurn, 0, 51)
	if err := h.db.WithContext(ctx).Where("conversation_id = ?", conversationID).Order("sequence DESC").Limit(51).Find(&turns).Error; err != nil {
		util.ErrorResponse(c, response.OperationFailed, "读取会话快照失败", nil)
		return
	}
	hasMoreTurns := len(turns) > 50
	if hasMoreTurns {
		turns = turns[:50]
	}
	for left, right := 0, len(turns)-1; left < right; left, right = left+1, right-1 {
		turns[left], turns[right] = turns[right], turns[left]
	}
	runtime := conversationstream.DefaultManager().RuntimeSnapshot(conversationID)
	for index := range turns {
		if isRecoverableTurnStatus(turns[index].Status) && runtime.ActiveTurn == nil && !conversationstream.DefaultManager().HasExecution(conversationID, turns[index].ID) {
			h.finalizeWebChatTurnRuntime(&turns[index], turns[index].UserMessageID, "interrupted", "runtime_restarted", true, "运行时已重启，可重试该 Turn")
			now := time.Now().UTC().Format(time.RFC3339Nano)
			turns[index].Status = "interrupted"
			turns[index].UpdatedAt = now
			turns[index].CompletedAt = now
		}
		turns[index].Items = make([]chat.AssistantTurnItem, 0)
		if err := h.db.WithContext(ctx).Where("turn_id = ?", turns[index].ID).Order("sequence ASC").Find(&turns[index].Items).Error; err != nil {
			util.ErrorResponse(c, response.OperationFailed, "读取会话内容块失败", nil)
			return
		}
	}
	messages := make([]chat.Message, 0, 51)
	if err := h.db.WithContext(ctx).Where("conversation_id = ? AND deleted_at IS NULL", conversationID).Order("sequence DESC").Limit(51).Find(&messages).Error; err != nil {
		util.ErrorResponse(c, response.OperationFailed, "读取会话消息失败", nil)
		return
	}
	hasMoreMessages := len(messages) > 50
	if hasMoreMessages {
		messages = messages[:50]
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	messageBefore := int64(0)
	if len(messages) > 0 {
		messageBefore = messages[0].Sequence
	}
	turnBefore := int64(0)
	if len(turns) > 0 {
		turnBefore = turns[0].Sequence
	}
	lastSequence, err := conversationstream.DefaultManager().LatestSequence(ctx, conversationID)
	if err != nil {
		lastSequence = runtime.LastEventSequence
	}
	if runtime.LastEventSequence > lastSequence {
		lastSequence = runtime.LastEventSequence
	}
	revision := int64(0)
	if len(turns) > 0 {
		revision = turns[len(turns)-1].Sequence
	}
	workspace := h.workspaceBindingForConversation(*conversation, webChatSpaceID(c))
	approvals := []any{}
	if h.approvalBroker != nil {
		items := h.approvalBroker.List(webChatSpaceID(c), conversationID)
		approvals = make([]any, 0, len(items))
		for _, item := range items {
			approvals = append(approvals, item)
		}
	}
	util.SuccessResponse(c, gin.H{
		"version": conversationstream.ProtocolVersion, "revision": revision, "lastEventSequence": lastSequence,
		"conversation": conversation, "workspace": workspace, "messages": messages, "turns": turns,
		"messageHistory": gin.H{"nextBefore": messageBefore, "hasMore": hasMoreMessages},
		"turnHistory":    gin.H{"nextBefore": turnBefore, "hasMore": hasMoreTurns},
		"activeTurn":     runtime.ActiveTurn, "approvals": approvals,
	})
}

func (h *Handler) WebChatConversationEvents(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Param("id"))
	if conversationID == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	if _, err := h.requireWebChatConversation(conversationID, webChatSpaceID(c)); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	afterSequence := parseEventSequence(c)
	subscriberID := strings.TrimSpace(c.Query("subscriberId"))
	if subscriberID == "" {
		subscriberID = strings.TrimSpace(c.Query("clientId"))
	}
	if subscriberID == "" {
		subscriberID = uuid.NewString()
	}
	live, replay, covered, cancel := conversationstream.DefaultManager().Subscribe(conversationID, subscriberID, afterSequence)
	defer cancel()
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	cursor := afterSequence
	if afterSequence > 0 && !covered {
		durable, err := conversationstream.DefaultManager().ListDurableAfter(c.Request.Context(), conversationID, afterSequence, 5000)
		if err != nil || !eventsCanRecover(afterSequence, durable) {
			h.writeSnapshotRequired(c, conversationID, afterSequence, "event_gap")
			return
		}
		for _, event := range durable {
			if err := writeAgentUIEvent(c, event); err != nil {
				return
			}
			cursor = event.EventSequence
		}
		latest, err := conversationstream.DefaultManager().LatestSequence(c.Request.Context(), conversationID)
		if err == nil && latest < afterSequence {
			h.writeSnapshotRequired(c, conversationID, afterSequence, "client_ahead")
			return
		}
		if err == nil && latest > cursor && len(durable) == 0 {
			h.writeSnapshotRequired(c, conversationID, afterSequence, "durable_gap")
			return
		}
	}
	for _, event := range replay {
		if event.EventSequence <= cursor {
			continue
		}
		if cursor > 0 && event.EventSequence != cursor+1 {
			h.writeSnapshotRequired(c, conversationID, cursor, "replay_gap")
			return
		}
		if err := writeAgentUIEvent(c, event); err != nil {
			return
		}
		cursor = event.EventSequence
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case event, ok := <-live:
			if !ok {
				return
			}
			if event.EventSequence <= cursor {
				continue
			}
			if cursor > 0 && event.EventSequence != cursor+1 {
				h.writeSnapshotRequired(c, conversationID, cursor, "live_gap")
				return
			}
			if err := writeAgentUIEvent(c, event); err != nil {
				return
			}
			cursor = event.EventSequence
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

func (h *Handler) WebChatInterruptTurn(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Param("id"))
	turnID := strings.TrimSpace(c.Param("turnId"))
	if conversationID == "" || turnID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话或 Turn ID", nil)
		return
	}
	if _, err := h.requireWebChatConversation(conversationID, webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	var turn chat.AssistantTurn
	if err := h.db.WithContext(c.Request.Context()).Where("id = ? AND conversation_id = ?", turnID, conversationID).First(&turn).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "Turn 不存在", nil)
		return
	}
	if !conversationstream.DefaultManager().Interrupt(conversationID, turnID) {
		util.ErrorResponse(c, response.OperationFailed, "当前 Turn 没有可中断的执行", nil)
		return
	}
	result := h.db.WithContext(c.Request.Context()).Model(&chat.AssistantTurn{}).
		Where("id = ? AND status NOT IN ?", turnID, []string{"completed", "failed", "interrupted"}).
		Updates(map[string]any{"status": "cancelling", "updated_at": time.Now().UTC().Format(time.RFC3339Nano)})
	if result.Error == nil && result.RowsAffected > 0 {
		_, _ = conversationstream.DefaultManager().Publish(c.Request.Context(), conversationstream.AgentUIEvent{ConversationID: conversationID, RequestID: turn.RequestID, ExecutionID: turn.ExecutionID, TurnID: turn.ID, TurnSequence: turn.Sequence, Type: "turn.cancelling", Status: "cancelling"}, true)
	}
	util.SuccessResponse(c, gin.H{"conversationId": conversationID, "turnId": turnID, "interrupted": true})
}

func (h *Handler) WebChatSteerTurn(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Param("id"))
	turnID := strings.TrimSpace(c.Param("turnId"))
	var body steerTurnRequest
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Content) == "" {
		util.ErrorResponse(c, response.InvalidParams, "干预内容不能为空", nil)
		return
	}
	if _, err := h.requireWebChatConversation(conversationID, webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	if !conversationstream.DefaultManager().Steer(conversationID, turnID, body.Content) {
		util.ErrorResponse(c, response.OperationFailed, "当前 Turn 不可干预", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"conversationId": conversationID, "turnId": turnID, "accepted": true})
}

func (h *Handler) WebChatRetryTurn(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Param("id"))
	turnID := strings.TrimSpace(c.Param("turnId"))
	spaceID := webChatSpaceID(c)
	if conversationID == "" || turnID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话或 Turn ID", nil)
		return
	}
	conversation, err := h.requireWebChatConversation(conversationID, spaceID)
	if err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	var sourceTurn chat.AssistantTurn
	if err := h.db.WithContext(c.Request.Context()).Where("id = ? AND conversation_id = ?", turnID, conversationID).First(&sourceTurn).Error; err != nil {
		util.ErrorResponse(c, response.NotFound, "Turn 不存在", nil)
		return
	}
	if !isRetryableTurnStatus(sourceTurn.Status) {
		util.ErrorResponse(c, response.OperationFailed, "当前 Turn 不可重试", nil)
		return
	}
	var userMessage chat.Message
	if err := h.db.WithContext(c.Request.Context()).Where("id = ? AND conversation_id = ?", sourceTurn.UserMessageID, conversationID).First(&userMessage).Error; err != nil {
		util.ErrorResponse(c, response.OperationFailed, "原始用户消息不存在", nil)
		return
	}
	newTurn := chat.AssistantTurn{
		ID: uuid.NewString(), ConversationID: conversationID, CharacterID: sourceTurn.CharacterID, UserMessageID: userMessage.ID,
		RequestID: sourceTurn.RequestID, ExecutionID: uuid.NewString(), Status: "queued", CreatedAt: time.Now().Format("2006-01-02 15:04:05"), UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&chat.AssistantTurn{}).Where("conversation_id = ?", conversationID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&newTurn.Sequence).Error; err != nil {
			return err
		}
		return tx.Create(&newTurn).Error
	}); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "创建重试执行失败", nil)
		return
	}
	_, _ = conversationstream.DefaultManager().Publish(c.Request.Context(), conversationstream.AgentUIEvent{ConversationID: conversationID, RequestID: newTurn.RequestID, ExecutionID: newTurn.ExecutionID, TurnID: newTurn.ID, TurnSequence: newTurn.Sequence, Type: "turn.queued", Status: "queued"}, true)
	genCtx, cancel, executionStarted := conversationstream.DefaultManager().BeginExecution(conversationID, newTurn.ID)
	if !executionStarted {
		h.finalizeWebChatTurnRuntime(&newTurn, "", "failed", "conversation_busy", true, "当前会话已有正在执行的 Turn")
		util.SuccessResponse(c, gin.H{"conversationId": conversationID, "turnId": newTurn.ID, "executionId": newTurn.ExecutionID, "requestId": newTurn.RequestID, "status": "failed", "errorCode": "conversation_busy"})
		return
	}
	binding := h.workspaceBindingForConversation(*conversation, spaceID)
	go func() {
		defer cancel()
		defer conversationstream.DefaultManager().ClearExecution(conversationID, newTurn.ID)
		result, runErr := h.handleUnifiedEntryWithWorkspace(genCtx, &interaction.UnifiedEntryRequest{
			ConversationID: conversationID, Channel: "web", Source: userMessage.Source, SpaceID: spaceID, RequestID: newTurn.RequestID, SessionID: conversationID,
			CharacterID: sourceTurn.CharacterID, Message: userMessage.Content, AudioUrl: userMessage.AudioUrl, AudioDuration: userMessage.AudioDuration, ImageUrl: userMessage.ImageUrl, VideoUrl: userMessage.VideoUrl,
			ReplyToMessageID: userMessage.ReplyToMessageID, ModelConfigID: conversation.ModelConfigID, ReasoningEffort: conversation.ReasoningEffort, PermissionMode: conversation.PermissionMode,
			TurnID: newTurn.ID, ExecutionID: newTurn.ExecutionID, ForceRegenerate: true,
		}, binding)
		if runErr != nil {
			if genCtx.Err() != nil {
				h.finalizeWebChatTurnRuntime(&newTurn, "", "interrupted", "interrupted", false, "已停止生成")
			} else {
				h.finalizeWebChatTurnRuntime(&newTurn, "", "failed", "generation_failed", true, "Agent 重试执行失败")
			}
			return
		}
		_ = result
	}()
	util.SuccessResponse(c, gin.H{"conversationId": conversationID, "turnId": newTurn.ID, "executionId": newTurn.ExecutionID, "requestId": newTurn.RequestID, "status": "queued"})
}

func isRetryableTurnStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "interrupted":
		return true
	default:
		return false
	}
}

func (h *Handler) writeSnapshotRequired(c *gin.Context, conversationID string, afterSequence int64, reason string) {
	latest, _ := conversationstream.DefaultManager().LatestSequence(c.Request.Context(), conversationID)
	payload, _ := json.Marshal(gin.H{"version": conversationstream.ProtocolVersion, "conversationId": conversationID, "afterSequence": afterSequence, "lastEventSequence": latest, "reason": reason})
	_, _ = fmt.Fprintf(c.Writer, "event: snapshot.required\ndata: %s\n\n", payload)
	c.Writer.Flush()
}

func writeAgentUIEvent(c *gin.Context, event conversationstream.AgentUIEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "id: %d\nevent: agent_ui_event\ndata: %s\n\n", event.EventSequence, payload); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}

func parseEventSequence(c *gin.Context) int64 {
	values := []string{c.GetHeader("Last-Event-ID"), c.Query("afterSequence")}
	for _, value := range values {
		sequence, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil && sequence > 0 {
			return sequence
		}
	}
	return 0
}

func eventsCanRecover(after int64, events []conversationstream.AgentUIEvent) bool {
	cursor := after
	for _, event := range events {
		if event.EventSequence <= cursor {
			continue
		}
		if cursor > 0 && event.EventSequence != cursor+1 && !eventRecoveryCheckpoint(event) {
			return false
		}
		cursor = event.EventSequence
	}
	return true
}

func eventRecoveryCheckpoint(event conversationstream.AgentUIEvent) bool {
	if event.Payload == nil {
		return false
	}
	value, ok := event.Payload["recoveryCheckpoint"]
	if !ok {
		return false
	}
	flag, ok := value.(bool)
	return ok && flag
}

func isRecoverableTurnStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "queued", "starting", "running", "waiting_tool", "waiting_approval", "waiting_user", "cancelling":
		return true
	default:
		return false
	}
}
