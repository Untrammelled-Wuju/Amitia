// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/modelerror"
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

	var typeFiltered map[string]bool
	if len(req.Types) > 0 {
		typeFiltered = make(map[string]bool, len(req.Types))
		for _, t := range req.Types {
			if normalized, ok := NormalizeMemoryType(t); ok {
				typeFiltered[string(normalized)] = true
			}
		}
	}

	var layerFiltered map[MemoryLayer]bool
	if len(req.Layers) > 0 {
		layerFiltered = make(map[MemoryLayer]bool, len(req.Layers))
		for _, layer := range req.Layers {
			if IsValidLayer(string(layer)) {
				layerFiltered[MemoryLayer(strings.ToLower(strings.TrimSpace(string(layer))))] = true
			}
		}
	}

	var timeScopedIDs map[string]bool
	if req.Time != nil && s.temporalRepo != nil {
		scopedIDs, err := s.queryTimeScopedMemoryIDs(req)
		if err != nil {
			return nil, err
		}
		timeScopedIDs = scopedIDs
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
		if typeFiltered != nil && !typeFiltered[m.MemoryType] {
			continue
		}
		if layerFiltered != nil {
			layer := CanonicalLayer(collectionKeyFromCollectionName(collectionNameForMemoryType(m.MemoryType)))
			if !layerFiltered[layer] {
				continue
			}
		}
		if timeScopedIDs != nil && !timeScopedIDs[m.ID] {
			continue
		}
		filtered = append(filtered, m)
		if len(filtered) >= limit {
			break
		}
	}
	return filtered, nil
}

func (s *service) queryTimeScopedMemoryIDs(req *SearchMemoryRequest) (map[string]bool, error) {
	tf := req.Time
	basis := temporal.MemoryTimeBasis(tf.Basis)
	if basis == "" {
		basis = temporal.TimeBasisOccurred
	}
	query := temporal.MemoryTemporalQuery{Basis: temporal.MemoryTimeBasis(basis), Order: "desc"}
	if tf.FromUTC != nil && *tf.FromUTC != "" {
		if t, err := parseFilterTime(*tf.FromUTC); err == nil {
			query.OccurredFromUTC = &t
		}
	}
	if tf.ToUTC != nil && *tf.ToUTC != "" {
		if t, err := parseFilterTime(*tf.ToUTC); err == nil {
			query.OccurredToUTC = &t
		}
	}
	if tf.AtUTC != nil && *tf.AtUTC != "" {
		if t, err := parseFilterTime(*tf.AtUTC); err == nil {
			query.ValidAtUTC = &t
		}
	}
	query.LocalDateFrom = tf.LocalDateFrom
	query.LocalDateTo = tf.LocalDateTo
	query.Dayparts = tf.Dayparts
	query.Precisions = tf.Precisions
	query.Limit = 500
	ids, _, err := s.temporalRepo.QueryMemoryIDsByTime(query)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

func parseFilterTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", raw)
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
	vector, rawModelError, err := s.embeddingSvc.EmbedWithRawError(queryText)
	if rawModelError != "" {
		modelerror.Report(modelerror.Event{ModelType: "vector", ConversationID: req.ConversationID, RequestID: req.RequestID, Channel: req.Channel, RawError: rawModelError})
	}
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
	return s.dynamicRecall(req)
}
