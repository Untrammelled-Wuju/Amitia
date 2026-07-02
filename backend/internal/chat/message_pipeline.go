// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/expression"
	"github.com/u-ai/backend/internal/interaction"
	applog "github.com/u-ai/backend/log"
)

func (s *service) ProcessMessage(ctx context.Context, req *ProcessMessageRequest) (*ProcessMessageResponse, error) {
	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		requestID = uuid.New().String()
	}
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = "web"
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "manual"
	}
	trace := newProcessTrace(requestID, strings.TrimSpace(req.ConversationID), strings.TrimSpace(req.CharacterID), channel)
	if strings.TrimSpace(req.PeerID) != "" {
		trace.User = strings.TrimSpace(req.PeerID)
	}
	applog.TraceInfo(trace.WithStage("input_received"), applog.Fields{
		"source":        source,
		"message_size":  len(req.Message),
		"voice_message": req.VoiceMessage,
		"has_audio":     strings.TrimSpace(req.AudioUrl) != "",
		"has_image":     strings.TrimSpace(req.ImageUrl) != "",
		"has_video":     strings.TrimSpace(req.VideoUrl) != "",
	}, "process message input received")
	runtimeProfile, err := s.getRoleRuntimeProfile(req.CharacterID)
	if err != nil {
		applog.TraceError(trace.WithStage("runtime_profile_load_failed"), nil, err, "process message runtime profile load failed")
		if req.CharacterID != "" {
			return nil, fmt.Errorf("角色不存在")
		}
		return nil, fmt.Errorf("没有可用角色")
	}
	charID := runtimeProfile.CharacterID
	charName := runtimeProfile.Name
	convID := req.ConversationID
	if convID == "" {
		var existing struct{ ID string }
		err := s.db.Table("conversations").Select("id").Where("character_id = ? AND channel = ?", charID, channel).Order("updated_at DESC").Limit(1).Row().Scan(&existing.ID)
		if err == nil && existing.ID != "" {
			convID = existing.ID
		} else {
			convID = uuid.New().String()
			s.repo.CreateConversation(&Conversation{ID: convID, CharacterID: charID, Title: req.Message, Channel: channel})
		}
	} else if err := s.validateConversationScope(convID, charID, channel); err != nil {
		applog.TraceError(trace.WithStage("conversation_scope_invalid"), applog.Fields{
			"conversation_id": convID,
			"channel":         channel,
			"character_id":    charID,
		}, err, "process message conversation scope invalid")
		return nil, fmt.Errorf("会话与角色或渠道不匹配")
	}
	trace = updateProcessTraceScope(trace, convID, charID, channel)
	existingUser, existingAssistants, hasExistingUser := s.findRequestMessages(convID, requestID)
	if len(existingAssistants) > 0 {
		applog.TraceInfo(trace.WithStage("idempotent_hit"), applog.Fields{
			"assistant_count": len(existingAssistants),
		}, "process message idempotent hit")
		msgIDs := make([]string, 0, len(existingAssistants))
		parts := make([]string, 0, len(existingAssistants))
		for _, msg := range existingAssistants {
			msgIDs = append(msgIDs, msg.ID)
			parts = append(parts, msg.Content)
		}
		return &ProcessMessageResponse{
			ConversationID: convID,
			Reply:          strings.Join(parts, "\n"),
			CharacterID:    charID,
			CharacterName:  charName,
			MessageIDs:     msgIDs,
			UserMessage:    &MessageItem{ID: existingUser.ID, ConversationID: convID, Sequence: existingUser.Sequence, Role: "user", Content: existingUser.Content, Source: existingUser.Source, CreatedAt: existingUser.CreatedAt},
			UserMessageID:  existingUser.ID,
			RequestID:      requestID,
		}, nil
	}
	userMsgID := existingUser.ID
	userMsgSequence := existingUser.Sequence
	userMsgCreatedAt := existingUser.CreatedAt
	if !hasExistingUser {
		userMsg := &Message{ID: uuid.New().String(), ConversationID: convID, Role: "user", Content: req.Message, MsgType: "text", Source: source, Status: "processing", AudioUrl: req.AudioUrl, AudioDuration: req.AudioDuration, ImageUrl: req.ImageUrl, VideoUrl: req.VideoUrl, RequestID: requestID}
		userMsgID = userMsg.ID
		if err := s.repo.CreateMessage(userMsg); err != nil {
			applog.TraceError(trace.WithStage("user_message_persist_failed"), applog.Fields{
				"user_message_id": userMsgID,
			}, err, "process message user message persist failed")
			return nil, err
		}
		userMsgSequence = userMsg.Sequence
		userMsgCreatedAt = userMsg.CreatedAt
		applog.TraceInfo(trace.WithStage("user_message_persisted"), applog.Fields{
			"user_message_id": userMsgID,
			"status":          "processing",
		}, "process message user message persisted")
	} else {
		s.db.Model(&Message{}).Where("id = ?", userMsgID).Updates(map[string]interface{}{"status": "processing", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
		applog.TraceInfo(trace.WithStage("user_message_reused"), applog.Fields{
			"user_message_id": userMsgID,
			"status":          "processing",
		}, "process message user message reused")
	}

	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		s.db.Model(&Message{}).Where("id = ?", userMsgID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
		applog.TraceError(trace.WithStage("model_config_missing"), applog.Fields{
			"user_message_id": userMsgID,
		}, err, "process message model config missing")
		return nil, fmt.Errorf("没有可用的模型配置")
	}

	sys1Parts := s.sys1Builder(runtimeProfile, req.Message)
	history := s.loadHistoryExcluding(convID, userMsgID)
	sys2Parts := s.sys2Builder(convID, charID, requestID, req.Channel, req.Message)

	messages := []map[string]interface{}{}
	if len(sys1Parts) > 0 {
		messages = append(messages, map[string]interface{}{"role": "system", "content": strings.Join(sys1Parts, "\n\n")})
	}
	for _, m := range history {
		messages = append(messages, map[string]interface{}{"role": m["role"], "content": m["content"]})
	}
	if len(sys2Parts) > 0 {
		messages = append(messages, map[string]interface{}{"role": "system", "content": strings.Join(sys2Parts, "\n\n")})
	}
	if strings.TrimSpace(req.RuntimeContext) != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": req.RuntimeContext})
	}

	kind := expression.ChannelWeb
	switch req.Channel {
	case "wechat":
		kind = expression.ChannelWechat
	case "qq":
		kind = expression.ChannelQQ
	}
	prompt := expression.CompileChannelPrompt(kind)
	if prompt.StyleInstruction != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": prompt.StyleInstruction})
	}

	userContent := req.Message
	if req.ImageContext != "" {
		userContent = req.ImageContext + "\n\n用户问：" + req.Message
	}
	applog.TraceInfo(trace.WithStage("prompt_ready"), applog.Fields{
		"history_count":      len(history),
		"system_part_count":  len(sys1Parts) + len(sys2Parts),
		"tool_count":         len(tool.GetAll()),
		"image_context_size": len(req.ImageContext),
	}, "process message prompt ready")
	messages = append(messages, map[string]interface{}{"role": "user", "content": userContent})
	toolDefs := tool.GetAll()
	seenTools := map[string]bool{}
	toolExecCtx, cancelTools := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTools()
	forceVoice := false

	reply, forceVoice, llmErr := s.invokeLLMWithTools(ctx, cfg, messages, trace, userMsgID, convID, charID, channel, requestID, toolDefs, seenTools, toolExecCtx)
	if llmErr != nil {
		return nil, llmErr
	}
	if reply == "" {
		applog.TraceWarn(trace.WithStage("reply_fallback"), nil, "process message reply fallback")
		reply = "操作已完成"
	}

	kind = expression.ChannelWeb
	switch channel {
	case "wechat":
		kind = expression.ChannelWechat
	case "qq":
		kind = expression.ChannelQQ
	}
	reply = expression.ApplyPostValidation(reply, kind)

	lines_ := strings.Split(strings.TrimSpace(reply), "\n")
	var realLines []string
	for _, line := range lines_ {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		realLines = append(realLines, line)
	}
	if len(realLines) == 0 {
		realLines = []string{reply}
	}
	if err := s.abortMessageCommitIfCancelled(ctx, trace, userMsgID); err != nil {
		return nil, err
	}
	var msgIDs []string
	var audioUrls []string
	applog.TraceInfo(trace.WithStage("db_commit_started"), applog.Fields{
		"reply_line_count": len(realLines),
		"user_message_id":  userMsgID,
	}, "process message db commit started")
	commitResult, err := s.commitInteraction(messageCommitPlan{
		Request:       req,
		Conversation:  convID,
		Character:     charID,
		CharacterName: charName,
		UserMessageID: userMsgID,
		Reply:         reply,
		Lines:         realLines,
		Source:        source,
		Runtime:       req.Runtime,
	})
	if err != nil {
		s.db.Model(&Message{}).Where("id = ?", userMsgID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
		applog.TraceError(trace.WithStage("db_commit_failed"), applog.Fields{
			"user_message_id": userMsgID,
		}, err, "process message db commit failed")
		return nil, err
	}
	msgIDs = commitResult.MessageIDs
	applog.TraceInfo(trace.WithStage("db_commit_completed"), applog.Fields{
		"user_message_id": userMsgID,
		"assistant_count": len(msgIDs),
	}, "process message db commit completed")

	if s.wmCache != nil {
		s.wmCache.UpdateSummary(convID, reply)
	}
	pipelineMessages := make([]map[string]string, 0, len(history)+2)
	pipelineMessages = append(pipelineMessages, history...)
	pipelineMessages = append(pipelineMessages, map[string]string{"role": "user", "content": req.Message})
	pipelineMessages = append(pipelineMessages, map[string]string{"role": "assistant", "content": reply})
	s.startPostProcessing(ctx, trace, convID, charID, source, pipelineMessages, reply)

	applog.TraceInfo(trace.WithStage("completed"), applog.Fields{
		"user_message_id": userMsgID,
		"assistant_count": len(msgIDs),
		"reply_size":      len(reply),
		"force_voice":     forceVoice,
	}, "process message completed")
	return &ProcessMessageResponse{
		ConversationID: convID,
		Reply:          reply,
		CharacterID:    charID,
		CharacterName:  charName,
		MessageIDs:     msgIDs,
		ForceVoice:     forceVoice,
		AudioUrls:      audioUrls,
		UserMessage:    &MessageItem{ID: userMsgID, ConversationID: convID, Sequence: userMsgSequence, Role: "user", Content: req.Message, Source: source, CreatedAt: userMsgCreatedAt},
		UserMessageID:  userMsgID,
		RequestID:      requestID,
		Events:         commitResult.Events,
	}, nil
}

func (s *service) startPostProcessing(ctx context.Context, trace applog.TraceFields, convID, charID, source string, pipelineMessages []map[string]string, reply string) {
	if err := ctx.Err(); err != nil {
		applog.TraceWarn(trace.WithStage("postprocess_skipped_cancelled"), applog.Fields{
			"conversation_id": convID,
		}, "process message postprocess skipped because request context was cancelled")
		return
	}
	if s.pipeline != nil {
		go func() {
			postCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if postCtx.Err() != nil {
				return
			}
			s.pipeline.Execute(postCtx, convID, pipelineMessages, reply)
		}()
	}
	go func() {
		postCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		s.trimContextWindow(postCtx, convID)
	}()
	go func() {
		postCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		s.moodRecoveryCheck(postCtx, convID, charID, source)
	}()
	if s.compressor != nil {
		go func() {
			postCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			s.compressor.MaybeCompress(postCtx, convID)
		}()
	}
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

func (s *service) ProcessMessageCtx(ctx context.Context, req *interaction.ProcessRequest) (*interaction.ProcessResponse, error) {
	chatReq := &ProcessMessageRequest{
		CharacterID:           req.CharacterID,
		Message:               req.Message,
		ConversationID:        req.ConversationID,
		Channel:               req.Channel,
		Source:                req.Source,
		PeerID:                req.PeerID,
		AudioUrl:              req.AudioUrl,
		AudioDuration:         req.AudioDuration,
		VoiceMessage:          req.VoiceMessage,
		ImageUrl:              req.ImageUrl,
		VideoUrl:              req.VideoUrl,
		ImageContext:          req.ImageContext,
		RequestID:             req.RequestID,
		RuntimeContext:        buildRuntimeContextPrompt(req.Runtime),
		InteractionID:         req.InteractionID,
		ExpectedStatusVersion: req.ExpectedStatusVersion,
		Runtime:               req.Runtime,
	}
	resp, err := s.ProcessMessage(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	return convertProcessMessageResponse(resp), nil
}

func buildRuntimeContextPrompt(runtime *interaction.RuntimeAssembly) string {
	if runtime == nil {
		return ""
	}
	payload := map[string]interface{}{
		"path":        runtime.Path,
		"safety":      runtime.Safety,
		"delivery":    runtime.Delivery,
		"transaction": runtime.Transaction.Name,
		"context": map[string]interface{}{
			"snapshotVersion": runtime.Context.SnapshotVersion(),
			"psyche":          runtime.Context.Psyche,
			"relationship":    runtime.Context.Relationship,
			"beliefs":         runtime.Context.Beliefs,
			"channel":         runtime.Context.Channel,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return "运行时编排上下文：" + string(raw)
}

func (s *service) updatePsycheState(charID string) error {
	if s.psycheStore == nil || charID == "" {
		return nil
	}
	return s.updatePsycheStateWithStore(s.psycheStore, charID)
}
