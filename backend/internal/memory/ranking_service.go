// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/temporal"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func (s *service) GetRankedMemories(characterID, userID, query string, limit int) ([]RankedMemory, error) {
	if limit <= 0 {
		limit = 10
	}
	if s.dataLifecycleCoordinator != nil && s.dataLifecycleCoordinator.IsRetrievalBlocked(characterID) {
		return nil, nil
	}

	allMemories, _, err := s.repo.List(MemoryListQuery{
		CharacterID: characterID,
		UserID:      userID,
		PageSize:    200,
		Page:        1,
	})
	if err != nil {
		return nil, err
	}
	policy := retrievalAuthorityPolicy{
		CharacterID: characterID,
		UserID:      userID,
		Now:         time.Now(),
	}

	vectorScores := make(map[string]float64)
	if qdrantDB.Client != nil && query != "" {
		vector, err := s.embeddingSvc.Embed(query)
		if err == nil {
			for _, filter := range rankedMemoryVectorFilters(characterID, userID) {
				results, err := qdrantDB.MultiSearch(vector, 50, filter)
				if err != nil {
					continue
				}
				for _, r := range results {
					if val, ok := r.Point.Payload["memory_id"]; ok {
						rawMemID := val.GetStringValue()
						if float64(r.Point.Score) > vectorScores[rawMemID] {
							vectorScores[rawMemID] = float64(r.Point.Score)
						}
					}
				}
			}
		}
	}

	queryLower := strings.ToLower(query)
	var ranked []RankedMemory
	for _, m := range allMemories {
		if !memoryAllowedBySQLiteAuthority(m, policy) {
			continue
		}
		vs := vectorScores[m.ID]
		ks := keywordMatchScore(queryLower, m.Key, m.Value)
		is := float64(m.Importance) / 10.0

		finalScore := vs*0.4 + ks*0.3 + is*0.3
		if finalScore > 0 {
			ranked = append(ranked, RankedMemory{
				Memory:         m,
				FinalScore:     math.Round(finalScore*10000) / 10000,
				VectorScore:    math.Round(vs*10000) / 10000,
				KeywordScore:   math.Round(ks*10000) / 10000,
				ImportanceNorm: math.Round(is*10000) / 10000,
			})
		}
	}
	if s.temporalReranker != nil && len(ranked) > 0 {
		candidates := make([]temporal.MemoryScoreCandidate, 0, len(ranked))
		for _, result := range ranked {
			candidates = append(candidates, temporal.MemoryScoreCandidate{MemoryID: result.Memory.ID, BaseScore: result.FinalScore, CreatedAt: result.Memory.CreatedAt, MemoryType: result.Memory.MemoryType})
		}
		if reranked, rerankErr := s.temporalReranker.RerankMemoryScores(context.Background(), query, candidates); rerankErr == nil {
			for index := range ranked {
				if score, exists := reranked[ranked[index].Memory.ID]; exists {
					ranked[index].FinalScore = score.FinalScore
					ranked[index].TemporalBoost = score.TemporalBoost
					ranked[index].ValidityPenalty = score.ValidityPenalty
					ranked[index].TemporalReference = score.ReferenceSource
				}
			}
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].FinalScore > ranked[j].FinalScore
	})

	if len(ranked) > limit {
		ranked = ranked[:limit]
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
	for _, id := range ids {
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := s.repo.Update(id, map[string]interface{}{
			"verified_status":  status,
			"last_verified_at": now,
		}); err != nil {
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
	for _, id := range ids {
		s.repo.Update(id, map[string]interface{}{"importance": importance})
	}
	return nil
}
