// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"errors"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/tts"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	applog "github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) MessagesStream(c *gin.Context) {
	convID := c.Query("conversationId")
	if convID == "" {
		c.Header("Content-Type", "text/event-stream")
		c.Writer.WriteString("event: error\ndata: missing conversationId\n\n")
		c.Writer.Flush()
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	sinceID := c.Query("since")
	sinceCreatedAt := ""
	if sinceID != "" {
		h.db.Table("messages").Select("created_at").Where("id = ?", sinceID).Row().Scan(&sinceCreatedAt)
	}
	if sinceCreatedAt == "" {
		sinceCreatedAt = "0001-01-01"
	}
	for {
		var msgs []map[string]interface{}
		rows, _ := h.db.Table("messages").Where("conversation_id = ? AND (created_at > ? OR (created_at = ? AND id > ?))", convID, sinceCreatedAt, sinceCreatedAt, sinceID).Order("created_at ASC, id ASC").Rows()
		for rows.Next() {
			var m map[string]interface{}
			h.db.ScanRows(rows, &m)
			msgs = append(msgs, m)
		}
		rows.Close()
		for _, m := range msgs {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			if role == "tool" {
				continue
			}
			if role == "assistant" && content == "" {
				continue
			}
			c.SSEvent("message", m)
			if ca, ok := m["created_at"].(string); ok {
				sinceCreatedAt = ca
			}
			if id, ok := m["id"].(string); ok {
				sinceID = id
			}
			c.Writer.Flush()
		}
		select {
		case <-c.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (h *Handler) RemindersStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	var lastCount int64
	var lastUpdated string
	h.db.Table("reminders").Count(&lastCount)
	h.db.Table("reminders").Select("MAX(updated_at)").Row().Scan(&lastUpdated)

	c.SSEvent("status", map[string]interface{}{"count": lastCount})
	c.Writer.Flush()

	for {
		select {
		case <-c.Done():
			return
		case <-time.After(5 * time.Second):
		}

		var curCount int64
		var curUpdated string
		h.db.Table("reminders").Count(&curCount)
		h.db.Table("reminders").Select("MAX(updated_at)").Row().Scan(&curUpdated)

		if curCount != lastCount || curUpdated != lastUpdated {
			lastCount = curCount
			lastUpdated = curUpdated
			c.SSEvent("changed", map[string]interface{}{"count": curCount, "updatedAt": curUpdated})
			c.Writer.Flush()
		}
	}
}

func (h *Handler) WebChatSendStream(c *gin.Context) {
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
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"status": "queued", "conversationId": convID, "requestId": requestID, "sessionId": sessionID}})
		return
	}

	mergedContent := strings.Join(bufferedMsgs, "\n")
	imageCtx := chat.GetBuffer().GetImageContexts(convID)
	applog.Info(fmt.Sprintf("[Webhook] imageCtx len=%d content=%s", len(imageCtx), imageCtx[:min(len(imageCtx), 200)]))

	if h.unifiedEntry == nil {
		util.ErrorResponse(c, response.InternalError, "统一入口未初始化", nil)
		return
	}

	orchResult, err := h.unifiedEntry.Handle(c.Request.Context(), &interaction.UnifiedEntryRequest{
		CharacterID: body.CharacterID, Message: mergedContent,
		ConversationID: convID, Channel: "web", Source: source,
		UserID: userID, PeerID: peerID, RequestID: requestID, SessionID: sessionID,
		AudioUrl: body.AudioUrl, AudioDuration: body.AudioDuration,
		VoiceMessage: body.VoiceMessage,
		ImageUrl:     body.ImageUrl,
		VideoUrl:     body.VideoUrl,
		ImageContext: imageCtx,
	})
	if errors.Is(err, interaction.ErrOrchestratorProcessing) {
		util.ErrorResponse(c, response.InternalError, "请求处理中", nil)
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	if orchResult.Response == nil {
		util.ErrorResponse(c, response.InternalError, "统一入口未返回回复", nil)
		return
	}
	result := orchResult.Response
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "SSE not supported", nil)
		return
	}
	lines := strings.Split(strings.TrimSpace(result.Reply), "\n")
	msgIDIdx := 0

	voiceChance := 0.20
	if body.VoiceMessage {
		voiceChance = 0.80
	}
	if result.ForceVoice {
		voiceChance = 1.0
	}
	applog.Info(fmt.Sprintf("[Voice] voiceMessage=%v forceVoice=%v voiceChance=%.0f%% replyLen=%d", body.VoiceMessage, result.ForceVoice, voiceChance*100, len(result.Reply)))
	var ttsCfg *tts.TtsConfig
	if (rand.Float64() < voiceChance || result.ForceVoice) && result.Reply != "" {
		ttsRepo := tts.NewRepository(h.db)
		charCfg, cfgErr := ttsRepo.GetByCharacterID(body.CharacterID)
		if cfgErr != nil {
			applog.Info(fmt.Sprintf("[Voice] GetByCharacterID err: %v", cfgErr))
		}
		if cfgErr == nil && charCfg.ApiKey != "" {
			cfg := &tts.TtsConfig{ApiKey: charCfg.ApiKey, ResourceId: charCfg.ResourceId, VoiceType: charCfg.VoiceType, Speed: charCfg.Speed, Pitch: charCfg.Pitch, Volume: charCfg.Volume}
			if cfg.ResourceId == "" {
				cfg.ResourceId = "seed-tts-2.0"
			}
			if cfg.VoiceType == "" {
				cfg.VoiceType = "zh_female_vv_uranus_bigtts"
			}
			if cfg.Speed == 0 {
				cfg.Speed = 1.0
			}
			if cfg.Pitch == 0 {
				cfg.Pitch = 1.0
			}
			if cfg.Volume == 0 {
				cfg.Volume = 1.0
			}
			ttsCfg = cfg
		} else if charCfg != nil && charCfg.ApiKey == "" {
			applog.Info("[Voice] TTS ApiKey empty")
		}
	} else {
		applog.Info(fmt.Sprintf("[Voice] skipped: chance=%.2f forceVoice=%v reply=%v", voiceChance, result.ForceVoice, result.Reply != ""))
	}

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || isReasoningLine(line) {
			continue
		}
		if i > 0 {
			delayMs := 300 + len([]rune(line))*80
			if delayMs > 3000 {
				delayMs = 3000
			}
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
		msgID := uuid.New().String()
		if msgIDIdx < len(result.MessageIDs) {
			msgID = result.MessageIDs[msgIDIdx]
		}
		msgIDIdx++

		var audioURL string
		var audioDuration float64
		if ttsCfg != nil {
			applog.Info(fmt.Sprintf("[Voice] TTS part: %s", line[:min(len(line), 30)]))
			synthResult, synthErr := ttsSynthesizeWithTimeout(ttsCfg, line, 8*time.Second)
			if synthErr != nil {
				applog.Info(fmt.Sprintf("[Voice] TTS err: %v", synthErr))
			} else {
				audioURL = synthResult.AudioURL
				audioDuration = synthResult.Duration
				h.db.Table("messages").Where("id = ?", msgID).Updates(map[string]interface{}{
					"audio_url":      audioURL,
					"audio_duration": audioDuration,
				})
			}
		}

		if ttsCfg != nil && audioURL != "" {
			audioData := gin.H{"messageId": msgID, "conversationId": result.ConversationID, "role": "assistant", "content": line, "createdAt": time.Now().Format("2006-01-02 15:04:05"), "audioUrl": audioURL, "duration": audioDuration}
			ad, _ := json.Marshal(audioData)
			applog.Info(fmt.Sprintf("[Voice] sending voice_audio: %s", audioURL))
			fmt.Fprintf(c.Writer, "event: voice_audio\ndata: %s\n\n", string(ad))
			flusher.Flush()
		} else if ttsCfg == nil {
			msg := gin.H{"id": msgID, "conversationId": result.ConversationID, "role": "assistant", "content": line, "createdAt": time.Now().Format("2006-01-02 15:04:05")}
			b, _ := json.Marshal(msg)
			fmt.Fprintf(c.Writer, "event: token\ndata: %s\n\n", string(b))
			flusher.Flush()
		}
	}
	doneData := gin.H{"conversationId": result.ConversationID, "requestId": requestID, "sessionId": sessionID, "userId": userID, "source": source}
	db, _ := json.Marshal(doneData)
	fmt.Fprintf(c.Writer, "event: done\ndata: %s\n\n", string(db))
	flusher.Flush()
}
