// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/interaction"
	promptir "github.com/u-ai/backend/internal/prompt"
	applog "github.com/u-ai/backend/log"
)

func (s *service) ProcessMessage(ctx context.Context, req *ProcessMessageRequest) (*ProcessMessageResponse, error) {
	computeResult, err := s.ComputeInteraction(ctx, req)
	if err != nil {
		return nil, err
	}
	if computeResult.HasExistingUser {
		return &ProcessMessageResponse{
			ConversationID: computeResult.ConversationID,
			Sequence:       computeResult.UserMessageSequence,
			Reply:          computeResult.Reply,
			Lines:          computeResult.Lines,
			CharacterID:    computeResult.CharacterID,
			CharacterName:  computeResult.CharacterName,
			UserMessageID:  computeResult.UserMessageID,
			RequestID:      computeResult.RequestID,
		}, nil
	}
	commitResult, err := s.commitInteraction(messageCommitPlan{
		Request:       req,
		Conversation:  computeResult.ConversationID,
		Character:     computeResult.CharacterID,
		CharacterName: computeResult.CharacterName,
		UserMessageID: computeResult.UserMessageID,
		Reply:         computeResult.Reply,
		Lines:         computeResult.Lines,
		Source:        computeResult.Source,
		TotalTokens:   computeResult.TotalTokens,
		Runtime:       req.Runtime,
	})
	if err != nil {
		s.db.Model(&Message{}).Where("id = ?", computeResult.UserMessageID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
		return nil, err
	}
	s.PostCommitActions(ctx, computeResult)
	return &ProcessMessageResponse{
		ConversationID: computeResult.ConversationID,
		Sequence:       commitResult.LastSequence,
		Reply:          computeResult.Reply,
		Lines:          computeResult.Lines,
		CharacterID:    computeResult.CharacterID,
		CharacterName:  computeResult.CharacterName,
		MessageIDs:     commitResult.MessageIDs,
		ForceVoice:     computeResult.ForceVoice,
		UserMessageID:  computeResult.UserMessageID,
		RequestID:      computeResult.RequestID,
		Events:         commitResult.Events,
	}, nil
}

const (
	postProcessEventPipelineExecute = "postprocess.pipeline.execute"
	postProcessEventContextTrim     = "postprocess.context.trim"
	postProcessEventMoodRecovery    = "postprocess.mood.recovery"
	postProcessEventCompressorMaybe = "postprocess.compressor.maybe"
	postProcessPayloadVersion       = "v1"
)

type PostProcessPayload struct {
	Version          string              `json:"version"`
	ConversationID   string              `json:"conversationId"`
	CharacterID      string              `json:"characterId"`
	Source           string              `json:"source"`
	RequestID        string              `json:"requestId"`
	Reply            string              `json:"reply"`
	PipelineMessages []map[string]string `json:"pipelineMessages"`
}

func (s *service) startPostProcessing(ctx context.Context, trace applog.TraceFields, convID, charID, source, requestID string, pipelineMessages []map[string]string, reply string) {
	if err := ctx.Err(); err != nil {
		applog.TraceWarn(trace.WithStage("postprocess_skipped_cancelled"), applog.Fields{
			"conversation_id": convID,
		}, "process message postprocess skipped because request context was cancelled")
		return
	}
	if s.outboxStore == nil {
		return
	}

	payload := PostProcessPayload{
		Version:          postProcessPayloadVersion,
		ConversationID:   convID,
		CharacterID:      charID,
		Source:           source,
		RequestID:        requestID,
		Reply:            reply,
		PipelineMessages: pipelineMessages,
	}
	data, _ := json.Marshal(payload)

	s.appendPostProcessOutbox(convID, postProcessEventContextTrim, requestID+"|"+postProcessEventContextTrim, data)
	s.appendPostProcessOutbox(convID, postProcessEventMoodRecovery, requestID+"|"+postProcessEventMoodRecovery, data)

	if s.pipeline != nil {
		s.appendPostProcessOutbox(convID, postProcessEventPipelineExecute, requestID+"|"+postProcessEventPipelineExecute, data)
	}
	if s.compressor != nil {
		s.appendPostProcessOutbox(convID, postProcessEventCompressorMaybe, requestID+"|"+postProcessEventCompressorMaybe, data)
	}
}

func (s *service) appendPostProcessOutbox(aggregateID, eventType, idempotencyKey string, payload []byte) {
	if s.outboxStore == nil {
		return
	}
	s.outboxStore.AppendOutboxWithKey(aggregateID, eventType, idempotencyKey, payload)
}

func (s *service) ReplayPostProcess(eventType string, payload []byte) error {
	var pp PostProcessPayload
	if err := json.Unmarshal(payload, &pp); err != nil {
		return err
	}
	ctx := context.Background()
	switch eventType {
	case postProcessEventPipelineExecute:
		if s.pipeline != nil {
			s.pipeline.Execute(ctx, pp.ConversationID, pp.PipelineMessages, pp.Reply)
		}
	case postProcessEventContextTrim:
		s.trimContextWindow(ctx, pp.ConversationID)
	case postProcessEventMoodRecovery:
		s.moodRecoveryCheck(ctx, pp.ConversationID, pp.CharacterID, pp.Source)
	case postProcessEventCompressorMaybe:
		if s.compressor != nil {
			s.compressor.MaybeCompress(ctx, pp.ConversationID)
		}
	default:
		return fmt.Errorf("unknown postprocess event type: %s", eventType)
	}
	return nil
}

func (s *service) abortMessageCommitIfCancelled(ctx context.Context, trace applog.TraceFields, userMsgID string) error {
	if err := ctx.Err(); err != nil {
		s.db.Model(&Message{}).Where("id = ?", userMsgID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
		applog.TraceWarn(trace.WithStage("request_cancelled_before_commit"), applog.Fields{
			"user_message_id": userMsgID,
		}, "process message request cancelled before db commit")
		return err
	}
	return nil
}

func (s *service) invokeLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, trace applog.TraceFields, userMsgID, convID, charID, channel, requestID string, toolDefs []tool.Tool, seenTools map[string]bool, toolExecCtx context.Context) (string, bool, int, error) {
	var reply string
	var totalTokens int
	forceVoice := false
	for round := 0; round < 3; round++ {
		applog.TraceInfo(trace.WithStage("model_call_started"), applog.Fields{
			"round":         round,
			"message_count": len(messages),
		}, "process message model call started")
		aiContent, reasoning, toolCalls, tok, llmErr := s.invokeProcessLLMWithTools(ctx, cfg, messages, toolDefs)
		totalTokens = tok
		if llmErr != nil {
			s.db.Model(&Message{}).Where("id = ?", userMsgID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
			applog.TraceError(trace.WithStage("model_call_failed"), applog.Fields{
				"round":           round,
				"user_message_id": userMsgID,
			}, llmErr, "process message model call failed")
			return "", false, 0, fmt.Errorf("AI 调用失败: %w", llmErr)
		}
		applog.TraceInfo(trace.WithStage("model_call_completed"), applog.Fields{
			"round":           round,
			"tool_call_count": len(toolCalls),
			"reply_size":      len(aiContent),
			"reasoning_size":  len(reasoning),
		}, "process message model call completed")
		if len(toolCalls) == 0 {
			reply = aiContent
			break
		}
		assistantToolCall := map[string]interface{}{
			"role":       "assistant",
			"content":    aiContent,
			"tool_calls": toolCalls,
		}
		if reasoning != "" {
			assistantToolCall["reasoning_content"] = reasoning
		}
		messages = append(messages, assistantToolCall)
		for _, tc := range toolCalls {
			name, _ := tc["function"].(map[string]interface{})["name"].(string)
			args, _ := tc["function"].(map[string]interface{})["arguments"].(string)
			toolCallID, _ := tc["id"].(string)
			if name == "create_schedule" {
				dedupKey := name + "|" + args
				if seenTools[dedupKey] {
					continue
				}
				seenTools[dedupKey] = true
			}
			if name == "create_schedule" {
				var toolArgs map[string]interface{}
				json.Unmarshal([]byte(args), &toolArgs)
				toolArgs["conversation_id"] = convID
				toolArgs["character_id"] = charID
				if channel == "web" {
					toolArgs["channel"] = "all"
				} else if channel != "" {
					toolArgs["channel"] = channel
				}
				newArgs, _ := json.Marshal(toolArgs)
				args = string(newArgs)
			}
			toolCtx := tool.ToolExecutionContext{
				ConversationID: convID,
				CharacterID:    charID,
				Channel:        channel,
				RequestID:      requestID,
				CorrelationID:  trace.CorrelationID,
				CausationID:    trace.CausationID,
				User:           trace.User,
				StateVersion:   trace.StateVersion,
				Path:           "chat.process_message.tool",
				ToolCallID:     toolCallID,
			}
			applog.TraceInfo(trace.WithStage("tool_call_started"), applog.Fields{
				"round":        round,
				"tool_name":    name,
				"tool_call_id": toolCallID,
				"args_size":    len(args),
			}, "process message tool call started")
			toolResult, ok := tool.ExecuteWithContextAndCancel(toolExecCtx, toolCtx, name, args)
			result := toolResult.VisibleText
			if result == "" {
				result = toolResult.Content
			}
			if toolResult.ForceVoice && toolResult.Status == tool.ToolStatusSuccess {
				forceVoice = true
			}
			applog.TraceInfo(trace.WithStage("tool_call_completed"), applog.Fields{
				"round":        round,
				"tool_name":    name,
				"tool_call_id": toolCallID,
				"ok":           ok,
				"status":       string(toolResult.Status),
				"error_code":   toolResult.ErrorCode,
				"result_size":  len(result),
				"force_voice":  toolResult.ForceVoice,
			}, "process message tool call completed")
			messages = append(messages, map[string]interface{}{"role": "tool", "tool_call_id": tc["id"], "content": result})
		}
	}
	return reply, forceVoice, totalTokens, nil
}

type processPromptInput struct {
	BaseIdentity             string
	CharacterConfig          string
	PersonalityConfig        string
	PersonalityRaw           string
	ProfileContext           string
	MemoryContext            string
	Worldbook                string
	EmotionFusionRaw         string
	AdultIntimacyRaw         string
	MemoryInjectRaw          string
	History                  []map[string]string
	AntiRepeatRaw            string
	Runtime                  *interaction.RuntimeAssembly
	StyleInstruction         string
	ProactiveScene           string
	ProactiveTimeContext     string
	ProactiveRecentContext   string
	ProactivePersonality     string
	ProactiveRelationship    string
	ProactiveEmotion         string
	ProactiveMemory          string
	ProactiveTaskInstruction string
	UserContent              string
}

func buildProcessPromptMessages(input processPromptInput) ([]map[string]interface{}, *promptir.PromptTrace) {
	gateway := promptir.NewGateway()

	runtimePlan := buildBehaviorPlanFromRuntime(input.Runtime)
	expressionPlan := buildExpressionPlanFromRuntime(input.Runtime)
	if input.StyleInstruction != "" {
		if expressionPlan != "" {
			expressionPlan += "\n" + input.StyleInstruction
		} else {
			expressionPlan = input.StyleInstruction
		}
	}
	gwMessages, promptTrace, err := gateway.BuildMessages(promptir.BuildRequest{
		CharacterConfig:          input.CharacterConfig,
		CompiledPersonality:      input.PersonalityConfig,
		BaseIdentity:             input.BaseIdentity,
		PersonalityRaw:           input.PersonalityRaw,
		EmotionFusionRaw:         input.EmotionFusionRaw,
		AdultIntimacyRaw:         input.AdultIntimacyRaw,
		MemoryInjectRaw:          input.MemoryInjectRaw,
		AntiRepeatRaw:            input.AntiRepeatRaw,
		ProfileContext:           input.ProfileContext,
		MemoryContext:            input.MemoryContext,
		Worldbook:                input.Worldbook,
		RuntimePlan:              runtimePlan,
		ExpressionPlan:           expressionPlan,
		History:                  renderHistoryForPromptIR(input.History),
		CurrentUserInput:         input.UserContent,
		ProactiveTaskInstruction: input.ProactiveTaskInstruction,
		ProactiveScene:           input.ProactiveScene,
		ProactiveTimeContext:     input.ProactiveTimeContext,
		ProactiveRecentContext:   input.ProactiveRecentContext,
		ProactivePersonality:     input.ProactivePersonality,
		ProactiveRelationship:    input.ProactiveRelationship,
		ProactiveEmotion:         input.ProactiveEmotion,
		ProactiveMemory:          input.ProactiveMemory,
	})
	if err != nil {
		applog.Warn("prompt gateway build failed, trying minimal build", applog.Fields{"error": err.Error()})
		gwMessages, promptTrace, err = gateway.BuildMessages(promptir.BuildRequest{
			CharacterConfig:          input.CharacterConfig,
			CompiledPersonality:      input.PersonalityConfig,
			BaseIdentity:             input.BaseIdentity,
			PersonalityRaw:           input.PersonalityRaw,
			EmotionFusionRaw:         input.EmotionFusionRaw,
			AdultIntimacyRaw:         input.AdultIntimacyRaw,
			MemoryInjectRaw:          input.MemoryInjectRaw,
			RuntimePlan:              runtimePlan,
			AntiRepeatRaw:            input.AntiRepeatRaw,
			ExpressionPlan:           expressionPlan,
			ProactivePersonality:     input.ProactivePersonality,
			ProactiveRelationship:    input.ProactiveRelationship,
			ProactiveEmotion:         input.ProactiveEmotion,
			ProactiveMemory:          input.ProactiveMemory,
			CurrentUserInput:         input.UserContent,
			ProactiveTaskInstruction: input.ProactiveTaskInstruction,
		})
		if err != nil {
			applog.Warn("minimal prompt build also failed, falling back to raw", applog.Fields{"error": err.Error()})
			safeContent := promptir.SanitizeCurrentUserMessage(input.UserContent)
			return []map[string]interface{}{
				{"role": "user", "content": "<current_user_message>\n" + safeContent + "\n</current_user_message>"},
			}, nil
		}
	}

	messages := make([]map[string]interface{}, 0, len(gwMessages))
	for _, m := range gwMessages {
		messages = append(messages, map[string]interface{}{"role": m.Role, "content": m.Content})
	}
	return messages, promptTrace
}

func renderHistoryForPromptIR(history []map[string]string) string {
	lines := make([]string, 0, len(history))
	for _, msg := range history {
		role := strings.TrimSpace(msg["role"])
		content := strings.TrimSpace(msg["content"])
		if content == "" {
			continue
		}
		if role == "" {
			role = "unknown"
		}
		lines = append(lines, role+": "+content)
	}
	return strings.Join(lines, "\n")
}

func (s *service) ProcessMessageCtx(ctx context.Context, req *interaction.ProcessRequest) (*interaction.ProcessResponse, error) {
	chatReq := &ProcessMessageRequest{
		CharacterID:              req.CharacterID,
		Message:                  req.Message,
		ConversationID:           req.ConversationID,
		Channel:                  req.Channel,
		Source:                   req.Source,
		PeerID:                   req.PeerID,
		UserID:                   req.UserID,
		AudioUrl:                 req.AudioUrl,
		AudioDuration:            req.AudioDuration,
		VoiceMessage:             req.VoiceMessage,
		ImageUrl:                 req.ImageUrl,
		VideoUrl:                 req.VideoUrl,
		ImageContext:             req.ImageContext,
		RequestID:                req.RequestID,
		InteractionID:            req.InteractionID,
		ExpectedStatusVersion:    req.ExpectedStatusVersion,
		Runtime:                  req.Runtime,
		IsInternal:               req.IsInternal,
		ProactiveTimeContext:     req.ProactiveTimeContext,
		ProactiveRecentContext:   req.ProactiveRecentContext,
		ProactiveTaskInstruction: req.ProactiveTaskInstruction,
		ProactiveRelationship:    req.ProactiveRelationship,
		ProactiveEmotion:         req.ProactiveEmotion,
		ProactiveMemory:          req.ProactiveMemory,
	}
	computeResult, err := s.ComputeInteraction(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	if computeResult.HasExistingUser {
		return &interaction.ProcessResponse{
			ConversationID: computeResult.ConversationID,
			Reply:          computeResult.Reply,
			Lines:          computeResult.Lines,
			CharacterID:    computeResult.CharacterID,
			CharacterName:  computeResult.CharacterName,
			RequestID:      computeResult.RequestID,
		}, nil
	}
	commitResult, err := s.commitInteraction(messageCommitPlan{
		Request:       chatReq,
		Conversation:  computeResult.ConversationID,
		Character:     computeResult.CharacterID,
		CharacterName: computeResult.CharacterName,
		UserMessageID: computeResult.UserMessageID,
		Reply:         computeResult.Reply,
		Lines:         computeResult.Lines,
		Source:        computeResult.Source,
		TotalTokens:   computeResult.TotalTokens,
		Runtime:       req.Runtime,
	})
	if err != nil {
		s.db.Model(&Message{}).Where("id = ?", computeResult.UserMessageID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
		return nil, err
	}
	s.PostCommitActions(ctx, computeResult)
	return &interaction.ProcessResponse{
		ConversationID: computeResult.ConversationID,
		Sequence:       commitResult.LastSequence,
		Reply:          computeResult.Reply,
		Lines:          computeResult.Lines,
		CharacterID:    computeResult.CharacterID,
		CharacterName:  computeResult.CharacterName,
		MessageIDs:     commitResult.MessageIDs,
		ForceVoice:     computeResult.ForceVoice,
		RequestID:      computeResult.RequestID,
		Events:         commitResult.Events,
	}, nil
}

func buildBehaviorPlanFromRuntime(runtime *interaction.RuntimeAssembly) string {
	if runtime == nil {
		return ""
	}
	if runtime.BehaviorPlan == nil {
		return buildMinimalRuntimeContextPrompt(runtime)
	}
	bp := runtime.BehaviorPlan
	var lines []string
	if bp.Intent != "" {
		lines = append(lines, "意图: "+bp.Intent)
	}
	if bp.Strategy != "" {
		lines = append(lines, "策略: "+bp.Strategy)
	}
	if bp.ResponseGoal != "" {
		lines = append(lines, "回复目标: "+bp.ResponseGoal)
	}
	if bp.ToneHint != "" {
		lines = append(lines, "语气提示: "+bp.ToneHint)
	}
	if len(bp.AllowedTopics) > 0 {
		lines = append(lines, "允许话题: "+strings.Join(bp.AllowedTopics, " / "))
	}
	if len(bp.ForbiddenTopics) > 0 {
		lines = append(lines, "禁止话题: "+strings.Join(bp.ForbiddenTopics, " / "))
	}
	if bp.Priority != "" {
		lines = append(lines, "优先级: "+string(bp.Priority))
	}
	if bp.SafetyLevel != "" {
		lines = append(lines, "安全级别: "+string(bp.SafetyLevel))
	}
	return strings.Join(lines, "\n")
}

func buildMinimalRuntimeContextPrompt(runtime *interaction.RuntimeAssembly) string {
	var lines []string
	if runtime.Path != "" {
		lines = append(lines, "路径: "+string(runtime.Path))
	}
	if len(runtime.Safety.Reasons) > 0 {
		lines = append(lines, "安全因素: "+strings.Join(runtime.Safety.Reasons, ", "))
	}
	return strings.Join(lines, "\n")
}

func buildExpressionPlanFromRuntime(runtime *interaction.RuntimeAssembly) string {
	if runtime == nil || runtime.ExpressionPlan == nil {
		return ""
	}
	ep := runtime.ExpressionPlan
	var lines []string

	lines = append(lines, "【回复约束 - 必须遵守】")

	switch ep.Length {
	case "short":
		lines = append(lines, "回复长度: 短（1-3句话，不超过80字）")
	case "medium":
		lines = append(lines, "回复长度: 中（3-6句话）")
	case "long":
		lines = append(lines, "回复长度: 长（5句以上，可充分展开）")
	default:
		lines = append(lines, "回复长度: 适中")
	}

	switch ep.Tone {
	case "warm":
		lines = append(lines, "语气: 温暖亲切，用词柔和有温度")
	case "neutral":
		lines = append(lines, "语气: 中性平和，客观自然")
	case "firm":
		lines = append(lines, "语气: 坚定明确，态度清晰")
	case "soft":
		lines = append(lines, "语气: 柔和克制，避免强烈表达")
	case "playful":
		lines = append(lines, "语气: 俏皮活泼，可适当幽默")
	case "concerned":
		lines = append(lines, "语气: 关切体贴，表达在意和关注")
	default:
		lines = append(lines, "语气: 自然适中")
	}

	if ep.EmotionIntensity > 0 {
		var emotionLabel string
		switch {
		case ep.EmotionIntensity < 0.3:
			emotionLabel = "低（克制冷静，减少情绪化表达）"
		case ep.EmotionIntensity < 0.6:
			emotionLabel = "中（适度表达情绪）"
		default:
			emotionLabel = "高（充分表达情绪感受）"
		}
		lines = append(lines, "情绪表达强度: "+emotionLabel)
	}

	switch ep.ExpressionType {
	case "question":
		lines = append(lines, "表达类型: 提问 - 应在回复中包含追问以推进对话")
	case "statement":
		lines = append(lines, "表达类型: 陈述 - 以提供信息和建议为主")
	case "greeting":
		lines = append(lines, "表达类型: 问候 - 以打招呼和寒暄为主")
	case "boundary":
		lines = append(lines, "表达类型: 设立边界 - 明确表达底线，不展开")
	case "silence":
		lines = append(lines, "表达类型: 沉默 - 尽量简短或不回复实质内容")
	}

	if ep.Suppressed {
		lines = append(lines, "表达抑制: 已启用 - 整体表达应该更加克制和收敛")
	}

	lines = append(lines, "内容密度: 每次回复必须包含有效信息，不使用无意义附和")
	lines = append(lines, "结论优先: 对技术/项目/代码/架构/审计/方案类问题，先给明确结论再解释")
	lines = append(lines, "禁用默认开头: 不得使用\"嗯嗯\"\"你说得对\"\"这个想法挺好\"\"我理解你\"\"确实可以\"\"稍微优化一下\"作为默认开头或万能缓冲句")
	lines = append(lines, "可以反驳: 允许指出用户错误和不完整，不默认认同用户")

	return strings.Join(lines, "\n")
}

func (s *service) updatePsycheState(charID string) error {
	if s.psycheStore == nil || charID == "" {
		return nil
	}
	return s.updatePsycheStateWithStore(s.psycheStore, charID, nil)
}
