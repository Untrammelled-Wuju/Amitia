// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/u-ai/backend/pkg/platform"
)

func absExe() string {
	if runtime.GOOS == "windows" {
		return `C:\abs\qdrant.exe`
	}
	return "/abs/qdrant"
}

func absRoot() string {
	if runtime.GOOS == "windows" {
		return `C:\root`
	}
	return "/root"
}

func TestEnvironment_Validate_EmptyBinaryPath(t *testing.T) {
	env := Environment{
		BinaryPath:       "",
		DistributionRoot: absRoot(),
		Source:           SourceExplicit,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "amd64",
		Installed:        true,
		Explicit:         true,
	}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for empty BinaryPath")
	}
}

func TestEnvironment_Validate_RelativeBinaryPath(t *testing.T) {
	env := Environment{
		BinaryPath:       filepath.Join("relative", "qdrant.exe"),
		DistributionRoot: absRoot(),
		Source:           SourceExplicit,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "amd64",
		Installed:        true,
		Explicit:         true,
	}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for relative BinaryPath")
	}
}

func TestEnvironment_Validate_EmptyDistributionRoot(t *testing.T) {
	env := Environment{
		BinaryPath:       absExe(),
		DistributionRoot: "",
		Source:           SourceExplicit,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "amd64",
		Installed:        true,
		Explicit:         true,
	}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for empty DistributionRoot")
	}
}

func TestEnvironment_Validate_EmptyArchitecture(t *testing.T) {
	env := Environment{
		BinaryPath:       absExe(),
		DistributionRoot: absRoot(),
		Source:           SourceExplicit,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "",
		Installed:        true,
		Explicit:         true,
	}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for empty Architecture")
	}
}

func TestEnvironment_Validate_UnknownGuest(t *testing.T) {
	env := Environment{
		BinaryPath:       absExe(),
		DistributionRoot: absRoot(),
		Source:           SourceExplicit,
		Guest:            platform.GuestPlatformUnknown,
		Architecture:     "amd64",
		Installed:        true,
		Explicit:         true,
	}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for unknown guest")
	}
}

func TestEnvironment_Validate_ExplicitRequiresSourceExplicit(t *testing.T) {
	env := Environment{
		BinaryPath:       absExe(),
		DistributionRoot: absRoot(),
		Source:           SourceRuntimePackage,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "amd64",
		Installed:        true,
		Explicit:         true,
	}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error when Explicit=true but Source!=SourceExplicit")
	}
}

func TestEnvironment_Validate_ExplicitCantBeUninstalled(t *testing.T) {
	env := Environment{
		BinaryPath:       absExe(),
		DistributionRoot: absRoot(),
		Source:           SourceExplicit,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "amd64",
		Installed:        false,
		Explicit:         true,
	}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error when Explicit=true but Installed=false")
	}
}

func TestEnvironment_Validate_ValidExplicit(t *testing.T) {
	env := Environment{
		BinaryPath:       absExe(),
		DistributionRoot: absRoot(),
		Source:           SourceExplicit,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "amd64",
		Installed:        true,
		Explicit:         true,
	}
	if err := env.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnvironment_Validate_ValidUninstalled(t *testing.T) {
	env := Environment{
		BinaryPath:       filepath.Join(absRoot(), "bin", "qdrant.exe"),
		DistributionRoot: absRoot(),
		Source:           SourceRuntimePackage,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "amd64",
		Installed:        false,
		Explicit:         false,
	}
	if err := env.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnvironment_Clone_ReturnsCopy(t *testing.T) {
	env := Environment{
		BinaryPath:       absExe(),
		DistributionRoot: absRoot(),
		Source:           SourceExplicit,
		Guest:            platform.GuestPlatformWindows,
		Architecture:     "amd64",
		Installed:        true,
		Explicit:         true,
	}
	clone := env.Clone()
	if clone.BinaryPath != env.BinaryPath {
		t.Fatal("clone should copy BinaryPath")
	}
	if clone.DistributionRoot != env.DistributionRoot {
		t.Fatal("clone should copy DistributionRoot")
	}
}
