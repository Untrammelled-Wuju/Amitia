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

func (s *service) invokeLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, trace applog.TraceFields, userMsgID, convID, charID, channel, requestID string, toolDefs []tool.Tool, seenTools map[string]bool, toolExecCtx context.Context) (string, bool, error) {
	var reply string
	forceVoice := false
	for round := 0; round < 3; round++ {
		applog.TraceInfo(trace.WithStage("model_call_started"), applog.Fields{
			"round":         round,
			"message_count": len(messages),
		}, "process message model call started")
		aiContent, reasoning, toolCalls, _, llmErr := s.invokeProcessLLMWithTools(ctx, cfg, messages, toolDefs)
		if llmErr != nil {
			s.db.Model(&Message{}).Where("id = ?", userMsgID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
			applog.TraceError(trace.WithStage("model_call_failed"), applog.Fields{
				"round":           round,
				"user_message_id": userMsgID,
			}, llmErr, "process message model call failed")
			return "", false, fmt.Errorf("AI 调用失败: %w", llmErr)
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
	return reply, forceVoice, nil
}

type processPromptInput struct {
	Sys1Parts        []string
	Sys2Parts        []string
	History          []map[string]string
	Runtime          *interaction.RuntimeAssembly
	StyleInstruction string
	UserContent      string
}

func buildProcessPromptMessages(input processPromptInput) []map[string]interface{} {
	sections := make([]promptir.Section, 0, 6)
	addSection := func(sectionType promptir.SectionType, priority, tokenBudget int, source string, sensitivity promptir.SensitivityLevel, trimmable, dataOnly bool, content string) {
		if strings.TrimSpace(content) == "" {
			return
		}
		cleaned := promptir.SanitizeContent(content, sensitivity)
		sections = append(sections, promptir.Section{
			Type:        sectionType,
			Priority:    priority,
			TokenBudget: tokenBudget,
			Source:      source,
			Sensitivity: sensitivity,
			Trimmable:   trimmable,
			DataOnly:    dataOnly,
			Content:     cleaned.Content,
		})
	}
	addSection(promptir.SectionTypeIdentity, 1000, 700, "chat.sys1", promptir.SensitivityInternal, false, false, strings.Join(input.Sys1Parts, "\n\n"))
	addSection(promptir.SectionTypeSystem, 980, 700, "chat.sys2", promptir.SensitivityInternal, false, false, strings.Join(input.Sys2Parts, "\n\n"))
	addSection(promptir.SectionTypeBehaviorPlan, 940, 520, "interaction.runtime", promptir.SensitivityInternal, false, true, buildBehaviorPlanFromRuntime(input.Runtime))
	addSection(promptir.SectionTypeBehaviorPlan, 920, 260, "expression.channel", promptir.SensitivityInternal, false, false, input.StyleInstruction)
	addSection(promptir.SectionTypeHistory, 700, 900, "chat.history", promptir.SensitivityUserData, true, true, renderHistoryForPromptIR(input.History))
	addSection(promptir.SectionTypeCurrentInput, 1100, 620, "chat.current_input", promptir.SensitivityUserData, false, false, input.UserContent)

	ir := promptir.CompileIR(sections, promptir.CompileOptions{
		MaxSections:       16,
		MaxTokenBudget:    1200,
		DropEmptySections: true,
	})
	budgeted := promptir.ApplyBudget(ir, promptir.BudgetPolicy{
		MaxPromptTokens: runtimePromptBudget(input.Runtime),
		SectionLimits: map[promptir.SectionType]promptir.SectionBudget{
			promptir.SectionTypeHistory:      {MaxTokens: 900, MinTokens: 0, TrimReason: "history_window_trimmed"},
			promptir.SectionTypeMemory:       {MaxTokens: 700, MinTokens: 0, TrimReason: "memory_window_trimmed"},
			promptir.SectionTypeWorldbook:    {MaxTokens: 500, MinTokens: 0, TrimReason: "worldbook_window_trimmed"},
			promptir.SectionTypeCurrentInput: {MaxTokens: 620, MinTokens: 64, TrimReason: "current_input_trimmed"},
		},
	})

	messages := make([]map[string]interface{}, 0, len(budgeted.Sections))
	currentInputs := make([]string, 0, 1)
	for _, section := range budgeted.Sections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}
		if section.Type == promptir.SectionTypeCurrentInput {
			currentInputs = append(currentInputs, content)
			continue
		}
		messages = append(messages, map[string]interface{}{"role": "system", "content": renderPromptIRSection(section)})
	}
	for _, content := range currentInputs {
		messages = append(messages, map[string]interface{}{"role": "user", "content": content})
	}
	return messages
}

func runtimePromptBudget(runtime *interaction.RuntimeAssembly) int {
	if runtime == nil || len(runtime.Budget) == 0 {
		return 3600
	}
	total := 0
	for _, plan := range runtime.Budget {
		if plan.Allocated > 0 {
			total += plan.Allocated
		}
	}
	if total < 1200 {
		return 1200
	}
	if total > 4800 {
		return 4800
	}
	return total
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

func renderPromptIRSection(section promptir.Section) string {
	header := "[" + string(section.Type) + "]"
	if section.DataOnly {
		header += "[data_only]"
	}
	return header + "\n" + strings.TrimSpace(section.Content)
}

func (s *service) ProcessMessageCtx(ctx context.Context, req *interaction.ProcessRequest) (*interaction.ProcessResponse, error) {
	chatReq := &ProcessMessageRequest{
		CharacterID:           req.CharacterID,
		Message:               req.Message,
		ConversationID:        req.ConversationID,
		Channel:               req.Channel,
		Source:                req.Source,
		PeerID:                req.PeerID,
		UserID:                req.UserID,
		AudioUrl:              req.AudioUrl,
		AudioDuration:         req.AudioDuration,
		VoiceMessage:          req.VoiceMessage,
		ImageUrl:              req.ImageUrl,
		VideoUrl:              req.VideoUrl,
		ImageContext:          req.ImageContext,
		RequestID:             req.RequestID,
		InteractionID:         req.InteractionID,
		ExpectedStatusVersion: req.ExpectedStatusVersion,
		Runtime:               req.Runtime,
		IsInternal:            req.IsInternal,
	}
	computeResult, err := s.ComputeInteraction(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	if computeResult.HasExistingUser {
		return &interaction.ProcessResponse{
			ConversationID: computeResult.ConversationID,
			Reply:          computeResult.Reply,
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

func (s *service) updatePsycheState(charID string) error {
	if s.psycheStore == nil || charID == "" {
		return nil
	}
	return s.updatePsycheStateWithStore(s.psycheStore, charID)
}
