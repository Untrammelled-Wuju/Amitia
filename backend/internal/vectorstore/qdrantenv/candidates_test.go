// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import (
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/pkg/platform"
)

func TestDeduplicateCandidates_RemovesDuplicatePaths(t *testing.T) {
	input := []string{
		"/a/qdrant.exe",
		"/a/qdrant.exe",
		"/b/qdrant.exe",
	}
	result := deduplicateCandidates(input)
	seen := make(map[string]int)
	for _, r := range result {
		seen[r]++
	}
	for path, count := range seen {
		if count > 1 {
			t.Fatalf("duplicate in result: %s appears %d times", path, count)
		}
	}
}

func TestDeduplicateCandidates_PreservesFirstOccurrenceOrder(t *testing.T) {
	input := []string{"/a/qdrant.exe", "/b/qdrant.exe", "/c/qdrant.exe"}
	result := deduplicateCandidates(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 unique, got %d", len(result))
	}
	for i, expected := range input {
		expectedClean := filepath.Clean(expected)
		if result[i] != expectedClean {
			t.Fatalf("order not preserved: expected %s at index %d, got %s", expectedClean, i, result[i])
		}
	}
}

func TestRuntimePackageCandidates_WindowsEndsWithExe(t *testing.T) {
	candidates := runtimePackageCandidates(platform.GuestPlatformWindows)
	for _, c := range candidates {
		base := filepath.Base(c)
		if base != "qdrant.exe" {
			t.Fatalf("Windows candidate should end with .exe: %s", c)
		}
	}
}

func TestRuntimePackageCandidates_LinuxNoExe(t *testing.T) {
	candidates := runtimePackageCandidates(platform.GuestPlatformLinux)
	for _, c := range candidates {
		base := filepath.Base(c)
		if base != "qdrant" {
			t.Fatalf("Linux candidate should be qdrant: %s", c)
		}
	}
}

func TestStandardInstallTarget(t *testing.T) {
	target := standardInstallTarget(platform.GuestPlatformWindows, "/runtime")
	if filepath.Base(target) != "qdrant.exe" {
		t.Fatalf("expected qdrant.exe, got %s", target)
	}
	target = standardInstallTarget(platform.GuestPlatformLinux, "/runtime")
	if filepath.Base(target) != "qdrant" {
		t.Fatalf("expected qdrant, got %s", target)
	}
}

func TestEmptyRootSkipsLegacyCandidates(t *testing.T) {
	candidates := legacyCandidates(platform.GuestPlatformWindows, "amd64", "", "")
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates with empty roots, got %d", len(candidates))
	}
}
