// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import "fmt"

type ID string

const (
	ProfileAuto            ID = "auto"
	ProfileDesktopDefault  ID = "desktop-default"
	ProfileMobileCompact   ID = "mobile-compact"
	ProfileMobileBalanced  ID = "mobile-balanced"
	ProfileMobilePerformance ID = "mobile-performance"
)

var validProfiles = map[ID]bool{
	ProfileAuto:              true,
	ProfileDesktopDefault:    true,
	ProfileMobileCompact:     true,
	ProfileMobileBalanced:    true,
	ProfileMobilePerformance: true,
}

func ParseProfile(s string) (ID, error) {
	id := ID(s)
	if !validProfiles[id] {
		return "", fmt.Errorf("%w: %q", ErrUnknownProfile, s)
	}
	return id, nil
}

func IsValidProfile(s string) bool {
	return validProfiles[ID(s)]
}
