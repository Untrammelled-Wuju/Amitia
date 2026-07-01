// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func (s *service) List(q MemoryListQuery) (*MemoryListResponse, error) {
	items, total, err := s.repo.List(q)
	if err != nil {
		return nil, err
	}
	return &MemoryListResponse{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (s *service) Create(req *CreateMemoryRequest) (*Memory, error) {
	if req.MemoryType == "" {
		req.MemoryType = "custom"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	if req.Importance < 0 {
		req.Importance = 0
	}
	if req.Importance > 10 {
		req.Importance = 10
	}
	if req.Confidence < 0 {
		req.Confidence = 0
	}
	if req.Confidence > 100 {
		req.Confidence = 100
	}
	if req.Confidence == 0 {
		req.Confidence = 50
	}
	if req.VerifiedStatus == "" {
		req.VerifiedStatus = "unverified"
	}
	if req.Scope == "" {
		req.Scope = "character"
	}

	resp, err := s.AutoResolveConflict(req.Key, req.Value, req.CharacterID, req.Confidence)
	if err == nil && resp != nil && resp.Resolved {
		return s.repo.FindByID(resp.MemoryID)
	}

	var expiresAt *string
	if req.ExpiresAt != "" {
		expiresAt = &req.ExpiresAt
	}

	m := &Memory{
		CharacterID:    req.CharacterID,
		MemoryType:     req.MemoryType,
		Source:         req.Source,
		Scope:          req.Scope,
		Key:            req.Key,
		Value:          req.Value,
		Importance:     req.Importance,
		Confidence:     req.Confidence,
		ExpiresAt:      expiresAt,
		EntityID:       req.EntityID,
		EntityType:     req.EntityType,
		SourceMsgID:    req.SourceMsgID,
		SourceConvID:   req.SourceConvID,
		VerifiedStatus: req.VerifiedStatus,
	}
	if err := s.repo.Create(m); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}

	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)

	s.logEvent(m.ID, "memory_created", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
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
		updates["memory_type"] = *req.MemoryType
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
	if err := s.repo.Update(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if before != nil && before.MemoryType != m.MemoryType {
		deleteVectorsFromCollections([]string{m.ID}, collectionNameForMemoryType(before.MemoryType))
	}
	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	s.logEvent(m.ID, "memory_edited", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	return m, nil
}

func (s *service) Delete(id string) error {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	s.logEvent(id, "memory_deleted", m.Key, m.Value, m.MemoryType, m.Importance, m.Source, m.CharacterID)
	deleteVectorsFromCollections([]string{id}, qdrantDB.CollectionNames()...)
	s.deleteGraph(m)
	return s.repo.Delete(id)
}

func (s *service) DeleteAll(characterID string) error {
	var ids []string
	query := s.db.Model(&Memory{})
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	query.Pluck("id", &ids)
	if characterID != "" {
		s.logEvent("", "memory_deleted_all", "", "", "", 0, "", characterID)
	}
	deleteVectorsFromCollections(ids, qdrantDB.CollectionNames()...)
	if s.graphSvc != nil {
		for _, id := range ids {
			_ = s.graphSvc.DeleteNode("memory:" + id)
		}
	}
	return s.repo.DeleteAll(characterID)
}

func (s *service) RecordUse(id string) (*Memory, error) {
	if err := s.repo.RecordUse(id); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}
