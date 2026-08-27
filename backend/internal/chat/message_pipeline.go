// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/interaction"
	applog "github.com/u-ai/backend/log"
)

func (s *service) ProcessMessage(ctx context.Context, req *ProcessMessageRequest) (*ProcessMessageResponse, error) {
	computeResult, err := s.ComputeInteraction(ctx, req)
	if err != nil {
		s.emitDesktopPetChat(ctx, req, req.CharacterID, req.ConversationID, "", "response.failed", 5)
		return nil, err
	}
	if computeResult.HasExistingUser {
		s.db.Model(&Message{}).Where("id = ?", computeResult.UserMessageID).Updates(map[string]interface{}{"status": "sent", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
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
		ForceVoice:    computeResult.ForceVoice,
		Runtime:       req.Runtime,
	})
	if err != nil {
		s.db.Model(&Message{}).Where("id = ?", computeResult.UserMessageID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
		s.emitDesktopPetChat(ctx, req, computeResult.CharacterID, computeResult.ConversationID, computeResult.UserMessageID, "response.failed", 5)
		return nil, err
	}
	s.emitDesktopPetChat(ctx, req, computeResult.CharacterID, computeResult.ConversationID, computeResult.UserMessageID, "response.completed", 5)
	s.PostCommitActions(ctx, computeResult)
	s.dispatchPluginAfterReply(req, computeResult, commitResult.MessageIDs)
	s.db.Model(&Message{}).Where("id = ?", computeResult.UserMessageID).Updates(map[string]interface{}{"status": "sent", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
	applog.TraceInfo(computeResult.Trace.WithStage("db_commit_completed"), applog.Fields{"message_count": len(commitResult.MessageIDs)}, "process message db commit completed")
	applog.TraceInfo(computeResult.Trace.WithStage("completed"), applog.Fields{"reply_size": len(computeResult.Reply)}, "process message completed")
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
		MessagePlan:    commitResult.MessagePlan,
		Events:         commitResult.Events,
	}, nil
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

func (s *service) ProcessMessageCtx(ctx context.Context, req *interaction.ProcessRequest) (*interaction.ProcessResponse, error) {
	chatReq := &ProcessMessageRequest{
		CharacterID:              req.CharacterID,
		Message:                  req.Message,
		ConversationID:           req.ConversationID,
		Channel:                  req.Channel,
		Source:                   req.Source,
		PeerID:                   req.PeerID,
		UserID:                   req.UserID,
		SessionID:                req.SessionID,
		AudioUrl:                 req.AudioUrl,
		AudioDuration:            req.AudioDuration,
		VoiceMessage:             req.VoiceMessage,
		ImageUrl:                 req.ImageUrl,
		VideoUrl:                 req.VideoUrl,
		ImageContext:             req.ImageContext,
		RequestID:                req.RequestID,
		InteractionID:            req.InteractionID,
		ReplyToMessageID:         req.ReplyToMessageID,
		ExpectedStatusVersion:    req.ExpectedStatusVersion,
		Runtime:                  req.Runtime,
		ExecContext:              req.ExecContext,
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
		s.emitDesktopPetChat(ctx, chatReq, chatReq.CharacterID, chatReq.ConversationID, "", "response.failed", 5)
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
		ForceVoice:    computeResult.ForceVoice,
		Runtime:       req.Runtime,
	})
	if err != nil {
		s.db.Model(&Message{}).Where("id = ?", computeResult.UserMessageID).Updates(map[string]interface{}{"status": "failed", "updated_at": time.Now().Format("2006-01-02 15:04:05")})
		s.emitDesktopPetChat(ctx, chatReq, computeResult.CharacterID, computeResult.ConversationID, computeResult.UserMessageID, "response.failed", 5)
		return nil, err
	}
	s.emitDesktopPetChat(ctx, chatReq, computeResult.CharacterID, computeResult.ConversationID, computeResult.UserMessageID, "response.completed", 5)
	s.PostCommitActions(ctx, computeResult)
	s.dispatchPluginAfterReply(chatReq, computeResult, commitResult.MessageIDs)
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
		MessagePlan:    commitResult.MessagePlan,
		Events:         commitResult.Events,
	}, nil
}

func (s *service) dispatchPluginAfterReply(req *ProcessMessageRequest, result *ComputeResult, messageIDs []string) {
	if result == nil || result.HasExistingUser || result.Reply == "" {
		return
	}
	if s.toolRuntime == nil {
		return
	}
	messageID := ""
	if len(messageIDs) > 0 {
		messageID = messageIDs[len(messageIDs)-1]
	}
	scope := extension.ExecutionScope{UserID: req.UserID, CharacterID: result.CharacterID, ConversationID: result.ConversationID, Channel: result.Channel, SessionID: req.SessionID, TraceID: result.RequestID, RequestID: result.RequestID, CorrelationID: result.Trace.CorrelationID, CausationID: result.Trace.CausationID}
	replyView := extension.ReplyView{MessageID: messageID, CharacterID: result.CharacterID, ConversationID: result.ConversationID, Channel: result.Channel, Content: result.Reply, CreatedAt: time.Now().UTC()}
	if s.toolRuntime != nil {
		toolScope := toolScopeFromExtension(scope)
		toolReply := ReplyView{MessageID: replyView.MessageID, CharacterID: replyView.CharacterID, ConversationID: replyView.ConversationID, Channel: replyView.Channel, Content: replyView.Content, CreatedAt: replyView.CreatedAt}
		s.toolRuntime.AfterReply(toolScope, toolReply)
		return
	}
}
