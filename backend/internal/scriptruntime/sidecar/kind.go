// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

type Kind string

const (
	KindWeChat Kind = "wechat"
	KindQQ     Kind = "qq"
)

func knownKind(k Kind) bool {
	switch k {
	case KindWeChat, KindQQ:
		return true
	}
	return false
}
