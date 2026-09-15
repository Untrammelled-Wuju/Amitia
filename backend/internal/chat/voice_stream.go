// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	appLog "github.com/u-ai/backend/log"
)

type VoiceStreamTurn struct {
	UserText   string
	SpeechText string
}

type voiceStreamSink struct {
	onDelta   func(string) error
	delivered bool
}

func (s *voiceStreamSink) Emit(ctx context.Context, event ModelEvent) error {
	if event.Type != ModelEventTextDelta {
		return nil
	}
	if event.TextDelta == "" {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.delivered = true
	return s.onDelta(event.TextDelta)
}

func (s *service) GenerateVoiceStream(ctx context.Context, spaceID, characterID uuid.UUID, systemPrompt string, history []VoiceStreamTurn, rollingSummary string, userText string, onDelta func(string) error) error {
	if onDelta == nil {
		return fmt.Errorf("voice stream delta sink is required")
	}
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return fmt.Errorf("获取模型配置失败: %w", err)
	}
	messages := make([]map[string]interface{}, 0, len(history)*2+3)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": systemPrompt})
	}
	if summary := strings.TrimSpace(rollingSummary); summary != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": "【本次通话较早内容摘要】\n" + summary})
	}
	for _, turn := range history {
		if text := strings.TrimSpace(turn.UserText); text != "" {
			messages = append(messages, map[string]interface{}{"role": "user", "content": text})
		}
		if text := strings.TrimSpace(turn.SpeechText); text != "" {
			messages = append(messages, map[string]interface{}{"role": "assistant", "content": text})
		}
	}
	if text := strings.TrimSpace(userText); text != "" {
		messages = append(messages, map[string]interface{}{"role": "user", "content": text})
	}
	sink := &voiceStreamSink{onDelta: onDelta}
	for attempt := 0; attempt < 2; attempt++ {
		_, err = s.callLLMStreamAdapter(ctx, cfg, messages, nil, true, true, sink)
		if err == nil {
			return nil
		}
		if sink.delivered || ctx.Err() != nil {
			return err
		}
		appLog.Warn("realtime voice stream failed before first delta:", err.Error())
		if attempt == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(120 * time.Millisecond):
			}
		}
	}
	fallback := `{"interaction_mode":"NORMAL","speech_instruction":"自然、简短、带一点疑惑和无奈","speech_text":"[hesitate]嗯？刚才有点没听清，再说一遍好吗？","user_affect":{"primary_emotion":"neutral","secondary_emotion":"none","intensity":0,"stress":0,"need":"none","advice_wanted":false,"openness":0.5,"severity":0,"possible_concealment":false,"confidence":0.5,"evidence":["none"]}}`
	if err := onDelta(fallback); err != nil {
		return err
	}
	return nil
}

func (s *service) SummarizeRealtimeVoiceRollingContext(ctx context.Context, existingSummary, rawTurns string) (string, error) {
	rawTurns = strings.TrimSpace(rawTurns)
	if rawTurns == "" {
		return "", nil
	}
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return "", fmt.Errorf("获取模型配置失败: %w", err)
	}
	var prompt strings.Builder
	prompt.WriteString("你正在压缩一段实时语音通话的较早片段，压缩结果只用于后续 AI 理解上下文，不会展示给用户。\n")
	prompt.WriteString("要求：\n")
	prompt.WriteString("1. 只输出压缩后的摘要正文，不要输出标题、序号、解释或 JSON。\n")
	prompt.WriteString("2. 保留用户说了什么、AI 回应了什么、以及已经形成的结论、约定或话题线。\n")
	prompt.WriteString("3. 不要编造原文没有的信息；无法确认的部分直接省略。\n")
	prompt.WriteString("4. 控制在 400 字以内。\n")
	if existing := strings.TrimSpace(existingSummary); existing != "" {
		prompt.WriteString("\n已有摘要：\n")
		prompt.WriteString(existing)
	}
	prompt.WriteString("\n\n需要压缩的较早片段：\n")
	prompt.WriteString(rawTurns)
	messages := []map[string]interface{}{
		{"role": "system", "content": "你是一个压缩通话上下文的助手。"},
		{"role": "user", "content": prompt.String()},
	}
	reply, _, err := s.callLLM(ctx, cfg, messages)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(reply), nil
}

func (s *service) RealtimeVoiceReady() error {
	if _, err := s.repo.GetActiveModel(); err != nil {
		return fmt.Errorf("未配置可用的模型来源")
	}
	return nil
}
