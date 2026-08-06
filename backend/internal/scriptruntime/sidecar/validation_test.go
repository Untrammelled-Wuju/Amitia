// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestKnownKind(t *testing.T) {
	if !knownKind(KindWeChat) {
		t.Fatal("KindWeChat should be known")
	}
	if !knownKind(KindQQ) {
		t.Fatal("KindQQ should be known")
	}
	if knownKind(Kind("invalid")) {
		t.Fatal("unknown kind should return false")
	}
	if knownKind(Kind("")) {
		t.Fatal("empty kind should return false")
	}
}

func TestKnownSource(t *testing.T) {
	sources := []Source{SourceExplicit, SourceRuntimePackage, SourceWorkspaceBundle, SourceWorkspaceSource}
	for _, s := range sources {
		if !knownSource(s) {
			t.Fatalf("source %q should be known", s)
		}
	}
	if knownSource(Source("unknown")) {
		t.Fatal("unknown source should return false")
	}
	if knownSource(Source("")) {
		t.Fatal("empty source should return false")
	}
}

func TestIsJSFile(t *testing.T) {
	jsFiles := []string{
		"/path/to/launcher.mjs",
		"/path/to/bundle.js",
		"/path/to/script.cjs",
		"/path/to/mixed.MJS",
	}
	for _, f := range jsFiles {
		if !isJSFile(f) {
			t.Errorf("expected %q to be detected as JS file", f)
		}
	}

	nonJSFiles := []string{
		"/path/to/app.exe",
		"/path/to/file.txt",
		"/path/to/noext",
		"/path/to/image.png",
	}
	for _, f := range nonJSFiles {
		if isJSFile(f) {
			t.Errorf("expected %q NOT to be detected as JS file", f)
		}
	}
}

func TestValidateArtifactSuccess(t *testing.T) {
	root := t.TempDir()
	artifact := Artifact{
		Kind:       KindWeChat,
		EntryPath:  filepath.Join(root, "launcher.mjs"),
		ArgsPrefix: nil,
		WorkingDir: root,
		Source:     SourceRuntimePackage,
	}
	if err := validateArtifact(artifact); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateArtifactRejectsUnknownKind(t *testing.T) {
	root := t.TempDir()
	artifact := Artifact{
		Kind:       Kind("unknown"),
		EntryPath:  filepath.Join(root, "launcher.mjs"),
		WorkingDir: root,
		Source:     SourceRuntimePackage,
	}
	err := validateArtifact(artifact)
	if !errors.Is(err, ErrSidecarArtifactInvalid) {
		t.Fatalf("expected ErrSidecarArtifactInvalid, got %v", err)
	}
}

func TestValidateArtifactRejectsEmptyEntryPath(t *testing.T) {
	root := t.TempDir()
	artifact := Artifact{
		Kind:       KindQQ,
		EntryPath:  "",
		WorkingDir: root,
		Source:     SourceWorkspaceBundle,
	}
	err := validateArtifact(artifact)
	if !errors.Is(err, ErrSidecarArtifactInvalid) {
		t.Fatalf("expected ErrSidecarArtifactInvalid, got %v", err)
	}
}

func TestValidateArtifactRejectsRelativeEntryPath(t *testing.T) {
	artifact := Artifact{
		Kind:       KindWeChat,
		EntryPath:  "relative/launcher.mjs",
		WorkingDir: "/tmp",
		Source:     SourceExplicit,
	}
	err := validateArtifact(artifact)
	if !errors.Is(err, ErrSidecarArtifactInvalid) {
		t.Fatalf("expected ErrSidecarArtifactInvalid, got %v", err)
	}
}

func TestValidateArtifactRejectsNonJS(t *testing.T) {
	tests := []string{
		"/tmp/app.exe",
		"/tmp/readme.md",
		"/tmp/noextension",
	}
	for _, path := range tests {
		err := validateArtifact(Artifact{
			Kind:       KindQQ,
			EntryPath:  path,
			WorkingDir: "/tmp",
			Source:     SourceExplicit,
		})
		if !errors.Is(err, ErrSidecarArtifactInvalid) {
			t.Fatalf("expected ErrSidecarArtifactInvalid for %q, got %v", path, err)
		}
	}
}

func TestValidateArtifactRejectsEmptyWorkingDir(t *testing.T) {
	root := t.TempDir()
	artifact := Artifact{
		Kind:       KindWeChat,
		EntryPath:  filepath.Join(root, "launcher.mjs"),
		WorkingDir: "",
		Source:     SourceRuntimePackage,
	}
	err := validateArtifact(artifact)
	if !errors.Is(err, ErrSidecarArtifactInvalid) {
		t.Fatalf("expected ErrSidecarArtifactInvalid, got %v", err)
	}
}

func TestValidateArtifactRejectsRelativeWorkingDir(t *testing.T) {
	root := t.TempDir()
	artifact := Artifact{
		Kind:       KindWeChat,
		EntryPath:  filepath.Join(root, "launcher.mjs"),
		WorkingDir: "relative/dir",
		Source:     SourceRuntimePackage,
	}
	err := validateArtifact(artifact)
	if !errors.Is(err, ErrSidecarArtifactInvalid) {
		t.Fatalf("expected ErrSidecarArtifactInvalid, got %v", err)
	}
}

func TestValidateArtifactRejectsUnknownSource(t *testing.T) {
	root := t.TempDir()
	artifact := Artifact{
		Kind:       KindWeChat,
		EntryPath:  filepath.Join(root, "launcher.mjs"),
		WorkingDir: root,
		Source:     Source("unknown"),
	}
	err := validateArtifact(artifact)
	if !errors.Is(err, ErrSidecarArtifactInvalid) {
		t.Fatalf("expected ErrSidecarArtifactInvalid, got %v", err)
	}
}

func TestSourceConstants(t *testing.T) {
	expected := map[Source]string{
		SourceExplicit:       "explicit",
		SourceRuntimePackage: "runtime-package",
		SourceWorkspaceBundle: "workspace-bundle",
		SourceWorkspaceSource: "workspace-source",
	}
	for src, exp := range expected {
		if string(src) != exp {
			t.Errorf("source %q != expected %q", src, exp)
		}
	}
}

func TestKindConstants(t *testing.T) {
	if string(KindWeChat) != "wechat" {
		t.Errorf("KindWeChat mismatch: %s", KindWeChat)
	}
	if string(KindQQ) != "qq" {
		t.Errorf("KindQQ mismatch: %s", KindQQ)
	}
}
