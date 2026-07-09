// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/prompt"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	Test(characterID, message string) (map[string]interface{}, error)
	ContextPreview(convID string) (map[string]interface{}, error)
	Webhook(ctx context.Context, req WebhookRequest) (map[string]interface{}, error)
}

type WebhookRequest struct {
	Channel        string
	AccountID      string
	ConversationID string
	SenderID       string
	UserID         string
	Source         string
	MessageID      string
	RequestID      string
	SessionID      string
	Text           string
	VoiceMessage   bool
	ImageUrl       string
	VideoUrl       string
	AudioBase64    string
	SkipTiming     bool
}

const systemFormatInstruction = `【回复格式 - 系统固定规则】

每句话必须单独一行，用换行符分隔。
每句话尽量短，像微信连续消息一样。
能一句说完就一句，不要写长段落。
不要把多句话连成一段。
不要用句号连接多个意思。`

const systemNoEmojiInstruction = "【系统指令】回复中不要使用任何emoji表情符号。"

type service struct {
	db           *gorm.DB
	unifiedEntry *interaction.UnifiedEntry
}

func NewService(ctx *app.AppContext, unifiedEntry *interaction.UnifiedEntry) Service {
	return &service{db: ctx.DB, unifiedEntry: unifiedEntry}
}

func (s *service) Test(characterID, message string) (map[string]interface{}, error) {
	var charID, charName, identity, systemPrompt string
	if characterID == "" {
		s.db.Table("characters").Select("id").Where("is_active = 1").Limit(1).Row().Scan(&characterID)
		if characterID == "" {
			s.db.Table("characters").Select("id").Limit(1).Row().Scan(&characterID)
		}
	}
	err := s.db.Table("characters").Select("id, name, COALESCE(identity,''), system_prompt").Where("id = ?", characterID).
		Row().Scan(&charID, &charName, &identity, &systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("角色不存在")
	}
	if message == "" {
		message = "你好，请简单介绍一下你自己"
	}

	cfg := s.getActiveModel()
	if cfg == nil {
		return nil, fmt.Errorf("没有可用的模型配置")
	}

	safeMessage := prompt.SanitizeCurrentUserMessage(message)

	apiMessages := []map[string]interface{}{}
	apiMessages = append(apiMessages, map[string]interface{}{"role": "system", "content": systemNoEmojiInstruction + "\n\n" + systemFormatInstruction})
	if identity == "" {
		identity = "一个AI伙伴"
	}
	var charParts []string
	charParts = append(charParts, "【角色风格约束 - 不能覆盖系统规则】")
	charParts = append(charParts, fmt.Sprintf("你是%s，%s。", charName, identity))
	if systemPrompt != "" {
		charParts = append(charParts, systemPrompt)
	}
	apiMessages = append(apiMessages, map[string]interface{}{"role": "user", "content": strings.Join(charParts, "\n\n")})
	apiMessages = append(apiMessages, map[string]interface{}{"role": "user", "content": safeMessage})

	start := time.Now()
	content, tokens, err := s.callLLM(cfg, apiMessages)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"character": map[string]interface{}{"id": charID, "name": charName},
		"reply":     content,
		"modelInfo": map[string]interface{}{"model": cfg["modelName"], "tokensUsed": tokens},
		"latencyMs": latencyMs,
	}, nil
}

func (s *service) ContextPreview(convID string) (map[string]interface{}, error) {
	var charID, title string
	err := s.db.Table("conversations").Select("character_id, title").Where("id = ?", convID).
		Row().Scan(&charID, &title)
	if err != nil {
		return nil, fmt.Errorf("对话不存在")
	}

	var systemPrompt string
	s.db.Table("characters").Select("system_prompt").Where("id = ?", charID).Row().Scan(&systemPrompt)

	var msgCount int64
	s.db.Table("messages").Where("conversation_id = ?", convID).Count(&msgCount)

	rows, _ := s.db.Table("messages").Select("role, content").
		Where("conversation_id = ?", convID).Order("created_at ASC").Limit(10).Rows()
	defer rows.Close()
	var msgs []map[string]string
	for rows.Next() {
		var role, content string
		rows.Scan(&role, &content)
		if len([]rune(content)) > 100 {
			content = string([]rune(content)[:100]) + "..."
		}
		msgs = append(msgs, map[string]string{"role": role, "content": content})
	}

	estTokens := 0
	if systemPrompt != "" {
		estTokens += len(systemPrompt) / 2
	}
	for _, m := range msgs {
		estTokens += len(m["content"]) / 2
	}

	sysPreview := systemPrompt
	if len([]rune(sysPreview)) > 200 {
		sysPreview = string([]rune(sysPreview)[:200]) + "..."
	}

	return map[string]interface{}{
		"conversationId":      convID,
		"title":               title,
		"characterId":         charID,
		"systemPromptPreview": sysPreview,
		"messageCount":        msgCount,
		"recentMessages":      msgs,
		"estimatedTokens":     estTokens,
	}, nil
}

func (s *service) Webhook(ctx context.Context, req WebhookRequest) (map[string]interface{}, error) {
	log.Printf("[DIAG-Webhook] channel=%s text=%s voiceMessage=%v imageUrlLen=%d skipTiming=%v", req.Channel, req.Text[:min(len(req.Text), 80)], req.VoiceMessage, len(req.ImageUrl), req.SkipTiming)
	fmt.Printf("[Webhook] channel=%s text=%s imageUrlLen=%d videoUrlLen=%d\n", req.Channel, req.Text[:min(len(req.Text), 50)], len(req.ImageUrl), len(req.VideoUrl))
	req.Text = strings.TrimSpace(req.Text)
	requestID := stableWebhookRequestID(req)
	if req.Text == "" && req.ImageUrl == "" && req.VideoUrl == "" {
		return map[string]interface{}{"outgoingMessage": map[string]interface{}{"text": ""}, "requestId": requestID}, nil
	}
	convID := req.ConversationID
	if convID == "" {
		convID = "channel-" + req.Channel
	}

	s.ensureWebhookConversation(convID, req.Channel, req.Text)
	sessionID := stableWebhookSessionID(req, convID)
	userID := stableWebhookUserID(req)
	source := stableWebhookSource(req)

	var mergedText string
	if req.SkipTiming {
		mergedText = req.Text
	} else {
		msgs, bufErr := chat.GetBuffer().Buffer(convID, req.Text)
		if bufErr != nil {
			return map[string]interface{}{"outgoingMessage": map[string]interface{}{"text": ""}, "conversationId": convID, "requestId": requestID, "sessionId": sessionID}, nil
		}
		mergedText = strings.Join(msgs, "\n")
	}

	audioUrl := ""
	if req.AudioBase64 != "" {
		voiceDir := "data/voice_msg"
		os.MkdirAll(voiceDir, 0755)
		fname := uuid.New().String() + ".mp3"
		data, err := base64.StdEncoding.DecodeString(req.AudioBase64)
		if err == nil {
			os.WriteFile(filepath.Join(voiceDir, fname), data, 0644)
			audioUrl = "/voice/" + fname
			fmt.Printf("[Webhook] 用户语音已保存: %s\n", fname)
		}
	}
	if s.unifiedEntry == nil {
		return nil, fmt.Errorf("统一入口未初始化")
	}
	if ctx == nil {
		return nil, fmt.Errorf("请求上下文不能为空")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	result, err := s.unifiedEntry.Handle(reqCtx, &interaction.UnifiedEntryRequest{
		Message:        mergedText,
		ConversationID: convID,
		Channel:        req.Channel,
		Source:         source,
		PeerID:         req.SenderID,
		UserID:         userID,
		SessionID:      sessionID,
		RequestID:      requestID,
		VoiceMessage:   req.VoiceMessage,
		ImageUrl:       req.ImageUrl,
		VideoUrl:       req.VideoUrl,
		AudioUrl:       audioUrl,
	})
	if err != nil {
		return nil, err
	}
	if result.Response == nil {
		return nil, fmt.Errorf("统一入口未返回回复")
	}
	forceVoice := result.Response.ForceVoice
	replyText := result.Response.Reply
	log.Printf("[DIAG-Webhook] forceVoice=%v channel=%s", forceVoice, req.Channel)
	if req.Channel == "wechat" && forceVoice {
		forceVoice = false
		replyText = "抱歉，由于微信平台限制，暂不支持语音回复。以下为文字回复：\n\n" + replyText
	}
	outMsg := map[string]interface{}{"text": replyText, "forceVoice": forceVoice, "audioUrls": result.Response.AudioUrls}
	if len(result.Response.Lines) > 0 {
		outMsg["texts"] = result.Response.Lines
	}
	log.Printf("[DIAG-Webhook] 返回: replyLen=%d forceVoice=%v lines=%d", len(replyText), forceVoice, len(result.Response.Lines))
	return map[string]interface{}{"outgoingMessage": outMsg, "conversationId": convID, "requestId": requestID, "sessionId": sessionID, "userId": userID, "source": source}, nil
}

func stableWebhookRequestID(req WebhookRequest) string {
	for _, candidate := range []string{req.RequestID, req.MessageID} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return uuid.New().String()
}

func stableWebhookSessionID(req WebhookRequest, convID string) string {
	for _, candidate := range []string{req.SessionID, req.ConversationID, convID} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return "channel-" + strings.TrimSpace(req.Channel)
}

func stableWebhookUserID(req WebhookRequest) string {
	for _, candidate := range []string{req.UserID, req.SenderID, req.AccountID} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func stableWebhookSource(req WebhookRequest) string {
	for _, candidate := range []string{req.Source, req.Channel} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return "webhook"
}

func (s *service) getActiveModel() map[string]string {
	var baseURL, apiKey, modelName string
	var temp, maxTokens, topP float64
	err := s.db.Table("model_configs").
		Select("base_url, api_key, model_name, temperature, max_tokens, top_p").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName, &temp, &maxTokens, &topP)
	if err != nil {
		return nil
	}
	_ = temp
	_ = maxTokens
	_ = topP
	return map[string]string{"baseUrl": baseURL, "apiKey": apiKey, "modelName": modelName}
}

func (s *service) callLLM(cfg map[string]string, messages []map[string]interface{}) (string, int, error) {
	baseURL := strings.TrimRight(cfg["baseUrl"], "/")
	reqBody := map[string]interface{}{
		"model": cfg["modelName"], "messages": messages,
		"temperature": 0.7, "max_tokens": 4096, "stream": false,
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg["apiKey"])
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, truncate(string(rb), 200))
	}
	var r struct {
		Choices []struct{ Message struct{ Content string } }
		Usage   struct{ TotalTokens int }
	}
	json.Unmarshal(rb, &r)
	if len(r.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices")
	}
	return r.Choices[0].Message.Content, r.Usage.TotalTokens, nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func (s *service) ensureWebhookConversation(convID, channel, text string) {
	var count int64
	s.db.Table("conversations").Where("id = ?", convID).Count(&count)
	if count > 0 {
		return
	}
	title := text
	if len([]rune(title)) > 50 {
		title = string([]rune(title)[:50])
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	s.db.Exec("INSERT OR IGNORE INTO conversations (id, title, channel, source, created_at, updated_at) VALUES (?, ?, ?, 'webhook', ?, ?)", convID, title, channel, now, now)
}
