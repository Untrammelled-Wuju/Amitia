// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func episodicLocalMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}
func episodicOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = strings.TrimSpace(requested)
	if requested != "" && stored == requested {
		return true
	}
	return episodicLocalMode() && requested != "" && (stored == "" || stored == "default")
}

func (s *service) requireEpisodicCharacterOwner(characterID, spaceID string) error {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return nil
	}
	var owner string
	if err := s.db.Table("characters").Select("space_id").Where("id = ?", characterID).Take(&owner).Error; err != nil {
		return err
	}
	if !episodicOwnerMatches(owner, spaceID) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *service) requireEpisodicConversationOwner(conversationID, spaceID, requestedCharacterID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	requestedCharacterID = cleanScope(requestedCharacterID)
	if conversationID == "" {
		if requestedCharacterID == "" {
			return "", gorm.ErrRecordNotFound
		}
		if err := s.requireEpisodicCharacterOwner(requestedCharacterID, spaceID); err != nil {
			return "", err
		}
		return requestedCharacterID, nil
	}
	var row struct {
		SpaceID     string `gorm:"column:space_id"`
		CharacterID string `gorm:"column:character_id"`
	}
	if err := s.db.Table("conversations").Select("space_id, character_id").Where("id = ? AND deleted_at IS NULL", conversationID).Take(&row).Error; err != nil {
		return "", err
	}
	if !episodicOwnerMatches(row.SpaceID, spaceID) {
		return "", gorm.ErrRecordNotFound
	}
	conversationCharacterID := cleanScope(row.CharacterID)
	if requestedCharacterID != "" && requestedCharacterID != conversationCharacterID {
		return "", gorm.ErrRecordNotFound
	}
	return conversationCharacterID, nil
}

func (s *service) conversationOwnerForEpisodic(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if s.db == nil || conversationID == "" {
		return requestidentity.CanonicalSpaceID()
	}
	var owner string
	if err := s.db.Table("conversations").Select("space_id").Where("id = ? AND deleted_at IS NULL", conversationID).Row().Scan(&owner); err != nil {
		return requestidentity.CanonicalSpaceID()
	}
	return requestidentity.NormalizeSpaceID(owner)
}

func (s *service) owned(id, spaceID string) (*EpisodicMemory, error) {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if m == nil || !episodicOwnerMatches(m.SpaceID, spaceID) {
		return nil, gorm.ErrRecordNotFound
	}
	return m, nil
}
func (s *service) ListForSpace(q EpisodicListQuery, spaceID string) (*EpisodicListResponse, error) {
	q.SpaceID = spaceID
	if episodicLocalMode() {
		// Local legacy rows are owned by the single local user even when the old schema stored default.
		base := s.db.Model(&EpisodicMemory{}).Where("space_id = ? OR space_id = '' OR space_id IS NULL OR space_id = 'default'", spaceID)
		if q.CharacterID != "" {
			base = base.Where("character_id = ?", q.CharacterID)
		}
		if q.SceneType != "" {
			base = base.Where("scene_type = ?", q.SceneType)
		}
		if q.RetentionLevel >= 1 && q.RetentionLevel <= 5 {
			base = base.Where("retention_level = ?", q.RetentionLevel)
		}
		if q.DecayState != "" {
			base = base.Where("decay_state = ?", q.DecayState)
		}
		if kw := strings.TrimSpace(q.Keyword); kw != "" {
			like := "%" + strings.ToLower(kw) + "%"
			base = base.Where("LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(trigger_keywords) LIKE ?", like, like, like)
		}
		var total int64
		if err := base.Count(&total).Error; err != nil {
			return nil, err
		}
		if q.Page <= 0 {
			q.Page = 1
		}
		if q.PageSize <= 0 {
			q.PageSize = 20
		}
		var items []EpisodicMemory
		if err := base.Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error; err != nil {
			return nil, err
		}
		pages := int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
		if pages < 1 {
			pages = 1
		}
		return &EpisodicListResponse{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: pages}, nil
	}
	return s.List(q)
}
func (s *service) CreateForSpace(req *CreateEpisodicRequest, spaceID string) (*EpisodicMemory, error) {
	if req == nil {
		return nil, gorm.ErrInvalidData
	}
	spaceID = cleanOwner(spaceID)
	characterID, err := s.requireEpisodicConversationOwner(req.SourceConvID, spaceID, req.CharacterID)
	if err != nil {
		return nil, err
	}
	cp := *req
	cp.SpaceID = spaceID
	cp.CharacterID = characterID
	return s.Create(&cp)
}
func (s *service) DeleteForSpace(id, spaceID string) error {
	if _, err := s.owned(id, spaceID); err != nil {
		return err
	}
	return s.Delete(id)
}
func (s *service) UpdateRetentionForSpace(id, spaceID string, level int) (*EpisodicMemory, error) {
	if _, err := s.owned(id, spaceID); err != nil {
		return nil, err
	}
	return s.UpdateRetention(id, level)
}
func (s *service) RestoreForSpace(id, spaceID string) (*EpisodicMemory, error) {
	if _, err := s.owned(id, spaceID); err != nil {
		return nil, err
	}
	return s.Restore(id)
}
func (s *service) GetDetailForSpace(id, spaceID string) (*EpisodicMemory, []map[string]interface{}, error) {
	if _, err := s.owned(id, spaceID); err != nil {
		return nil, nil, err
	}
	return s.GetDetail(id)
}
func (s *service) GetForSpace(spaceID, characterID string) ([]EpisodicMemory, error) {
	r, err := s.ListForSpace(EpisodicListQuery{CharacterID: characterID, Page: 1, PageSize: 100}, spaceID)
	if err != nil {
		return nil, err
	}
	return r.Items, nil
}
func (s *service) ExtractForSpace(spaceID, convID string, messages []map[string]string, characterID string) error {
	spaceID = cleanOwner(spaceID)
	if _, err := s.requireEpisodicConversationOwner(convID, spaceID, characterID); err != nil {
		return err
	}
	return s.ExtractFromConversation(spaceID, convID, messages, characterID)
}
func (s *service) SystemPromptForSpace(spaceID, characterID string) string {
	return s.ToSystemPrompt(spaceID, characterID)
}
