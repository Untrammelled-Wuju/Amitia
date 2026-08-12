package mcp_manifest

import (
	"strings"
	"testing"
)

func TestParseSpec_ValidStdio(t *testing.T) {
	raw := map[string]any{
		"schemaVersion": 1,
		"transport": map[string]any{
			"type": "stdio",
			"stdio": map[string]any{
				"command": "npx",
				"args":    []any{"-y", "@modelcontextprotocol/server-filesystem"},
				"workDir": "amitia://workspace/",
			},
		},
	}
	spec, err := ParseSpec(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.SchemaVersion != 1 {
		t.Errorf("expected SchemaVersion=1, got %d", spec.SchemaVersion)
	}
	if spec.Transport.Type != MCPTransportTypeStdio {
		t.Errorf("expected Transport.Type=stdio, got %s", spec.Transport.Type)
	}
	if spec.Transport.Stdio == nil {
		t.Fatal("expected Stdio transport spec")
	}
	if spec.Transport.Stdio.Command != "npx" {
		t.Errorf("expected Command=npx, got %s", spec.Transport.Stdio.Command)
	}
}

func TestParseSpec_ValidRemote(t *testing.T) {
	raw := map[string]any{
		"schemaVersion": 1,
		"transport": map[string]any{
			"type": "streamable_http",
			"remote": map[string]any{
				"url": "https://example.com/mcp",
				"auth": map[string]any{
					"type":      "bearer_token",
					"secretRef": "secret://mcp/example-token",
				},
			},
		},
	}
	spec, err := ParseSpec(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Transport.Remote == nil {
		t.Fatal("expected Remote transport spec")
	}
	if spec.Transport.Remote.URL != "https://example.com/mcp" {
		t.Errorf("expected URL, got %s", spec.Transport.Remote.URL)
	}
}

func TestParseSpec_UnknownField(t *testing.T) {
	raw := map[string]any{
		"schemaVersion": 1,
		"transport": map[string]any{
			"tpye": "stdio",
		},
	}
	_, err := ParseSpec(raw)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(err.Error(), "parse spec") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseSpec_MissingTransport(t *testing.T) {
	raw := map[string]any{
		"schemaVersion": 1,
	}
	_, err := ParseSpec(raw)
	if err != nil {
		t.Fatalf("ParseSpec should not error on missing transport: %v", err)
	}
}

func TestValidate_MissingTransport(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if strings.Contains(e.Code, "mcp_transport") && strings.Contains(e.Path, "transport") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected transport error, got: %v", errors)
	}
}

func TestParseSpec_ValidCapabilities(t *testing.T) {
	raw := map[string]any{
		"schemaVersion": 1,
		"transport": map[string]any{
			"type": "stdio",
			"stdio": map[string]any{
				"command": "npx",
			},
		},
		"capabilities": map[string]any{
			"server": map[string]any{
				"tools":     "required",
				"resources": "optional",
			},
		},
	}
	spec, err := ParseSpec(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Capabilities.Server.Tools != MCPCapabilityStateRequired {
		t.Errorf("expected tools=required, got %s", spec.Capabilities.Server.Tools)
	}
}

func TestParseSpec_ValidLifecycle(t *testing.T) {
	raw := map[string]any{
		"schemaVersion": 1,
		"transport": map[string]any{
			"type": "stdio",
			"stdio": map[string]any{
				"command": "npx",
			},
		},
		"lifecycle": map[string]any{
			"autoStart":     true,
			"restartPolicy": "always",
		},
	}
	spec, err := ParseSpec(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spec.Lifecycle.AutoStart {
		t.Error("expected autoStart=true")
	}
	if spec.Lifecycle.RestartPolicy != MCPRestartPolicyAlways {
		t.Errorf("expected restartPolicy=always, got %s", spec.Lifecycle.RestartPolicy)
	}
}

func TestComputeConfigurationHash_Stable(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
				Args:    []string{"-y", "mcp-server"},
			},
		},
	}
	hash1 := spec.ComputeConfigurationHash()
	hash2 := spec.ComputeConfigurationHash()
	if hash1 != hash2 {
		t.Errorf("configuration hash not stable: %s != %s", hash1, hash2)
	}
}

func TestComputeConfigurationHash_ChangesWithEndpoint(t *testing.T) {
	spec1 := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp1",
			},
		},
	}
	spec2 := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp2",
			},
		},
	}
	if spec1.ComputeConfigurationHash() == spec2.ComputeConfigurationHash() {
		t.Error("expected different hashes for different endpoints")
	}
}

func TestComputeConfigurationHash_ChangesWithDisplayName(t *testing.T) {
	spec1 := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
	}
	spec2 := spec1
	spec2.Metadata = map[string]string{"displayName": "Different"}
	if spec1.ComputeConfigurationHash() != spec2.ComputeConfigurationHash() {
		t.Error("metadata should not change hash")
	}
}

func TestCanonicalize_Defaults(t *testing.T) {
	spec := MCPServerSpec{
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
	}
	canon := spec.Canonicalize()
	if canon.SchemaVersion != MCPServerSpecVersion {
		t.Errorf("expected SchemaVersion=%d, got %d", MCPServerSpecVersion, canon.SchemaVersion)
	}
	if canon.Lifecycle.RestartPolicy != MCPRestartPolicyOnFailure {
		t.Errorf("expected default restartPolicy=on_failure, got %s", canon.Lifecycle.RestartPolicy)
	}
	if canon.Lifecycle.StartupTimeout != "10s" {
		t.Errorf("expected default startupTimeout=10s, got %s", canon.Lifecycle.StartupTimeout)
	}
	if canon.Lifecycle.ShutdownTimeout != "3s" {
		t.Errorf("expected default shutdownTimeout=3s, got %s", canon.Lifecycle.ShutdownTimeout)
	}
}
