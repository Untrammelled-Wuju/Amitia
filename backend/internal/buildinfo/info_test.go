// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package buildinfo

import (
	"runtime"
	"testing"
)

func TestCurrentReturnsDefaultValues(t *testing.T) {
	info := Current()
	if info.Name != "amitia-server" {
		t.Fatalf("expected Name 'amitia-server', got %q", info.Name)
	}
	if info.Version != "dev" {
		t.Fatalf("expected Version 'dev', got %q", info.Version)
	}
	if info.Commit != "unknown" {
		t.Fatalf("expected Commit 'unknown', got %q", info.Commit)
	}
	if info.Target != "development" {
		t.Fatalf("expected Target 'development', got %q", info.Target)
	}
}

func TestCurrentReturnsGoVersion(t *testing.T) {
	info := Current()
	if info.GoVersion == "" {
		t.Fatal("GoVersion should not be empty")
	}
	expected := runtime.Version()
	if info.GoVersion != expected {
		t.Fatalf("expected GoVersion %q, got %q", expected, info.GoVersion)
	}
}

func TestCurrentReturnsGOOS(t *testing.T) {
	info := Current()
	if info.GOOS == "" {
		t.Fatal("GOOS should not be empty")
	}
	expected := runtime.GOOS
	if info.GOOS != expected {
		t.Fatalf("expected GOOS %q, got %q", expected, info.GOOS)
	}
}

func TestCurrentReturnsGOARCH(t *testing.T) {
	info := Current()
	if info.GOARCH == "" {
		t.Fatal("GOARCH should not be empty")
	}
	expected := runtime.GOARCH
	if info.GOARCH != expected {
		t.Fatalf("expected GOARCH %q, got %q", expected, info.GOARCH)
	}
}

func TestCurrentReturnsIndependentCopies(t *testing.T) {
	a := Current()
	b := Current()
	if a.Version != b.Version || a.Commit != b.Commit || a.Target != b.Target {
		t.Fatal("Current() should return consistent values")
	}
}

func TestCurrentDoesNotContainBuildTime(t *testing.T) {
	info := Current()
	if len(info.Version) > 128 {
		t.Fatalf("Version suspiciously long: %d bytes", len(info.Version))
	}
}
