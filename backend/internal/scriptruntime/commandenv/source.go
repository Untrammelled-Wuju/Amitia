// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

type Source string

const (
	SourceManagedNode  Source = "managed-node"
	SourceNativePath   Source = "native-path"
	SourceNativeLookUp Source = "native-look-path"
)

func knownSource(s Source) bool {
	switch s {
	case SourceManagedNode, SourceNativePath, SourceNativeLookUp:
		return true
	}
	return false
}
