// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/business"
	promptir "github.com/u-ai/backend/internal/prompt"
)

func (s *service) profileExtractionSpaceID(convID, charID string) string {
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

func (s *service) buildMemoryInjectItems(results []business.HybridSearchResult) string {
	if len(results) == 0 {
		return promptir.BuildMemoryInjectGuardrailOnly()
	}
	if len(results) > business.MaxMemoryInjectTotal {
		results = results[:business.MaxMemoryInjectTotal]
	}
	injectItems := make([]promptir.MemoryInjectItem, 0, len(results))
	for _, r := range results {
		if isMemoryBlocked(r.Memory) {
			continue
		}
		cat := string(business.CanonicalMemoryType(r.Memory.MemoryType))
		tier := mapMemoryToTier(r)
		injectItems = append(injectItems, promptir.MemoryInjectItem{
			Content:  r.Memory.Value,
			Category: cat,
			Tier:     tier,
		})
	}
	return promptir.BuildMemoryInjectRawSection(injectItems)
}

func mapMemoryToTier(result business.HybridSearchResult) promptir.MemoryInjectTier {
	if result.SourceType == "episodic" {
		return promptir.TierRecent
	}
	switch result.Memory.RetentionLevel {
	case business.RetentionL1, business.RetentionL2:
		return promptir.TierLongTerm
	case business.RetentionL3:
		return promptir.TierRoleRel
	default:
		return promptir.TierRecent
	}
}

func isMemoryBlocked(m business.Memory) bool {
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
