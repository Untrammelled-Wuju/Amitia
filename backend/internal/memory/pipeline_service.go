// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"github.com/u-ai/backend/log"
)

func (s *service) Name() string { return "结构化事实" }

func (s *service) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	manager := pipelinecheckpoint.New(s.db)
	leaseOwner := fmt.Sprintf("memory:%s:%d", convID, time.Now().UTC().UnixNano())
	pending, maxSequence, acquired, err := manager.AcquirePendingRange(convID, "memory", 0, leaseOwner, 10*time.Minute)
	if err != nil || !acquired || len(pending) == 0 {
		return err
	}
	filteredMsgs := filterExtractableMessages(pending)
	if len(filteredMsgs) == 0 {
		return manager.AdvanceLeased(convID, "memory", maxSequence, fmt.Sprintf("memory:%s:%d", convID, maxSequence), leaseOwner)
	}
	candidates, err := s.generateCandidatesFromMessages(convID, filteredMsgs)
	if err != nil {
		return err
	}
	existingKeys := make(map[string]bool)
	var existingMemories []struct {
		Key   string
		Value string
	}
	s.db.Table("memories").Select("key, value").Find(&existingMemories)
	for _, m := range existingMemories {
		existingKeys[m.Key+"|"+m.Value] = true
	}

	acceptedCount := 0
	for _, c := range candidates {
		if c.Importance < 7 {
			continue
		}
		if existingKeys[c.Key+"|"+c.Value] {
			continue
		}

		autoRes, err := s.AutoResolveConflict(c.Key, c.Value, c.CharacterID, 50)
		if err == nil && autoRes.Resolved {
			continue
		}

		existingKeys[c.Key+"|"+c.Value] = true
		mem, err := s.AcceptCandidate(c.ID)
		if err == nil && mem != nil {
			s.SyncEmbedding(mem.ID, mem.Key, mem.Value, mem.CharacterID, mem.MemoryType)
			acceptedCount++
		}
	}

	if acceptedCount > 0 {
		s.consolidationNeeded(convID)
	}

	return manager.AdvanceLeased(convID, "memory", maxSequence, fmt.Sprintf("memory:%s:%d", convID, maxSequence), leaseOwner)
}

func (s *service) consolidationNeeded(convID string) {
	var charID string
	if err := s.db.Table("conversations").Select("character_id").Where("id = ?", convID).Row().Scan(&charID); err != nil {
		return
	}
	var count int64
	s.db.Table("memories").Where("character_id = ? AND verified_status != 'replaced' AND verified_status != 'tombstone'", charID).Count(&count)
	if count < 20 {
		return
	}

	var lastConsolidation string
	s.db.Table("memory_events").Select("created_at").
		Where("character_id = ? AND event_type = 'consolidation'", charID).
		Order("created_at DESC").Limit(1).Row().Scan(&lastConsolidation)

	if lastConsolidation != "" {
		lastT, err := time.Parse("2006-01-02 15:04:05", lastConsolidation)
		if err == nil && time.Since(lastT) < 2*time.Hour {
			return
		}
	}

	go s.runConsolidation(charID)
}

func (s *service) runConsolidation(charID string) {
	var count int64
	s.db.Table("memories").Where("character_id = ? AND verified_status != 'replaced' AND verified_status != 'tombstone'", charID).Count(&count)
	if count < 10 {
		return
	}

	var lastConsolidation string
	s.db.Table("memory_events").Select("created_at").
		Where("character_id = ? AND event_type IN ('memory_created', 'memory_reinforced', 'memory_merged')", charID).
		Order("created_at DESC").Limit(1).Row().Scan(&lastConsolidation)

	if lastConsolidation != "" {
		lastT, err := time.Parse("2006-01-02 15:04:05", lastConsolidation)
		if err == nil && time.Since(lastT) < 2*time.Hour {
			return
		}
	}

	_, err := s.RunConsolidation(&ConsolidationRequest{
		CharacterID: charID,
		Source:      "auto",
	})
	if err != nil {
		log.Warn("Pipeline consolidation failed:", err)
	}
}

func (s *service) logRetrieval(conversationID, characterID, requestID, channel, queryText string, memoryIDs []string, results []HybridSearchResult) {
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	memIDsJSON, _ := json.Marshal(memoryIDs)
	scoringDetails := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		scoringDetails = append(scoringDetails, map[string]interface{}{
			"id":         r.Memory.ID,
			"score":      r.Score,
			"matchType":  r.MatchType,
			"memoryType": r.Memory.MemoryType,
			"layer":      r.MemoryLayer,
		})
	}
	detailsJSON, _ := json.Marshal(scoringDetails)
	s.db.Exec(
		"INSERT INTO retrieval_logs (id, conversation_id, character_id, request_id, channel, query_text, retrieved_memory_ids, scoring_details, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, conversationID, characterID, requestID, channel, queryText, string(memIDsJSON), string(detailsJSON), now,
	)
}

func (s *service) RetrieveStats() (map[string]interface{}, error) {
	type logRow struct {
		QueryText          string `json:"queryText"`
		RetrievedMemoryIDs string `json:"retrievedMemoryIDs"`
		ScoringDetails     string `json:"scoringDetails"`
		CreatedAt          string `json:"createdAt"`
	}
	var rows []logRow
	s.db.Table("retrieval_logs").Order("created_at DESC").Limit(50).Find(&rows)
	if rows == nil {
		rows = []logRow{}
	}
	var total int64
	s.db.Table("retrieval_logs").Count(&total)
	return map[string]interface{}{
		"recentLogs": rows,
		"totalCount": total,
	}, nil
}
