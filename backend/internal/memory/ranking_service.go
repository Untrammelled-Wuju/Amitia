// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *service) GetRankedMemories(characterID, userID, query string, limit int) ([]RankedMemory, error) {
	if limit <= 0 {
		limit = 10
	}
	results, err := s.dynamicRecall(&VectorSearchRequest{
		Query: query, CharacterID: characterID, UserID: userID, Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	ranked := make([]RankedMemory, 0, len(results))
	for _, result := range results {
		ranked = append(ranked, RankedMemory{
			Memory: result.Memory, FinalScore: result.Score, VectorScore: result.VectorScore,
			KeywordScore: result.KeywordScore, ImportanceNorm: round4(float64(result.Memory.Importance) / 10),
			TemporalBoost: result.TemporalBoost, ValidityPenalty: result.ValidityPenalty, TemporalReference: result.TemporalReference,
		})
	}
	return ranked, nil
}

func rankedMemoryVectorFilters(characterID, userID string) []map[string]interface{} {
	characterID = strings.TrimSpace(characterID)
	userID = strings.TrimSpace(userID)
	filters := make([]map[string]interface{}, 0, 2)
	if characterID != "" {
		filters = append(filters, map[string]interface{}{"character_id": characterID})
	}
	if userID != "" && userID != characterID {
		filters = append(filters, map[string]interface{}{"user_id": userID, "scope_type": "user"})
	}
	if len(filters) == 0 {
		filters = append(filters, map[string]interface{}{})
	}
	return filters
}

func (s *service) BatchVerify(ids []string, status string) error {
	operationID := uuid.New().String()
	for _, id := range ids {
		now := time.Now().Format("2006-01-02 15:04:05")
		_, err := s.updateCanonicalMemory(id, canonicalUpdateRequest{
			Updates: map[string]interface{}{
				"verified_status":  status,
				"last_verified_at": now,
			},
			OperationID: operationID,
			EventType:   "memory_verified",
			EventReason: "batch_verify",
		})
		if err != nil {
			return err
		}
		if memoryStatusBlocksRetrieval(status) {
			if m, err := s.repo.FindByID(id); err == nil {
				deleteVectorsFromCollections([]string{id})
				_ = s.repo.UnmarkEmbedded(id)
				s.deleteGraph(m)
			}
		}
	}
	return nil
}

func (s *service) BatchSetImportance(ids []string, importance int) error {
	operationID := uuid.New().String()
	for _, id := range ids {
		_, err := s.updateCanonicalMemory(id, canonicalUpdateRequest{
			Updates: map[string]interface{}{
				"importance": importance,
			},
			OperationID: operationID,
			EventType:   "memory_updated",
			EventReason: "batch_set_importance",
		})
		if err != nil {
			return err
		}
	}
	return nil
}
