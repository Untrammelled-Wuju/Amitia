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

func feedbackOwnerScope(query *gorm.DB, userID string) *gorm.DB {
	owner := requestidentity.NormalizeUserID(userID)
	if feedbackLocalSingleUserMode() {
		return query.Where("c.user_id = ? OR c.user_id = '' OR c.user_id IS NULL OR c.user_id = ?", owner, requestidentity.DefaultUserID)
	}
	return query.Where("c.user_id = ?", owner)
}
