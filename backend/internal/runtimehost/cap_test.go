// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

func NewTestCapabilities(ids ...HostCapabilityID) *HostCapabilities {
	caps := newHostCapabilities()
	for _, id := range ids {
		caps.set(id, SupportSupported)
	}
	return caps
}
