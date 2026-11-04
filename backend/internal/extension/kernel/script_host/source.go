// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

type Source string

const (
	SourceExplicit       Source = "explicit"
	SourceRuntimePackage Source = "runtime-package"
)

func knownSource(s Source) bool {
	switch s {
	case SourceExplicit, SourceRuntimePackage:
		return true
	}
	return false
}
