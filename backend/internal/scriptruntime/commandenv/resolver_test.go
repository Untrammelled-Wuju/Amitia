// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/pkg/platform"
)

type fakeNodeResolver struct {
	env nodeenv.Environment
	err error
}

func (f *fakeNodeResolver) Resolve(_ context.Context) (nodeenv.Environment, error) {
	if f.err != nil {
		return nodeenv.Environment{}, f.err
	}
	return f.env, nil
}

func makeTestEnvironment(root string) nodeenv.Environment {
	return nodeenv.Environment{
		NodeBinary:       filepath.Join(root, "bin", "node"),
		NPMCLI:           filepath.Join(root, "bin", "npm"),
		NPXCLI:           filepath.Join(root, "bin", "npx"),
		WorkDir:          filepath.Join(root, "workspace"),
		DistributionRoot: root,
		Source:           nodeenv.SourceRuntimePackage,
		Guest:             platform.GuestPlatformLinux,
		Architecture:      "amd64",
	}
}

type stubFileInfo struct {
	mode os.FileMode
}

func (stubFileInfo) Name() string       { return "stub" }
func (stubFileInfo) Size() int64        { return 0 }
func (s stubFileInfo) Mode() os.FileMode { return s.mode }
func (stubFileInfo) ModTime() time.Time { return time.Time{} }
func (stubFileInfo) IsDir() bool        { return false }
func (stubFileInfo) Sys() interface{}   { return nil }

type stubFileInspector struct {
	files map[string]bool
	absFn func(string) (string, error)
}

func (s *stubFileInspector) Stat(path string) (os.FileInfo, error) {
	if s.files[path] {
		return stubFileInfo{mode: 0755}, nil
	}
	return nil, os.ErrNotExist
}

func (s *stubFileInspector) Abs(path string) (string, error) {
	if s.absFn != nil {
		return s.absFn(path)
	}
	return filepath.Abs(path)
}

func TestResolverRejectsEmptyCommand(t *testing.T) {
	nodeResolver := &fakeNodeResolver{env: makeTestEnvironment(t.TempDir())}
	r, err := NewResolver(ResolveContext{NodeResolver: nodeResolver})
	if err != nil {
		t.Fatalf("NewResolver failed: %v", err)
	}
	_, err = r.Resolve(context.Background(), Request{Command: "", Args: nil})
	if !errors.Is(err, ErrCommandRequired) {
		t.Fatalf("expected ErrCommandRequired, got %v", err)
	}
}

func TestResolverRejectsShellCommand(t *testing.T) {
	nodeResolver := &fakeNodeResolver{env: makeTestEnvironment(t.TempDir())}
	r, _ := NewResolver(ResolveContext{NodeResolver: nodeResolver})

	shells := []string{"bash", "/bin/bash", "powershell", "cmd"}
	for _, sh := range shells {
		_, err := r.Resolve(context.Background(), Request{Command: sh})
		if !errors.Is(err, ErrShellCommandForbidden) {
			t.Fatalf("expected ErrShellCommandForbidden for %q, got %v", sh, err)
		}
	}
}

func TestResolverNodeManagedCommand(t *testing.T) {
	root := t.TempDir()
	env := makeTestEnvironment(root)
	nodeResolver := &fakeNodeResolver{env: env}
	r, _ := NewResolver(ResolveContext{NodeResolver: nodeResolver})

	inv, err := r.Resolve(context.Background(), Request{Command: "node", Args: []string{"--version"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.Kind != KindNode {
		t.Fatalf("expected KindNode, got %v", inv.Kind)
	}
	if inv.Source != SourceManagedNode {
		t.Fatalf("expected SourceManagedNode, got %v", inv.Source)
	}
	if inv.Executable != env.NodeBinary {
		t.Fatalf("expected NodeBinary %q, got %q", env.NodeBinary, inv.Executable)
	}
	if len(inv.Args) != 1 || inv.Args[0] != "--version" {
		t.Fatalf("expected preserved args [--version], got %v", inv.Args)
	}
}

func TestResolverNPMManagedCommand(t *testing.T) {
	root := t.TempDir()
	env := makeTestEnvironment(root)
	nodeResolver := &fakeNodeResolver{env: env}
	r, _ := NewResolver(ResolveContext{NodeResolver: nodeResolver})

	inv, err := r.Resolve(context.Background(), Request{Command: "npm", Args: []string{"install", "express"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.Kind != KindNPM {
		t.Fatalf("expected KindNPM, got %v", inv.Kind)
	}
	if inv.Source != SourceManagedNode {
		t.Fatalf("expected SourceManagedNode, got %v", inv.Source)
	}
	if inv.Executable != env.NodeBinary {
		t.Fatalf("expected NodeBinary %q, got %q", env.NodeBinary, inv.Executable)
	}
	if len(inv.Args) != 3 || inv.Args[0] != env.NPMCLI || inv.Args[1] != "install" || inv.Args[2] != "express" {
		t.Fatalf("expected NPMCLI prefix args, got %v", inv.Args)
	}
}

func TestResolverNPXManagedCommand(t *testing.T) {
	root := t.TempDir()
	env := makeTestEnvironment(root)
	nodeResolver := &fakeNodeResolver{env: env}
	r, _ := NewResolver(ResolveContext{NodeResolver: nodeResolver})

	inv, err := r.Resolve(context.Background(), Request{Command: "npx", Args: []string{"create-react-app"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.Kind != KindNPX {
		t.Fatalf("expected KindNPX, got %v", inv.Kind)
	}
	if inv.Executable != env.NodeBinary {
		t.Fatalf("expected NodeBinary %q, got %q", env.NodeBinary, inv.Executable)
	}
	if inv.Args[0] != env.NPXCLI {
		t.Fatalf("expected NPXCLI prefix, got %v", inv.Args[0])
	}
}

func TestResolverAbsolutePathNonNodePassesThrough(t *testing.T) {
	loc := newFakeLocator(nil)
	absPath := filepath.Join(t.TempDir(), "python")
	r, _ := NewResolver(ResolveContext{
		NodeResolver:      &fakeNodeResolver{env: makeTestEnvironment(t.TempDir())},
		ExecutableLocator: loc,
		FileInspector:     &stubFileInspector{files: map[string]bool{absPath: true}},
	})

	inv, err := r.Resolve(context.Background(), Request{Command: absPath})
	if err != nil {
		t.Fatalf("unexpected error for absolute native path: %v", err)
	}
	if inv.Kind != KindNative {
		t.Fatalf("expected KindNative, got %v", inv.Kind)
	}
	if inv.Source != SourceNativePath {
		t.Fatalf("expected SourceNativePath, got %v", inv.Source)
	}
}

func TestResolverBareNodeNotConfusedWithAbsPath(t *testing.T) {
	loc := newFakeLocator(map[string]string{"python": "/usr/bin/python"})
	r, _ := NewResolver(ResolveContext{
		NodeResolver:      &fakeNodeResolver{env: makeTestEnvironment(t.TempDir())},
		ExecutableLocator: loc,
		FileInspector:     &stubFileInspector{files: map[string]bool{"/usr/bin/python": true}},
	})

	inv, err := r.Resolve(context.Background(), Request{Command: "python"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Kind != KindNative {
		t.Fatalf("expected KindNative, got %v", inv.Kind)
	}
	if inv.Source != SourceNativeLookUp {
		t.Fatalf("expected SourceNativeLookUp, got %v", inv.Source)
	}
}

func TestResolverNodeEnvironmentUnavailableWrapped(t *testing.T) {
	innerErr := errors.New("runtime-root-missing")
	nodeResolver := &fakeNodeResolver{env: nodeenv.Environment{}, err: innerErr}
	r, _ := NewResolver(ResolveContext{NodeResolver: nodeResolver})

	_, err := r.Resolve(context.Background(), Request{Command: "node"})
	if !errors.Is(err, ErrNodeEnvironmentUnavailable) {
		t.Fatalf("expected ErrNodeEnvironmentUnavailable, got %v", err)
	}
	if !errors.Is(err, innerErr) {
		t.Fatalf("expected unwrap to innerErr, got %v", err)
	}
}

func TestResolverLookupNativeCommand(t *testing.T) {
	loc := newFakeLocator(map[string]string{
		"myapp": "/usr/local/bin/myapp",
	})
	r, _ := NewResolver(ResolveContext{
		NodeResolver:      &fakeNodeResolver{env: makeTestEnvironment(t.TempDir())},
		ExecutableLocator: loc,
		FileInspector:     &stubFileInspector{absFn: func(p string) (string, error) { return p, nil }},
	})

	inv, err := r.Resolve(context.Background(), Request{Command: "myapp", Args: []string{"--flag"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.Kind != KindNative {
		t.Fatalf("expected KindNative, got %v", inv.Kind)
	}
	if inv.Source != SourceNativeLookUp {
		t.Fatalf("expected SourceNativeLookUp, got %v", inv.Source)
	}
	if inv.Executable != "/usr/local/bin/myapp" {
		t.Fatalf("expected /usr/local/bin/myapp, got %v", inv.Executable)
	}
	if len(inv.Args) != 1 || inv.Args[0] != "--flag" {
		t.Fatalf("expected preserved args, got %v", inv.Args)
	}
}

func TestResolverLookupNotFound(t *testing.T) {
	loc := newFakeLocator(map[string]string{})
	r, _ := NewResolver(ResolveContext{
		NodeResolver:      &fakeNodeResolver{env: makeTestEnvironment(t.TempDir())},
		ExecutableLocator: loc,
	})

	_, err := r.Resolve(context.Background(), Request{Command: "missing-binary-xyz"})
	if !errors.Is(err, ErrCommandNotFound) {
		t.Fatalf("expected ErrCommandNotFound, got %v", err)
	}
}

func TestResolverAbsoluteNativeCommand(t *testing.T) {
	root := t.TempDir()
	absPath := filepath.Join(root, "myapp")
	r, _ := NewResolver(ResolveContext{
		NodeResolver:  &fakeNodeResolver{env: makeTestEnvironment(t.TempDir())},
		FileInspector: &stubFileInspector{files: map[string]bool{absPath: true}},
	})

	inv, err := r.Resolve(context.Background(), Request{Command: absPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.Kind != KindNative {
		t.Fatalf("expected KindNative, got %v", inv.Kind)
	}
	if inv.Source != SourceNativePath {
		t.Fatalf("expected SourceNativePath, got %v", inv.Source)
	}
}

func TestResolverAbsolutePathMissing(t *testing.T) {
	absPath := filepath.Join(t.TempDir(), "nonexistent")
	r, _ := NewResolver(ResolveContext{
		NodeResolver:  &fakeNodeResolver{env: makeTestEnvironment(t.TempDir())},
		FileInspector: &stubFileInspector{files: map[string]bool{}},
	})

	_, err := r.Resolve(context.Background(), Request{Command: absPath})
	if !errors.Is(err, ErrExecutableInvalid) {
		t.Fatalf("expected ErrExecutableInvalid, got %v", err)
	}
}

func TestResolverDefaultsToUnavailableNodeResolver(t *testing.T) {
	r, err := NewResolver(ResolveContext{})
	if err != nil {
		t.Fatalf("NewResolver with empty context should not error: %v", err)
	}
	_, err = r.Resolve(context.Background(), Request{Command: "node"})
	if !errors.Is(err, ErrNodeEnvironmentUnavailable) {
		t.Fatalf("expected ErrNodeEnvironmentUnavailable with no NodeResolver, got %v", err)
	}
}

func TestResolverDefaultsToDefaultLocator(t *testing.T) {
	r, err := NewResolver(ResolveContext{})
	if err != nil {
		t.Fatalf("NewResolver defaults should not error: %v", err)
	}

	_, err = r.Resolve(context.Background(), Request{Command: "sh"})
	if !errors.Is(err, ErrShellCommandForbidden) {
		t.Fatalf("expected ErrShellCommandForbidden for shell command, got %v", err)
	}
}

func TestResolverWithNilNodeResolver(t *testing.T) {
	r, err := NewResolver(ResolveContext{
		NodeResolver:      nil,
		ExecutableLocator: newFakeLocator(map[string]string{"echo": "/bin/echo"}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = r.Resolve(context.Background(), Request{Command: "node"})
	if !errors.Is(err, ErrNodeEnvironmentUnavailable) {
		t.Fatalf("expected ErrNodeEnvironmentUnavailable, got %v", err)
	}
}

func TestResolverInvocationValidateRoundtrip(t *testing.T) {
	root := t.TempDir()
	env := makeTestEnvironment(root)
	nodeResolver := &fakeNodeResolver{env: env}
	r, _ := NewResolver(ResolveContext{NodeResolver: nodeResolver})

	inv, err := r.Resolve(context.Background(), Request{Command: "node", Args: []string{"app.js"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := inv.Validate(); err != nil {
		t.Fatalf("resolved invocation should validate: %v", err)
	}
}
