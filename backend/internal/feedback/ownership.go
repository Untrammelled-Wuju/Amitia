// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

import (
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

func feedbackLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func feedbackOwnerScope(query *gorm.DB, spaceID string) *gorm.DB {
	owner := requestidentity.NormalizeSpaceID(spaceID)
	if feedbackLocalSingleUserMode() {
		return query.Where("c.space_id = ? OR c.space_id = '' OR c.space_id IS NULL OR c.space_id = ?", owner, requestidentity.LegacySpaceID)
	}
	return query.Where("c.space_id = ?", owner)
}
