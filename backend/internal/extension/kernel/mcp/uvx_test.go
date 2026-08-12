// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/scriptruntime/commandenv"
)

func TestUvxLaunchSpec_StartTimeoutOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"zero", 0, UVXDefaultReadyTimeout},
		{"within-limit", 60 * time.Second, 60 * time.Second},
		{"exceeds-max", 600 * time.Second, UVXMaxReadyTimeout},
		{"default", UVXDefaultReadyTimeout, UVXDefaultReadyTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := UvxLaunchSpec{StartTimeout: tt.timeout}
			got := spec.StartTimeoutOrDefault()
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestPythonToolRequirement_Canonical(t *testing.T) {
	tests := []struct {
		name     string
		req      PythonToolRequirement
		expected string
	}{
		{"name-only", PythonToolRequirement{Name: "mcp-server"}, "mcp-server"},
		{"with-version", PythonToolRequirement{Name: "mcp-server", VersionSpec: "==1.2.3"}, "mcp-server==1.2.3"},
		{"with-extras", PythonToolRequirement{Name: "mcp-server", Extras: []string{"ftp"}}, "mcp-server[ftp]"},
		{"with-both", PythonToolRequirement{Name: "mcp-server", Extras: []string{"ftp", "ssh"}, VersionSpec: "==1.2.3"}, "mcp-server[ftp,ssh]==1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.Canonical()
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestParsePythonToolRequirement_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected PythonToolRequirement
	}{
		{"mcp-server", PythonToolRequirement{Name: "mcp-server"}},
		{"mcp-server==1.2.3", PythonToolRequirement{Name: "mcp-server", VersionSpec: "==1.2.3"}},
		{"mcp-server[ftp]", PythonToolRequirement{Name: "mcp-server", Extras: []string{"ftp"}}},
		{"mcp-server[ftp,ssh]==1.2.3", PythonToolRequirement{Name: "mcp-server", Extras: []string{"ftp", "ssh"}, VersionSpec: "==1.2.3"}},
		{"MCP-Server==1.0", PythonToolRequirement{Name: "mcp-server", VersionSpec: "==1.0"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParsePythonToolRequirement(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got.Name != tt.expected.Name {
				t.Errorf("expected name %s, got %s", tt.expected.Name, got.Name)
			}
			if got.VersionSpec != tt.expected.VersionSpec {
				t.Errorf("expected version %s, got %s", tt.expected.VersionSpec, got.VersionSpec)
			}
			if len(got.Extras) != len(tt.expected.Extras) {
				t.Errorf("expected %d extras, got %d", len(tt.expected.Extras), len(got.Extras))
			}
		})
	}
}

func TestParsePythonToolRequirement_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"direct-url", "mcp-server @ https://example.com/mcp-server.tar.gz"},
		{"git", "git+https://github.com/user/repo.git"},
		{"latest", "mcp-server@latest"},
		{"star", "mcp-server==*"},
		{"caret", "mcp-server>=1.0"},
		{"tilde", "mcp-server~=1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePythonToolRequirement(tt.input)
			if err == nil {
				t.Errorf("expected error for %q, got nil", tt.input)
			}
		})
	}
}

func TestValidateUvxCommand_Valid(t *testing.T) {
	valid := []string{"mcp-server", "server", "python", "my.server", "mcp_server", "mcp-server-fetch"}
	for _, cmd := range valid {
		t.Run(cmd, func(t *testing.T) {
			if err := ValidateUvxCommand(cmd); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestValidateUvxCommand_Invalid(t *testing.T) {
	invalid := []string{"", "foo bar", "foo;rm", "foo&&bar", "foo||bar", "foo|bar", "foo>bar", "foo<bar", "foo/bar", "foo\\bar", "foo\x00bar"}
	for _, cmd := range invalid {
		t.Run(cmd, func(t *testing.T) {
			if err := ValidateUvxCommand(cmd); err == nil {
				t.Errorf("expected invalid for %q", cmd)
			}
		})
	}
}

func TestValidateUvxArgs(t *testing.T) {
	t.Run("valid-args", func(t *testing.T) {
		err := ValidateUvxArgs([]string{"--port", "8080"}, 128, 16384, 65536)
		if err != nil {
			t.Errorf("expected valid, got error: %v", err)
		}
	})

	t.Run("too-many-args", func(t *testing.T) {
		args := make([]string, 129)
		err := ValidateUvxArgs(args, 128, 16384, 65536)
		if err == nil {
			t.Error("expected error for too many args")
		}
	})

	t.Run("arg-too-long", func(t *testing.T) {
		longArg := strings.Repeat("a", 16385)
		err := ValidateUvxArgs([]string{longArg}, 128, 16384, 65536)
		if err == nil {
			t.Error("expected error for arg too long")
		}
	})

	t.Run("total-too-large", func(t *testing.T) {
		args := []string{strings.Repeat("a", 32768), strings.Repeat("b", 32768)}
		err := ValidateUvxArgs(args, 128, 16384, 65536)
		if err == nil {
			t.Error("expected error for total too large")
		}
	})
}

func TestValidateUvxPythonSelector(t *testing.T) {
	valid := []string{"", "3.10", "3.11", "3.12", "3.13", "pypy@3.10"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			if err := ValidateUvxPythonSelector(v); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}

	invalid := []string{"3", "3.10.1.1", "abc", "3.x", "python3"}
	for _, v := range invalid {
		t.Run(v, func(t *testing.T) {
			if err := ValidateUvxPythonSelector(v); err == nil {
				t.Errorf("expected invalid for %q", v)
			}
		})
	}
}

func TestValidateUvxWorkDir(t *testing.T) {
	valid := []string{"", "/tmp/work", "relative/path", "C:\\work"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			if err := ValidateUvxWorkDir(v); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}

	invalid := []string{"../escape", "/foo/../bar", "foo\x00bar"}
	for _, v := range invalid {
		t.Run(v, func(t *testing.T) {
			if err := ValidateUvxWorkDir(v); err == nil {
				t.Errorf("expected invalid for %q", v)
			}
		})
	}
}

func TestBuildUvxInvocation_Basic(t *testing.T) {
	resolver, err := commandenv.NewResolver(commandenv.ResolveContext{})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	spec := UvxLaunchSpec{
		Package: "mcp-server-fetch==1.2.3",
		Command: "mcp-server-fetch",
		Args:    []string{"--port", "8080"},
	}

	policy := UvxPolicy{
		RequireExactVersion: true,
	}

	inv, env, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.Executable == "" {
		t.Error("expected non-empty executable")
	}

	foundToolRun := false
	foundNoConfig := false
	foundNoPythonDownloads := false
	foundIsolated := false
	foundNoProgress := false
	foundFrom := false
	foundCommand := false

	for i, arg := range inv.Args {
		switch arg {
		case "tool":
			if i+1 < len(inv.Args) && inv.Args[i+1] == "run" {
				foundToolRun = true
			}
		case "--no-config":
			foundNoConfig = true
		case "--no-python-downloads":
			foundNoPythonDownloads = true
		case "--isolated":
			foundIsolated = true
		case "--no-progress":
			foundNoProgress = true
		case "--from":
			foundFrom = true
		case "mcp-server-fetch":
			foundCommand = true
		}
	}

	if !foundToolRun {
		t.Error("expected 'tool run' in args")
	}
	if !foundNoConfig {
		t.Error("expected '--no-config' in args")
	}
	if !foundNoPythonDownloads {
		t.Error("expected '--no-python-downloads' in args")
	}
	if !foundIsolated {
		t.Error("expected '--isolated' in args")
	}
	if !foundNoProgress {
		t.Error("expected '--no-progress' in args")
	}
	if !foundFrom {
		t.Error("expected '--from' in args")
	}
	if !foundCommand {
		t.Error("expected command 'mcp-server-fetch' in args")
	}

	if env == nil {
		t.Error("expected non-nil env")
	}
}

func TestBuildUvxInvocation_WithPython(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()

	spec := UvxLaunchSpec{
		Package: "mcp-server==1.0.0",
		Command: "mcp-server",
		Python:  "3.12",
	}

	policy := UvxPolicy{RequireExactVersion: true}

	inv, _, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundPython := false
	for i, arg := range inv.Args {
		if arg == "--python" && i+1 < len(inv.Args) && inv.Args[i+1] == "3.12" {
			foundPython = true
		}
	}

	if !foundPython {
		t.Error("expected '--python 3.12' in args")
	}
}

func TestBuildUvxInvocation_Offline(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()

	spec := UvxLaunchSpec{
		Package: "mcp-server==1.0.0",
		Command: "mcp-server",
		Offline: true,
	}

	policy := UvxPolicy{RequireExactVersion: true}

	inv, _, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundOffline := false
	for _, arg := range inv.Args {
		if arg == "--offline" {
			foundOffline = true
		}
	}

	if !foundOffline {
		t.Error("expected '--offline' in args")
	}
}

func TestBuildUvxInvocation_NoExactVersion(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()

	spec := UvxLaunchSpec{
		Package: "mcp-server",
		Command: "mcp-server",
	}

	policy := UvxPolicy{RequireExactVersion: true}

	_, _, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err == nil {
		t.Error("expected error for missing exact version with strict policy")
	}
}

func TestBuildUvxInvocation_CommandInjection(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()

	spec := UvxLaunchSpec{
		Package: "mcp-server==1.0.0",
		Command: "foo; rm -rf /",
	}

	policy := UvxPolicy{RequireExactVersion: true}

	_, _, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err == nil {
		t.Error("expected error for command injection")
	}
}

func TestBuildUvxInvocation_DirectURL(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()

	spec := UvxLaunchSpec{
		Package: "mcp-server @ https://evil.com/pkg.tar.gz",
		Command: "mcp-server",
	}

	policy := UvxPolicy{RequireExactVersion: true}

	_, _, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err == nil {
		t.Error("expected error for direct URL")
	}
}

func TestBuildUvxInvocation_GitDependency(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()

	spec := UvxLaunchSpec{
		Package: "git+https://github.com/user/repo.git",
		Command: "mcp-server",
	}

	policy := UvxPolicy{RequireExactVersion: true}

	_, _, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err == nil {
		t.Error("expected error for git dependency")
	}
}

func TestBuildUvxInvocation_NilResolver(t *testing.T) {
	spec := UvxLaunchSpec{
		Package: "mcp-server==1.0.0",
		Command: "mcp-server",
	}

	policy := UvxPolicy{RequireExactVersion: true}

	_, _, err := BuildUvxInvocation(context.Background(), spec, policy, nil)
	if err == nil {
		t.Error("expected error for nil resolver")
	}
}

func TestUVXLauncher_NilCommandResolver(t *testing.T) {
	launcher := NewUVXLauncher(nil)

	_, err := launcher.Resolve(context.Background(), MCPStdioSpec{Command: "uv"})
	if err == nil {
		t.Error("expected error for nil command resolver")
	}
}

func TestUVXLauncher_Resolve(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()
	launcher := NewUVXLauncher(resolver)

	spec := MCPStdioSpec{
		Command: "uv",
		Args:    []string{"tool", "run", "--help"},
	}

	inv, err := launcher.Resolve(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inv.Executable == "" {
		t.Error("expected non-empty executable")
	}
}

func TestMCPLauncherKind_String(t *testing.T) {
	if string(MCPLauncherExecutable) != "executable" {
		t.Errorf("expected 'executable', got %s", MCPLauncherExecutable)
	}
	if string(MCPLauncherNPX) != "npx" {
		t.Errorf("expected 'npx', got %s", MCPLauncherNPX)
	}
	if string(MCPLauncherUVX) != "uvx" {
		t.Errorf("expected 'uvx', got %s", MCPLauncherUVX)
	}
}

func TestIsUvxReservedEnvVar(t *testing.T) {
	reserved := []string{"UV_CACHE_DIR", "UV_TOOL_DIR", "UV_INDEX_URL", "PATH", "HOME", "SYSTEMROOT"}
	for _, v := range reserved {
		if !isUvxReservedEnvVar(v) {
			t.Errorf("expected %s to be reserved", v)
		}
	}

	nonReserved := []string{"MY_API_KEY", "FOO", "BAR"}
	for _, v := range nonReserved {
		if isUvxReservedEnvVar(v) {
			t.Errorf("expected %s to not be reserved", v)
		}
	}
}

func TestBuildUvxEnvironment(t *testing.T) {
	spec := UvxLaunchSpec{
		Environment: map[string]string{
			"MY_API_KEY":    "secret123",
			"UV_CACHE_DIR":  "should-be-ignored",
			"FOO\x00BAR":    "invalid",
			"VALID":         "value",
		},
	}

	policy := UvxPolicy{}

	env := buildUvxEnvironment(spec, policy)

	if _, exists := env["UV_CACHE_DIR"]; exists {
		t.Error("UV_CACHE_DIR should be filtered")
	}
	if _, exists := env["FOO\x00BAR"]; exists {
		t.Error("env var with null byte should be filtered")
	}
	if env["MY_API_KEY"] != "secret123" {
		t.Errorf("expected MY_API_KEY to be preserved, got %s", env["MY_API_KEY"])
	}
	if env["VALID"] != "value" {
		t.Errorf("expected VALID to be preserved, got %s", env["VALID"])
	}
}

func TestBuildUvxInvocation_CustomIndex(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()

	spec := UvxLaunchSpec{
		Package: "mcp-server==1.0.0",
		Command: "mcp-server",
		Index: &PythonIndexSpec{
			DefaultIndex: "https://private.pypi.example.com/simple",
		},
	}

	policy := UvxPolicy{
		RequireExactVersion: true,
		AllowCustomIndex:    true,
	}

	inv, _, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundIndex := false
	for i, arg := range inv.Args {
		if arg == "--index-url" && i+1 < len(inv.Args) && inv.Args[i+1] == "https://private.pypi.example.com/simple" {
			foundIndex = true
		}
	}

	if !foundIndex {
		t.Error("expected '--index-url' with custom index in args")
	}
}

func TestBuildUvxInvocation_CustomIndexDenied(t *testing.T) {
	resolver := commandenv.NewNativeLookupResolver()

	spec := UvxLaunchSpec{
		Package: "mcp-server==1.0.0",
		Command: "mcp-server",
		Index: &PythonIndexSpec{
			DefaultIndex: "https://private.pypi.example.com/simple",
		},
	}

	policy := UvxPolicy{
		RequireExactVersion: true,
		AllowCustomIndex:    false,
	}

	inv, _, err := BuildUvxInvocation(context.Background(), spec, policy, resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, arg := range inv.Args {
		if arg == "--index-url" {
			t.Error("did not expect '--index-url' when AllowCustomIndex is false")
		}
	}
}
