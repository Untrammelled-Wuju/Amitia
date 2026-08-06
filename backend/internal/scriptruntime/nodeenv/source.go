// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

type Source string

const (
	SourceExplicit       Source = "explicit"
	SourceRuntimePackage Source = "runtime-package"
	SourceLegacyBundled  Source = "legacy-bundled"
)
