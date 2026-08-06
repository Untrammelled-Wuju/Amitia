// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFileForArtifact(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestArtifact_Validate_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "plugin-host")
	entryFile := filepath.Join(pluginDir, "dist", "index.js")
	writeFileForArtifact(t, entryFile)

	a := Artifact{
		Kind:             KindPluginHost,
		EntryPath:        entryFile,
		DistributionRoot: pluginDir,
		Source:           SourceExplicit,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
}

func TestArtifact_Validate_UnknownKind(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "test.js")
	writeFileForArtifact(t, entryFile)

	a := Artifact{
		Kind:             Kind("unknown"),
		EntryPath:        entryFile,
		DistributionRoot: tmpDir,
		Source:           SourceExplicit,
	}
	err := a.Validate()
	if !errors.Is(err, ErrInvalidHostArtifact) {
		t.Fatalf("expected ErrInvalidHostArtifact, got: %v", err)
	}
}

func TestArtifact_Validate_EmptyEntryPath(t *testing.T) {
	a := Artifact{
		Kind:             KindPluginHost,
		EntryPath:        "",
		DistributionRoot: "ignored",
		Source:           SourceExplicit,
	}
	err := a.Validate()
	if !errors.Is(err, ErrInvalidHostArtifact) {
		t.Fatalf("expected ErrInvalidHostArtifact, got: %v", err)
	}
}

func TestArtifact_Validate_RelativeEntryPath(t *testing.T) {
	a := Artifact{
		Kind:             KindPluginHost,
		EntryPath:        "runtime/plugin-host/dist/index.js",
		DistributionRoot: "ignored",
		Source:           SourceExplicit,
	}
	err := a.Validate()
	if !errors.Is(err, ErrInvalidHostArtifact) {
		t.Fatalf("expected ErrInvalidHostArtifact, got: %v", err)
	}
}

func TestArtifact_Validate_EmptyDistributionRoot(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "test.js")
	writeFileForArtifact(t, entryFile)

	a := Artifact{
		Kind:             KindPluginHost,
		EntryPath:        entryFile,
		DistributionRoot: "",
		Source:           SourceExplicit,
	}
	err := a.Validate()
	if !errors.Is(err, ErrInvalidHostArtifact) {
		t.Fatalf("expected ErrInvalidHostArtifact, got: %v", err)
	}
}

func TestArtifact_Validate_RelativeDistributionRoot(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "test.js")
	writeFileForArtifact(t, entryFile)

	a := Artifact{
		Kind:             KindPluginHost,
		EntryPath:        entryFile,
		DistributionRoot: "runtime/plugin-host",
		Source:           SourceExplicit,
	}
	err := a.Validate()
	if !errors.Is(err, ErrInvalidHostArtifact) {
		t.Fatalf("expected ErrInvalidHostArtifact, got: %v", err)
	}
}

func TestArtifact_Validate_UnknownSource(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "test.js")
	writeFileForArtifact(t, entryFile)

	a := Artifact{
		Kind:             KindPluginHost,
		EntryPath:        entryFile,
		DistributionRoot: tmpDir,
		Source:           Source("unknown"),
	}
	err := a.Validate()
	if !errors.Is(err, ErrInvalidHostArtifact) {
		t.Fatalf("expected ErrInvalidHostArtifact, got: %v", err)
	}
}

func TestArtifact_Validate_UnsupportedExtension(t *testing.T) {
	tmpDir := t.TempDir()
	entryFile := filepath.Join(tmpDir, "index.sh")
	writeFileForArtifact(t, entryFile)

	a := Artifact{
		Kind:             KindPluginHost,
		EntryPath:        entryFile,
		DistributionRoot: tmpDir,
		Source:           SourceExplicit,
	}
	err := a.Validate()
	if !errors.Is(err, ErrUnsupportedHostEntry) {
		t.Fatalf("expected ErrUnsupportedHostEntry, got: %v", err)
	}
}

func TestIsHostEntryExtension(t *testing.T) {
	cases := map[string]bool{
		"/path/index.js":   true,
		"/path/index.mjs":  true,
		"/path/index.cjs":  true,
		"/path/index.JS":   true,
		"/path/index.ts":   false,
		"/path/index.json": false,
		"/path/index.sh":   false,
		"/path/index":      false,
	}
	for path, want := range cases {
		got := isHostEntryExtension(path)
		if got != want {
			t.Errorf("isHostEntryExtension(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestDeriveDistributionRoot(t *testing.T) {
	tmpDir := t.TempDir()
	cases := map[string]string{
		filepath.Join(tmpDir, "runtime", "plugin-host", "dist", "index.js"):  filepath.Clean(filepath.Join(tmpDir, "runtime", "plugin-host")),
		filepath.Join(tmpDir, "runtime", "plugin-host", "dist", "index.mjs"): filepath.Clean(filepath.Join(tmpDir, "runtime", "plugin-host")),
		filepath.Join(tmpDir, "runtime", "plugin-host", "dist", "index.cjs"): filepath.Clean(filepath.Join(tmpDir, "runtime", "plugin-host")),
		filepath.Join(tmpDir, "runtime", "plugin-host", "src", "main.js"):    filepath.Clean(filepath.Join(tmpDir, "runtime", "plugin-host", "src")),
		filepath.Join(tmpDir, "runtime", "plugin-host", "main.js"):           filepath.Clean(filepath.Join(tmpDir, "runtime", "plugin-host")),
	}
	for entry, want := range cases {
		got := deriveDistributionRoot(entry)
		if got != want {
			t.Errorf("deriveDistributionRoot(%q) = %q, want %q", entry, got, want)
		}
	}
}

func TestKnownKind(t *testing.T) {
	if !knownKind(KindPluginHost) {
		t.Error("expected KindPluginHost to be known")
	}
	if !knownKind(KindTaskHost) {
		t.Error("expected KindTaskHost to be known")
	}
	if knownKind(Kind("unknown")) {
		t.Error("expected unknown Kind to not be known")
	}
	if knownKind(Kind("")) {
		t.Error("expected empty Kind to not be known")
	}
}

func TestKnownSource(t *testing.T) {
	if !knownSource(SourceExplicit) {
		t.Error("expected SourceExplicit to be known")
	}
	if !knownSource(SourceRuntimePackage) {
		t.Error("expected SourceRuntimePackage to be known")
	}
	if !knownSource(SourceLegacyWorkspace) {
		t.Error("expected SourceLegacyWorkspace to be known")
	}
	if knownSource(Source("unknown")) {
		t.Error("expected unknown Source to not be known")
	}
	if knownSource(Source("")) {
		t.Error("expected empty Source to not be known")
	}
}

func TestUnavailableNodeResolver(t *testing.T) {
	r := UnavailableNodeResolver()
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
	_, err := r.Resolve(nil)
	if !errors.Is(err, ErrNodeResolverUnavailable) {
		t.Fatalf("expected ErrNodeResolverUnavailable, got: %v", err)
	}
}

func TestUnavailableArtifactResolver(t *testing.T) {
	r := UnavailableArtifactResolver()
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
	_, err := r.Resolve(nil, KindPluginHost)
	if !errors.Is(err, ErrArtifactResolverUnavailable) {
		t.Fatalf("expected ErrArtifactResolverUnavailable, got: %v", err)
	}
}
