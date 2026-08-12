// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "testing"

func TestMCPBinding_IsRemote(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"streamable_http", true},
		{"remote", true},
		{"stdio", false},
		{"npx", false},
		{"uvx", false},
		{"executable", false},
		{"", false},
	}

	for _, tt := range tests {
		b := MCPBinding{Transport: MCPTransportSpec{Kind: tt.kind}}
		if got := b.IsRemote(); got != tt.want {
			t.Errorf("IsRemote(kind=%q)=%v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestMCPBinding_IsLocal(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"streamable_http", false},
		{"remote", false},
		{"stdio", true},
		{"npx", true},
		{"uvx", true},
		{"executable", true},
	}

	for _, tt := range tests {
		b := MCPBinding{Transport: MCPTransportSpec{Kind: tt.kind}}
		if got := b.IsLocal(); got != tt.want {
			t.Errorf("IsLocal(kind=%q)=%v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestMCPBinding_RequiresLocalProcess(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{"executable", true},
		{"npx", true},
		{"uvx", true},
		{"stdio", false},
		{"streamable_http", false},
		{"remote", false},
	}

	for _, tt := range tests {
		b := MCPBinding{Launcher: &MCPLauncherSpec{Kind: tt.kind}}
		if got := b.RequiresLocalProcess(); got != tt.want {
			t.Errorf("RequiresLocalProcess(kind=%q)=%v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestMCPBinding_RequiresLocalProcess_NilLauncher(t *testing.T) {
	b := MCPBinding{}
	if b.RequiresLocalProcess() {
		t.Error("expected false for nil launcher")
	}
}

func TestMCPBindingScope_Values(t *testing.T) {
	tests := []struct {
		scope MCPBindingScope
		want  string
	}{
		{MCPScopeUser, "user"},
		{MCPScopeExtension, "extension"},
		{MCPScopeBuiltin, "builtin"},
	}

	for _, tt := range tests {
		if string(tt.scope) != tt.want {
			t.Errorf("got %q, want %q", tt.scope, tt.want)
		}
	}
}

func TestExtensionOwnerRef_Fields(t *testing.T) {
	ref := ExtensionOwnerRef{Type: "extension", ID: "ext-1"}
	if ref.Type != "extension" {
		t.Errorf("expected type 'extension', got %q", ref.Type)
	}
	if ref.ID != "ext-1" {
		t.Errorf("expected ID 'ext-1', got %q", ref.ID)
	}
}

func TestMCPTransportSpec_Fields(t *testing.T) {
	spec := MCPTransportSpec{
		Kind:     "streamable_http",
		Endpoint: "https://mcp.example.com/sse",
		Headers:  map[string]string{"Authorization": "Bearer token"},
	}
	if spec.Kind != "streamable_http" {
		t.Errorf("expected kind 'streamable_http', got %q", spec.Kind)
	}
	if spec.Endpoint != "https://mcp.example.com/sse" {
		t.Errorf("unexpected endpoint: %q", spec.Endpoint)
	}
	if spec.Headers["Authorization"] != "Bearer token" {
		t.Errorf("unexpected header: %q", spec.Headers["Authorization"])
	}
}

func TestMCPLauncherSpec_Fields(t *testing.T) {
	spec := MCPLauncherSpec{
		Kind:    "npx",
		Package: "mcp-server",
		Command: "mcp-server",
		Version: "1.0.0",
		Args:    []string{"--stdio"},
	}
	if spec.Kind != "npx" {
		t.Errorf("expected kind 'npx', got %q", spec.Kind)
	}
	if spec.Package != "mcp-server" {
		t.Errorf("expected package 'mcp-server', got %q", spec.Package)
	}
	if spec.Command != "mcp-server" {
		t.Errorf("expected command 'mcp-server', got %q", spec.Command)
	}
}
