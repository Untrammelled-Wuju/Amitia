// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

func normalizeMemoryOwnerID(spaceID string) string {
	return requestidentity.NormalizeSpaceID(spaceID)
}

func localSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func memoryOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = strings.TrimSpace(requested)
	if stored == requested && requested != "" {
		return true
	}
	return localSingleUserMode() && (stored == "" || stored == "default") && requested != ""
}

func (s *service) memoryForSpace(id, spaceID string) (*Memory, error) {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if m == nil || !memoryOwnerMatches(m.SpaceID, spaceID) {
		return nil, gorm.ErrRecordNotFound
	}
	return m, nil
}

func (s *service) candidateForSpace(id, spaceID string) (*MemoryCandidateModel, error) {
	candidate, err := s.repo.GetCandidateByID(id)
	if err != nil {
		return nil, err
	}
	if candidate == nil || !memoryOwnerMatches(candidate.SpaceID, spaceID) {
		return nil, gorm.ErrRecordNotFound
	}
	return candidate, nil
}

func (s *service) requireCharacterOwnerForMemory(characterID, spaceID string) error {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return nil
	}
	var owner string
	if err := s.db.Table("characters").Select("space_id").Where("id = ?", characterID).Take(&owner).Error; err != nil {
		return err
	}
	if !memoryOwnerMatches(owner, spaceID) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *service) requireConversationOwnerForMemory(conversationID, spaceID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", gorm.ErrRecordNotFound
	}
	var row struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := s.db.Table("conversations").Select("space_id, character_id").Where("id = ? AND deleted_at IS NULL", conversationID).Take(&row).Error; err != nil {
		return "", err
	}
	if !memoryOwnerMatches(row.SpaceID, spaceID) {
		return "", gorm.ErrRecordNotFound
	}
	return strings.TrimSpace(row.CharacterID), nil
}

func (s *service) SubmitCandidateForSpace(req *SubmitCandidateRequest, spaceID string) (*MemoryCandidate, error) {
	if req == nil {
		return nil, fmt.Errorf("candidate request is required")
	}
	characterID, err := s.requireConversationOwnerForMemory(req.ConversationID, spaceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.CharacterID) != "" && strings.TrimSpace(req.CharacterID) != characterID {
		return nil, gorm.ErrRecordNotFound
	}
	copyReq := *req
	copyReq.SpaceID = spaceID
	copyReq.CharacterID = characterID
	return s.SubmitCandidate(&copyReq)
}

func (s *service) ListForSpace(q MemoryListQuery, spaceID string) (*MemoryListResponse, error) {
	q.SpaceID = spaceID
	return s.List(q)
}

func (s *service) CreateForSpace(req *CreateMemoryRequest, spaceID string) (*Memory, error) {
	if req == nil {
		return nil, fmt.Errorf("memory request is required")
	}
	spaceID = normalizeMemoryOwnerID(spaceID)
	copyReq := *req
	copyReq.SpaceID = spaceID
	if sourceConvID := strings.TrimSpace(copyReq.SourceConvID); sourceConvID != "" {
		conversationCharacterID, err := s.requireConversationOwnerForMemory(sourceConvID, spaceID)
		if err != nil {
			return nil, err
		}
		if requestedCharacterID := strings.TrimSpace(copyReq.CharacterID); requestedCharacterID != "" && requestedCharacterID != conversationCharacterID {
			return nil, gorm.ErrRecordNotFound
		}
		copyReq.CharacterID = conversationCharacterID
	} else if err := s.requireCharacterOwnerForMemory(copyReq.CharacterID, spaceID); err != nil {
		return nil, err
	}
	return s.Create(&copyReq)
}

func (s *service) UpdateForSpace(id, spaceID string, req *UpdateMemoryRequest) (*Memory, error) {
	if req == nil {
		return nil, fmt.Errorf("memory update request is required")
	}
	if _, err := s.memoryForSpace(id, spaceID); err != nil {
		return nil, err
	}
	if req.CharacterID != nil {
		if err := s.requireCharacterOwnerForMemory(*req.CharacterID, spaceID); err != nil {
			return nil, err
		}
	}
	return s.Update(id, req)
}

func (s *service) RestoreForSpace(id, spaceID string) (*Memory, error) {
	if _, err := s.memoryForSpace(id, spaceID); err != nil {
		return nil, err
	}
	return s.Restore(id)
}

func (s *service) DeleteForSpace(id, spaceID string) error {
	if _, err := s.memoryForSpace(id, spaceID); err != nil {
		return err
	}
	return s.Delete(id)
}

func (s *service) SearchForSpace(req *SearchMemoryRequest, spaceID string) ([]Memory, error) {
	if req == nil {
		return nil, fmt.Errorf("search request is required")
	}
	copyReq := *req
	copyReq.SpaceID = spaceID
	return s.Search(&copyReq)
}

func (s *service) VectorSearchForSpace(req *VectorSearchRequest, spaceID string) ([]VectorSearchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("search request is required")
	}
	copyReq := *req
	copyReq.SpaceID = spaceID
	return s.VectorSearch(&copyReq)
}

func (s *service) HybridSearchForSpace(req *VectorSearchRequest, spaceID string) ([]HybridSearchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("search request is required")
	}
	copyReq := *req
	copyReq.SpaceID = spaceID
	return s.HybridSearch(&copyReq)
}

func (s *service) RecordUseForSpace(id, spaceID string) (*Memory, error) {
	if _, err := s.memoryForSpace(id, spaceID); err != nil {
		return nil, err
	}
	return s.RecordUse(id)
}

func (s *service) DeleteAllForSpace(characterID, spaceID string) error {
	q := s.db.Model(&Memory{})
	q = applyMemoryScopeQuery(q, characterID, spaceID)
	var ids []string
	if err := q.Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.DeleteForSpace(id, spaceID); err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
	}
	return nil
}

func (s *service) GetTimelineForSpace(page, pageSize int, spaceID, source, memoryType, timelineType string) ([]map[string]interface{}, int64, error) {
	return s.getTimelineOwned(page, pageSize, spaceID, source, memoryType, timelineType)
}

func (s *service) CheckConflictForSpace(req *CheckConflictRequest, spaceID string) (*CheckConflictResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("conflict request is required")
	}
	copyReq := *req
	copyReq.SpaceID = spaceID
	return s.checkConflictOwned(&copyReq)
}

func (s *service) ResolveConflictForSpace(req *ResolveConflictRequest, spaceID string) (*ResolveConflictResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("conflict request is required")
	}
	copyReq := *req
	copyReq.SpaceID = spaceID
	if copyReq.ConflictID != "" {
		if _, err := s.memoryForSpace(copyReq.ConflictID, spaceID); err != nil {
			return nil, err
		}
	}
	return s.ResolveConflict(&copyReq)
}

func (s *service) GenerateCandidatesForSpace(conversationID, spaceID string) ([]MemoryCandidate, error) {
	if _, err := s.requireConversationOwnerForMemory(conversationID, spaceID); err != nil {
		return nil, err
	}
	return s.generateCandidatesForSpace(conversationID, spaceID)
}

func (s *service) ListCandidatesForSpace(spaceID string) []MemoryCandidate {
	var models []MemoryCandidateModel
	query := s.db.Model(&MemoryCandidateModel{})
	if localSingleUserMode() {
		query = query.Where("space_id = ? OR space_id = '' OR space_id IS NULL OR space_id = 'default'", spaceID)
	} else {
		query = query.Where("space_id = ?", spaceID)
	}
	if err := query.Order("created_at DESC").Find(&models).Error; err != nil {
		return []MemoryCandidate{}
	}
	result := make([]MemoryCandidate, 0, len(models))
	for i := range models {
		result = append(result, *candidateModelToDTO(&models[i]))
	}
	return result
}

func (s *service) AcceptCandidateForSpace(id, spaceID string) (*Memory, error) {
	if _, err := s.candidateForSpace(id, spaceID); err != nil {
		return nil, err
	}
	return s.AcceptCandidate(id)
}

func (s *service) RejectCandidateForSpace(id, spaceID string) error {
	if _, err := s.candidateForSpace(id, spaceID); err != nil {
		return err
	}
	return s.RejectCandidate(id)
}

func (s *service) UpdateCandidateForSpace(id, spaceID string, req *UpdateCandidateRequest) (*MemoryCandidate, error) {
	if _, err := s.candidateForSpace(id, spaceID); err != nil {
		return nil, err
	}
	return s.UpdateCandidate(id, req)
}

func (s *service) DeleteCandidateForSpace(id, spaceID string) error {
	return s.RejectCandidateForSpace(id, spaceID)
}

func (s *service) BatchAcceptCandidatesForSpace(ids []string, spaceID string) ([]Memory, error) {
	for _, id := range ids {
		if _, err := s.candidateForSpace(id, spaceID); err != nil {
			return nil, err
		}
	}
	memories := make([]Memory, 0, len(ids))
	for _, id := range ids {
		m, err := s.AcceptCandidateForSpace(id, spaceID)
		if err != nil {
			return nil, err
		}
		memories = append(memories, *m)
	}
	return memories, nil
}

func (s *service) BatchVerifyForSpace(ids []string, status, spaceID string) error {
	for _, id := range ids {
		if _, err := s.memoryForSpace(id, spaceID); err != nil {
			return err
		}
	}
	return s.BatchVerify(ids, status)
}

func (s *service) BatchSetImportanceForSpace(ids []string, importance int, spaceID string) error {
	for _, id := range ids {
		if _, err := s.memoryForSpace(id, spaceID); err != nil {
			return err
		}
	}
	return s.BatchSetImportance(ids, importance)
}

func (s *service) GetRankedMemoriesForSpace(characterID, spaceID, query string, limit int) ([]RankedMemory, error) {
	return s.GetRankedMemories(characterID, spaceID, query, limit)
}

func (s *service) RebuildEmbeddingsForSpace(spaceID string) (map[string]interface{}, error) {
	var memories []Memory
	query := s.db.Model(&Memory{})
	query = applyMemoryScopeQuery(query, "", spaceID)
	if err := query.Find(&memories).Error; err != nil {
		return nil, err
	}
	successCount, failCount := 0, 0
	for i := range memories {
		m := memories[i]
		if !memoryAllowedBySQLiteAuthority(m, retrievalAuthorityPolicy{SpaceID: spaceID, Now: time.Now()}) {
			continue
		}
		if s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType) {
			successCount++
		} else {
			failCount++
		}
		s.syncGraph(&m)
	}
	status := "completed"
	if failCount > 0 {
		status = "partial_failed"
	}
	return map[string]interface{}{"totalMemories": len(memories), "embedded": successCount, "failed": failCount, "status": status}, nil
}

func (s *service) RebuildIndexForSpace(spaceID string) (map[string]interface{}, error) {
	return s.RebuildEmbeddingsForSpace(spaceID)
}

func (s *service) GetVectorStatusForSpace(spaceID string) map[string]interface{} {
	query := s.db.Model(&Memory{})
	query = applyMemoryScopeQuery(query, "", spaceID)
	var total int64
	_ = query.Count(&total).Error
	var embedded int64
	_ = s.db.Table("memory_embeddings AS e").Joins("JOIN memories AS m ON m.id = e.memory_id").Where("m.space_id = ?", spaceID).Count(&embedded).Error
	return map[string]interface{}{"totalMemories": total, "totalEmbedded": embedded, "notEmbedded": maxInt64(total-embedded, 0), "enabled": qdrantDB.Client != nil, "providerName": "Qdrant"}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *service) RetrieveStatsForSpace(spaceID string) (map[string]interface{}, error) {
	if !s.db.Migrator().HasColumn("retrieval_logs", "space_id") {
		if localSingleUserMode() {
			return s.RetrieveStats()
		}
		return map[string]interface{}{"recentLogs": []map[string]interface{}{}, "totalCount": 0}, nil
	}
	var rows []map[string]interface{}
	q := s.db.Table("retrieval_logs").Where("space_id = ?", spaceID)
	if err := q.Order("created_at DESC").Limit(50).Find(&rows).Error; err != nil {
		return nil, err
	}
	var total int64
	if err := s.db.Table("retrieval_logs").Where("space_id = ?", spaceID).Count(&total).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"recentLogs": rows, "totalCount": total}, nil
}
