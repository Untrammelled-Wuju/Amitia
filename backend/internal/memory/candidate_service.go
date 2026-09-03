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
	memoryType, ok := NormalizeMemoryType(req.MemoryType)
	if !ok {
		return nil, fmt.Errorf("invalid memory type: %s", req.MemoryType)
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	confidence := req.Confidence
	if confidence <= 0 {
		confidence = 50
	}
	if confidence > 100 {
		confidence = 100
	}
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "character"
	}
	sensitivity := strings.TrimSpace(req.SensitivityLevel)
	if sensitivity == "" {
		sensitivity = "internal"
	}
	retentionLevel := req.RetentionLevel
	if retentionLevel < RetentionL1 || retentionLevel > RetentionL5 {
		retentionLevel = 0
	}
	memoryStrength := req.MemoryStrength
	if memoryStrength < 0 || memoryStrength > 1 {
		memoryStrength = 0
	}
	decayState := strings.TrimSpace(req.DecayState)
	if decayState != "" && decayState != DecayStateActive && decayState != DecayStateFading && decayState != DecayStateArchived {
		decayState = ""
	}
	model := &MemoryCandidateModel{
		ID: uuid.New().String(), Key: strings.TrimSpace(req.Key), Value: strings.TrimSpace(req.Value), MemoryType: string(memoryType), MemorySubtype: strings.TrimSpace(req.MemorySubtype), Importance: req.Importance,
		RetentionLevel: retentionLevel, MemoryStrength: memoryStrength, StrengthUpdatedAt: req.StrengthUpdatedAt, LastReinforcedAt: req.LastReinforcedAt, ReinforceCount: req.ReinforceCount, DecayState: decayState, Pinned: req.Pinned, ArchivedAt: req.ArchivedAt,
		Scope: scope, SensitivityLevel: sensitivity, AllowProactiveMention: req.AllowProactiveMention, RequiresConfirmation: req.RequiresConfirmation, SourceText: strings.TrimSpace(req.SourceText), ConversationID: req.ConversationID, CharacterID: req.CharacterID, CreatedAt: now, CandidateKind: req.CandidateKind, ConfidenceReal: float64(confidence) / 100.0, DerivationKey: req.DerivationKey, Reason: req.Reason,
	}
	if model.CandidateKind == "" {
		model.CandidateKind = string(CandidateKindExtracted)
	}
	if err := s.repo.CreateCandidate(model); err != nil {
		return nil, err
	}
	return candidateModelToDTO(model), nil
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
		mt, ok := NormalizeMemoryType(candidates[i].MemoryType)
		if !ok {
			mt = MemoryTypeFact
		}
		if subtype := strings.TrimSpace(candidates[i].MemorySubtype); subtype != "" {
			if subtypeType, subtypeOK := NormalizeMemoryType(memoryTypeForSubtype(subtype)); subtypeOK {
				mt = subtypeType
			}
		}
		candidates[i].MemoryType = string(mt)
		if candidates[i].Confidence <= 0 {
			candidates[i].Confidence = 50
		}
		if candidates[i].Confidence > 100 {
			candidates[i].Confidence = 100
		}
		if candidates[i].Scope == "" {
			candidates[i].Scope = "character"
		}
		if candidates[i].SensitivityLevel == "" {
			candidates[i].SensitivityLevel = "internal"
		}
		model := &MemoryCandidateModel{
			ID: candidates[i].ID, Key: candidates[i].Key, Value: candidates[i].Value,
			MemoryType: candidates[i].MemoryType, MemorySubtype: candidates[i].MemorySubtype, Importance: candidates[i].Importance,
			Scope: candidates[i].Scope, SensitivityLevel: candidates[i].SensitivityLevel, AllowProactiveMention: candidates[i].AllowProactiveMention, RequiresConfirmation: candidates[i].RequiresConfirmation,
			SourceText: candidates[i].SourceText, ConversationID: candidates[i].ConversationID,
			CharacterID: candidates[i].CharacterID, ConfidenceReal: float64(candidates[i].Confidence) / 100.0,
			CreatedAt: candidates[i].CreatedAt, CandidateKind: string(CandidateKindExtracted),
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
		result[i] = *candidateModelToDTO(&m)
	}
	return result
}

func (s *service) AcceptCandidate(id string) (*Memory, error) {
	model, err := s.repo.GetCandidateByID(id)
	if err != nil || model == nil {
		return nil, fmt.Errorf("候选记忆不存在")
	}
	candidateKey := "candidate_accept:" + id
	if model.DerivationKey != "" {
		candidateKey = model.DerivationKey
	}
	if existing, _ := s.repo.FindByDerivationKey(candidateKey); existing != nil && existing.ID != "" {
		s.repo.DeleteCandidate(id)
		return existing, nil
	}

	operationID := uuid.New().String()
	memoryType, ok := NormalizeMemoryType(model.MemoryType)
	if !ok {
		memoryType = MemoryTypeFact
	}

	var derivations []MemoryDerivationInput
	for i, srcID := range parseSourceIDs(model.SourceMemoryIDsJSON) {
		srcVersion := 1
		if i < len(parseSourceInts(model.SourceVersionsJSON)) {
			srcVersion = parseSourceInts(model.SourceVersionsJSON)[i]
		}
		inputSnapshotHash := ""
		if srcMem, err := s.repo.FindByID(srcID); err == nil {
			inputSnapshotHash = computeMemorySnapshotHashCanonical(srcMem)
		}
		derivations = append(derivations, MemoryDerivationInput{
			InputMemoryID:     srcID,
			InputVersion:      srcVersion,
			InputSnapshotHash: inputSnapshotHash,
			DerivationKind:    string(DerivationKindMerge),
			Ordinal:           i,
		})
	}

	m, err := s.createCanonicalMemory(canonicalCreateRequest{
		CharacterID:           model.CharacterID,
		MemoryType:            memoryType,
		MemorySubtype:         model.MemorySubtype,
		Source:                "auto",
		Scope:                 model.Scope,
		Key:                   model.Key,
		Value:                 model.Value,
		Importance:            model.Importance,
		Confidence:            candidateModelConfidence(model),
		SensitivityLevel:      model.SensitivityLevel,
		AllowProactiveMention: model.AllowProactiveMention,
		RequiresConfirmation:  model.RequiresConfirmation,
		RetentionLevel:        model.RetentionLevel,
		MemoryStrength:        model.MemoryStrength,
		StrengthUpdatedAt:     model.StrengthUpdatedAt,
		LastReinforcedAt:      model.LastReinforcedAt,
		ReinforceCount:        model.ReinforceCount,
		DecayState:            model.DecayState,
		Pinned:                model.Pinned,
		ArchivedAt:            model.ArchivedAt,
		SourceConvID:          model.ConversationID,
		DerivationKey:         candidateKey,
		OperationID:           operationID,
		EventType:             "memory_created",
		EventReason:           "candidate_accept",
		Derivations:           derivations,
	})
	if err != nil {
		return nil, err
	}
	s.repo.DeleteCandidate(id)
	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
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
	if req.MemorySubtype != nil {
		updates["memory_subtype"] = strings.TrimSpace(*req.MemorySubtype)
	}
	if req.Confidence != nil {
		confidence := *req.Confidence
		if confidence < 0 {
			confidence = 0
		}
		if confidence > 100 {
			confidence = 100
		}
		updates["confidence"] = float64(confidence) / 100.0
	}
	if req.RetentionLevel != nil {
		level := *req.RetentionLevel
		if level < RetentionL1 || level > RetentionL5 {
			level = 0
		}
		updates["retention_level"] = level
	}
	if req.MemoryStrength != nil {
		updates["memory_strength"] = clamp01(*req.MemoryStrength)
	}
	if req.Pinned != nil {
		updates["pinned"] = *req.Pinned
	}
	if req.Scope != nil {
		updates["scope"] = strings.TrimSpace(*req.Scope)
	}
	if req.SensitivityLevel != nil {
		updates["sensitivity_level"] = strings.TrimSpace(*req.SensitivityLevel)
	}
	if req.AllowProactiveMention != nil {
		updates["allow_proactive_mention"] = *req.AllowProactiveMention
	}
	if req.RequiresConfirmation != nil {
		updates["requires_confirmation"] = *req.RequiresConfirmation
	}
	if err := s.repo.UpdateCandidate(id, updates); err != nil {
		return nil, err
	}
	model, err := s.repo.GetCandidateByID(id)
	if err != nil {
		return nil, err
	}
	return candidateModelToDTO(model), nil
}

func candidateModelConfidence(model *MemoryCandidateModel) int {
	if model == nil {
		return 50
	}
	v := model.ConfidenceReal
	if v <= 1 {
		v *= 100
	}
	if v <= 0 {
		return 50
	}
	if v > 100 {
		v = 100
	}
	return int(v + 0.5)
}

func candidateModelToDTO(model *MemoryCandidateModel) *MemoryCandidate {
	if model == nil {
		return nil
	}
	return &MemoryCandidate{
		ID: model.ID, Key: model.Key, Value: model.Value, MemoryType: model.MemoryType, MemorySubtype: model.MemorySubtype,
		Importance: model.Importance, Confidence: candidateModelConfidence(model), RetentionLevel: model.RetentionLevel, MemoryStrength: model.MemoryStrength,
		StrengthUpdatedAt: model.StrengthUpdatedAt, LastReinforcedAt: model.LastReinforcedAt, ReinforceCount: model.ReinforceCount, DecayState: model.DecayState, Pinned: model.Pinned, ArchivedAt: model.ArchivedAt,
		Scope: model.Scope, SensitivityLevel: model.SensitivityLevel, AllowProactiveMention: model.AllowProactiveMention, RequiresConfirmation: model.RequiresConfirmation, SourceText: model.SourceText,
		ConversationID: model.ConversationID, CharacterID: model.CharacterID, CreatedAt: model.CreatedAt,
	}
}

func (s *service) DeleteCandidate(id string) error {
	return s.RejectCandidate(id)
}

func (s *service) ExtractCandidates() ([]MemoryCandidate, error) {
	return s.ListCandidates(), nil
}

func parseSourceInts(raw string) []int {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var ints []int
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		_ = json.Unmarshal([]byte(raw), &ints)
		return ints
	}
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var val int
		if _, err := fmt.Sscanf(p, "%d", &val); err == nil {
			ints = append(ints, val)
		}
	}
	return ints
}
