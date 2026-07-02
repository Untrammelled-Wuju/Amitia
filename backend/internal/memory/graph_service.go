// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"strings"
)

func (s *service) SyncGraphMemory(id string) bool {
	m, err := s.repo.FindByID(id)
	if err != nil || m == nil {
		return false
	}
	s.syncGraph(m)
	return true
}

func (s *service) syncGraph(m *Memory) {
	if s.graphSvc == nil || m == nil {
		return
	}
	userID := strings.TrimSpace(m.CharacterID)
	if m.Scope == "user" && m.CharacterID != "" {
		userID = m.CharacterID
	}
	if userID == "" || userID == "default" {
		return
	}
	label := strings.TrimSpace(m.Key)
	if label == "" {
		label = strings.TrimSpace(m.Value)
	}
	_ = s.graphSvc.SyncNode("memory", m.ID, label, map[string]interface{}{
		"key":             m.Key,
		"value":           m.Value,
		"memory_type":     m.MemoryType,
		"source":          m.Source,
		"scope":           m.Scope,
		"importance":      m.Importance,
		"confidence":      m.Confidence,
		"character_id":    m.CharacterID,
		"user_id":         userID,
		"entity_type":     m.EntityType,
		"source_msg_id":   m.SourceMsgID,
		"source_conv_id":  m.SourceConvID,
		"verified_status": m.VerifiedStatus,
		"created_at":      m.CreatedAt,
		"updated_at":      m.UpdatedAt,
	})
	_ = s.graphSvc.SyncNode("user", userID, userID, map[string]interface{}{"user_id": userID})
	if m.CharacterID != "" {
		_ = s.graphSvc.SyncNode("character", m.CharacterID, m.CharacterID, map[string]interface{}{"character_id": m.CharacterID, "user_id": userID})
	}
	if m.EntityID != "" {
		entityType := strings.TrimSpace(m.EntityType)
		if entityType == "" {
			entityType = "entity"
		}
		_ = s.graphSvc.SyncNode(entityType, m.EntityID, m.EntityID, map[string]interface{}{"user_id": userID})
	}
}

func (s *service) deleteGraph(m *Memory) {
	if s.graphSvc == nil || m == nil {
		return
	}
	_ = s.graphSvc.DeleteNode("memory:" + m.ID)
	if m.CharacterID != "" {
		_ = s.graphSvc.DeleteNodeIfOrphan("character:" + m.CharacterID)
	}
	if m.EntityID != "" {
		entityType := strings.TrimSpace(m.EntityType)
		if entityType == "" {
			entityType = "entity"
		}
		_ = s.graphSvc.DeleteNodeIfOrphan(entityType + ":" + m.EntityID)
	}
}
