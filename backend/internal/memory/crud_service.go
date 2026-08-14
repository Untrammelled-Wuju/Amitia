// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"

	"github.com/google/uuid"
)

func (s *service) List(q MemoryListQuery) (*MemoryListResponse, error) {
	items, total, err := s.repo.List(q)
	if err != nil {
		return nil, err
	}
	return &MemoryListResponse{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
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
		AllowProactiveMention: req.AllowProactiveMention,
		RequiresConfirmation:  req.RequiresConfirmation,
		OperationID:           operationID,
		EventType:             "memory_created",
		EventReason:           "manual_create",
	})
	if err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}

	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)

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
	if memoryStatusBlocksRetrieval(m.VerifiedStatus) {
		deleteVectorsFromCollections([]string{m.ID})
		_ = s.repo.UnmarkEmbedded(m.ID)
		s.deleteGraph(m)
		return m, nil
	}
	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	return m, nil
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
