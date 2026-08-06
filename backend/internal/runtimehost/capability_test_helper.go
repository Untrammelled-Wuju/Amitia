// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

func NewTestCapabilitiesForTest(support map[HostCapabilityID]CapabilitySupport) *HostCapabilities {
	caps := newHostCapabilities()
	for id, level := range support {
		caps.set(id, level)
	}
	return caps
}
