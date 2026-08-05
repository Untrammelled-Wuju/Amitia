// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package resourceuri

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/u-ai/backend/pkg/util"
)

func TestPhysicalRootsFromRuntimePaths(t *testing.T) {
	paths := util.RuntimePaths{
		Root:         "/runtime",
		ConfigDir:    "/config",
		DataDir:      "/data",
		LogDir:       "/log",
		WorkspaceDir: "/ws",
		CacheDir:     "/cache",
		TempDir:      "/temp",
	}
	roots := PhysicalRootsFromRuntimePaths(paths)
	if roots.Workspace != "/ws" {
		t.Errorf("Workspace=%q", roots.Workspace)
	}
	if roots.Attachments != filepath.Join("/data", "attachments") {
		t.Errorf("Attachments=%q", roots.Attachments)
	}
	if roots.Data != "/data" {
		t.Errorf("Data=%q", roots.Data)
	}
	if roots.Cache != "/cache" {
		t.Errorf("Cache=%q", roots.Cache)
	}
	if roots.Runtime != "/runtime" {
		t.Errorf("Runtime=%q", roots.Runtime)
	}
	if roots.Config != "/config" {
		t.Errorf("Config=%q", roots.Config)
	}
	if roots.Extensions != filepath.Join("/data", "extensions") {
		t.Errorf("Extensions=%q", roots.Extensions)
	}
	if roots.Logs != "/log" {
		t.Errorf("Logs=%q", roots.Logs)
	}
	if roots.Temp != "/temp" {
		t.Errorf("Temp=%q", roots.Temp)
	}
}

func TestPhysicalRootsFromRuntimePathsEmptyData(t *testing.T) {
	paths := util.RuntimePaths{
		DataDir: "",
	}
	roots := PhysicalRootsFromRuntimePaths(paths)
	if roots.Attachments != "" || roots.Extensions != "" {
		t.Errorf("Attachments and Extensions should be empty when DataDir is empty, got %q %q", roots.Attachments, roots.Extensions)
	}
}

func TestPhysicalResolverResolvesFilesystemRoots(t *testing.T) {
	ws := t.TempDir()
	att := t.TempDir()
	data := t.TempDir()
	cache := t.TempDir()
	rt := t.TempDir()
	cfg := t.TempDir()
	ext := t.TempDir()
	logs := t.TempDir()
	tmp := t.TempDir()

	roots := PhysicalRoots{
		Workspace:   ws,
		Attachments: att,
		Data:        data,
		Cache:       cache,
		Runtime:     rt,
		Config:      cfg,
		Extensions:  ext,
		Logs:        logs,
		Temp:        tmp,
	}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatalf("NewPhysicalResolver failed: %v", err)
	}

	cases := []struct {
		uri       string
		wantLocal string
	}{
		{"amitia://workspace/", ws},
		{"amitia://workspace/a/b", filepath.Join(ws, "a", "b")},
		{"amitia://data/", data},
		{"amitia://data/records/2024", filepath.Join(data, "records", "2024")},
		{"amitia://attachments/", att},
		{"amitia://cache/x", filepath.Join(cache, "x")},
		{"amitia://config/app.yml", filepath.Join(cfg, "app.yml")},
		{"amitia://extensions/ext1/mod.mjs", filepath.Join(ext, "ext1", "mod.mjs")},
		{"amitia://logs/app.log", filepath.Join(logs, "app.log")},
		{"amitia://temp/tmp.txt", filepath.Join(tmp, "tmp.txt")},
	}
	for _, tc := range cases {
		uri, err := Parse(tc.uri)
		if err != nil {
			t.Fatalf("Parse(%q) failed: %v", tc.uri, err)
		}
		resolved, err := r.Resolve(uri)
		if err != nil {
			t.Fatalf("Resolve(%q) failed: %v", tc.uri, err)
		}
		if resolved.Kind != ResourceKindFilesystem {
			t.Fatalf("Resolve(%q).Kind=%v, want Filesystem", tc.uri, resolved.Kind)
		}
		if resolved.LocalPath != tc.wantLocal {
			t.Fatalf("Resolve(%q).LocalPath=%q, want %q", tc.uri, resolved.LocalPath, tc.wantLocal)
		}
	}
}

func TestPhysicalResolverDoesNotCreateDirectories(t *testing.T) {
	base := t.TempDir()
	nonExistent := filepath.Join(base, "does-not-exist")
	roots := PhysicalRoots{
		Workspace: nonExistent,
	}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatalf("NewPhysicalResolver failed: %v", err)
	}
	uri := MustParse("amitia://workspace/sub/file.txt")
	_, err = r.Resolve(uri)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(nonExistent, "sub", "file.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("directory should not exist, got err=%v", statErr)
	}
}

func TestPhysicalResolverRejectsUnconfiguredRoot(t *testing.T) {
	roots := PhysicalRoots{}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatalf("NewPhysicalResolver failed: %v", err)
	}
	uri := MustParse("amitia://workspace/file.txt")
	_, err = r.Resolve(uri)
	if !errors.Is(err, ErrRootNotConfigured) {
		t.Fatalf("expected ErrRootNotConfigured, got %v", err)
	}
}

func TestPhysicalResolverRejectsNativeResource(t *testing.T) {
	roots := PhysicalRoots{Workspace: t.TempDir()}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}
	uri := MustParse("amitia://native/clipboard/read")
	resolved, err := r.Resolve(uri)
	if !errors.Is(err, ErrNonFilesystemResource) {
		t.Fatalf("expected ErrNonFilesystemResource, got %v", err)
	}
	if resolved.LocalPath != "" {
		t.Fatalf("LocalPath should be empty for virtual resource, got %q", resolved.LocalPath)
	}
	if _, statErr := os.Stat(filepath.Join(roots.Workspace, "native")); !os.IsNotExist(statErr) {
		t.Fatalf("native dir should not be created")
	}
}

func TestPhysicalResolverKeepsResolvedPathInsideRoot(t *testing.T) {
	ws := t.TempDir()
	similar := filepath.Join(filepath.Dir(ws), "workspace2")
	if err := os.MkdirAll(similar, 0755); err == nil {
		defer os.RemoveAll(similar)
	}
	roots := PhysicalRoots{Workspace: ws}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}
	uri := MustParse("amitia://workspace/legit/file.txt")
	resolved, err := r.Resolve(uri)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if !strings.HasPrefix(resolved.LocalPath, ws) {
		t.Fatalf("path outside root: %q", resolved.LocalPath)
	}
}

func TestPhysicalResolverReverse(t *testing.T) {
	ws := t.TempDir()
	att := filepath.Join(t.TempDir(), "att")
	ext := filepath.Join(t.TempDir(), "ext")
	data := t.TempDir()
	logs := t.TempDir()
	tmp := filepath.Join(t.TempDir(), "tmp")

	roots := PhysicalRoots{
		Workspace:   ws,
		Attachments: att,
		Extensions:  ext,
		Data:        data,
		Logs:        logs,
		Temp:        tmp,
	}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		local string
		want  string
	}{
		{ws, "amitia://workspace/"},
		{filepath.Join(ws, "project", "file.txt"), "amitia://workspace/project/file.txt"},
		{att, "amitia://attachments/"},
		{filepath.Join(att, "avatar.png"), "amitia://attachments/avatar.png"},
		{ext, "amitia://extensions/"},
		{filepath.Join(ext, "mod.mjs"), "amitia://extensions/mod.mjs"},
		{logs, "amitia://logs/"},
		{filepath.Join(logs, "app.log"), "amitia://logs/app.log"},
	}
	for _, tc := range cases {
		if err := os.MkdirAll(filepath.Dir(tc.local), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		uri, err := r.Reverse(tc.local)
		if err != nil {
			t.Fatalf("Reverse(%q) failed: %v", tc.local, err)
		}
		if uri.String() != tc.want {
			t.Fatalf("Reverse(%q)=%q, want %q", tc.local, uri.String(), tc.want)
		}
	}
}

func TestPhysicalResolverReverseUsesLongestRoot(t *testing.T) {
	data := t.TempDir()
	att := filepath.Join(data, "attachments")
	ext := filepath.Join(data, "extensions")
	if err := os.MkdirAll(att, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ext, 0755); err != nil {
		t.Fatal(err)
	}
	roots := PhysicalRoots{
		Data:        data,
		Attachments: att,
		Extensions:  ext,
	}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}

	uri, err := r.Reverse(filepath.Join(att, "avatar.png"))
	if err != nil {
		t.Fatalf("Reverse failed: %v", err)
	}
	if uri.Root() != ResourceRootAttachments {
		t.Fatalf("expected root=attachments, got %s", uri.Root())
	}
	if uri.RelativePath() != "avatar.png" {
		t.Fatalf("relativePath=%q", uri.RelativePath())
	}

	uri, err = r.Reverse(filepath.Join(ext, "mod.mjs"))
	if err != nil {
		t.Fatalf("Reverse failed: %v", err)
	}
	if uri.Root() != ResourceRootExtensions {
		t.Fatalf("expected root=extensions, got %s", uri.Root())
	}
}

func TestPhysicalResolverReverseUsesStablePriorityForEqualRoots(t *testing.T) {
	dir := t.TempDir()
	roots := PhysicalRoots{
		Data:        dir,
		Attachments: dir,
		Extensions:  dir,
	}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}
	const knownFile = "marker.txt"
	if err := os.WriteFile(filepath.Join(dir, knownFile), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	uri, err := r.Reverse(filepath.Join(dir, knownFile))
	if err != nil {
		t.Fatalf("Reverse failed: %v", err)
	}
	if uri.Root() != ResourceRootAttachments {
		t.Fatalf("expected root=attachments (stable priority), got %s", uri.Root())
	}
	if uri.RelativePath() != knownFile {
		t.Fatalf("relativePath=%q", uri.RelativePath())
	}
}

func TestPhysicalResolverReverseRejectsOutsidePath(t *testing.T) {
	ws := t.TempDir()
	roots := PhysicalRoots{Workspace: ws}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(ws), "outside")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outside)

	_, err = r.Reverse(filepath.Join(outside, "x"))
	if !errors.Is(err, ErrResourceOutsideRoots) {
		t.Fatalf("expected ErrResourceOutsideRoots, got %v", err)
	}
}

func TestNewPhysicalResolverNormalizesRelativeRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chdir-based test not supported on windows")
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	roots := PhysicalRoots{
		Workspace: "relws",
	}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}
	uri := MustParse("amitia://workspace/file")
	resolved, err := r.Resolve(uri)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if !filepath.IsAbs(resolved.LocalPath) {
		t.Fatalf("expected absolute path, got %q", resolved.LocalPath)
	}
	expected := filepath.Join(tmp, "relws", "file")
	if resolved.LocalPath != expected {
		t.Fatalf("LocalPath=%q, want %q", resolved.LocalPath, expected)
	}
}

func TestPhysicalResolverUsesPlatformSeparators(t *testing.T) {
	ws := t.TempDir()
	roots := PhysicalRoots{Workspace: ws}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}
	uri := MustParse("amitia://workspace/a/b/c.txt")
	resolved, err := r.Resolve(uri)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.ContainsRune(resolved.LocalPath, rune(filepath.Separator)) {
		t.Fatalf("LocalPath should use platform separator: %q", resolved.LocalPath)
	}
	rev, err := r.Reverse(resolved.LocalPath)
	if err != nil {
		t.Fatalf("Reverse failed: %v", err)
	}
	if strings.Contains(rev.RelativePath(), "\\") && filepath.Separator == '/' {
		t.Fatalf("RelativePath should not contain backslash: %q", rev.RelativePath())
	}
}

func TestPhysicalResolverDoesNotDependOnRuntimePlatform(t *testing.T) {
	roots := PhysicalRoots{
		Workspace: t.TempDir(),
	}
	r, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}
	uri := MustParse("amitia://workspace/x")
	a, err := r.Resolve(uri)
	if err != nil {
		t.Fatal(err)
	}

	r2, err := NewPhysicalResolver(roots)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r2.Resolve(uri)
	if err != nil {
		t.Fatal(err)
	}
	if a.LocalPath != b.LocalPath {
		t.Fatalf("should be same regardless of env: %q vs %q", a.LocalPath, b.LocalPath)
	}
}

func TestRuntimePathsCanBuildPhysicalResourceRoots(t *testing.T) {
	paths := util.RuntimePaths{
		Root:         "/rt",
		ConfigDir:    "/cfg",
		DataDir:      "/data",
		LogDir:       "/log",
		WorkspaceDir: "/ws",
		CacheDir:     "/cache",
		TempDir:      "/tmp",
	}
	roots := PhysicalRootsFromRuntimePaths(paths)
	if roots.Workspace != "/ws" {
		t.Errorf("Workspace mismatch")
	}
	expectedAtt := filepath.Join("/data", "attachments")
	if roots.Attachments != expectedAtt {
		t.Errorf("Attachments=%q, want %q", roots.Attachments, expectedAtt)
	}
	expectedExt := filepath.Join("/data", "extensions")
	if roots.Extensions != expectedExt {
		t.Errorf("Extensions=%q, want %q", roots.Extensions, expectedExt)
	}
}
