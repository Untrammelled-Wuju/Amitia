package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memory"
	"strings"
	"time"
)

func memorySubmissionFingerprint(values ...string) string {
	encoded, _ := json.Marshal(values)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

type canonicalChatMemoryRuntime struct {
	engine            *business.Engine
	spaceID, deviceID string
	begin             chat.InferenceAuthority
}

func (r *canonicalChatMemoryRuntime) request(spaceID, role string) (business.Request, error) {
	if spaceID != r.spaceID || role == "" {
		return business.Request{}, coordination.ErrWrongOwner
	}
	return business.Request{SpaceID: r.spaceID, DeviceID: r.deviceID, CoreID: r.spaceID, RoleID: role, RequestID: "chat-memory"}, nil
}

func (r *canonicalChatMemoryRuntime) Recall(ctx context.Context, spaceID, role, query string, limit int) ([]memory.HybridSearchResult, error) {
	request, err := r.request(spaceID, role)
	if err != nil {
		return nil, err
	}
	result, err := r.engine.SearchOwnedMemories(ctx, request, business.MemoryManagementQuery{Query: query, Mode: "hybrid", Limit: limit})
	if err != nil {
		return nil, err
	}
	rows := []memory.HybridSearchResult{}
	for _, resource := range result.Resources {
		if !coordination.ResourceUsable(resource.Body, time.Now()) {
			continue
		}
		var document struct {
			Key     string `json:"key"`
			Content struct {
				Value      string `json:"value"`
				Text       string `json:"text"`
				MemoryType string `json:"memoryType"`
				Importance int    `json:"importance"`
			} `json:"content"`
		}
		if json.Unmarshal(resource.Body, &document) != nil {
			return nil, errors.New("记忆记录格式无效")
		}
		value := document.Content.Value
		if value == "" {
			value = document.Content.Text
		}
		rows = append(rows, memory.HybridSearchResult{Memory: memory.Memory{ID: resource.ID, SpaceID: r.spaceID, CharacterID: resource.RoleID, Key: document.Key, Value: value, MemoryType: document.Content.MemoryType, Importance: document.Content.Importance, Version: int(resource.Revision)}, Score: result.Scores[resource.ID], SourceType: "memory"})
	}
	return rows, nil
}

func (r *canonicalChatMemoryRuntime) Observe(ctx context.Context, payload chat.PostProcessPayload) error {
	ctx, finish, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer finish()
	request, err := r.request(payload.SpaceID, payload.CharacterID)
	if err != nil {
		return err
	}
	for index := len(payload.PipelineMessages) - 1; index >= 0; index-- {
		if payload.PipelineMessages[index]["role"] == "user" {
			request.Message = strings.TrimSpace(payload.PipelineMessages[index]["content"])
			break
		}
	}
	request.RequestID, request.ConversationID = payload.RequestID+"|observed-memory", payload.ConversationID
	request.ExpectedScope = payload.ExecutionScope
	request.RecallEvidence = payload.RecallEvidence
	_, err = r.engine.ObserveMemory(ctx, request, business.Generation{Text: payload.Reply})
	return err
}
