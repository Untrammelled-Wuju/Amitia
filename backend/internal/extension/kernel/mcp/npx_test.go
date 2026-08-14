// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
)

func TestMCPNPXSpec_Struct(t *testing.T) {
	spec := MCPNPXSpec{
		Package:              "@modelcontextprotocol/server-filesystem@1.2.3",
		Binary:               "mcp-server",
		Args:                 []string{"/tmp"},
		FetchPolicy:          "allow",
		AllowFloatingVersion: false,
		WorkDir:              "/tmp/work",
		Environment:          map[string]string{"API_KEY": "secret"},
		CredentialRef:        "cred-123",
		StartTimeout:         0,
	}

	if spec.Package != "@modelcontextprotocol/server-filesystem@1.2.3" {
		t.Error("Package mismatch")
	}
}

func TestMCPNPXSpec_FetchPolicyOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		policy   string
		expected NPXFetchPolicy
	}{
		{"empty", "", NPXFetchDeny},
		{"deny", "deny", NPXFetchDeny},
		{"allow", "allow", NPXFetchAllow},
		{"unknown", "unknown", NPXFetchDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := MCPNPXSpec{FetchPolicy: tt.policy}
			if spec.FetchPolicyOrDefault() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, spec.FetchPolicyOrDefault())
			}
		})
	}
}

func TestMCPNPXSpec_StartTimeoutOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		timeout  int
		expected int
	}{
		{"zero", 0, 60},
		{"within-limit", 120, 120},
		{"exceeds-max", 300, 180},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := MCPNPXSpec{StartTimeout: 0}
			_ = spec.StartTimeoutOrDefault()
		})
	}
}

func TestValidateNPXPackageName_Valid(t *testing.T) {
	validPackages := []string{
		"foo",
		"foo@1.2.3",
		"@scope/foo",
		"@scope/foo@1.2.3",
		"foo-bar",
		"foo_bar",
		"foo.bar",
		"foo123",
	}

	for _, pkg := range validPackages {
		t.Run(pkg, func(t *testing.T) {
			if err := ValidateNPXPackageName(pkg); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestValidateNPXPackageName_Invalid(t *testing.T) {
	invalidPackages := []struct {
		name   string
		pkg    string
		errMsg string
	}{
		{"empty", "", "MCP_NPX_PACKAGE_REQUIRED"},
		{"url", "https://example.com/pkg", "MCP_NPX_PACKAGE_INVALID"},
		{"git", "git+https://github.com/user/repo", "MCP_NPX_PACKAGE_INVALID"},
		{"github", "github:user/repo", "MCP_NPX_PACKAGE_INVALID"},
		{"file", "file:./local", "MCP_NPX_PACKAGE_INVALID"},
		{"local-path", "../local", "MCP_NPX_PACKAGE_INVALID"},
		{"absolute-path", "/bin/server", "MCP_NPX_PACKAGE_INVALID"},
		{"null-byte", "foo\x00bar", "MCP_NPX_PACKAGE_INVALID"},
		{"whitespace", "foo bar", "MCP_NPX_PACKAGE_INVALID"},
		{"tab", "foo\tbar", "MCP_NPX_PACKAGE_INVALID"},
		{"newline", "foo\nbar", "MCP_NPX_PACKAGE_INVALID"},
		{"latest", "foo@latest", "MCP_NPX_VERSION_REQUIRED"},
		{"star", "foo@*", "MCP_NPX_VERSION_REQUIRED"},
		{"caret", "foo@^1.2.3", "MCP_NPX_VERSION_REQUIRED"},
		{"tilde", "foo@~1.2.3", "MCP_NPX_VERSION_REQUIRED"},
	}

	for _, tt := range invalidPackages {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNPXPackageName(tt.pkg)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
				return
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %s, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestValidateNPXBinaryName_Valid(t *testing.T) {
	validBinaries := []string{
		"",
		"mcp-server",
		"server-filesystem",
		"my.server",
	}

	for _, binary := range validBinaries {
		t.Run(binary, func(t *testing.T) {
			if err := ValidateNPXBinaryName(binary); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestValidateNPXBinaryName_Invalid(t *testing.T) {
	invalidBinaries := []struct {
		name   string
		binary string
		errMsg string
	}{
		{"path-traversal", "../server", "MCP_NPX_BINARY_INVALID"},
		{"absolute-path", "/bin/server", "MCP_NPX_BINARY_INVALID"},
		{"slash", "bin/server", "MCP_NPX_BINARY_INVALID"},
		{"backslash", "bin\\server", "MCP_NPX_BINARY_INVALID"},
		{"cmd", "cmd.exe", "MCP_NPX_BINARY_INVALID"},
		{"sh", "sh", "MCP_NPX_BINARY_INVALID"},
		{"bash", "bash", "MCP_NPX_BINARY_INVALID"},
		{"null-byte", "server\x00", "MCP_NPX_BINARY_INVALID"},
	}

	for _, tt := range invalidBinaries {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNPXBinaryName(tt.binary)
			if err == nil {
				t.Errorf("expected error for %s", tt.name)
				return
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %s, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestIsNPXReservedEnvVar(t *testing.T) {
	reservedVars := []string{
		"NPM_CONFIG_CACHE",
		"NPM_CONFIG_USERCONFIG",
		"NPM_CONFIG_REGISTRY",
		"NPM_CONFIG_IGNORE_SCRIPTS",
		"NPM_CONFIG_YES",
		"NODE_OPTIONS",
		"PATH",
		"HOME",
	}

	for _, v := range reservedVars {
		if !IsNPXReservedEnvVar(v) {
			t.Errorf("expected %s to be reserved", v)
		}
	}

	nonReservedVars := []string{
		"API_KEY",
		"AUTH_TOKEN",
		"DATABASE_URL",
	}

	for _, v := range nonReservedVars {
		if IsNPXReservedEnvVar(v) {
			t.Errorf("expected %s to NOT be reserved", v)
		}
	}
}

func TestBuildNPXControlArgs(t *testing.T) {
	spec := MCPNPXSpec{
		Package:     "test",
		FetchPolicy: "deny",
	}
	policy := NPXPolicy{}

	args := buildNPXControlArgs(spec, policy)
	if len(args) < 3 {
		t.Error("expected at least 3 control args")
	}

	spec.FetchPolicy = "allow"
	args = buildNPXControlArgs(spec, policy)
	if len(args) < 3 {
		t.Error("expected at least 3 control args")
	}
}

func TestBuildNPXEnvironment(t *testing.T) {
	spec := MCPNPXSpec{
		Environment: map[string]string{
			"API_KEY":          "secret",
			"NODE_OPTIONS":     "--require foo",
			"NPM_CONFIG_CACHE": "/tmp/cache",
		},
	}
	policy := NPXPolicy{
		CacheDir: "/amitia/cache",
	}

	env := buildNPXEnvironment(spec, policy)

	if env["API_KEY"] != "secret" {
		t.Error("expected API_KEY to be set")
	}

	if _, ok := env["NODE_OPTIONS"]; ok {
		t.Error("expected NODE_OPTIONS to be filtered out")
	}

	if env["NPM_CONFIG_CACHE"] != "/amitia/cache" {
		t.Error("expected NPM_CONFIG_CACHE to be overridden by policy")
	}
}

func TestValidateExactVersion(t *testing.T) {
	tests := []struct {
		name    string
		pkg     string
		isValid bool
	}{
		{"no-version", "foo", false},
		{"exact-version", "foo@1.2.3", true},
		{"latest", "foo@latest", false},
		{"star", "foo@*", false},
		{"caret", "foo@^1.2.3", false},
		{"tilde", "foo@~1.2.3", false},
		{"scoped-no-version", "@scope/foo", true},
		{"scoped-exact", "@scope/foo@1.2.3", true},
		{"scoped-latest", "@scope/foo@latest", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExactVersion(tt.pkg)
			if tt.isValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.isValid && err == nil {
				t.Errorf("expected error for %s", tt.name)
			}
		})
	}
}

func TestNPXPolicy_Struct(t *testing.T) {
	policy := NPXPolicy{
		AllowFloatingVersion: true,
		CacheDir:             "/amitia/cache",
		UserConfigFile:       "/amitia/npmrc",
		Registry:             "https://registry.npmjs.org/",
	}

	if !policy.AllowFloatingVersion {
		t.Error("AllowFloatingVersion mismatch")
	}
	if policy.CacheDir != "/amitia/cache" {
		t.Error("CacheDir mismatch")
	}
}

func TestNPXLauncher_NilCommandResolver(t *testing.T) {
	launcher := &NPXLauncher{commandResolver: nil}
	_, err := launcher.Resolve(nil, MCPStdioSpec{Command: "npx"})
	if err == nil {
		t.Error("expected error when command resolver is nil")
	}
}

func TestProcessLauncher_Interface(t *testing.T) {
	var launcher ProcessLauncher = &NPXLauncher{}
	if launcher == nil {
		t.Error("expected non-nil launcher")
	}
}

func TestBuildNPXPath(t *testing.T) {
	tests := []struct {
		name     string
		nodeBin  string
		expected string
	}{
		{"empty", "", ""},
		{"absolute", "/usr/bin/node", filepath.Dir("/usr/bin/node")},
		{"windows", "C:\\Program Files\\nodejs\\node.exe", "C:\\Program Files\\nodejs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := nodeenv.Environment{NodeBinary: tt.nodeBin}
			path := BuildNPXPath(env)
			if path != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, path)
			}
		})
	}
}

func TestIsValidNPXPackageFormat(t *testing.T) {
	tests := []struct {
		name    string
		pkg     string
		isValid bool
	}{
		{"valid", "foo", true},
		{"scoped", "@scope/foo", true},
		{"with-version", "foo@1.2.3", true},
		{"scoped-with-version", "@scope/foo@1.2.3", true},
		{"hyphen", "foo-bar", true},
		{"underscore", "foo_bar", true},
		{"dot", "foo.bar", true},
		{"empty", "", false},
		{"invalid-char", "foo bar", false},
		{"slash", "foo/bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidNPXPackageFormat(tt.pkg)
			if result != tt.isValid {
				t.Errorf("expected %v, got %v", tt.isValid, result)
			}
		})
	}
}
