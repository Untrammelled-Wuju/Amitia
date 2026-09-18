// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func worldbookLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func normalizeWorldbookOwner(spaceID string) string {
	return requestidentity.NormalizeSpaceID(spaceID)
}

func worldbookOwnerScope(db *gorm.DB, column, spaceID string) *gorm.DB {
	owner := normalizeWorldbookOwner(spaceID)
	if worldbookLocalSingleUserMode() {
		return db.Where("("+column+" = ? OR "+column+" = '' OR "+column+" IS NULL OR "+column+" = ?)", owner, requestidentity.LegacySpaceID)
	}
	return db.Where(column+" = ?", owner)
}

func requireWorldbookCharacterOwner(db *gorm.DB, characterID, spaceID string) error {
	characterID = strings.TrimSpace(characterID)
	if characterID == "" {
		return nil
	}
	var count int64
	q := db.Table("characters").Where("id = ? AND deleted_at IS NULL", characterID)
	q = worldbookOwnerScope(q, "space_id", spaceID)
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
