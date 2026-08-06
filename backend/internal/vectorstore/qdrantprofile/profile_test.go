// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import "testing"

func TestParseProfile_ValidProfiles(t *testing.T) {
	valid := []ID{
		ProfileAuto,
		ProfileDesktopDefault,
		ProfileMobileCompact,
		ProfileMobileBalanced,
		ProfileMobilePerformance,
	}
	for _, id := range valid {
		got, err := ParseProfile(string(id))
		if err != nil {
			t.Errorf("ParseProfile(%q) error: %v", id, err)
		}
		if got != id {
			t.Errorf("ParseProfile(%q) = %q", id, got)
		}
	}
}

func TestParseProfile_EmptyStringIsInvalid(t *testing.T) {
	_, err := ParseProfile("")
	if err == nil {
		t.Error("ParseProfile(\"\") should fail")
	}
}

func TestParseProfile_CaseSensitive(t *testing.T) {
	_, err := ParseProfile("MOBILE-BALANCED")
	if err == nil {
		t.Error("ParseProfile should be case sensitive")
	}
}

func TestParseProfile_UnknownRejected(t *testing.T) {
	invalid := []string{"custom", "default", "low", "high", "android", "ios", "unlimited"}
	for _, s := range invalid {
		_, err := ParseProfile(s)
		if err == nil {
			t.Errorf("ParseProfile(%q) should fail", s)
		}
	}
}

func TestProfileStringsStable(t *testing.T) {
	if string(ProfileAuto) != "auto" {
		t.Errorf("ProfileAuto = %q", ProfileAuto)
	}
	if string(ProfileDesktopDefault) != "desktop-default" {
		t.Errorf("ProfileDesktopDefault = %q", ProfileDesktopDefault)
	}
	if string(ProfileMobileCompact) != "mobile-compact" {
		t.Errorf("ProfileMobileCompact = %q", ProfileMobileCompact)
	}
	if string(ProfileMobileBalanced) != "mobile-balanced" {
		t.Errorf("ProfileMobileBalanced = %q", ProfileMobileBalanced)
	}
	if string(ProfileMobilePerformance) != "mobile-performance" {
		t.Errorf("ProfileMobilePerformance = %q", ProfileMobilePerformance)
	}
}

func TestIsValidProfile(t *testing.T) {
	if !IsValidProfile("auto") {
		t.Error("auto should be valid")
	}
	if IsValidProfile("invalid") {
		t.Error("invalid should not be valid")
	}
}
