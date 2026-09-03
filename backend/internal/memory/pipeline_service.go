// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	existingByExact := make(map[string]string)
	var existingMemories []struct {
		ID    string
		Key   string
		Value string
	}
	s.db.Table("memories").Select("id, key, value").Where("verified_status NOT IN (?, ?)", "replaced", "tombstone").Find(&existingMemories)
	for _, m := range existingMemories {
		existingByExact[m.Key+"|"+m.Value] = m.ID
	}

	acceptedCount := 0
	for i := range candidates {
		c := candidates[i]
		if strings.TrimSpace(c.Key) == "" || strings.TrimSpace(c.Value) == "" || c.Importance <= 0 {
			continue
		}
		exactKey := c.Key + "|" + c.Value
		if existingID := existingByExact[exactKey]; existingID != "" {
			if _, err := s.reinforceCanonicalMemory(existingID, &c); err == nil {
				_ = s.repo.DeleteCandidate(c.ID)
				acceptedCount++
			}
			continue
		}

		confidence := c.Confidence
		if confidence <= 0 {
			confidence = 50
		}
		autoRes, err := s.AutoResolveConflict(c.Key, c.Value, c.CharacterID, confidence)
		if err == nil && autoRes.Resolved {
			_ = s.repo.DeleteCandidate(c.ID)
			continue
		}

		mem, err := s.AcceptCandidate(c.ID)
		if err == nil && mem != nil {
			existingByExact[exactKey] = mem.ID
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
	if err := s.db.Table("conversations").Select("character_id").Where("id = ?", convID).Row().Scan(&charID); err != nil || strings.TrimSpace(charID) == "" {
		return
	}
	var count int64
	s.db.Table("memories").Where("character_id = ? AND verified_status NOT IN (?, ?)", charID, "replaced", "tombstone").Count(&count)
	if count < 10 {
		return
	}

	var lastMarker string
	s.db.Table("memory_events").Select("created_at").
		Where("character_id = ? AND event_type = 'memory_consolidated'", charID).
		Order("created_at DESC").Limit(1).Row().Scan(&lastMarker)
	if lastMarker != "" {
		if lastT, err := time.Parse("2006-01-02 15:04:05", lastMarker); err == nil && time.Since(lastT) < 2*time.Hour {
			return
		}
	}

	var pendingCount int64
	q := s.db.Table("memory_events").Where("character_id = ? AND event_type IN (?, ?)", charID, "memory_created", "memory_reinforced")
	if lastMarker != "" {
		q = q.Where("created_at > ?", lastMarker)
	}
	q.Count(&pendingCount)
	if pendingCount < 5 {
		return
	}
	go s.runConsolidation(charID)
}

func (s *service) runConsolidation(charID string) {
	result, err := s.RunConsolidation(&ConsolidationRequest{
		CharacterID: charID,
		Source:      "auto",
	})
	if err != nil {
		log.Warn("Pipeline consolidation failed:", err)
		return
	}
	if result == nil {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_ = s.db.Exec(`INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, confidence, source, character_id, created_at, version, operation_id, snapshot_hash, event_reason)
		VALUES (?, '', 'memory_consolidated', '', '', '', 0, 0, 'auto', ?, ?, 1, ?, '', 'consolidation_cycle')`,
		uuid.New().String(), charID, now, result.OperationID).Error
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
