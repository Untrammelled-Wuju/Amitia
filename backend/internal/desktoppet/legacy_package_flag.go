// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

const LegacyPackageWritesDisabled = true

const ErrCodeLegacyPackageWriteDisabled = "LEGACY_PACKAGE_WRITE_DISABLED"

var (
	legacyInstallationWriteDisabled bool
	legacyEditingWriteDisabled      bool
)

func SetLegacyInstallationWriteDisabled(disabled bool) {
	legacyInstallationWriteDisabled = disabled
}

func IsLegacyInstallationWriteDisabled() bool {
	return legacyInstallationWriteDisabled
}

func SetLegacyEditingWriteDisabled(disabled bool) {
	legacyEditingWriteDisabled = disabled
}

func IsLegacyEditingWriteDisabled() bool {
	return legacyEditingWriteDisabled
}
