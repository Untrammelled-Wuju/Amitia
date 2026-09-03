// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *service) reinforceCanonicalMemory(memoryID string, candidate *MemoryCandidate) (*Memory, error) {
	if strings.TrimSpace(memoryID) == "" {
		return nil, fmt.Errorf("memory id required")
	}
	existing, err := s.repo.FindByID(memoryID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	strength := reinforcementStrength(memoryEffectiveStrength(*existing, time.Now()), existing.ReinforceCount)
	level := suggestedRetentionAfterReinforce(existing.RetentionLevel, strength, existing.ReinforceCount+1)
	updates := map[string]interface{}{
		"last_reinforced_at":  now,
		"strength_updated_at": now,
		"reinforce_count":     existing.ReinforceCount + 1,
		"memory_strength":     strength,
		"retention_level":     level,
		"decay_state":         DecayStateActive,
		"archived_at":         nil,
	}
	if candidate != nil {
		if candidate.Importance > existing.Importance {
			updates["importance"] = candidate.Importance
		}
		if candidate.Confidence > existing.Confidence {
			updates["confidence"] = candidate.Confidence
		}
		if strings.TrimSpace(candidate.MemorySubtype) != "" && strings.TrimSpace(existing.MemorySubtype) == "" {
			updates["memory_subtype"] = strings.TrimSpace(candidate.MemorySubtype)
		}
	}
	m, err := s.updateCanonicalMemory(memoryID, canonicalUpdateRequest{
		Updates:     updates,
		OperationID: uuid.New().String(),
		EventType:   "memory_reinforced",
		EventReason: "independent_evidence",
	})
	if err != nil {
		return nil, err
	}
	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	return m, nil
}

func (s *service) maintainRetentionForMemory(m *Memory, now time.Time) {
	if s == nil || s.db == nil || m == nil || m.ID == "" || m.Pinned || memoryStatusBlocksRetrieval(m.VerifiedStatus) {
		return
	}
	strength := memoryEffectiveStrength(*m, now)
	state, level := decayStateFor(m.RetentionLevel, strength)
	if state == m.DecayState && level == normalizeRetentionLevel(m.RetentionLevel) {
		m.MemoryStrength = strength
		return
	}
	anchor := now.Format("2006-01-02 15:04:05")
	updates := map[string]interface{}{
		"retention_level":     level,
		"decay_state":         state,
		"memory_strength":     strength,
		"strength_updated_at": anchor,
	}
	if state == DecayStateArchived {
		ts := now.Format("2006-01-02 15:04:05")
		updates["archived_at"] = &ts
	}
	_ = s.db.Model(&Memory{}).Where("id = ?", m.ID).Updates(updates).Error
	m.RetentionLevel = level
	m.DecayState = state
	m.MemoryStrength = strength
	m.StrengthUpdatedAt = &anchor
	if state == DecayStateArchived {
		s.invalidateProfileProjection(m.ID)
		s.deleteGraph(m)
	}
}

func (s *service) refreshRetentionForListScope(characterID, userID string) {
	if s == nil || s.db == nil {
		return
	}
	key := strings.TrimSpace(characterID) + "|" + strings.TrimSpace(userID)
	now := time.Now()
	s.retentionRefreshMu.Lock()
	if s.retentionRefreshed == nil {
		s.retentionRefreshed = map[string]time.Time{}
	}
	if refreshedAt, ok := s.retentionRefreshed[key]; ok && now.Sub(refreshedAt) < 2*time.Minute {
		s.retentionRefreshMu.Unlock()
		return
	}
	s.retentionRefreshed[key] = now
	s.retentionRefreshMu.Unlock()

	query := applyMemoryScopeQuery(s.db.Model(&Memory{}), characterID, userID).
		Where("pinned = 0").
		Where("(decay_state IS NULL OR decay_state = '' OR decay_state != ?)", DecayStateArchived).
		Where("verified_status NOT IN (?, ?)", "replaced", "tombstone")
	var items []Memory
	if err := query.Find(&items).Error; err != nil {
		return
	}
	for i := range items {
		s.maintainRetentionForMemory(&items[i], now)
	}
}
