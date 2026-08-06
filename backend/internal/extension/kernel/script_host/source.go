// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

type Source string

const (
	SourceExplicit       Source = "explicit"
	SourceRuntimePackage Source = "runtime-package"
	SourceLegacyWorkspace Source = "legacy-workspace"
)

func knownSource(s Source) bool {
	switch s {
	case SourceExplicit, SourceRuntimePackage, SourceLegacyWorkspace:
		return true
	}
	return false
}
