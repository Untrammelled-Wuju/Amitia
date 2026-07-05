package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/expression"
	applog "github.com/u-ai/backend/log"
)

type ComputeResult struct {
	RequestID            string
	ConversationID       string
	CharacterID          string
	CharacterName        string
	UserMessageID        string
	UserMessageSequence  int64
	UserMessageCreatedAt string
	Reply                string
	Lines                []string
	Source               string
	ForceVoice           bool
	HasExistingUser      bool
	Channel              string
	Trace                applog.TraceFields
	PipelineMessages     []map[string]string
}

func (s *service) ComputeInteraction(ctx context.Context, req *ProcessMessageRequest) (*ComputeResult, error) {
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
	} else if strings.TrimSpace(req.CharacterID) != "" {
		trace.User = strings.TrimSpace(req.CharacterID)
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
		return &ComputeResult{
			RequestID:            requestID,
			ConversationID:       convID,
			CharacterID:          charID,
			CharacterName:        charName,
			UserMessageID:        existingUser.ID,
			UserMessageSequence:  existingUser.Sequence,
			UserMessageCreatedAt: existingUser.CreatedAt,
			Reply: strings.TrimSpace(strings.Join(func() []string {
				parts := make([]string, len(existingAssistants))
				for i, msg := range existingAssistants {
					parts[i] = msg.Content
				}
				return parts
			}(), "\n")),
			Source:          source,
			HasExistingUser: true,
			Channel:         channel,
			Trace:           trace,
		}, nil
	}
	userMsgID := existingUser.ID
	userMsgSequence := existingUser.Sequence
	userMsgCreatedAt := existingUser.CreatedAt
	if !hasExistingUser {
		if req.IsInternal {
			userMsgID = uuid.New().String()
		} else {
			msgRole := "user"
			if source == "proactive" {
				msgRole = "system"
			}
			userMsg := &Message{ID: uuid.New().String(), ConversationID: convID, Role: msgRole, Content: req.Message, MsgType: "text", Source: source, Status: "processing", AudioUrl: req.AudioUrl, AudioDuration: req.AudioDuration, ImageUrl: req.ImageUrl, VideoUrl: req.VideoUrl, RequestID: requestID}
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
		}
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

	sys1Parts := s.sys1Builder(runtimeProfile, req.Message, req.Runtime)
	history := s.loadHistoryExcluding(convID, userMsgID)
	sys2Parts := s.sys2Builder(convID, charID, requestID, req.Channel, req.Message)

	kind := expression.ChannelWeb
	switch req.Channel {
	case "wechat":
		kind = expression.ChannelWechat
	case "qq":
		kind = expression.ChannelQQ
	}
	channelPrompt := expression.CompileChannelPrompt(kind)

	userContent := req.Message
	if req.ImageContext != "" {
		userContent = req.ImageContext + "\n\n用户问：" + req.Message
	}
	messages := buildProcessPromptMessages(processPromptInput{
		Sys1Parts:        sys1Parts,
		Sys2Parts:        sys2Parts,
		History:          history,
		Runtime:          req.Runtime,
		StyleInstruction: channelPrompt.StyleInstruction,
		UserContent:      userContent,
	})
	applog.TraceInfo(trace.WithStage("prompt_ready"), applog.Fields{
		"history_count":      len(history),
		"system_part_count":  len(sys1Parts) + len(sys2Parts),
		"tool_count":         len(tool.GetAll()),
		"image_context_size": len(req.ImageContext),
	}, "process message prompt ready")
	toolDefs := tool.GetAll()
	seenTools := map[string]bool{}
	toolExecCtx, cancelTools := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTools()

	if err := s.abortMessageCommitIfCancelled(ctx, trace, userMsgID); err != nil {
		return nil, err
	}
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

	pipelineMessages := make([]map[string]string, 0, len(history)+2)
	pipelineMessages = append(pipelineMessages, history...)
	pipelineMessages = append(pipelineMessages, map[string]string{"role": "user", "content": req.Message})
	pipelineMessages = append(pipelineMessages, map[string]string{"role": "assistant", "content": reply})

	return &ComputeResult{
		RequestID:            requestID,
		ConversationID:       convID,
		CharacterID:          charID,
		CharacterName:        charName,
		UserMessageID:        userMsgID,
		UserMessageSequence:  userMsgSequence,
		UserMessageCreatedAt: userMsgCreatedAt,
		Reply:                reply,
		Lines:                realLines,
		Source:               source,
		ForceVoice:           forceVoice,
		HasExistingUser:      hasExistingUser,
		Channel:              channel,
		Trace:                trace,
		PipelineMessages:     pipelineMessages,
	}, nil
}

func (s *service) PostCommitActions(ctx context.Context, result *ComputeResult) {
	if result.HasExistingUser {
		return
	}
	if s.wmCache != nil {
		s.wmCache.UpdateSummary(result.ConversationID, result.Reply)
	}
	s.startPostProcessing(ctx, result.Trace, result.ConversationID, result.CharacterID, result.Source, result.RequestID, result.PipelineMessages, result.Reply)
}
