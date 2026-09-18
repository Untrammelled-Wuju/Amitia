// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"errors"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func characterLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func normalizeCharacterOwner(spaceID string) string {
	return requestidentity.NormalizeSpaceID(spaceID)
}

func characterOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = normalizeCharacterOwner(requested)
	if stored == requested {
		return true
	}
	return characterLocalSingleUserMode() && (stored == "" || stored == requestidentity.LegacySpaceID)
}

func (s *service) requireCharacterOwner(id, spaceID string) (*Character, error) {
	var c Character
	if err := s.db.Where("id = ? AND deleted_at IS NULL", strings.TrimSpace(id)).First(&c).Error; err != nil {
		return nil, err
	}
	if !characterOwnerMatches(c.SpaceID, spaceID) {
		return nil, gorm.ErrRecordNotFound
	}
	return &c, nil
}

func (s *service) characterOwnerQuery(db *gorm.DB, spaceID string) *gorm.DB {
	owner := normalizeCharacterOwner(spaceID)
	if characterLocalSingleUserMode() {
		return db.Where("(space_id = ? OR space_id = '' OR space_id IS NULL OR space_id = ?)", owner, requestidentity.LegacySpaceID)
	}
	return db.Where("space_id = ?", owner)
}

func characterNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("角色不存在")
	}
	return err
}
