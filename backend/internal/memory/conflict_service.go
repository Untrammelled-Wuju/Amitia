// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/prompt/textlib"
)

func (s *service) CheckConflict(req *CheckConflictRequest) (*CheckConflictResponse, error) {
	existing, err := s.repo.SearchByKey(req.Key, req.CharacterID)
	if err != nil {
		return &CheckConflictResponse{HasConflict: false}, nil
	}

	var conflicts []ConflictItem

	for _, m := range existing {
		if m.ID == "" {
			continue
		}

		if m.Key == req.Key && m.Value == req.Value {
			conflicts = append(conflicts, ConflictItem{Memory: m, Reason: "exact_match"})
			continue
		}

		if m.Key == req.Key && m.Value != req.Value {
			sim := jaccardSimilarity(req.Value, m.Value)
			if sim > 0.85 {
				conflicts = append(conflicts, ConflictItem{Memory: m, Reason: fmt.Sprintf("semantic_similar(%.2f)", sim)})
				continue
			}
		}

		if m.Key == req.Key {
			isContradict, _ := s.llmCheckContradiction(req.Value, m.Value)
			if isContradict {
				conflicts = append(conflicts, ConflictItem{Memory: m, Reason: "llm_contradiction"})
			}
		}
	}

	return &CheckConflictResponse{
		HasConflict: len(conflicts) > 0,
		Conflicts:   conflicts,
	}, nil
}

func (s *service) ResolveConflict(req *ResolveConflictRequest) (*ResolveConflictResponse, error) {
	resp := &ResolveConflictResponse{Resolved: true}
	var existing *Memory
	if req.ConflictID != "" {
		if m, err := s.repo.FindByID(req.ConflictID); err == nil {
			existing = m
		}
	}
	newKey := req.NewKey
	if newKey == "" && existing != nil {
		newKey = existing.Key
	}
	newType := req.NewType
	if newType == "" {
		if existing != nil && existing.MemoryType != "" {
			newType = existing.MemoryType
		} else {
			newType = "custom"
		}
	}
	characterID := req.CharacterID
	if characterID == "" && existing != nil {
		characterID = existing.CharacterID
	}
	importance := req.Importance
	if importance == 0 && existing != nil {
		importance = existing.Importance
	}
	if importance < 0 {
		importance = 0
	}
	if importance > 10 {
		importance = 10
	}

	memoryType, ok := NormalizeMemoryType(newType)
	if !ok {
		memoryType = MemoryTypeFact
	}

	switch req.Action {
	case "replace", "replace_old":
		if req.ConflictID != "" && existing != nil {
			operationID := uuid.New().String()
			updates := map[string]interface{}{
				"key":             newKey,
				"value":           req.NewValue,
				"memory_type":     string(memoryType),
				"importance":      importance,
				"verified_status": "user_verified",
			}
			m, err := s.updateCanonicalMemory(existing.ID, canonicalUpdateRequest{
				Updates:     updates,
				OperationID: operationID,
				EventType:   "memory_replaced",
				EventReason: "conflict_replace",
			})
			if err != nil {
				return nil, err
			}
			go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
			s.syncGraph(m)
			resp.MemoryID = m.ID
			return resp, nil
		}
		operationID := uuid.New().String()
		m, err := s.createCanonicalMemory(canonicalCreateRequest{
			CharacterID:    characterID,
			MemoryType:     memoryType,
			Source:         "manual",
			Key:            newKey,
			Value:          req.NewValue,
			Importance:     importance,
			Confidence:     50,
			VerifiedStatus: "user_verified",
			OperationID:    operationID,
			EventType:      "memory_created",
			EventReason:    "conflict_replace",
		})
		if err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		resp.MemoryID = m.ID
		return resp, nil
	case "keep_existing", "keep_old":
		resp.MemoryID = req.ConflictID
		return resp, nil
	case "keep_both":
		operationID := uuid.New().String()
		m, err := s.createCanonicalMemory(canonicalCreateRequest{
			CharacterID:    characterID,
			MemoryType:     memoryType,
			Source:         "manual",
			Key:            newKey,
			Value:          req.NewValue,
			Importance:     importance,
			Confidence:     50,
			VerifiedStatus: "user_verified",
			OperationID:    operationID,
			EventType:      "memory_created",
			EventReason:    "conflict_keep_both",
		})
		if err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		resp.MemoryID = m.ID
		return resp, nil
	case "merge":
		newValue := req.NewValue
		if existing != nil {
			newValue = existing.Value + "; " + req.NewValue
			operationID := uuid.New().String()
			updates := map[string]interface{}{
				"value":            newValue,
				"importance":       importance,
				"confidence":       maxInt(existing.Confidence, 50),
				"verified_status":  "user_verified",
				"last_verified_at": time.Now().Format("2006-01-02 15:04:05"),
			}
			m, err := s.updateCanonicalMemory(existing.ID, canonicalUpdateRequest{
				Updates:     updates,
				OperationID: operationID,
				EventType:   "memory_merged",
				EventReason: "conflict_merge",
			})
			if err != nil {
				return nil, err
			}
			go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
			s.syncGraph(m)
			resp.MemoryID = m.ID
			return resp, nil
		}
		operationID := uuid.New().String()
		m, err := s.createCanonicalMemory(canonicalCreateRequest{
			CharacterID:    characterID,
			MemoryType:     memoryType,
			Source:         "manual",
			Key:            newKey,
			Value:          newValue,
			Importance:     importance,
			Confidence:     50,
			VerifiedStatus: "user_verified",
			OperationID:    operationID,
			EventType:      "memory_created",
			EventReason:    "conflict_merge",
		})
		if err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		resp.MemoryID = m.ID
		return resp, nil
	default:
		return nil, fmt.Errorf("未知的冲突解决动作: %s", req.Action)
	}
}

func (s *service) AutoResolveConflict(key, value, characterID string, newConfidence int) (*ResolveConflictResponse, error) {
	existing, err := s.repo.SearchByKey(key, characterID)
	if err != nil || len(existing) == 0 {
		return &ResolveConflictResponse{Resolved: false}, nil
	}

	for _, m := range existing {
		if m.Key != key {
			continue
		}
		if m.Value == value {
			if newConfidence > m.Confidence+10 {
				operationID := uuid.New().String()
				_, err := s.updateCanonicalMemory(m.ID, canonicalUpdateRequest{
					Updates: map[string]interface{}{
						"confidence": newConfidence,
					},
					OperationID: operationID,
					EventType:   "memory_updated",
					EventReason: "auto_resolve_confidence",
				})
				if err != nil {
					return nil, err
				}
				if updated, err := s.repo.FindByID(m.ID); err == nil {
					s.syncGraph(updated)
				}
			}
			return &ResolveConflictResponse{Resolved: true, MemoryID: m.ID}, nil
		}
		confDiff := newConfidence - m.Confidence
		if confDiff >= 40 {
			operationID := uuid.New().String()
			err := s.deleteCanonicalMemory(m.ID, canonicalDeleteRequest{
				OperationID: operationID,
				EventReason: "auto_replaced",
				HardDelete:  false,
			})
			if err != nil {
				return nil, err
			}
			deleteVectorsFromCollections([]string{m.ID})
			_ = s.repo.UnmarkEmbedded(m.ID)
			s.deleteGraph(&m)
			return &ResolveConflictResponse{Resolved: false}, nil
		}
	}

	return &ResolveConflictResponse{Resolved: false}, nil
}

func (s *service) llmCheckContradiction(newVal, oldVal string) (bool, error) {
	cfg := s.getActiveModel()
	if cfg == nil {
		return false, nil
	}

	userMsg := fmt.Sprintf(
		textlib.MemoryContradictionUserMsgTemplate,
		"", "", oldVal,
		"", "", newVal,
	)

	messages := []map[string]interface{}{
		{"role": "system", "content": textlib.MemoryContradictionSystemPrompt},
		{"role": "user", "content": userMsg},
	}
	content, _, err := s.callLLM(cfg, messages)
	if err != nil {
		return false, err
	}

	content = strings.TrimSpace(content)
	lower := strings.ToLower(content)
	return strings.Contains(lower, "strong_conflict") || strings.Contains(lower, "weak_conflict"), nil
}
