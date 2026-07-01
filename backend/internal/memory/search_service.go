// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	internalQdrant "github.com/u-ai/backend/internal/qdrant"

	"github.com/u-ai/backend/log"
)

func (s *service) Search(req *SearchMemoryRequest) ([]Memory, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	return s.repo.Search(req.Keyword, req.CharacterID, limit)
}

func (s *service) VectorSearch(req *VectorSearchRequest) ([]VectorSearchResult, error) {
	if qdrantDB.Client == nil {
		return nil, fmt.Errorf("向量数据库未初始化")
	}
	queryText := req.Query
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
	filter := internalQdrant.FilterBuilder{CharacterID: req.CharacterID}.Build()
	log.Info("VectorSearch with filter",
		"characterID", filter.CharacterID,
		"scopeType", filter.ScopeType,
		"memoryKind", filter.MemoryKind,
		"query", queryText,
		"limit", limit,
	)
	client := &internalQdrant.QdrantClient{}
	results, err := client.SearchWithFilter(context.Background(), "memory_embeddings", vector, filter, limit+1)
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Point.Score > results[j].Point.Score
	})
	var vsResults []VectorSearchResult
	seen := map[string]bool{}
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
		if req.CharacterID != "" && m.CharacterID != req.CharacterID {
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
		"characterID", filter.CharacterID,
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

	vectorFetchLimit := limit * 2
	if vectorFetchLimit < 20 {
		vectorFetchLimit = 20
	}
	vectorResults, _ := s.VectorSearch(&VectorSearchRequest{
		Query:       queryText,
		CharacterID: req.CharacterID,
		Limit:       vectorFetchLimit,
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

	keywordResults, err := s.repo.Search(queryText, req.CharacterID, limit*2)
	if err != nil {
		keywordResults = nil
	}
	queryLower := strings.ToLower(queryText)
	for _, m := range keywordResults {
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

	s.logRetrieval(req.CharacterID, queryText, memoryIDs, results)

	return results, nil
}
