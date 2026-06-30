// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package agent

import (
	"bytes"
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
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	Test(characterID, message string) (map[string]interface{}, error)
	ContextPreview(convID string) (map[string]interface{}, error)
	Webhook(channel, senderID, conversationID, text string, voiceMessage bool, imageUrl string, videoUrl string, audioBase64 string, skipTiming bool) (map[string]interface{}, error)
}

const systemFormatInstruction = `【回复格式 - 系统固定规则】

每句话必须单独一行，用换行符分隔。
每句话尽量短，像微信连续消息一样。
能一句说完就一句，不要写长段落。
不要把多句话连成一段。
不要用句号连接多个意思。`

const systemNoEmojiInstruction = "【系统指令】回复中不要使用任何emoji表情符号。"

type service struct {
	db      *gorm.DB
	chatSvc chat.Service
}

func NewService(ctx *app.AppContext, chatSvc chat.Service) Service {
	return &service{db: ctx.DB, chatSvc: chatSvc}
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

	systemParts := []string{systemNoEmojiInstruction}
	if identity == "" {
		identity = "一个AI伙伴"
	}
	systemParts = append(systemParts, fmt.Sprintf("你是%s，%s。", charName, identity))
	if systemPrompt != "" {
		systemParts = append(systemParts, systemPrompt)
	}
	systemParts = append(systemParts, systemFormatInstruction)
	apiMessages := []map[string]interface{}{}
	apiMessages = append(apiMessages, map[string]interface{}{"role": "system", "content": strings.Join(systemParts, "\n\n")})
	apiMessages = append(apiMessages, map[string]interface{}{"role": "user", "content": message})
	apiMessages = append(apiMessages, map[string]interface{}{"role": "user", "content": message})

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

func (s *service) Webhook(channel, senderID, conversationID, text string, voiceMessage bool, imageUrl string, videoUrl string, audioBase64 string, skipTiming bool) (map[string]interface{}, error) {
	log.Printf("[DIAG-Webhook] channel=%s text=%s voiceMessage=%v imageUrlLen=%d skipTiming=%v", channel, text[:min(len(text), 80)], voiceMessage, len(imageUrl), skipTiming)
	fmt.Printf("[Webhook] channel=%s text=%s imageUrlLen=%d videoUrlLen=%d\n", channel, text[:min(len(text), 50)], len(imageUrl), len(videoUrl))
	text = strings.TrimSpace(text)
	if text == "" && imageUrl == "" && videoUrl == "" {
		return map[string]interface{}{"outgoingMessage": map[string]interface{}{"text": ""}}, nil
	}
	convID := conversationID
	if convID == "" {
		convID = "channel-" + channel
	}

	var mergedText string
	if skipTiming {
		mergedText = text
	} else {
		msgs, bufErr := chat.GetBuffer().Buffer(convID, text)
		if bufErr != nil {
			return map[string]interface{}{"outgoingMessage": map[string]interface{}{"text": ""}}, nil
		}
		mergedText = strings.Join(msgs, "\n")
	}

	audioUrl := ""
	if audioBase64 != "" {
		voiceDir := "data/voice_msg"
		os.MkdirAll(voiceDir, 0755)
		fname := uuid.New().String() + ".mp3"
		data, err := base64.StdEncoding.DecodeString(audioBase64)
		if err == nil {
			os.WriteFile(filepath.Join(voiceDir, fname), data, 0644)
			audioUrl = "/voice/" + fname
			fmt.Printf("[Webhook] 用户语音已保存: %s\n", fname)
		}
	}
	pmReq := &chat.ProcessMessageRequest{
		CharacterID:    "",
		Message:        mergedText,
		ConversationID: convID,
		Channel:        channel,
		Source:         channel,
		PeerID:         senderID,
		VoiceMessage:   voiceMessage,
		ImageUrl:       imageUrl,
		VideoUrl:       videoUrl,
		AudioUrl:       audioUrl,
	}
	result, err := s.chatSvc.ProcessMessage(pmReq)
	if err != nil {
		return nil, err
	}
	forceVoice := result.ForceVoice
	replyText := result.Reply
	log.Printf("[DIAG-Webhook] forceVoice=%v channel=%s", forceVoice, channel)
	if channel == "wechat" && forceVoice {
		forceVoice = false
		replyText = "抱歉，由于微信平台限制，暂不支持语音回复。以下为文字回复：\n\n" + replyText
	}
	log.Printf("[DIAG-Webhook] 返回: replyLen=%d forceVoice=%v", len(replyText), forceVoice)
	return map[string]interface{}{"outgoingMessage": map[string]interface{}{"text": replyText, "forceVoice": forceVoice, "audioUrls": result.AudioUrls}}, nil
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
