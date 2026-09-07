// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/config"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

func normalizeMemoryOwnerID(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "default"
	}
	return userID
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

func (s *service) memoryForUser(id, userID string) (*Memory, error) {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if m == nil || !memoryOwnerMatches(m.UserID, userID) {
		return nil, gorm.ErrRecordNotFound
	}
	return m, nil
}

func (s *service) candidateForUser(id, userID string) (*MemoryCandidateModel, error) {
	candidate, err := s.repo.GetCandidateByID(id)
	if err != nil {
		return nil, err
	}
	if candidate == nil || !memoryOwnerMatches(candidate.UserID, userID) {
		return nil, gorm.ErrRecordNotFound
	}
	return candidate, nil
}

func (s *service) requireCharacterOwnerForMemory(characterID, userID string) error {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return nil
	}
	var owner string
	if err := s.db.Table("characters").Select("user_id").Where("id = ?", characterID).Take(&owner).Error; err != nil {
		return err
	}
	if !memoryOwnerMatches(owner, userID) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *service) requireConversationOwnerForMemory(conversationID, userID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", gorm.ErrRecordNotFound
	}
	var row struct {
		UserID      string `gorm:"column:user_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := s.db.Table("conversations").Select("user_id, character_id").Where("id = ? AND deleted_at IS NULL", conversationID).Take(&row).Error; err != nil {
		return "", err
	}
	if !memoryOwnerMatches(row.UserID, userID) {
		return "", gorm.ErrRecordNotFound
	}
	return strings.TrimSpace(row.CharacterID), nil
}

func (s *service) SubmitCandidateForUser(req *SubmitCandidateRequest, userID string) (*MemoryCandidate, error) {
	if req == nil {
		return nil, fmt.Errorf("candidate request is required")
	}
	characterID, err := s.requireConversationOwnerForMemory(req.ConversationID, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.CharacterID) != "" && strings.TrimSpace(req.CharacterID) != characterID {
		return nil, gorm.ErrRecordNotFound
	}
	copyReq := *req
	copyReq.UserID = userID
	copyReq.CharacterID = characterID
	return s.SubmitCandidate(&copyReq)
}

func (s *service) ListForUser(q MemoryListQuery, userID string) (*MemoryListResponse, error) {
	q.UserID = userID
	return s.List(q)
}

func (s *service) CreateForUser(req *CreateMemoryRequest, userID string) (*Memory, error) {
	if req == nil {
		return nil, fmt.Errorf("memory request is required")
	}
	userID = normalizeMemoryOwnerID(userID)
	copyReq := *req
	copyReq.UserID = userID
	if sourceConvID := strings.TrimSpace(copyReq.SourceConvID); sourceConvID != "" {
		conversationCharacterID, err := s.requireConversationOwnerForMemory(sourceConvID, userID)
		if err != nil {
			return nil, err
		}
		if requestedCharacterID := strings.TrimSpace(copyReq.CharacterID); requestedCharacterID != "" && requestedCharacterID != conversationCharacterID {
			return nil, gorm.ErrRecordNotFound
		}
		copyReq.CharacterID = conversationCharacterID
	} else if err := s.requireCharacterOwnerForMemory(copyReq.CharacterID, userID); err != nil {
		return nil, err
	}
	return s.Create(&copyReq)
}

func (s *service) UpdateForUser(id, userID string, req *UpdateMemoryRequest) (*Memory, error) {
	if req == nil {
		return nil, fmt.Errorf("memory update request is required")
	}
	if _, err := s.memoryForUser(id, userID); err != nil {
		return nil, err
	}
	if req.CharacterID != nil {
		if err := s.requireCharacterOwnerForMemory(*req.CharacterID, userID); err != nil {
			return nil, err
		}
	}
	return s.Update(id, req)
}

func (s *service) RestoreForUser(id, userID string) (*Memory, error) {
	if _, err := s.memoryForUser(id, userID); err != nil {
		return nil, err
	}
	return s.Restore(id)
}

func (s *service) DeleteForUser(id, userID string) error {
	if _, err := s.memoryForUser(id, userID); err != nil {
		return err
	}
	return s.Delete(id)
}

func (s *service) SearchForUser(req *SearchMemoryRequest, userID string) ([]Memory, error) {
	if req == nil {
		return nil, fmt.Errorf("search request is required")
	}
	copyReq := *req
	copyReq.UserID = userID
	return s.Search(&copyReq)
}

func (s *service) VectorSearchForUser(req *VectorSearchRequest, userID string) ([]VectorSearchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("search request is required")
	}
	copyReq := *req
	copyReq.UserID = userID
	return s.VectorSearch(&copyReq)
}

func (s *service) HybridSearchForUser(req *VectorSearchRequest, userID string) ([]HybridSearchResult, error) {
	if req == nil {
		return nil, fmt.Errorf("search request is required")
	}
	copyReq := *req
	copyReq.UserID = userID
	return s.HybridSearch(&copyReq)
}

func (s *service) RecordUseForUser(id, userID string) (*Memory, error) {
	if _, err := s.memoryForUser(id, userID); err != nil {
		return nil, err
	}
	return s.RecordUse(id)
}

func (s *service) DeleteAllForUser(characterID, userID string) error {
	q := s.db.Model(&Memory{})
	q = applyMemoryScopeQuery(q, characterID, userID)
	var ids []string
	if err := q.Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.DeleteForUser(id, userID); err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
	}
	return nil
}

func (s *service) GetTimelineForUser(page, pageSize int, userID, source, memoryType, timelineType string) ([]map[string]interface{}, int64, error) {
	return s.getTimelineOwned(page, pageSize, userID, source, memoryType, timelineType)
}

func (s *service) CheckConflictForUser(req *CheckConflictRequest, userID string) (*CheckConflictResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("conflict request is required")
	}
	copyReq := *req
	copyReq.UserID = userID
	return s.checkConflictOwned(&copyReq)
}

func (s *service) ResolveConflictForUser(req *ResolveConflictRequest, userID string) (*ResolveConflictResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("conflict request is required")
	}
	copyReq := *req
	copyReq.UserID = userID
	if copyReq.ConflictID != "" {
		if _, err := s.memoryForUser(copyReq.ConflictID, userID); err != nil {
			return nil, err
		}
	}
	return s.ResolveConflict(&copyReq)
}

func (s *service) GenerateCandidatesForUser(conversationID, userID string) ([]MemoryCandidate, error) {
	if _, err := s.requireConversationOwnerForMemory(conversationID, userID); err != nil {
		return nil, err
	}
	return s.generateCandidatesForUser(conversationID, userID)
}

func (s *service) ListCandidatesForUser(userID string) []MemoryCandidate {
	var models []MemoryCandidateModel
	query := s.db.Model(&MemoryCandidateModel{})
	if localSingleUserMode() {
		query = query.Where("user_id = ? OR user_id = '' OR user_id IS NULL OR user_id = 'default'", userID)
	} else {
		query = query.Where("user_id = ?", userID)
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

func (s *service) AcceptCandidateForUser(id, userID string) (*Memory, error) {
	if _, err := s.candidateForUser(id, userID); err != nil {
		return nil, err
	}
	return s.AcceptCandidate(id)
}

func (s *service) RejectCandidateForUser(id, userID string) error {
	if _, err := s.candidateForUser(id, userID); err != nil {
		return err
	}
	return s.RejectCandidate(id)
}

func (s *service) UpdateCandidateForUser(id, userID string, req *UpdateCandidateRequest) (*MemoryCandidate, error) {
	if _, err := s.candidateForUser(id, userID); err != nil {
		return nil, err
	}
	return s.UpdateCandidate(id, req)
}

func (s *service) DeleteCandidateForUser(id, userID string) error {
	return s.RejectCandidateForUser(id, userID)
}

func (s *service) BatchAcceptCandidatesForUser(ids []string, userID string) ([]Memory, error) {
	for _, id := range ids {
		if _, err := s.candidateForUser(id, userID); err != nil {
			return nil, err
		}
	}
	memories := make([]Memory, 0, len(ids))
	for _, id := range ids {
		m, err := s.AcceptCandidateForUser(id, userID)
		if err != nil {
			return nil, err
		}
		memories = append(memories, *m)
	}
	return memories, nil
}

func (s *service) BatchVerifyForUser(ids []string, status, userID string) error {
	for _, id := range ids {
		if _, err := s.memoryForUser(id, userID); err != nil {
			return err
		}
	}
	return s.BatchVerify(ids, status)
}

func (s *service) BatchSetImportanceForUser(ids []string, importance int, userID string) error {
	for _, id := range ids {
		if _, err := s.memoryForUser(id, userID); err != nil {
			return err
		}
	}
	return s.BatchSetImportance(ids, importance)
}

func (s *service) GetRankedMemoriesForUser(characterID, userID, query string, limit int) ([]RankedMemory, error) {
	return s.GetRankedMemories(characterID, userID, query, limit)
}

func (s *service) RebuildEmbeddingsForUser(userID string) (map[string]interface{}, error) {
	var memories []Memory
	query := s.db.Model(&Memory{})
	query = applyMemoryScopeQuery(query, "", userID)
	if err := query.Find(&memories).Error; err != nil {
		return nil, err
	}
	successCount, failCount := 0, 0
	for i := range memories {
		m := memories[i]
		if !memoryAllowedBySQLiteAuthority(m, retrievalAuthorityPolicy{UserID: userID, Now: time.Now()}) {
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

func (s *service) RebuildIndexForUser(userID string) (map[string]interface{}, error) {
	return s.RebuildEmbeddingsForUser(userID)
}

func (s *service) GetVectorStatusForUser(userID string) map[string]interface{} {
	query := s.db.Model(&Memory{})
	query = applyMemoryScopeQuery(query, "", userID)
	var total int64
	_ = query.Count(&total).Error
	var embedded int64
	_ = s.db.Table("memory_embeddings AS e").Joins("JOIN memories AS m ON m.id = e.memory_id").Where("m.user_id = ?", userID).Count(&embedded).Error
	return map[string]interface{}{"totalMemories": total, "totalEmbedded": embedded, "notEmbedded": maxInt64(total-embedded, 0), "enabled": qdrantDB.Client != nil, "providerName": "Qdrant"}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *service) RetrieveStatsForUser(userID string) (map[string]interface{}, error) {
	if !s.db.Migrator().HasColumn("retrieval_logs", "user_id") {
		if localSingleUserMode() {
			return s.RetrieveStats()
		}
		return map[string]interface{}{"recentLogs": []map[string]interface{}{}, "totalCount": 0}, nil
	}
	var rows []map[string]interface{}
	q := s.db.Table("retrieval_logs").Where("user_id = ?", userID)
	if err := q.Order("created_at DESC").Limit(50).Find(&rows).Error; err != nil {
		return nil, err
	}
	var total int64
	if err := s.db.Table("retrieval_logs").Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"recentLogs": rows, "totalCount": total}, nil
}
