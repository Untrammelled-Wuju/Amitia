// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"
	"strings"
	"time"

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

	now := time.Now().Format("2006-01-02 15:04:05")

	switch req.Action {
	case "replace", "replace_old":
		if req.ConflictID != "" {
			if existing != nil {
				s.preserveHistory(existing, "replaced", now)
				s.deleteGraph(existing)
			}
			s.repo.Delete(req.ConflictID)
		}
		m := &Memory{
			Key: newKey, Value: req.NewValue, MemoryType: newType,
			Importance: importance, CharacterID: characterID, Source: "manual",
			Confidence: 50, VerifiedStatus: "user_verified",
		}
		if err := s.repo.Create(m); err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
		resp.MemoryID = m.ID
		return resp, nil
	case "keep_existing", "keep_old":
		resp.MemoryID = req.ConflictID
		return resp, nil
	case "keep_both":
		m := &Memory{
			Key: newKey, Value: req.NewValue, MemoryType: newType,
			Importance: importance, CharacterID: characterID, Source: "manual",
			Confidence: 50, VerifiedStatus: "user_verified",
		}
		if err := s.repo.Create(m); err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
		resp.MemoryID = m.ID
		return resp, nil
	case "merge":
		newValue := req.NewValue
		if existing != nil {
			newValue = existing.Value + "; " + req.NewValue
			s.preserveHistory(existing, "merged", now)
			updates := map[string]interface{}{
				"value":            newValue,
				"importance":       importance,
				"confidence":       maxInt(existing.Confidence, 50),
				"verified_status":  "user_verified",
				"last_verified_at": now,
			}
			if err := s.repo.Update(existing.ID, updates); err != nil {
				return nil, err
			}
			go s.SyncEmbedding(existing.ID, newKey, newValue, characterID, newType)
			if updated, err := s.repo.FindByID(existing.ID); err == nil {
				s.syncGraph(updated)
			}
			resp.MemoryID = existing.ID
			return resp, nil
		}
		m := &Memory{
			Key: newKey, Value: newValue, MemoryType: newType,
			Importance: importance, CharacterID: characterID, Source: "manual",
			Confidence: 50, VerifiedStatus: "user_verified",
		}
		if err := s.repo.Create(m); err != nil {
			return nil, err
		}
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
		s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
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

	now := time.Now().Format("2006-01-02 15:04:05")

	for _, m := range existing {
		if m.Key != key {
			continue
		}
		if m.Value == value {
			if newConfidence > m.Confidence+10 {
				s.repo.Update(m.ID, map[string]interface{}{
					"confidence": newConfidence,
					"updated_at": now,
				})
				if updated, err := s.repo.FindByID(m.ID); err == nil {
					s.syncGraph(updated)
				}
			}
			return &ResolveConflictResponse{Resolved: true, MemoryID: m.ID}, nil
		}
		confDiff := newConfidence - m.Confidence
		if confDiff >= 40 {
			s.preserveHistory(&m, "auto_replaced", now)
			s.deleteGraph(&m)
			s.repo.Delete(m.ID)
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

func (s *service) preserveHistory(memory *Memory, action, timestamp string) {
	if s.db == nil || memory == nil {
		return
	}
	s.db.Exec(
		"INSERT INTO memory_history (memory_id, previous_value, action, changed_at) VALUES (?, ?, ?, ?)",
		memory.ID, memory.Value, action, timestamp,
	)
}
