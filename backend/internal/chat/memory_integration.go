// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"strings"

	"github.com/u-ai/backend/internal/memory"
	promptir "github.com/u-ai/backend/internal/prompt"
)

func (s *service) detectEpisodicMoment(convID, charID string) {
	if s.episodicPort == nil {
		return
	}
	messages := s.loadHistory(convID)
	if len(messages) == 0 {
		return
	}
	s.episodicPort.ExtractFromConversation(charID, convID, messages)
}

func (s *service) extractProfile(convID, charID string) {
	if s.profilePort == nil {
		return
	}
	messages := s.loadHistory(convID)
	if len(messages) == 0 {
		return
	}
	s.profilePort.ExtractFromConversation(s.profileExtractionUserID(convID, charID), convID, messages, charID)
}

func (s *service) autoExtractMemories(convID, charID string) {
	if s.memoryPort == nil {
		return
	}
	candidates, err := s.memoryPort.GenerateCandidates(convID)
	if err != nil || len(candidates) == 0 {
		return
	}

	existingKeys := map[string]bool{}
	var existingMemories []struct {
		Key   string
		Value string
	}
	s.db.Table("memories").Select("key, value").Find(&existingMemories)
	for _, m := range existingMemories {
		existingKeys[m.Key+"|"+m.Value] = true
	}

	for _, c := range candidates {
		if existingKeys[c.Key+"|"+c.Value] {
			continue
		}
		existingKeys[c.Key+"|"+c.Value] = true
		s.memoryPort.AcceptCandidate(c.ID)
	}
}

func (s *service) profileExtractionUserID(convID, charID string) string {
	fallback := strings.TrimSpace(charID)
	if s.db == nil || strings.TrimSpace(convID) == "" {
		return fallback
	}
	var peerID string
	if err := s.db.Table("conversations").Select("peer_id").Where("id = ?", convID).Row().Scan(&peerID); err != nil {
		return fallback
	}
	peerID = strings.TrimSpace(peerID)
	if peerID == "" || peerID == "default" {
		return fallback
	}
	return peerID
}

func (s *service) buildMemoryInjectItems(results []memory.HybridSearchResult) string {
	if len(results) == 0 {
		return promptir.BuildMemoryInjectGuardrailOnly()
	}
	if len(results) > memory.MaxMemoryInjectTotal {
		results = results[:memory.MaxMemoryInjectTotal]
	}
	injectItems := make([]promptir.MemoryInjectItem, 0, len(results))
	for _, r := range results {
		if isMemoryBlocked(r.Memory) {
			continue
		}
		cat := string(memory.CanonicalMemoryType(r.Memory.MemoryType))
		tier := mapMemoryToTier(r)
		injectItems = append(injectItems, promptir.MemoryInjectItem{
			Content:  r.Memory.Value,
			Category: cat,
			Tier:     tier,
		})
	}
	return promptir.BuildMemoryInjectRawSection(injectItems)
}

func mapMemoryToTier(result memory.HybridSearchResult) promptir.MemoryInjectTier {
	if result.SourceType == "episodic" {
		return promptir.TierRecent
	}
	switch result.Memory.RetentionLevel {
	case memory.RetentionL1, memory.RetentionL2:
		return promptir.TierLongTerm
	case memory.RetentionL3:
		return promptir.TierRoleRel
	default:
		return promptir.TierRecent
	}
}

func isMemoryBlocked(m memory.Memory) bool {
	if m.AllowContextUse != nil && !*m.AllowContextUse {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(m.VerifiedStatus))
	switch status {
	case "deleted", "invalidated", "expired", "rejected", "tombstone", "tombstoned", "inactive":
		return true
	}
	return false
}
