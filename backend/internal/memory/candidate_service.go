// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/prompt/textlib"
	"github.com/u-ai/backend/log"
)

func (s *service) GenerateCandidates(conversationID string) ([]MemoryCandidate, error) {
	messages, err := s.repo.GetConversationMessages(conversationID, 100)
	if err != nil || len(messages) == 0 {
		return nil, err
	}
	typedMessages := make([]map[string]string, 0, len(messages))
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		typedMessages = append(typedMessages, map[string]string{
			"role":    role,
			"content": content,
		})
	}
	return s.generateCandidatesFromMessages(conversationID, typedMessages)
}
func (s *service) SubmitCandidate(req *SubmitCandidateRequest) (*MemoryCandidate, error) {
	if req == nil || strings.TrimSpace(req.Key) == "" || strings.TrimSpace(req.Value) == "" || strings.TrimSpace(req.SourceText) == "" || strings.TrimSpace(req.ConversationID) == "" || strings.TrimSpace(req.CharacterID) == "" {
		return nil, fmt.Errorf("候选记忆缺少 key、value、sourceText、conversationId 或 characterId")
	}
	if req.Importance < 1 {
		req.Importance = 5
	}
	if req.Importance > 10 {
		req.Importance = 10
	}
	if req.MemoryType == "" {
		req.MemoryType = "fact"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	model := &MemoryCandidateModel{ID: uuid.New().String(), Key: strings.TrimSpace(req.Key), Value: strings.TrimSpace(req.Value), MemoryType: req.MemoryType, Importance: req.Importance, SourceText: strings.TrimSpace(req.SourceText), ConversationID: req.ConversationID, CharacterID: req.CharacterID, CreatedAt: now}
	if err := s.repo.CreateCandidate(model); err != nil {
		return nil, err
	}
	return &MemoryCandidate{ID: model.ID, Key: model.Key, Value: model.Value, MemoryType: model.MemoryType, Importance: model.Importance, SourceText: model.SourceText, ConversationID: model.ConversationID, CharacterID: model.CharacterID, CreatedAt: model.CreatedAt}, nil
}

func (s *service) buildExtractionUserMsg(messages []map[string]string) (userParts, assistantParts []string) {
	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg["role"]))
		content := strings.TrimSpace(msg["content"])
		if content == "" {
			continue
		}
		if role == "user" {
			userParts = append(userParts, content)
		} else if role == "assistant" {
			assistantParts = append(assistantParts, content)
		}
	}
	return userParts, assistantParts
}

func (s *service) generateCandidatesFromMessages(conversationID string, messages []map[string]string) ([]MemoryCandidate, error) {
	if len(messages) == 0 {
		return nil, nil
	}
	userParts, assistantParts := s.buildExtractionUserMsg(messages)
	if len(userParts) == 0 {
		return nil, nil
	}
	userText := strings.Join(userParts, "\n")
	assistantText := strings.Join(assistantParts, "\n")

	var characterID string
	s.db.Table("conversations").Select("character_id").Where("id = ?", conversationID).Row().Scan(&characterID)
	cfg := s.getActiveModel()
	if cfg == nil {
		return nil, fmt.Errorf("no active model")
	}

	userMsg := fmt.Sprintf(textlib.MemoryExtractUserMsgTemplate, userText, assistantText)

	messagesLLM := []map[string]interface{}{
		{"role": "system", "content": textlib.MemoryExtractSystemPrompt},
		{"role": "user", "content": userMsg},
	}
	content, _, err := s.callLLM(cfg, messagesLLM)
	if err != nil {
		return nil, err
	}
	content = extractJSONArray(content)
	var candidates []MemoryCandidate
	if err := json.Unmarshal([]byte(content), &candidates); err != nil {
		return nil, nil
	}
	for i := range candidates {
		candidates[i].ID = uuid.New().String()
		candidates[i].SourceText = userText
		candidates[i].ConversationID = conversationID
		candidates[i].CharacterID = characterID
		candidates[i].CreatedAt = time.Now().Format("2006-01-02 15:04:05")
		model := &MemoryCandidateModel{
			ID: candidates[i].ID, Key: candidates[i].Key, Value: candidates[i].Value,
			MemoryType: candidates[i].MemoryType, Importance: candidates[i].Importance,
			SourceText: candidates[i].SourceText, ConversationID: candidates[i].ConversationID,
			CharacterID: candidates[i].CharacterID,
			CreatedAt:   candidates[i].CreatedAt,
		}
		if err := s.repo.CreateCandidate(model); err != nil {
			log.Error("保存候选记忆失败:", err)
		}
	}
	return candidates, nil
}

func (s *service) ListCandidates() []MemoryCandidate {
	models, err := s.repo.ListCandidates()
	if err != nil || len(models) == 0 {
		return []MemoryCandidate{}
	}
	result := make([]MemoryCandidate, len(models))
	for i, m := range models {
		result[i] = MemoryCandidate{
			ID: m.ID, Key: m.Key, Value: m.Value,
			MemoryType: m.MemoryType, Importance: m.Importance,
			SourceText: m.SourceText, ConversationID: m.ConversationID,
			CharacterID: m.CharacterID,
			CreatedAt:   m.CreatedAt,
		}
	}
	return result
}

func (s *service) AcceptCandidate(id string) (*Memory, error) {
	model, err := s.repo.GetCandidateByID(id)
	if err != nil || model == nil {
		return nil, fmt.Errorf("候选记忆不存在")
	}
	m := &Memory{
		CharacterID:  model.CharacterID,
		MemoryType:   model.MemoryType,
		Source:       "auto",
		Key:          model.Key,
		Value:        model.Value,
		Importance:   model.Importance,
		Confidence:   50,
		SourceConvID: model.ConversationID,
	}
	if err := s.repo.Create(m); err != nil {
		return nil, err
	}
	s.repo.DeleteCandidate(id)
	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	return m, nil
}

func (s *service) RejectCandidate(id string) error {
	return s.repo.DeleteCandidate(id)
}

func (s *service) BatchAcceptCandidates(ids []string) ([]Memory, error) {
	var memories []Memory
	for _, id := range ids {
		m, err := s.AcceptCandidate(id)
		if err != nil {
			continue
		}
		memories = append(memories, *m)
	}
	return memories, nil
}

func (s *service) UpdateCandidate(id string, req *UpdateCandidateRequest) (*MemoryCandidate, error) {
	updates := make(map[string]interface{})
	if req.Key != nil {
		updates["key"] = *req.Key
	}
	if req.Value != nil {
		updates["value"] = *req.Value
	}
	if req.MemoryType != nil {
		updates["memory_type"] = *req.MemoryType
	}
	if req.Importance != nil {
		updates["importance"] = *req.Importance
	}
	if err := s.repo.UpdateCandidate(id, updates); err != nil {
		return nil, err
	}
	model, err := s.repo.GetCandidateByID(id)
	if err != nil {
		return nil, err
	}
	return &MemoryCandidate{
		ID: model.ID, Key: model.Key, Value: model.Value,
		MemoryType: model.MemoryType, Importance: model.Importance,
		SourceText: model.SourceText, ConversationID: model.ConversationID,
		CreatedAt: model.CreatedAt,
	}, nil
}

func (s *service) DeleteCandidate(id string) error {
	return s.RejectCandidate(id)
}

func (s *service) ExtractCandidates() ([]MemoryCandidate, error) {
	return s.ListCandidates(), nil
}
