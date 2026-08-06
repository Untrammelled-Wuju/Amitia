// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

type Kind string

const (
	KindNode   Kind = "node"
	KindNPM    Kind = "npm"
	KindNPX    Kind = "npx"
	KindNative Kind = "native"
)

func knownKind(k Kind) bool {
	switch k {
	case KindNode, KindNPM, KindNPX, KindNative:
		return true
	}
	return false
}
