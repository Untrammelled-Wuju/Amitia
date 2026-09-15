// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package profile

import (
	"strings"

	"github.com/u-ai/backend/config"
	"gorm.io/gorm"
)

func profileLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func profileOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = strings.TrimSpace(requested)
	if requested != "" && stored == requested {
		return true
	}
	return profileLocalSingleUserMode() && requested != "" && (stored == "" || stored == "default")
}

func (s *service) requireProfileCharacterOwner(characterID, spaceID string) error {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return nil
	}
	var owner string
	if err := s.db.Table("characters").Select("space_id").Where("id = ?", characterID).Take(&owner).Error; err != nil {
		return err
	}
	if !profileOwnerMatches(owner, spaceID) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *service) requireProfileConversationOwner(conversationID, spaceID, requestedCharacterID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		if err := s.requireProfileCharacterOwner(requestedCharacterID, spaceID); err != nil {
			return "", err
		}
		return strings.TrimSpace(requestedCharacterID), nil
	}
	var row struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := s.db.Table("conversations").Select("space_id, character_id").Where("id = ? AND deleted_at IS NULL", conversationID).Take(&row).Error; err != nil {
		return "", err
	}
	if !profileOwnerMatches(row.SpaceID, spaceID) {
		return "", gorm.ErrRecordNotFound
	}
	conversationCharacterID := strings.TrimSpace(row.CharacterID)
	requestedCharacterID = strings.TrimSpace(requestedCharacterID)
	if requestedCharacterID != "" && requestedCharacterID != conversationCharacterID {
		return "", gorm.ErrRecordNotFound
	}
	return conversationCharacterID, nil
}

func (s *service) CreateForSpace(req *CreateProfileRequest, spaceID string) (*UserProfile, error) {
	if req == nil {
		return nil, gorm.ErrInvalidData
	}
	characterID, err := s.requireProfileConversationOwner(req.SourceConvID, spaceID, req.CharacterID)
	if err != nil {
		return nil, err
	}
	copyReq := *req
	copyReq.SpaceID = strings.TrimSpace(spaceID)
	copyReq.CharacterID = characterID
	return s.Create(&copyReq)
}
