// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package kernel

import (
	"database/sql"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
)

func resolveSnapshotOwner(db *sql.DB, conversationID, characterID string) string {
	if db != nil {
		if conversationID = strings.TrimSpace(conversationID); conversationID != "" {
			var owner string
			if err := db.QueryRow("SELECT space_id FROM conversations WHERE id = ? AND deleted_at IS NULL", conversationID).Scan(&owner); err == nil {
				if owner = strings.TrimSpace(owner); owner != "" {
					return owner
				}
			}
		}
		if characterID = strings.TrimSpace(characterID); characterID != "" {
			var owner string
			if err := db.QueryRow("SELECT space_id FROM characters WHERE id = ?", characterID).Scan(&owner); err == nil {
				if owner = strings.TrimSpace(owner); owner != "" {
					return owner
				}
			}
		}
	}
	if config.AppCfg == nil || strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user") {
		return requestidentity.CanonicalSpaceID()
	}
	return ""
}
