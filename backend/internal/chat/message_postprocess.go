// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"fmt"

	applog "github.com/u-ai/backend/log"
)

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
		applog.TraceWarn(trace.WithStage("postprocess_skipped_cancelled"), applog.Fields{"conversation_id": convID}, "process message postprocess skipped because request context was cancelled")
		return
	}
	if s.outboxStore == nil {
		return
	}
	payload := PostProcessPayload{Version: postProcessPayloadVersion, ConversationID: convID, CharacterID: charID, Source: source, RequestID: requestID, Reply: reply, PipelineMessages: pipelineMessages}
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

func (s *service) updatePsycheState(charID string) error {
	if s.psycheStore == nil || charID == "" {
		return nil
	}
	return s.updatePsycheStateWithStore(s.psycheStore, charID, nil)
}
