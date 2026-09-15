// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"strings"

	"github.com/u-ai/backend/internal/requestidentity"
	"time"

	"github.com/google/uuid"
)

func (s *service) syncProfileProjection(m *Memory) {
	if s == nil || s.db == nil || m == nil || m.ID == "" {
		return
	}
	category, ok := profileCategoryForMemory(*m)
	if !ok {
		s.invalidateProfileProjection(m.ID)
		return
	}
	status := "active"
	if memoryStatusBlocksRetrieval(m.VerifiedStatus) || strings.EqualFold(m.DecayState, DecayStateArchived) {
		status = "archived"
	}
	spaceID, characterID := s.profileProjectionScope(m)
	if spaceID == "" {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")

	var existingID string
	_ = s.db.Table("user_profiles").Select("id").Where("source_memory_id = ?", m.ID).Limit(1).Row().Scan(&existingID)
	if existingID == "" {
		_ = s.db.Table("user_profiles").Select("id").Where(
			"space_id = ? AND character_id = ? AND category = ? AND attribute_name = ?",
			spaceID, characterID, category, strings.TrimSpace(m.Key),
		).Limit(1).Row().Scan(&existingID)
	}
	values := map[string]interface{}{
		"space_id":          spaceID,
		"character_id":      characterID,
		"category":          category,
		"attribute_name":    strings.TrimSpace(m.Key),
		"attribute_value":   strings.TrimSpace(m.Value),
		"source":            "memory_projection",
		"confidence":        m.Confidence,
		"source_conv_id":    m.SourceConvID,
		"source_memory_id":  m.ID,
		"projection_status": status,
		"updated_at":        now,
	}
	if existingID != "" {
		_ = s.db.Table("user_profiles").Where("id = ?", existingID).Updates(values).Error
		return
	}
	values["id"] = uuid.New().String()
	values["created_at"] = now
	_ = s.db.Table("user_profiles").Create(values).Error
}

func (s *service) profileProjectionScope(m *Memory) (string, string) {
	if m == nil {
		return "", ""
	}
	spaceID := strings.TrimSpace(m.SpaceID)
	characterID := strings.TrimSpace(m.CharacterID)
	if s != nil && s.db != nil && strings.TrimSpace(m.SourceConvID) != "" {
		var peerID, convCharacterID string
		_ = s.db.Table("conversations").Select("peer_id, character_id").Where("id = ?", m.SourceConvID).Row().Scan(&peerID, &convCharacterID)
		peerID = strings.TrimSpace(peerID)
		convCharacterID = strings.TrimSpace(convCharacterID)
		if spaceID == "" && peerID != "" && peerID != "default" {
			spaceID = peerID
		}
		if convCharacterID != "" {
			characterID = convCharacterID
		}
	}
	spaceID = requestidentity.NormalizeSpaceID(spaceID)
	return spaceID, characterID
}

func (s *service) invalidateProfileProjection(memoryID string) {
	if s == nil || s.db == nil || strings.TrimSpace(memoryID) == "" {
		return
	}
	_ = s.db.Table("user_profiles").Where("source_memory_id = ?", memoryID).Updates(map[string]interface{}{
		"projection_status": "archived",
		"updated_at":        time.Now().Format("2006-01-02 15:04:05"),
	}).Error
}

func profileCategoryForMemory(m Memory) (string, bool) {
	subtype := strings.ToUpper(strings.TrimSpace(m.MemorySubtype))
	switch subtype {
	case "BASIC_PROFILE":
		return "personal_info", true
	case "TASTES", "LIFESTYLE":
		return "preference", true
	case "ROUTINES", "PROCEDURES":
		return "habit", true
	case "OUR_BOND", "FAMILY", "FRIENDS", "PARTNER":
		return "relationship", true
	case "HEALTH":
		return "health", true
	case "VULNERABILITIES":
		return "fear", true
	case "PLANS", "GOALS", "COMMITMENTS":
		return "plan", true
	}
	switch CanonicalMemoryType(m.MemoryType) {
	case MemoryTypePersonalInfo:
		return "personal_info", true
	case MemoryTypePreference, MemoryTypeHobby:
		return "preference", true
	case MemoryTypeHabit:
		return "habit", true
	case MemoryTypeRelationship:
		return "relationship", true
	case MemoryTypePlan:
		return "plan", true
	default:
		return "", false
	}
}
