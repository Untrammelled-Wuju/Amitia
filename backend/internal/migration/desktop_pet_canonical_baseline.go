// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const desktopPetCanonicalBaselineOperationID = "baseline-desktop-pet-v2"

// MarkDesktopPetCanonicalBaselineCutover records that a brand-new database was
// created directly on the canonical Desktop Pet V2 schema. It must only be
// called from the new-database startup path: upgraded databases still require
// the explicit domain cutover workflow and must never be implicitly marked.
func MarkDesktopPetCanonicalBaselineCutover(db *gorm.DB) error {
	if db == nil {
		return errors.New("desktop pet canonical baseline cutover: database is nil")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	metadata := `{"planId":"desktop-pet-v2-cutover","sourceVersion":"canonical-baseline","targetVersion":"v2","checkpoint":"baseline","processedCount":0,"conflictCount":0,"backupId":"","lease":""}`

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT OR REPLACE INTO desktop_pet_migration_operations
(id, kind, status, started_at, updated_at, completed_at, error, metadata)
VALUES (?, ?, ?, ?, ?, ?, '', ?)`,
			desktopPetCanonicalBaselineOperationID,
			"desktop-pet-v2-cutover",
			"completed",
			now,
			now,
			now,
			metadata,
		).Error; err != nil {
			return err
		}

		if err := tx.Exec(`INSERT OR REPLACE INTO desktop_pet_read_cutovers
(id, operation_id, step_name, cutover_at, verified)
VALUES (?, ?, ?, ?, 1)`,
			desktopPetCanonicalBaselineOperationID+"-read",
			desktopPetCanonicalBaselineOperationID,
			"v2_read_path",
			now,
		).Error; err != nil {
			return err
		}

		for _, stepName := range []string{"installation", "editing"} {
			if err := tx.Exec(`INSERT OR REPLACE INTO desktop_pet_write_cutovers
(id, operation_id, step_name, cutover_at, verified)
VALUES (?, ?, ?, ?, 1)`,
				desktopPetCanonicalBaselineOperationID+"-write-"+stepName,
				desktopPetCanonicalBaselineOperationID,
				stepName,
				now,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
