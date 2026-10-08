package business

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) transcribeAudio(ctx context.Context, inference *Inference, input coordination.Mutation) (string, int64, error) {
	var audio *Attachment
	for i := range inference.Attachments {
		if inference.Attachments[i].Kind == "audio" {
			audio = &inference.Attachments[i]
		}
	}
	if audio == nil {
		return "", 1, nil
	}
	model, ok := e.model.(AudioModel)
	if !ok {
		return "", 1, errors.New("当前 Core 未提供私有语音识别服务")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return "", 1, err
	}
	text, err := model.TranscribeOwnedAudio(ctx, *inference, *audio)
	if err != nil {
		return "", 1, err
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 64<<10 {
		return "", 1, errors.New("语音转写为空或超过大小限制")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return "", 1, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, inference.Scope); err != nil {
		return "", 1, err
	}
	var document map[string]any
	if err := json.Unmarshal(input.Body, &document); err != nil {
		return "", 1, err
	}
	document["transcription"] = text
	document["transcriptionSourceContent"] = inference.Message
	input.Body = body(document)
	input.ExpectedRevision = 1
	if err := e.submit(ctx, inference.Scope, "transcription", []coordination.Mutation{input}, coordination.ResourceVersion{Kind: "message", ID: input.ID, Revision: 1}); err != nil {
		return "", 1, fmt.Errorf("语音转写尚未确认保存: %w", err)
	}
	inference.Message = text
	if semantic, ok := e.model.(SemanticModel); ok {
		vector, fingerprint, err := semantic.OwnedQueryVector(ctx, text)
		if err != nil {
			return text, 2, err
		}
		query := coordination.DataQuery{RequestID: inference.Scope.RequestID, ConversationID: inference.ConversationID, Query: text, Vector: vector, VectorModel: fingerprint, Limit: 128}
		snapshot, err := e.data.Snapshot(ctx, inference.Scope, query)
		if err != nil {
			return text, 2, err
		}
		if err := coordination.ValidateSnapshot(inference.Scope, snapshot); err != nil {
			return text, 2, err
		}
		resources := make([]coordination.Resource, 0, len(snapshot.Resources))
		for _, resource := range snapshot.Resources {
			if resource.Kind == "message" && (resource.ID == inference.Scope.RequestID+"/user" || resource.ID == inference.Scope.RequestID+"/assistant") {
				continue
			}
			resources = append(resources, resource)
		}
		snapshot.Resources = resources
		inference.Snapshot = snapshot
	}
	if inference.Emit != nil {
		if err := inference.Emit(Event{Type: "transcribed", Text: text}); err != nil {
			return text, 2, err
		}
	}
	return text, 2, nil
}
