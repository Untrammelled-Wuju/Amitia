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

func normalizeCharacterOwner(userID string) string {
	return requestidentity.NormalizeUserID(userID)
}

func characterOwnerMatches(stored, requested string) bool {
	stored = strings.TrimSpace(stored)
	requested = normalizeCharacterOwner(requested)
	if stored == requested {
		return true
	}
	return characterLocalSingleUserMode() && (stored == "" || stored == requestidentity.DefaultUserID)
}

func (s *service) requireCharacterOwner(id, userID string) (*Character, error) {
	var c Character
	if err := s.db.Where("id = ? AND deleted_at IS NULL", strings.TrimSpace(id)).First(&c).Error; err != nil {
		return nil, err
	}
	if !characterOwnerMatches(c.UserID, userID) {
		return nil, gorm.ErrRecordNotFound
	}
	return &c, nil
}

func (s *service) characterOwnerQuery(db *gorm.DB, userID string) *gorm.DB {
	owner := normalizeCharacterOwner(userID)
	if characterLocalSingleUserMode() {
		return db.Where("(user_id = ? OR user_id = '' OR user_id IS NULL OR user_id = ?)", owner, requestidentity.DefaultUserID)
	}
	return db.Where("user_id = ?", owner)
}

func characterNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("角色不存在")
	}
	return err
}
