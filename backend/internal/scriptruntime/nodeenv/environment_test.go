// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/platform"
)

func TestEnvironmentCloneReturnsCopy(t *testing.T) {
	root := t.TempDir()
	original := Environment{
		NodeBinary:                 filepath.Join(root, "bin", "node"),
		NPMCLI:                     filepath.Join(root, "bin", "npm"),
		NPXCLI:                     filepath.Join(root, "bin", "npx"),
		WorkDir:                    filepath.Join(root, "workspace"),
		DistributionRoot:           root,
		Source:                     SourceRuntimePackage,
		Guest:                      platform.GuestPlatformLinux,
		Architecture:               "amd64",
		PackageManagementAvailable: true,
	}

	cloned := original.Clone()

	if cloned.NodeBinary != original.NodeBinary {
		t.Fatal("NodeBinary mismatch")
	}
	if cloned.NPMCLI != original.NPMCLI {
		t.Fatal("NPMCLI mismatch")
	}
	if cloned.NPXCLI != original.NPXCLI {
		t.Fatal("NPXCLI mismatch")
	}
	if cloned.WorkDir != original.WorkDir {
		t.Fatal("WorkDir mismatch")
	}
	if cloned.DistributionRoot != original.DistributionRoot {
		t.Fatal("DistributionRoot mismatch")
	}
	if cloned.Source != original.Source {
		t.Fatal("Source mismatch")
	}
	if cloned.Guest != original.Guest {
		t.Fatal("Guest mismatch")
	}
	if cloned.Architecture != original.Architecture {
		t.Fatal("Architecture mismatch")
	}
	if cloned.PackageManagementAvailable != original.PackageManagementAvailable {
		t.Fatal("PackageManagementAvailable mismatch")
	}

	cloned.NodeBinary = filepath.Join(root, "changed")
	if original.NodeBinary == cloned.NodeBinary {
		t.Fatal("Clone should return value copy")
	}
}

func TestEnvironmentValidateSuccess(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:                 filepath.Join(root, "bin", "node"),
		NPMCLI:                     filepath.Join(root, "bin", "npm"),
		NPXCLI:                     filepath.Join(root, "bin", "npx"),
		WorkDir:                    filepath.Join(root, "workspace"),
		DistributionRoot:           root,
		Source:                     SourceRuntimePackage,
		Guest:                      platform.GuestPlatformLinux,
		Architecture:               "amd64",
		PackageManagementAvailable: true,
	}

	if err := env.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestEnvironmentValidateWithoutPackageManagers(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:                 filepath.Join(root, "bin", "node"),
		WorkDir:                    filepath.Join(root, "workspace"),
		DistributionRoot:           root,
		Source:                     SourceLegacyBundled,
		Guest:                      platform.GuestPlatformWindows,
		Architecture:               "amd64",
		PackageManagementAvailable: false,
	}

	if err := env.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestEnvironmentValidateRejectsEmptyNodeBinary(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       "",
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: root,
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidNodeBinary) {
		t.Fatalf("expected ErrInvalidNodeBinary, got %v", err)
	}
}

func TestEnvironmentValidateRejectsRelativeNodeBinary(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       "usr/bin/node",
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: root,
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidNodeBinary) {
		t.Fatalf("expected ErrInvalidNodeBinary, got %v", err)
	}
}

func TestEnvironmentValidateRejectsEmptyWorkDir(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		WorkDir:          "",
		DistributionRoot: root,
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidWorkDir) {
		t.Fatalf("expected ErrInvalidWorkDir, got %v", err)
	}
}

func TestEnvironmentValidateRejectsRelativeWorkDir(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		WorkDir:          "workspace",
		DistributionRoot: root,
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidWorkDir) {
		t.Fatalf("expected ErrInvalidWorkDir, got %v", err)
	}
}

func TestEnvironmentValidateRejectsEmptyDistributionRoot(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: "",
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidNodeBinary) {
		t.Fatalf("expected ErrInvalidNodeBinary for empty root, got %v", err)
	}
}

func TestEnvironmentValidateRejectsRelativeDistributionRoot(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: "usr",
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidNodeBinary) {
		t.Fatalf("expected ErrInvalidNodeBinary for relative root, got %v", err)
	}
}

func TestEnvironmentValidateRejectsRelativeNPMPath(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		NPMCLI:           "usr/bin/npm",
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: root,
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidPackageManagerCLI) {
		t.Fatalf("expected ErrInvalidPackageManagerCLI, got %v", err)
	}
}

func TestEnvironmentValidateRejectsRelativeNPXPath(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		NPXCLI:           "usr/bin/npx",
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: root,
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidPackageManagerCLI) {
		t.Fatalf("expected ErrInvalidPackageManagerCLI, got %v", err)
	}
}

func TestEnvironmentValidateRejectsIncompletePackageManagers(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:                 filepath.Join(root, "bin", "node"),
		NPMCLI:                     "",
		NPXCLI:                     "",
		WorkDir:                    filepath.Join(root, "workspace"),
		DistributionRoot:           root,
		Guest:                      platform.GuestPlatformLinux,
		Architecture:               "amd64",
		PackageManagementAvailable: true,
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidPackageManagerCLI) {
		t.Fatalf("expected ErrInvalidPackageManagerCLI, got %v", err)
	}
}

func TestEnvironmentValidateRejectsPartialPackageManagers(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:                 filepath.Join(root, "bin", "node"),
		NPMCLI:                     filepath.Join(root, "bin", "npm"),
		NPXCLI:                     "",
		WorkDir:                    filepath.Join(root, "workspace"),
		DistributionRoot:           root,
		Guest:                      platform.GuestPlatformLinux,
		Architecture:               "amd64",
		PackageManagementAvailable: true,
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidPackageManagerCLI) {
		t.Fatalf("expected ErrInvalidPackageManagerCLI for partial, got %v", err)
	}
}

func TestEnvironmentValidateRejectsUnknownGuest(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: root,
		Guest:            platform.GuestPlatformUnknown,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if !errors.Is(err, ErrUnsupportedGuestPlatform) {
		t.Fatalf("expected ErrUnsupportedGuestPlatform, got %v", err)
	}
}

func TestEnvironmentValidateRejectsEmptyArchitecture(t *testing.T) {
	root := t.TempDir()
	env := Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: root,
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "",
	}

	err := env.Validate()
	if !errors.Is(err, ErrInvalidNodeBinary) {
		t.Fatalf("expected ErrInvalidNodeBinary for empty arch, got %v", err)
	}
}

func TestErrorTypesIsMethod(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		err    error
		target error
	}{
		{newUnsupportedGuest(platform.GuestPlatformAndroid), ErrUnsupportedGuestPlatform},
		{newHostCapabilityError("process.spawn"), ErrHostCapabilityUnsupported},
		{newNodeNotFound(SourceExplicit), ErrNodeNotFound},
		{newInvalidNodeBinary(filepath.Join(root, "bad"), "test"), ErrInvalidNodeBinary},
		{newNodeNotExecutable(filepath.Join(root, "bin", "node")), ErrNodeNotExecutable},
		{newShellWrapper(filepath.Join(root, "bin", "npm.sh")), ErrShellWrapperUnsupported},
		{newNativeResource(filepath.Join(root, "native", "node")), ErrNativeResourceNotAllowed},
		{newRuntimePathError("no runtime root"), ErrRuntimeRootUnavailable},
	}

	for _, tt := range tests {
		if !errors.Is(tt.err, tt.target) {
			t.Fatalf("expected %v to match %v", tt.err, tt.target)
		}
	}
}

func TestErrorTypesIsMethodNegative(t *testing.T) {
	if errors.Is(ErrScriptRuntimeDisabled, ErrNodeNotFound) {
		t.Fatal("ErrScriptRuntimeDisabled should not match ErrNodeNotFound")
	}
	if errors.Is(ErrInvalidNodeBinary, ErrInvalidWorkDir) {
		t.Fatal("ErrInvalidNodeBinary should not match ErrInvalidWorkDir")
	}
}

func TestDetectionSnapshotClone(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	original := DetectionSnapshot{
		State: DetectionStateReady,
		Environment: Environment{
			NodeBinary:       filepath.Join(root, "bin", "node"),
			WorkDir:          filepath.Join(root, "workspace"),
			DistributionRoot: root,
			Guest:            platform.GuestPlatformLinux,
			Architecture:     "amd64",
		},
		Diagnostics: []CandidateDiagnostic{
			{Kind: CandidateKindNode, Source: SourceRuntimePackage, Path: filepath.Join(root, "bin", "node"), Result: CandidateResultSelected},
		},
		LastError:  "",
		DetectedAt: now,
	}

	cloned := original.clone()

	if cloned.State != original.State {
		t.Fatal("State mismatch")
	}
	if len(cloned.Diagnostics) != 1 {
		t.Fatalf("Diagnostics length mismatch: %d", len(cloned.Diagnostics))
	}
	if cloned.Diagnostics[0].Path != filepath.Join(root, "bin", "node") {
		t.Fatal("Diagnostic mismatch")
	}

	cloned.Diagnostics[0].Path = "/changed"
	if original.Diagnostics[0].Path == "/changed" {
		t.Fatal("Clone should deep copy Diagnostics slice elements")
	}
}

func TestDetectionStateConstants(t *testing.T) {
	states := map[DetectionState]string{
		DetectionStateNotStarted: "not-started",
		DetectionStateReady:      "ready",
		DetectionStatePartial:    "partial",
		DetectionStateFailed:     "failed",
	}

	for state, expected := range states {
		if string(state) != expected {
			t.Fatalf("expected %s, got %s", expected, string(state))
		}
	}
}

func TestCandidateKindConstants(t *testing.T) {
	kinds := map[CandidateKind]string{
		CandidateKindNode:    "node",
		CandidateKindNPMCLI:  "npm-cli",
		CandidateKindNPXCLI:  "npx-cli",
		CandidateKindWorkDir: "work-dir",
	}

	for kind, expected := range kinds {
		if string(kind) != expected {
			t.Fatalf("expected %s, got %s", expected, string(kind))
		}
	}
}

func TestCandidateResultConstants(t *testing.T) {
	results := map[CandidateResult]string{
		CandidateResultSelected:           "selected",
		CandidateResultNotFound:           "not-found",
		CandidateResultInvalidFile:        "invalid-file",
		CandidateResultNotExecutable:      "not-executable",
		CandidateResultUnsupportedWrapper: "unsupported-wrapper",
		CandidateResultRootUnavailable:    "root-unavailable",
		CandidateResultSkipped:            "skipped",
	}

	for result, expected := range results {
		if string(result) != expected {
			t.Fatalf("expected %s, got %s", expected, string(result))
		}
	}
}

func TestSourceConstants(t *testing.T) {
	if string(SourceExplicit) != "explicit" {
		t.Fatalf("SourceExplicit mismatch: %s", SourceExplicit)
	}
	if string(SourceRuntimePackage) != "runtime-package" {
		t.Fatalf("SourceRuntimePackage mismatch: %s", SourceRuntimePackage)
	}
	if string(SourceLegacyBundled) != "legacy-bundled" {
		t.Fatalf("SourceLegacyBundled mismatch: %s", SourceLegacyBundled)
	}
}

func TestEnvironmentAbsPathIsCrossPlatform(t *testing.T) {
	var separator string
	if filepath.Separator == '\\' {
		separator = "\\"
	} else {
		separator = "/"
	}

	nodePath := "usr" + separator + "bin" + separator + "node"
	root := t.TempDir()
	env := Environment{
		NodeBinary:       nodePath,
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: root,
		Guest:            platform.GuestPlatformLinux,
		Architecture:     "amd64",
	}

	err := env.Validate()
	if err == nil || !errors.Is(err, ErrInvalidNodeBinary) {
		t.Fatalf("relative path should fail validation: %v", err)
	}
}

func TestInvalidWorkDirErrorIs(t *testing.T) {
	err := &invalidWorkDirError{reason: "test"}
	if !errors.Is(err, ErrInvalidWorkDir) {
		t.Fatal("invalidWorkDirError should match ErrInvalidWorkDir")
	}
	if errors.Is(err, ErrInvalidNodeBinary) {
		t.Fatal("invalidWorkDirError should not match ErrInvalidNodeBinary")
	}
}

func TestInvalidPackageManagerCLIErrorIs(t *testing.T) {
	err := &invalidPackageManagerCLIError{path: "/bad/npm", reason: "test"}
	if !errors.Is(err, ErrInvalidPackageManagerCLI) {
		t.Fatal("invalidPackageManagerCLIError should match ErrInvalidPackageManagerCLI")
	}
}

func TestShellWrapperErrorFormatting(t *testing.T) {
	err := &shellWrapperError{path: "/bin/npm.sh"}
	expected := "nodeenv: shell wrapper not supported as package manager entry: path=/bin/npm.sh"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestUnsupportedGuestErrorFormatting(t *testing.T) {
	err := &unsupportedGuestError{guest: platform.GuestPlatformAndroid}
	expected := "nodeenv: unsupported guest platform for node: guest=android"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestHostCapabilityErrorFormatting(t *testing.T) {
	err := &hostCapabilityError{capability: "process.spawn"}
	expected := "nodeenv: host capability unsupported: capability=process.spawn"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestNodeNotFoundErrorFormatting(t *testing.T) {
	err := &nodeNotFoundError{source: SourceExplicit}
	expected := "nodeenv: node binary not found: source=explicit"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestRuntimePathErrorFormatting(t *testing.T) {
	err := &runtimePathError{reason: "no runtime root"}
	expected := "nodeenv: runtime root unavailable: no runtime root"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}
