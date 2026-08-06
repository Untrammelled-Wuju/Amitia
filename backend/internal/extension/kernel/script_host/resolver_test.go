// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/pkg/util"
)

func TestNewArtifactResolver_NilHost(t *testing.T) {
	_, err := NewArtifactResolver(ResolveContext{Host: nil})
	if err == nil {
		t.Fatal("expected error for nil host")
	}
}

func TestNewArtifactResolver_DefaultInspector(t *testing.T) {
	host := newFakeRuntimeHost(util.RuntimePaths{})
	_, err := NewArtifactResolver(ResolveContext{Host: host})
	if err != nil {
		t.Fatalf("expected success with nil inspector: %v", err)
	}
}

func TestResolver_UnknownKind(t *testing.T) {
	host := newFakeRuntimeHost(util.RuntimePaths{})
	r, err := NewArtifactResolver(ResolveContext{Host: host})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve(context.Background(), Kind("unknown"))
	if !errors.Is(err, ErrUnknownHostKind) {
		t.Fatalf("expected ErrUnknownHostKind, got: %v", err)
	}
}

func TestResolver_ExplicitURI_ResolvesOverDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	explicitDir := filepath.Join(tmpDir, "custom-plugin")
	if err := os.MkdirAll(explicitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	explicitFile := filepath.Join(explicitDir, "host.js")
	if err := os.WriteFile(explicitFile, []byte("// explicit"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := util.RuntimePaths{WorkspaceDir: tmpDir}
	host := newFakeRuntimeHost(paths)
	inspector := newFakeFileInspector()
	inspector.addFile(explicitFile, false)

	explicitURI := "amitia://workspace/custom-plugin/host.js"
	r, err := NewArtifactResolver(ResolveContext{
		Host:               host,
		PluginHostEntryURI: explicitURI,
		Inspector:          inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := r.Resolve(context.Background(), KindPluginHost)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if artifact.Source != SourceExplicit {
		t.Errorf("expected SourceExplicit, got %s", artifact.Source)
	}
	if filepath.Clean(artifact.EntryPath) != filepath.Clean(explicitFile) {
		t.Errorf("expected %s, got %s", explicitFile, artifact.EntryPath)
	}
}

func TestResolver_ExplicitURI_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	paths := util.RuntimePaths{WorkspaceDir: tmpDir}
	host := newFakeRuntimeHost(paths)
	inspector := newFakeFileInspector()

	r, err := NewArtifactResolver(ResolveContext{
		Host:               host,
		PluginHostEntryURI: "amitia://workspace/missing.js",
		Inspector:          inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Resolve(context.Background(), KindPluginHost)
	if !errors.Is(err, ErrHostArtifactNotFound) {
		t.Fatalf("expected ErrHostArtifactNotFound, got: %v", err)
	}
}

func TestResolver_ExplicitURI_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	paths := util.RuntimePaths{WorkspaceDir: tmpDir}
	host := newFakeRuntimeHost(paths)
	inspector := newFakeFileInspector()
	inspector.addFile(subDir, true)

	r, err := NewArtifactResolver(ResolveContext{
		Host:               host,
		PluginHostEntryURI: "amitia://workspace/subdir",
		Inspector:          inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Resolve(context.Background(), KindPluginHost)
	if !errors.Is(err, ErrInvalidHostArtifact) {
		t.Fatalf("expected ErrInvalidHostArtifact, got: %v", err)
	}
}

func TestResolver_ExplicitURI_UnsupportedExtension(t *testing.T) {
	tmpDir := t.TempDir()
	shPath := filepath.Join(tmpDir, "script.sh")
	if err := os.WriteFile(shPath, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	paths := util.RuntimePaths{WorkspaceDir: tmpDir}
	host := newFakeRuntimeHost(paths)
	inspector := newFakeFileInspector()
	inspector.addFile(shPath, false)

	r, err := NewArtifactResolver(ResolveContext{
		Host:               host,
		PluginHostEntryURI: "amitia://workspace/script.sh",
		Inspector:          inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Resolve(context.Background(), KindPluginHost)
	if !errors.Is(err, ErrUnsupportedHostEntry) {
		t.Fatalf("expected ErrUnsupportedHostEntry, got: %v", err)
	}
}

func TestResolver_TaskHost_ExplicitResolves(t *testing.T) {
	tmpDir := t.TempDir()
	taskFile := filepath.Join(tmpDir, "task-host.js")
	if err := os.WriteFile(taskFile, []byte("// task"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := util.RuntimePaths{WorkspaceDir: tmpDir}
	host := newFakeRuntimeHost(paths)
	inspector := newFakeFileInspector()
	inspector.addFile(taskFile, false)

	r, err := NewArtifactResolver(ResolveContext{
		Host:             host,
		TaskHostEntryURI: "amitia://workspace/task-host.js",
		Inspector:        inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := r.Resolve(context.Background(), KindTaskHost)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if artifact.Kind != KindTaskHost {
		t.Errorf("expected KindTaskHost, got %s", artifact.Kind)
	}
	if artifact.Source != SourceExplicit {
		t.Errorf("expected SourceExplicit, got %s", artifact.Source)
	}
}

func TestResolver_LegacyWorkspace_Resolves(t *testing.T) {
	tmpDir := t.TempDir()
	runtimeRoot := t.TempDir()
	legacyDir := filepath.Join(tmpDir, "runtime", "plugin-host", "dist")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(legacyDir, "index.js")
	if err := os.WriteFile(legacyFile, []byte("// legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	host := newFakeRuntimeHost(util.RuntimePaths{WorkspaceDir: tmpDir, Root: runtimeRoot})
	inspector := newFakeFileInspector()
	inspector.addFile(legacyFile, false)

	r, err := NewArtifactResolver(ResolveContext{
		Host:      host,
		Inspector: inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := r.Resolve(context.Background(), KindPluginHost)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if artifact.Source != SourceLegacyWorkspace {
		t.Errorf("expected SourceLegacyWorkspace, got %s", artifact.Source)
	}
}

func TestResolver_LegacyWorkspace_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	runtimeRoot := t.TempDir()

	host := newFakeRuntimeHost(util.RuntimePaths{WorkspaceDir: tmpDir, Root: runtimeRoot})
	inspector := newFakeFileInspector()

	r, err := NewArtifactResolver(ResolveContext{
		Host:      host,
		Inspector: inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Resolve(context.Background(), KindPluginHost)
	if !errors.Is(err, ErrHostArtifactNotFound) {
		t.Fatalf("expected ErrHostArtifactNotFound, got: %v", err)
	}
}

func TestResolver_LegacyWorkspace_EmptyWorkspaceDir(t *testing.T) {
	runtimeRoot := t.TempDir()
	host := newFakeRuntimeHost(util.RuntimePaths{WorkspaceDir: "", Root: runtimeRoot})
	inspector := newFakeFileInspector()

	r, err := NewArtifactResolver(ResolveContext{
		Host:      host,
		Inspector: inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Resolve(context.Background(), KindPluginHost)
	if !errors.Is(err, ErrWorkspaceUnavailable) {
		t.Fatalf("expected ErrWorkspaceUnavailable, got: %v", err)
	}
}

func TestResolver_ContextCancellation(t *testing.T) {
	host := newFakeRuntimeHost(util.RuntimePaths{})
	inspector := newFakeFileInspector()

	r, err := NewArtifactResolver(ResolveContext{
		Host:      host,
		Inspector: inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = r.Resolve(ctx, KindPluginHost)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestResolver_ExplicitPriority_OverridesLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	explicitFile := filepath.Join(tmpDir, "override.js")
	if err := os.WriteFile(explicitFile, []byte("// override"), 0o644); err != nil {
		t.Fatal(err)
	}

	legacyDir := filepath.Join(tmpDir, "runtime", "plugin-host", "dist")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(legacyDir, "index.js")
	if err := os.WriteFile(legacyFile, []byte("// legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	host := newFakeRuntimeHost(util.RuntimePaths{WorkspaceDir: tmpDir})
	inspector := newFakeFileInspector()
	inspector.addFile(explicitFile, false)
	inspector.addFile(legacyFile, false)

	r, err := NewArtifactResolver(ResolveContext{
		Host:               host,
		PluginHostEntryURI: "amitia://workspace/override.js",
		Inspector:          inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := r.Resolve(context.Background(), KindPluginHost)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if artifact.Source != SourceExplicit {
		t.Errorf("expected SourceExplicit to take priority, got %s", artifact.Source)
	}
}

func TestResolver_RuntimeFallsBackToLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	runtimeRoot := t.TempDir()
	legacyDir := filepath.Join(tmpDir, "runtime", "task-host", "dist")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(legacyDir, "index.js")
	if err := os.WriteFile(legacyFile, []byte("// task fallback"), 0o644); err != nil {
		t.Fatal(err)
	}

	host := newFakeRuntimeHost(util.RuntimePaths{WorkspaceDir: tmpDir, Root: runtimeRoot})
	inspector := newFakeFileInspector()
	inspector.addFile(legacyFile, false)

	r, err := NewArtifactResolver(ResolveContext{
		Host:      host,
		Inspector: inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := r.Resolve(context.Background(), KindTaskHost)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if artifact.Source != SourceLegacyWorkspace {
		t.Errorf("expected SourceLegacyWorkspace fallback, got %s", artifact.Source)
	}
}

func TestResolver_DistributionRoot_DerivedFromIndex(t *testing.T) {
	tmpDir := t.TempDir()
	explicitDir := filepath.Join(tmpDir, "nested", "dist")
	if err := os.MkdirAll(explicitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	explicitFile := filepath.Join(explicitDir, "index.js")
	if err := os.WriteFile(explicitFile, []byte("// entry"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := util.RuntimePaths{WorkspaceDir: tmpDir}
	host := newFakeRuntimeHost(paths)
	inspector := newFakeFileInspector()
	inspector.addFile(explicitFile, false)

	r, err := NewArtifactResolver(ResolveContext{
		Host:               host,
		PluginHostEntryURI: "amitia://workspace/nested/dist/index.js",
		Inspector:          inspector,
	})
	if err != nil {
		t.Fatal(err)
	}

	artifact, err := r.Resolve(context.Background(), KindPluginHost)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	want := filepath.Clean(filepath.Join(tmpDir, "nested"))
	if artifact.DistributionRoot != want {
		t.Errorf("expected DistributionRoot %s, got %s", want, artifact.DistributionRoot)
	}
}
