// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/log"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

func (s *service) GetVectorStatus() map[string]interface{} {
	totalMem, embedded := s.repo.VectorStatus()
	collections := make([]map[string]interface{}, 0)
	totalEmbeddings := uint64(0)
	enabled := qdrantDB.Client != nil
	for _, collectionName := range qdrantDB.CollectionNames() {
		count := uint64(0)
		status := "disabled"
		if enabled {
			if c, err := qdrantDB.GetVectorCount(collectionName); err == nil {
				count = c
				status = "ready"
				totalEmbeddings += c
			} else {
				status = "error"
			}
		}
		collectionKey := collectionKeyFromCollectionName(collectionName)
		collections = append(collections, map[string]interface{}{
			"key":             collectionKey,
			"name":            collectionName,
			"label":           memoryLayerLabel(collectionKey),
			"totalEmbeddings": count,
			"status":          status,
		})
	}
	if totalEmbeddings > 0 {
		embedded = int64(totalEmbeddings)
	}
	notEmbedded := totalMem - embedded
	if notEmbedded < 0 {
		notEmbedded = 0
	}
	return map[string]interface{}{
		"totalMemories":   totalMem,
		"totalEmbedded":   embedded,
		"notEmbedded":     notEmbedded,
		"enabled":         enabled,
		"providerName":    "Qdrant",
		"totalEmbeddings": totalEmbeddings,
		"collections":     collections,
	}
}

func (s *service) SyncEmbedding(memID, key, value, characterID, memoryType string) bool {
	if qdrantDB.Client == nil {
		return false
	}
	unlock := s.acquireEmbeddingLock(memID)
	defer unlock()
	mem, errMem := s.repo.FindByID(memID)
	if errMem != nil || mem == nil {
		deleteVectorsFromCollections([]string{memID})
		_ = s.repo.UnmarkEmbedded(memID)
		return false
	}
	if !memoryAllowedBySQLiteAuthority(*mem, retrievalAuthorityPolicy{Now: time.Now()}) {
		deleteVectorsFromCollections([]string{memID})
		_ = s.repo.UnmarkEmbedded(memID)
		return false
	}
	key = mem.Key
	value = mem.Value
	characterID = mem.CharacterID
	memoryType = mem.MemoryType
	scopeType := mem.Scope
	userID := ""
	if strings.EqualFold(strings.TrimSpace(mem.Scope), "user") || strings.EqualFold(strings.TrimSpace(mem.Scope), "user_global") {
		userID = mem.CharacterID
	}
	signature := memoryEmbeddingSignature(*mem)
	text := key + " " + value
	vector, err := s.embeddingSvc.Embed(text)
	if err != nil {
		return false
	}
	current, errCurrent := s.repo.FindByID(memID)
	if errCurrent != nil || current == nil {
		deleteVectorsFromCollections([]string{memID})
		_ = s.repo.UnmarkEmbedded(memID)
		return false
	}
	if !memoryAllowedBySQLiteAuthority(*current, retrievalAuthorityPolicy{Now: time.Now()}) {
		deleteVectorsFromCollections([]string{memID})
		_ = s.repo.UnmarkEmbedded(memID)
		return false
	}
	if memoryEmbeddingSignature(*current) != signature {
		return false
	}
	payload := map[string]interface{}{
		"memory_id":    memID,
		"character_id": characterID,
		"memory_type":  memoryType,
		"scope_type":   scopeType,
		"memory_kind":  memoryType,
		"user_id":      userID,
		"value":        value,
	}
	collectionName := collectionNameForMemoryType(memoryType)
	err = qdrantDB.UpsertVectors([]qdrantDB.VectorPoint{
		{ID: memID, Vector: vector, Payload: payload},
	}, collectionName)
	if err != nil {
		log.Error("存储嵌入失败:", memID, err)
		return false
	}
	current, errCurrent = s.repo.FindByID(memID)
	if errCurrent != nil || current == nil || !memoryAllowedBySQLiteAuthority(*current, retrievalAuthorityPolicy{Now: time.Now()}) || memoryEmbeddingSignature(*current) != signature {
		deleteVectorsFromCollections([]string{memID})
		_ = s.repo.UnmarkEmbedded(memID)
		return false
	}
	if err := s.repo.MarkEmbedded(memID); err != nil {
		log.Warn("标记嵌入状态失败:", memID, err)
	}
	return true
}

func (s *service) acquireEmbeddingLock(memID string) func() {
	s.embedMu.Lock()
	if s.embedLocks == nil {
		s.embedLocks = map[string]*sync.Mutex{}
	}
	lock := s.embedLocks[memID]
	if lock == nil {
		lock = &sync.Mutex{}
		s.embedLocks[memID] = lock
	}
	s.embedMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func memoryEmbeddingSignature(m Memory) string {
	return strings.Join([]string{
		m.ID,
		m.Key,
		m.Value,
		m.CharacterID,
		m.MemoryType,
		m.Scope,
		m.VerifiedStatus,
		m.UpdatedAt,
	}, "\x00")
}

func (s *service) RebuildIndex() (map[string]interface{}, error) {
	return s.RebuildEmbeddings()
}

func (s *service) RebuildEmbeddings() (map[string]interface{}, error) {
	totalMem, embedded := s.repo.VectorStatus()
	if qdrantDB.Client == nil {
		var memories []Memory
		s.db.Find(&memories)
		for _, m := range memories {
			if !memoryAllowedBySQLiteAuthority(m, retrievalAuthorityPolicy{Now: time.Now()}) {
				continue
			}
			s.syncGraph(&m)
		}
		return map[string]interface{}{
			"totalMemories": totalMem,
			"embedded":      embedded,
			"status":        "qdrant_not_available",
		}, nil
	}
	var memories []Memory
	s.db.Find(&memories)
	successCount := 0
	failCount := 0
	for _, m := range memories {
		if !memoryAllowedBySQLiteAuthority(m, retrievalAuthorityPolicy{Now: time.Now()}) {
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
	return map[string]interface{}{
		"totalMemories": totalMem,
		"embedded":      int64(successCount),
		"failed":        int64(failCount),
		"status":        status,
	}, nil
}
