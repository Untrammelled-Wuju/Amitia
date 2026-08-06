// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

type Source string

const (
	SourceExplicit      Source = "explicit"
	SourceRuntimePackage Source = "runtime-package"
	SourceWorkspaceBundle Source = "workspace-bundle"
	SourceWorkspaceSource Source = "workspace-source"
)

func knownSource(s Source) bool {
	switch s {
	case SourceExplicit, SourceRuntimePackage, SourceWorkspaceBundle, SourceWorkspaceSource:
		return true
	}
	return false
}
