// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package character

import (
	"errors"
	"strings"

	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func normalizeCharacterOwner(spaceID string) string {
	return requestidentity.NormalizeSpaceID(spaceID)
}

func characterOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = normalizeCharacterOwner(requested)
	return stored != "" && stored == requested
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
	return db.Where("space_id = ?", owner)
}

func characterNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("角色不存在")
	}
	return err
}
