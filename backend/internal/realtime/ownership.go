// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"strings"

	"github.com/u-ai/backend/config"
	"gorm.io/gorm"
)

func realtimeLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func realtimeOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = strings.TrimSpace(requested)
	if requested != "" && stored == requested {
		return true
	}
	return realtimeLocalSingleUserMode() && requested != "" && (stored == "" || stored == "default")
}

func realtimeEffectiveUserID(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID != "" {
		return userID
	}
	if config.AppCfg == nil || realtimeLocalSingleUserMode() {
		return "default"
	}
	return ""
}

func requireRealtimeConversationOwner(conversationID, userID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", nil
	}
	if dbInstance == nil {
		return "", gorm.ErrInvalidDB
	}
	var row struct {
		UserID      string `gorm:"column:user_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := dbInstance.Table("conversations").Select("user_id, character_id").Where("id = ? AND deleted_at IS NULL", conversationID).Take(&row).Error; err != nil {
		return "", err
	}
	if !realtimeOwnerMatches(row.UserID, userID) {
		return "", gorm.ErrRecordNotFound
	}
	return strings.TrimSpace(row.CharacterID), nil
}

func requireRealtimeCharacterOwner(characterID, userID string) error {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return nil
	}
	if dbInstance == nil {
		return gorm.ErrInvalidDB
	}
	var row struct {
		UserID string `gorm:"column:user_id"`
	}
	if err := dbInstance.Table("characters").Select("user_id").Where("id = ?", characterID).Take(&row).Error; err != nil {
		return err
	}
	if !realtimeOwnerMatches(row.UserID, userID) {
		return gorm.ErrRecordNotFound
	}
	return nil
}
