// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestValidateCommandStringRejectsEmpty(t *testing.T) {
	err := validateCommandString("")
	if !errors.Is(err, ErrCommandRequired) {
		t.Fatalf("expected ErrCommandRequired, got %v", err)
	}
}

func TestValidateCommandStringRejectsWhitespaceOnly(t *testing.T) {
	err := validateCommandString("   ")
	if !errors.Is(err, ErrCommandRequired) {
		t.Fatalf("expected ErrCommandRequired, got %v", err)
	}
}

func TestValidateCommandStringRejectsInvalidUTF8(t *testing.T) {
	err := validateCommandString("node\xff")
	if !errors.Is(err, ErrCommandInvalid) {
		t.Fatalf("expected ErrCommandInvalid, got %v", err)
	}
}

func TestValidateCommandStringRejectsNullByte(t *testing.T) {
	err := validateCommandString("node\x00")
	if !errors.Is(err, ErrCommandInvalid) {
		t.Fatalf("expected ErrCommandInvalid, got %v", err)
	}
}

func TestValidateCommandStringAcceptsValid(t *testing.T) {
	if err := validateCommandString("node"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validateCommandString("/usr/bin/node"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateArgsRejectsNullByte(t *testing.T) {
	err := validateArgs([]string{"--foo\x00"})
	if !errors.Is(err, ErrCommandInvalid) {
		t.Fatalf("expected ErrCommandInvalid, got %v", err)
	}
}

func TestValidateArgsAcceptsValid(t *testing.T) {
	if err := validateArgs([]string{"--foo", "bar"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvocationValidateRejectsEmptyExecutable(t *testing.T) {
	inv := Invocation{Kind: KindNative, Source: SourceNativePath}
	err := inv.Validate()
	if !errors.Is(err, ErrCommandRequired) {
		t.Fatalf("expected ErrCommandRequired, got %v", err)
	}
}

func TestInvocationValidateRejectsRelativeExecutable(t *testing.T) {
	inv := Invocation{Executable: "node", Kind: KindNative, Source: SourceNativePath}
	err := inv.Validate()
	if !errors.Is(err, ErrExecutableInvalid) {
		t.Fatalf("expected ErrExecutableInvalid, got %v", err)
	}
}

func TestInvocationValidateRejectsUnknownKind(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "app")
	inv := Invocation{Executable: abs, Kind: Kind("unknown"), Source: SourceNativePath}
	err := inv.Validate()
	if !errors.Is(err, ErrCommandInvalid) {
		t.Fatalf("expected ErrCommandInvalid, got %v", err)
	}
}

func TestInvocationValidateRejectsUnknownSource(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "app")
	inv := Invocation{Executable: abs, Kind: KindNative, Source: Source("unknown")}
	err := inv.Validate()
	if !errors.Is(err, ErrCommandInvalid) {
		t.Fatalf("expected ErrCommandInvalid, got %v", err)
	}
}

func TestInvocationValidateRejectsNullInArgs(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "app")
	inv := Invocation{Executable: abs, Kind: KindNative, Source: SourceNativePath, Args: []string{"foo\x00"}}
	err := inv.Validate()
	if !errors.Is(err, ErrCommandInvalid) {
		t.Fatalf("expected ErrCommandInvalid, got %v", err)
	}
}

func TestInvocationValidateManagedNodeRejectsNonNodeBase(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "notnode")
	inv := Invocation{Executable: abs, Kind: KindNode, Source: SourceManagedNode}
	err := inv.Validate()
	if !errors.Is(err, ErrExecutableInvalid) {
		t.Fatalf("expected ErrExecutableInvalid, got %v", err)
	}
}

func TestInvocationValidateManagedNodeAcceptsNodeBase(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "node")
	inv := Invocation{Executable: abs, Kind: KindNode, Source: SourceManagedNode}
	if err := inv.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeCommandTrimsSpace(t *testing.T) {
	if got := normalizeCommand("  node  "); got != "node" {
		t.Fatalf("expected 'node', got %q", got)
	}
}

func TestClassifyCommand(t *testing.T) {
	tests := []struct {
		cmd       string
		wantKind  Kind
		wantManaged bool
	}{
		{"node", KindNode, true},
		{"node.exe", KindNode, true},
		{"NODE", KindNode, true},
		{"npm", KindNPM, true},
		{"npm.cmd", KindNPM, true},
		{"npm.exe", KindNPM, true},
		{"npx", KindNPX, true},
		{"npx.cmd", KindNPX, true},
		{"npx.exe", KindNPX, true},
		{"python", KindNative, false},
		{"/usr/bin/node", KindNative, false},
	}

	for _, tt := range tests {
		kind, managed := classifyCommand(tt.cmd)
		if kind != tt.wantKind {
			t.Errorf("classifyCommand(%q): kind = %v, want %v", tt.cmd, kind, tt.wantKind)
		}
		if managed != tt.wantManaged {
			t.Errorf("classifyCommand(%q): managed = %v, want %v", tt.cmd, managed, tt.wantManaged)
		}
	}
}

func TestIsShellCommand(t *testing.T) {
	shells := []string{"sh", "bash", "zsh", "fish", "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe"}
	for _, s := range shells {
		if !isShellCommand(s) {
			t.Errorf("expected %q to be detected as shell command", s)
		}
		if !isShellCommand("/usr/bin/" + s) {
			t.Errorf("expected /usr/bin/%q to be detected as shell command", s)
		}
	}

	nonShells := []string{"node", "npm", "npx", "python", "myapp"}
	for _, s := range nonShells {
		if isShellCommand(s) {
			t.Errorf("expected %q NOT to be detected as shell command", s)
		}
	}
}

func TestBaseName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"node", "node"},
		{"/usr/bin/node", "node"},
		{"C:\\Windows\\node.exe", "node.exe"},
		{"/usr/local/bin/", ""},
	}

	for _, tt := range tests {
		if got := baseName(tt.input); got != tt.want {
			t.Errorf("baseName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCopyArgsNil(t *testing.T) {
	if copyArgs(nil) != nil {
		t.Fatal("copyArgs(nil) should return nil")
	}
}

func TestCopyArgsIndependence(t *testing.T) {
	src := []string{"a", "b"}
	dst := copyArgs(src)
	dst[0] = "changed"
	if src[0] != "a" {
		t.Fatal("copyArgs should return independent slice")
	}
}

func TestSamePath(t *testing.T) {
	if !samePath("/usr/bin/node", "/usr/bin/./node") {
		t.Fatal("samePath should handle dot segments")
	}
	if samePath("/usr/bin/node", "/usr/bin/python") {
		t.Fatal("samePath should differ for different paths")
	}
}

func TestSamePathBase(t *testing.T) {
	if !samePathBase("/usr/bin/node", "node") {
		t.Fatal("samePathBase should match")
	}
	if !samePathBase("/usr/bin/NODE", "node") {
		t.Fatal("samePathBase should be case-insensitive")
	}
	if samePathBase("/usr/bin/node.exe", "node") {
		t.Fatal("samePathBase should not match different extensions")
	}
}

func TestCommandErrorKindAndCommand(t *testing.T) {
	err := newCommandError(ErrShellCommandForbidden, KindNative, "bash")
	if !errors.Is(err, ErrShellCommandForbidden) {
		t.Fatal("commandError.Is should match ErrShellCommandForbidden")
	}
	if errors.Is(err, ErrCommandRequired) {
		t.Fatal("commandError.Is should not match unrelated error")
	}
}

func TestWrappedCommandErrorUnwrap(t *testing.T) {
	inner := errors.New("inner failure")
	err := wrapNodeError(inner, KindNode, "node")
	var wrapped *wrappedCommandError
	if !errors.As(err, &wrapped) {
		t.Fatal("expected wrappedCommandError type")
	}
	if !errors.Is(err, ErrNodeEnvironmentUnavailable) {
		t.Fatal("wrappedCommandError should match ErrNodeEnvironmentUnavailable")
	}
	if !errors.Is(err, inner) {
		t.Fatal("wrappedCommandError should unwrap to inner error")
	}
}

func TestErrCommandVariablesAreDistinct(t *testing.T) {
	errs := []error{
		ErrCommandRequired,
		ErrCommandInvalid,
		ErrCommandNotFound,
		ErrShellCommandForbidden,
		ErrNodeEnvironmentUnavailable,
		ErrNodeCLIUnavailable,
		ErrUnmanagedNodeCommand,
		ErrExecutableInvalid,
	}
	for i, a := range errs {
		for j, b := range errs {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("error %v should not match %v", a, b)
			}
		}
	}
}
