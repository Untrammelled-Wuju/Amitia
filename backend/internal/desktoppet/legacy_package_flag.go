// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/u-ai/backend/internal/desktoppet/migration"
	"gorm.io/gorm"
)

const LegacyPackageWritesDisabled = true

const ErrCodeLegacyPackageWriteDisabled = "LEGACY_PACKAGE_WRITE_DISABLED"

var (
	legacyInstallationWriteDisabled atomic.Bool
	legacyEditingWriteDisabled      atomic.Bool
)

func SetLegacyInstallationWriteDisabled(disabled bool) {
	legacyInstallationWriteDisabled.Store(disabled)
}

func IsLegacyInstallationWriteDisabled() bool {
	return legacyInstallationWriteDisabled.Load()
}

func SetLegacyEditingWriteDisabled(disabled bool) {
	legacyEditingWriteDisabled.Store(disabled)
}

func IsLegacyEditingWriteDisabled() bool {
	return legacyEditingWriteDisabled.Load()
}

// RefreshLegacyWriteFlagsFromDB refreshes the in-process legacy writer guards from
// the durable migration cutover records. Database read failures fail closed: both
// legacy writers are blocked and the caller receives the error so readiness or the
// migration runner cannot report a successful cutover from stale cached state.
func RefreshLegacyWriteFlagsFromDB(db *gorm.DB) error {
	if db == nil {
		legacyInstallationWriteDisabled.Store(true)
		legacyEditingWriteDisabled.Store(true)
		return errors.New("desktop pet legacy write flag refresh: database is nil")
	}

	repo := migration.NewDBRepository(db)
	installationDisabled, installationErr := repo.LegacyInstallationWriteDisabled()
	editingDisabled, editingErr := repo.LegacyEditingWriteDisabled()
	if installationErr != nil || editingErr != nil {
		legacyInstallationWriteDisabled.Store(true)
		legacyEditingWriteDisabled.Store(true)
		return errors.Join(
			wrapLegacyCutoverQueryError("installation", installationErr),
			wrapLegacyCutoverQueryError("editing", editingErr),
		)
	}

	legacyInstallationWriteDisabled.Store(installationDisabled)
	legacyEditingWriteDisabled.Store(editingDisabled)
	return nil
}

// LegacyWriteCutoverReady verifies the durable cutover records directly instead
// of trusting the cached writer guards. This prevents canonical_cutover readiness
// from becoming green because of stale or partially refreshed process state.
func LegacyWriteCutoverReady(db *gorm.DB) error {
	if db == nil {
		return errors.New("desktop pet legacy write cutover: database is nil")
	}

	repo := migration.NewDBRepository(db)
	installationDisabled, err := repo.LegacyInstallationWriteDisabled()
	if err != nil {
		return fmt.Errorf("desktop pet installation legacy write cutover check: %w", err)
	}
	editingDisabled, err := repo.LegacyEditingWriteDisabled()
	if err != nil {
		return fmt.Errorf("desktop pet editing legacy write cutover check: %w", err)
	}
	if !installationDisabled {
		return errors.New("legacy desktop pet installation writes are still enabled")
	}
	if !editingDisabled {
		return errors.New("legacy desktop pet editing writes are still enabled")
	}
	if !IsLegacyInstallationWriteDisabled() {
		return errors.New("legacy desktop pet installation write guard is not active")
	}
	if !IsLegacyEditingWriteDisabled() {
		return errors.New("legacy desktop pet editing write guard is not active")
	}
	return nil
}

func wrapLegacyCutoverQueryError(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("desktop pet %s legacy write cutover query: %w", name, err)
}
