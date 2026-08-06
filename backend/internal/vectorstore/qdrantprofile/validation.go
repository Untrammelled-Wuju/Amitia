// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

func ValidateProfileID(s string) error {
	id := ID(s)
	switch id {
	case ProfileAuto, ProfileDesktopDefault, ProfileMobileCompact, ProfileMobileBalanced, ProfileMobilePerformance:
		return nil
	default:
		return ErrUnknownProfile
	}
}

func IsMobileProfile(id ID) bool {
	return id.isMobileProfile()
}

func IsDesktopProfile(id ID) bool {
	return id == ProfileDesktopDefault
}

func IsAutoProfile(id ID) bool {
	return id == ProfileAuto
}
