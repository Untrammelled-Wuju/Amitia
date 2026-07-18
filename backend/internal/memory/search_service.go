// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/temporal"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"

	"github.com/u-ai/backend/log"
)

func (s *service) Search(req *SearchMemoryRequest) ([]Memory, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	fetchLimit := limit * 3
	if fetchLimit < limit+20 {
		fetchLimit = limit + 20
	}
	items, err := s.repo.Search(req.Keyword, req.CharacterID, req.UserID, fetchLimit)
	if err != nil {
		return nil, err
	}
	tombstoneBlocked := tombstoneTargetsFromMemorySearch(s.dataLifecycleCoordinator, req.CharacterID)
	policy := retrievalAuthorityPolicy{
		CharacterID: req.CharacterID,
		UserID:      req.UserID,
		Now:         time.Now(),
	}
	filtered := make([]Memory, 0, min(limit, len(items)))
	for _, m := range items {
		if tombstoneBlocked != nil && (tombstoneBlocked[m.CharacterID] || tombstoneBlocked[m.ID]) {
			continue
		}
		if !memoryAllowedBySQLiteAuthority(m, policy) {
			continue
		}
		filtered = append(filtered, m)
		if len(filtered) >= limit {
			break
		}
	}
	return filtered, nil
}

func (s *service) VectorSearch(req *VectorSearchRequest) ([]VectorSearchResult, error) {
	if qdrantDB.Client == nil {
		return nil, fmt.Errorf("向量数据库未初始化")
	}
	if s.dataLifecycleCoordinator != nil && s.dataLifecycleCoordinator.IsRetrievalBlocked(req.CharacterID) {
		return nil, fmt.Errorf("数据已标记删除")
	}
	queryText := req.Query

	var blockedMemoryIDs map[string]bool
	if s.dataLifecycleCoordinator != nil {
		memoryBlockedIDs := s.dataLifecycleCoordinator.BlockedEntityIDsByType("memory")
		if len(memoryBlockedIDs) > 0 {
			blockedMemoryIDs = make(map[string]bool, len(memoryBlockedIDs))
			for _, id := range memoryBlockedIDs {
				blockedMemoryIDs[id] = true
			}
		}
	}
	if queryText == "" {
		queryText = req.Keyword
	}
	if queryText == "" {
		return nil, fmt.Errorf("缺少查询文本")
	}
	vector, err := s.embeddingSvc.Embed(queryText)
	if err != nil {
		return nil, fmt.Errorf("向量化失败: %w", err)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}
	filters := rankedMemoryVectorFilters(req.CharacterID, req.UserID)
	log.Info("VectorSearch with filter",
		"characterID", req.CharacterID,
		"userID", req.UserID,
		"query", queryText,
		"limit", limit,
	)
	results := make([]qdrantDB.CollectionScoredPoint, 0, limit*len(filters))
	var lastErr error
	for _, filter := range filters {
		scopedResults, err := qdrantDB.MultiSearch(vector, limit+1, filter, "memory_embeddings")
		if err != nil {
			lastErr = err
			continue
		}
		results = append(results, scopedResults...)
	}
	if len(results) == 0 && lastErr != nil {
		return nil, fmt.Errorf("向量检索失败: %w", lastErr)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Point.Score > results[j].Point.Score
	})
	var vsResults []VectorSearchResult
	seen := map[string]bool{}
	policy := retrievalAuthorityPolicy{
		CharacterID:      req.CharacterID,
		UserID:           req.UserID,
		ProactiveMention: req.ProactiveMention,
		Now:              time.Now(),
	}
	for _, r := range results {
		memID := ""
		if val, ok := r.Point.Payload["memory_id"]; ok {
			memID = val.GetStringValue()
		}
		if memID == "" || seen[memID] {
			continue
		}
		m, err := s.repo.FindByID(memID)
		if err != nil {
			continue
		}
		if !memoryAllowedBySQLiteAuthority(*m, policy) {
			continue
		}
		if blockedMemoryIDs != nil && blockedMemoryIDs[memID] {
			continue
		}
		seen[memID] = true
		vsResults = append(vsResults, VectorSearchResult{
			Memory:         *m,
			Score:          float32(r.Point.Score),
			CollectionName: r.CollectionName,
			MemoryLayer:    memoryLayerLabel(collectionKeyFromCollectionName(r.CollectionName)),
			MatchType:      "vector",
		})
		if len(vsResults) >= limit {
			break
		}
	}
	log.Info("VectorSearch completed",
		"characterID", req.CharacterID,
		"userID", req.UserID,
		"results", len(vsResults),
		"total", len(results),
	)
	return vsResults, nil
}

func (s *service) HybridSearch(req *VectorSearchRequest) ([]HybridSearchResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	queryText := req.Query
	if queryText == "" {
		queryText = req.Keyword
	}
	if queryText == "" {
		return nil, fmt.Errorf("缺少查询文本")
	}
	var blockedMemoryIDs map[string]bool
	if s.dataLifecycleCoordinator != nil {
		memoryBlockedIDs := s.dataLifecycleCoordinator.BlockedEntityIDsByType("memory")
		if len(memoryBlockedIDs) > 0 {
			blockedMemoryIDs = make(map[string]bool, len(memoryBlockedIDs))
			for _, id := range memoryBlockedIDs {
				blockedMemoryIDs[id] = true
			}
		}
	}

	vectorFetchLimit := limit * 2
	if vectorFetchLimit < 20 {
		vectorFetchLimit = 20
	}
	vectorResults, _ := s.VectorSearch(&VectorSearchRequest{
		Query:            queryText,
		CharacterID:      req.CharacterID,
		UserID:           req.UserID,
		Limit:            vectorFetchLimit,
		ConversationID:   req.ConversationID,
		RequestID:        req.RequestID,
		Channel:          req.Channel,
		ProactiveMention: req.ProactiveMention,
	})

	scorer := &RetrievalScorer{}
	pipelineResults := scorer.Pipeline(vectorResults)

	merged := map[string]*struct {
		m              Memory
		vectorScore    float64
		keywordScore   float64
		collectionName string
		matchType      string
	}{}
	for _, pr := range pipelineResults {
		merged[pr.Memory.ID] = &struct {
			m              Memory
			vectorScore    float64
			keywordScore   float64
			collectionName string
			matchType      string
		}{m: pr.Memory, vectorScore: pr.VectorScore, collectionName: pr.CollectionName, matchType: pr.MatchType}
	}

	keywordResults, err := s.repo.Search(queryText, req.CharacterID, req.UserID, limit*2)
	if err != nil {
		keywordResults = nil
	}
	queryLower := strings.ToLower(queryText)
	policy := retrievalAuthorityPolicy{
		CharacterID:      req.CharacterID,
		UserID:           req.UserID,
		ProactiveMention: req.ProactiveMention,
		Now:              time.Now(),
	}
	for _, m := range keywordResults {
		if !memoryAllowedBySQLiteAuthority(m, policy) {
			continue
		}
		if blockedMemoryIDs != nil && blockedMemoryIDs[m.ID] {
			continue
		}
		item, exists := merged[m.ID]
		if !exists {
			item = &struct {
				m              Memory
				vectorScore    float64
				keywordScore   float64
				collectionName string
				matchType      string
			}{m: m, collectionName: collectionNameForMemoryType(m.MemoryType), matchType: "keyword"}
			merged[m.ID] = item
		}
		score := keywordMatchScore(queryLower, m.Key, m.Value)
		if score <= 0 {
			score = 0.5
		}
		if score > item.keywordScore {
			item.keywordScore = score
		}
		if item.matchType == "vector" {
			item.matchType = "hybrid"
		}
	}

	results := make([]HybridSearchResult, 0, len(merged))
	for _, item := range merged {
		score := item.vectorScore*0.6 + item.keywordScore*0.4
		if item.matchType == "hybrid" {
			score += 0.1
		}
		collectionKey := collectionKeyFromCollectionName(item.collectionName)
		results = append(results, HybridSearchResult{
			Memory:         item.m,
			Score:          math.Round(score*10000) / 10000,
			VectorScore:    math.Round(item.vectorScore*10000) / 10000,
			KeywordScore:   math.Round(item.keywordScore*10000) / 10000,
			MatchType:      item.matchType,
			CollectionName: item.collectionName,
			MemoryLayer:    memoryLayerLabel(collectionKey),
		})
	}
	if s.temporalReranker != nil && len(results) > 0 {
		candidates := make([]temporal.MemoryScoreCandidate, 0, len(results))
		for _, result := range results {
			candidates = append(candidates, temporal.MemoryScoreCandidate{MemoryID: result.Memory.ID, BaseScore: result.Score, CreatedAt: result.Memory.CreatedAt, MemoryType: result.Memory.MemoryType})
		}
		if reranked, rerankErr := s.temporalReranker.RerankMemoryScores(context.Background(), queryText, candidates); rerankErr == nil {
			for index := range results {
				if score, exists := reranked[results[index].Memory.ID]; exists {
					results[index].Score = score.FinalScore
					results[index].TemporalBoost = score.TemporalBoost
					results[index].ValidityPenalty = score.ValidityPenalty
					results[index].TemporalReference = score.ReferenceSource
				}
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}

	memoryIDs := make([]string, len(results))
	for i, r := range results {
		memoryIDs[i] = r.Memory.ID
	}

	s.logRetrieval(req.ConversationID, req.CharacterID, req.RequestID, req.Channel, queryText, memoryIDs, results)

	return results, nil
}
