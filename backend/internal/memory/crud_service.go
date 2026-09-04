// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (s *service) List(q MemoryListQuery) (*MemoryListResponse, error) {
	s.refreshRetentionForListScope(q.CharacterID, q.UserID)
	items, total, err := s.repo.List(q)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range items {
		s.maintainRetentionForMemory(&items[i], now)
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	return &MemoryListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *service) Create(req *CreateMemoryRequest) (*Memory, error) {
	memoryType, ok := NormalizeMemoryType(req.MemoryType)
	if !ok {
		memoryType = MemoryTypeFact
	}

	if req.Source == "" {
		req.Source = "manual"
	}
	if req.VerifiedStatus == "" {
		req.VerifiedStatus = "unverified"
	}
	if req.Scope == "" {
		req.Scope = "character"
	}
	if req.Confidence == 0 {
		req.Confidence = 50
	}

	resp, err := s.AutoResolveConflict(req.Key, req.Value, req.CharacterID, req.Confidence)
	if err == nil && resp != nil && resp.Resolved {
		// Conflict resolution may reuse an existing canonical memory. Preserve
		// explicit retention choices from the create request instead of silently
		// dropping the user's L1-L5/pinned selection.
		update := &UpdateMemoryRequest{}
		needsUpdate := false
		if req.RetentionLevel >= RetentionL1 && req.RetentionLevel <= RetentionL5 {
			level := req.RetentionLevel
			update.RetentionLevel = &level
			needsUpdate = true
		}
		if req.MemoryStrength > 0 {
			strength := req.MemoryStrength
			update.MemoryStrength = &strength
			needsUpdate = true
		}
		if req.Pinned {
			pinned := true
			update.Pinned = &pinned
			needsUpdate = true
		}
		if req.MemorySubtype != "" {
			subtype := req.MemorySubtype
			update.MemorySubtype = &subtype
			needsUpdate = true
		}
		if req.AllowContextUse != nil {
			allowContextUse := *req.AllowContextUse
			update.AllowContextUse = &allowContextUse
			needsUpdate = true
		}
		if needsUpdate {
			return s.Update(resp.MemoryID, update)
		}
		return s.repo.FindByID(resp.MemoryID)
	}

	var expiresAt *string
	if req.ExpiresAt != "" {
		expiresAt = &req.ExpiresAt
	}

	operationID := uuid.New().String()

	m, err := s.createCanonicalMemory(canonicalCreateRequest{
		CharacterID:           req.CharacterID,
		MemoryType:            memoryType,
		MemorySubtype:         req.MemorySubtype,
		Source:                req.Source,
		Scope:                 req.Scope,
		Key:                   req.Key,
		Value:                 req.Value,
		Importance:            req.Importance,
		Confidence:            req.Confidence,
		ExpiresAt:             expiresAt,
		EntityID:              req.EntityID,
		EntityType:            req.EntityType,
		SourceMsgID:           req.SourceMsgID,
		SourceConvID:          req.SourceConvID,
		VerifiedStatus:        req.VerifiedStatus,
		SensitivityLevel:      req.SensitivityLevel,
		AllowContextUse:       req.AllowContextUse,
		AllowProactiveMention: req.AllowProactiveMention,
		RequiresConfirmation:  req.RequiresConfirmation,
		RetentionLevel:        req.RetentionLevel,
		MemoryStrength:        req.MemoryStrength,
		Pinned:                req.Pinned,
		OperationID:           operationID,
		EventType:             "memory_created",
		EventReason:           "manual_create",
	})
	if err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}

	if memoryContextUseAllowed(*m) {
		go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
		s.syncGraph(m)
	}

	return m, nil
}

func (s *service) Update(id string, req *UpdateMemoryRequest) (*Memory, error) {
	before, _ := s.repo.FindByID(id)
	updates := make(map[string]interface{})
	if req.Key != nil {
		updates["key"] = *req.Key
	}
	if req.Value != nil {
		updates["value"] = *req.Value
	}
	if req.MemoryType != nil {
		memoryType, ok := NormalizeMemoryType(*req.MemoryType)
		if !ok {
			memoryType = MemoryTypeFact
		}
		updates["memory_type"] = string(memoryType)
	}
	if req.MemorySubtype != nil {
		updates["memory_subtype"] = *req.MemorySubtype
	}
	if req.CharacterID != nil {
		updates["character_id"] = *req.CharacterID
	}
	if req.Importance != nil {
		updates["importance"] = *req.Importance
	}
	if req.Confidence != nil {
		updates["confidence"] = *req.Confidence
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}
	if req.EntityID != nil {
		updates["entity_id"] = *req.EntityID
	}
	if req.EntityType != nil {
		updates["entity_type"] = *req.EntityType
	}
	if req.VerifiedStatus != nil {
		updates["verified_status"] = *req.VerifiedStatus
	}
	if req.Scope != nil {
		updates["scope"] = *req.Scope
	}
	if req.SensitivityLevel != nil {
		updates["sensitivity_level"] = *req.SensitivityLevel
	}
	if req.AllowContextUse != nil {
		updates["allow_context_use"] = *req.AllowContextUse
	}
	if req.AllowProactiveMention != nil {
		updates["allow_proactive_mention"] = *req.AllowProactiveMention
	}
	if req.RequiresConfirmation != nil {
		updates["requires_confirmation"] = *req.RequiresConfirmation
	}
	retentionChanged := false
	if req.RetentionLevel != nil {
		level := normalizeRetentionLevel(*req.RetentionLevel)
		updates["retention_level"] = level
		if before == nil || level != normalizeRetentionLevel(before.RetentionLevel) {
			retentionChanged = true
			if req.MemoryStrength == nil {
				updates["memory_strength"] = defaultStrengthForLevel(level)
			}
		}
	}
	if req.MemoryStrength != nil {
		updates["memory_strength"] = clamp01(*req.MemoryStrength)
		retentionChanged = true
	}
	if req.Pinned != nil {
		updates["pinned"] = *req.Pinned
		if *req.Pinned {
			updates["retention_level"] = RetentionL1
			updates["memory_strength"] = 1.0
			updates["decay_state"] = DecayStateActive
			updates["archived_at"] = nil
			retentionChanged = true
		} else if before != nil && before.Pinned && req.RetentionLevel == nil {
			memoryType := before.MemoryType
			memorySubtype := before.MemorySubtype
			importance := before.Importance
			if req.MemoryType != nil {
				if normalized, ok := NormalizeMemoryType(*req.MemoryType); ok {
					memoryType = string(normalized)
				}
			}
			if req.MemorySubtype != nil {
				memorySubtype = *req.MemorySubtype
			}
			if req.Importance != nil {
				importance = *req.Importance
			}
			assignment := AssignRetention(memoryType, memorySubtype, importance, false)
			updates["retention_level"] = assignment.Level
			if req.MemoryStrength == nil {
				updates["memory_strength"] = assignment.Strength
			}
			updates["decay_state"] = DecayStateActive
			updates["archived_at"] = nil
			retentionChanged = true
		}
	}
	if retentionChanged {
		now := time.Now().Format("2006-01-02 15:04:05")
		updates["strength_updated_at"] = now
		if pinned, ok := updates["pinned"].(bool); !ok || !pinned {
			level := RetentionL3
			strength := defaultStrengthForLevel(level)
			if before != nil {
				level = normalizeRetentionLevel(before.RetentionLevel)
				strength = memoryEffectiveStrength(*before, time.Now())
			}
			if value, ok := updates["retention_level"].(int); ok {
				level = value
			}
			if value, ok := updates["memory_strength"].(float64); ok {
				strength = value
			}
			state, normalizedLevel := decayStateFor(level, strength)
			updates["retention_level"] = normalizedLevel
			updates["decay_state"] = state
			if state == DecayStateArchived {
				updates["archived_at"] = now
			} else {
				updates["archived_at"] = nil
			}
		}
	}
	if len(updates) == 0 {
		return s.repo.FindByID(id)
	}

	operationID := uuid.New().String()

	m, err := s.updateCanonicalMemory(id, canonicalUpdateRequest{
		Updates:     updates,
		OperationID: operationID,
		EventType:   "memory_updated",
		EventReason: "manual_update",
	})
	if err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}

	if before != nil && before.MemoryType != m.MemoryType {
		deleteVectorsFromCollections([]string{m.ID}, collectionNameForMemoryType(before.MemoryType))
	}
	if memoryStatusBlocksRetrieval(m.VerifiedStatus) || !memoryContextUseAllowed(*m) {
		deleteVectorsFromCollections([]string{m.ID})
		_ = s.repo.UnmarkEmbedded(m.ID)
		if !memoryContextUseAllowed(*m) {
			s.deleteGraph(m)
		}
		return m, nil
	}
	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	return m, nil
}

func (s *service) Restore(id string) (*Memory, error) {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	level := normalizeRetentionLevel(m.RetentionLevel)
	strength := memoryEffectiveStrength(*m, time.Now())
	minimum := defaultStrengthForLevel(level)
	if level == RetentionL5 {
		minimum = 0.36
	}
	if strength < minimum {
		strength = minimum
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	restored, err := s.updateCanonicalMemory(id, canonicalUpdateRequest{
		Updates: map[string]interface{}{
			"memory_strength":     strength,
			"strength_updated_at": now,
			"decay_state":         DecayStateActive,
			"archived_at":         nil,
		},
		OperationID: uuid.New().String(),
		EventType:   "memory_restored",
		EventReason: "manual_restore",
	})
	if err != nil {
		return nil, err
	}
	if memoryContextUseAllowed(*restored) {
		go s.SyncEmbedding(restored.ID, restored.Key, restored.Value, restored.CharacterID, restored.MemoryType)
		s.syncGraph(restored)
	}
	return restored, nil
}

func (s *service) Delete(id string) error {
	operationID := uuid.New().String()

	m, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	err = s.deleteCanonicalMemory(id, canonicalDeleteRequest{
		OperationID: operationID,
		EventReason: "manual_delete",
	})
	if err != nil {
		return err
	}

	deleteVectorsFromCollections([]string{id})
	_ = s.repo.UnmarkEmbedded(id)
	s.deleteGraph(m)
	return nil
}

func (s *service) DeleteAll(characterID string) error {
	var ids []string
	query := s.db.Model(&Memory{})
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	query.Pluck("id", &ids)

	for _, id := range ids {
		operationID := uuid.New().String()
		if err := s.deleteCanonicalMemory(id, canonicalDeleteRequest{
			OperationID: operationID,
			EventReason: "bulk_delete",
		}); err != nil {
			return err
		}
	}

	deleteVectorsFromCollections(ids)
	for _, id := range ids {
		_ = s.repo.UnmarkEmbedded(id)
	}
	if s.graphSvc != nil {
		for _, id := range ids {
			_ = s.graphSvc.DeleteNode("memory:" + id)
		}
	}

	return nil
}

func (s *service) RecordUse(id string) (*Memory, error) {
	if err := s.repo.RecordUse(id); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}
