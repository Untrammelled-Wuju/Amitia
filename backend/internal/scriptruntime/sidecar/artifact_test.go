// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

import (
	"errors"
	"testing"
)

func TestSidecarRuntimeURI(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindWeChat, "amitia://runtime/sidecar/launcher.mjs"},
		{KindQQ, "amitia://runtime/qq-sidecar/launcher.mjs"},
		{Kind(""), ""},
		{Kind("unknown"), ""},
	}
	for _, tt := range tests {
		if got := sidecarRuntimeURI(tt.kind); got != tt.want {
			t.Errorf("sidecarRuntimeURI(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestSidecarFilenames(t *testing.T) {
	tests := []struct {
		kind         Kind
		wantLauncher string
		wantBundle   string
	}{
		{KindWeChat, "launcher.mjs", "bundle.mjs"},
		{KindQQ, "launcher.mjs", "bundle.mjs"},
		{Kind(""), "", ""},
		{Kind("unknown"), "", ""},
	}
	for _, tt := range tests {
		launcher, bundle := sidecarFilenames(tt.kind)
		if launcher != tt.wantLauncher {
			t.Errorf("sidecarFilenames(%q) launcher = %q, want %q", tt.kind, launcher, tt.wantLauncher)
		}
		if bundle != tt.wantBundle {
			t.Errorf("sidecarFilenames(%q) bundle = %q, want %q", tt.kind, bundle, tt.wantBundle)
		}
	}
}

func TestSidecarSubdir(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindWeChat, "sidecar"},
		{KindQQ, "qq-sidecar"},
		{Kind(""), ""},
		{Kind("unknown"), ""},
	}
	for _, tt := range tests {
		if got := sidecarSubdir(tt.kind); got != tt.want {
			t.Errorf("sidecarSubdir(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestSidecarSourceSubdir(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindWeChat, "sidecar"},
		{KindQQ, "qq-sidecar"},
		{Kind(""), ""},
	}
	for _, tt := range tests {
		if got := sidecarSourceSubdir(tt.kind); got != tt.want {
			t.Errorf("sidecarSourceSubdir(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestNewArtifactResolverRejectsNilHost(t *testing.T) {
	_, err := NewArtifactResolver(ResolveContext{})
	if !errors.Is(err, ErrWorkspaceUnavailable) {
		t.Fatalf("expected ErrWorkspaceUnavailable for nil host, got %v", err)
	}
}

func TestNewArtifactResolverDefaultsFileInspector(t *testing.T) {
	host := &fakeHost{}
	resolver, err := NewArtifactResolver(ResolveContext{Host: host})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestResolverRejectsUnknownKind(t *testing.T) {
	host := &fakeHost{paths: testPaths()}
	resolver, err := NewArtifactResolver(ResolveContext{Host: host})
	if err != nil {
		t.Fatalf("NewArtifactResolver failed: %v", err)
	}
	_, err = resolver.Resolve(nil, Kind("unknown"))
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	var sidecarErr *sidecarResolveError
	if !errors.As(err, &sidecarErr) {
		t.Fatalf("expected sidecarResolveError, got %T: %v", err, err)
	}
	if !errors.Is(err, ErrUnknownSidecarKind) {
		t.Fatalf("expected ErrUnknownSidecarKind, got %v", err)
	}
}

func TestResolverReturnsNotFoundWhenNoArtifact(t *testing.T) {
	host := &fakeHost{paths: testPaths()}
	resolver, err := NewArtifactResolver(ResolveContext{
		Host:          host,
		FileInspector: &stubInspector{files: map[string]bool{}},
	})
	if err != nil {
		t.Fatalf("NewArtifactResolver failed: %v", err)
	}
	_, err = resolver.Resolve(nil, KindWeChat)
	if err == nil {
		t.Fatal("expected error when no artifact found")
	}
	if !errors.Is(err, ErrSidecarArtifactNotFound) {
		t.Fatalf("expected ErrSidecarArtifactNotFound, got %v", err)
	}
}

func TestSidecarResolveErrorIsUnwrap(t *testing.T) {
	inner := errors.New("inner failure")
	err := newSidecarError(KindWeChat, SourceExplicit, inner)

	if !errors.Is(err, inner) {
		t.Fatal("sidecarResolveError should unwrap to inner")
	}
}

func TestSidecarResolveErrorFormat(t *testing.T) {
	inner := errors.New("not found")
	err := newSidecarError(KindQQ, SourceRuntimePackage, inner)
	expected := "sidecar: kind=qq source=runtime-package: not found"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestErrVariablesAreDistinct(t *testing.T) {
	errs := []error{
		ErrUnknownSidecarKind,
		ErrSidecarArtifactNotFound,
		ErrSidecarArtifactInvalid,
		ErrSidecarBundleIncomplete,
		ErrSidecarSourceIncomplete,
		ErrWorkspaceUnavailable,
	}
	for i, a := range errs {
		for j, b := range errs {
			if i == j {
				continue
			}
			if a == b {
				t.Errorf("error %v at index %d should not be identical to error %v at index %d", a, i, b, j)
			}
		}
	}
}
