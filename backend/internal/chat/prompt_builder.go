// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/expression"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/pkg/app")

func (s *service) compiledSystemInstruction(channel string) string {
	kind := expression.ChannelKind(strings.ToLower(channel))
	if channel == "" {
		kind = expression.ChannelWeb
	}
	cp := expression.CompileChannelPrompt(kind)
	return cp.SystemInstruction
}

func (s *service) compiledChannelPrompt(channel string) expression.CompiledPrompt {
	kind := expression.ChannelKind(strings.ToLower(channel))
	if channel == "" {
		kind = expression.ChannelWeb
	}
	return expression.CompileChannelPrompt(kind)
}

func (s *service) sys1Builder(profile *character.RoleRuntimeProfile, userMessage string) []string {
	parts := buildRoleSystemParts(profile)
	characterID := ""
	if profile != nil {
		characterID = strings.TrimSpace(profile.CharacterID)
	}
	if s.profileSvc != nil {
		profilePrompt := s.profileSvc.ToSystemPrompt("default", characterID)
		if profilePrompt != "" {
			parts = append(parts, profilePrompt)
		}
	}
	if s.episodicSvc != nil {
		epiPrompt := s.episodicSvc.ToSystemPrompt(characterID)
		if epiPrompt != "" {
			parts = append(parts, epiPrompt)
		}
	}
	if s.worldBookSvc != nil {
		wbPrompt := s.worldBookSvc.ToSystemPrompt(userMessage, "")
		if wbPrompt != "" {
			parts = append(parts, wbPrompt)
		}
	}
	return parts
}

func (s *service) sys2Builder(convID, charID, requestID, channel, userMessage string) []string {
	parts := []string{s.compiledSystemInstruction(channel)}
	if s.wmCache != nil {
		wm := s.wmCache.Get(convID)
		if wm != nil && wm.State != nil && wm.State.Summary != "" {
			parts = append(parts, "【工作记忆】\n"+wm.State.Summary)
		}
	}
	if s.compressor != nil {
		status := s.compressor.GetCompressionStatus(convID)
		if summary, ok := status["latestSummary"].(string); ok && summary != "" {
			parts = append(parts, "【对话历史摘要】\n"+summary)
		}
	}
	if s.memorySvc != nil && userMessage != "" {
		results, err := s.memorySvc.HybridSearch(&memory.VectorSearchRequest{
			Query:          userMessage,
			CharacterID:    charID,
			ConversationID: convID,
			RequestID:      requestID,
			Channel:        channel,
			Limit:          8,
		})
		if err == nil && len(results) > 0 {
			layerLines := map[string][]string{}
			layerOrder := []string{"当前摘要", "用户画像", "情景回忆", "事实记忆"}
			for _, r := range results {
				layer := r.MemoryLayer
				if layer == "" {
					layer = "事实记忆"
				}
				typeLabel := r.Memory.MemoryType
				if typeLabel == "" {
					typeLabel = "fact"
				}
				line := fmt.Sprintf("- [%s %s %.0f%% 置信%d%%] %s", typeLabel, r.MatchType, r.Score*100, r.Memory.Confidence, r.Memory.Value)
				layerLines[layer] = append(layerLines[layer], line)
				go s.memorySvc.RecordUse(r.Memory.ID)
			}
			for _, layer := range layerOrder {
				if lines := layerLines[layer]; len(lines) > 0 {
					parts = append(parts, "【"+layer+"】\n"+strings.Join(lines, "\n"))
				}
			}
		}
	}
	return parts
}

func (s *service) rewriteQueryForSearch(userMessage string) string {
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return userMessage
	}
	prompt := []map[string]interface{}{
		{"role": "system", "content": "把用户输入转成用于记忆检索的简洁关键词，去除语气词和寒暄，直接输出关键词不要解释。如果输入已经是简短的关键词则原样返回。"},
		{"role": "user", "content": userMessage},
	}
	rewritten, _, err := s.callLLM(cfg, prompt)
	if err != nil || rewritten == "" {
		return userMessage
	}
	rewritten = strings.TrimSpace(rewritten)
	if len([]rune(rewritten)) < 2 {
		return userMessage
	}
	return rewritten
}

func shouldRetrieveMemory(msg string) bool {
	trimmed := strings.TrimSpace(msg)
	if len([]rune(trimmed)) < 4 {
		return false
	}
	greetings := []string{"嗯", "好", "哦", "啊", "哈", "嗨", "喂", "在吗", "在不在", "好的", "好吧", "行", "可以", "知道了", "明白了", "懂了", "嗯嗯", "哈哈", "呵呵", "嘿嘿", "谢谢", "多谢", "再见", "拜拜", "晚安", "早安", "早上好", "晚上好", "ok", "OK", "Ok", "hi", "Hi", "hello", "Hello", "bye", "Bye"}
	lower := strings.ToLower(trimmed)
	for _, g := range greetings {
		if lower == strings.ToLower(g) {
			return false
		}
	}
	return true
}

func buildRoleSystemParts(profile *character.RoleRuntimeProfile) []string {
	parts := []string{}
	if profile == nil {
		return parts
	}
	identity := profile.Identity
	if identity == "" {
		identity = "一个AI伙伴"
	}
	parts = append(parts, fmt.Sprintf("你是%s，%s。", profile.Name, identity))
	if profile.SystemPrompt != "" {
		parts = append(parts, profile.SystemPrompt)
	}
	if profile.Personality != "" {
		parts = append(parts, "【角色性格】\n"+profile.Personality)
	}
	if profile.SpeakingStyle != "" {
		parts = append(parts, "【聊天风格】\n"+profile.SpeakingStyle)
	}
	if profile.RelationshipStyle != "" {
		parts = append(parts, "【关系风格】\n"+profile.RelationshipStyle)
	}
	if profile.BoundaryRules != "" {
		parts = append(parts, "【场景规则】\n"+profile.BoundaryRules)
	}
	appendRuntimeConfig := func(label string, data map[string]interface{}) {
		if len(data) == 0 {
			return
		}
		if len(data) == 1 {
			if _, ok := data["version"]; ok {
				return
			}
		}
		raw, err := json.Marshal(data)
		if err == nil && string(raw) != "{}" {
			parts = append(parts, label+"\n"+string(raw))
		}
	}
	appendRuntimeConfig("【性格配置】", profile.PersonalityConfig)
	appendRuntimeConfig("【对话风格配置】", profile.ChatStyleConfig)
	appendRuntimeConfig("【场景配置】", profile.SceneRules)
	return parts
}

func (s *service) getRoleRuntimeProfile(characterID string) (*character.RoleRuntimeProfile, error) {
	if s.charRepo != nil {
		return s.charRepo.GetRuntimeProfile(characterID)
	}
	return character.NewRepository(app.NewAppContext(s.db, nil)).GetRuntimeProfile(characterID)
}
