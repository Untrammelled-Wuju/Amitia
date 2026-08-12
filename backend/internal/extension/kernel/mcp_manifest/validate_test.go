package mcp_manifest

import (
	"strings"
	"testing"
)

func TestValidate_InvalidSchemaVersion(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: 999,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_spec_unsupported_version" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_spec_unsupported_version, got: %v", errors)
	}
}

func TestValidate_EmptyTransport(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_transport_missing" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_transport_missing, got: %v", errors)
	}
}

func TestValidate_StdioMissingCommand(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type:  MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_stdio_command_missing" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_stdio_command_missing, got: %v", errors)
	}
}

func TestValidate_StdioCommandWithShell(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "sh -c echo",
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_stdio_command_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_stdio_command_invalid, got: %v", errors)
	}
}

func TestValidate_BothTransportsConflict(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp",
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_transport_remote_conflict" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_transport_remote_conflict, got: %v", errors)
	}
}

func TestValidate_RemoteMissingURL(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_remote_url_missing" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_remote_url_missing, got: %v", errors)
	}
}

func TestValidate_RemoteURLWithUserinfo(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://user:pass@example.com/mcp",
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_remote_url_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_remote_url_invalid, got: %v", errors)
	}
}

func TestValidate_RemoteHTTPNonLoopback(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "http://example.com/mcp",
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_remote_url_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_remote_url_invalid for HTTP non-loopback, got: %v", errors)
	}
}

func TestValidate_RemoteHTTPLoopback(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "http://localhost:8080/mcp",
			},
		},
	}
	errors := Validate(spec, "test")
	if len(errors) > 0 {
		t.Fatalf("unexpected validation errors for localhost HTTP: %v", errors)
	}
}

func TestValidate_InlineEnvSecret(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
				Env: MCPEnvSpec{
					Values: map[string]string{
						"OPENAI_API_KEY": "sk-xxx",
					},
				},
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_secret_inline_forbidden" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_secret_inline_forbidden, got: %v", errors)
	}
}

func TestValidate_SecretRefEnv(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
				Env: MCPEnvSpec{
					Secrets: map[string]string{
						"API_KEY": "secret://mcp/api-key",
					},
				},
			},
		},
	}
	errors := Validate(spec, "test")
	if len(errors) > 0 {
		t.Fatalf("unexpected validation errors: %v", errors)
	}
}

func TestValidate_WorkDirHostAbsolute(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
				WorkDir: "/etc",
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_stdio_workdir_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_stdio_workdir_invalid, got: %v", errors)
	}
}

func TestValidate_WorkDirWindowsAbsolute(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
				WorkDir: `C:\Users\test`,
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_stdio_workdir_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_stdio_workdir_invalid for Windows path, got: %v", errors)
	}
}

func TestValidate_WorkDirResourceURI(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
				WorkDir: "amitia://workspace/project",
			},
		},
	}
	errors := Validate(spec, "test")
	if len(errors) > 0 {
		t.Fatalf("unexpected validation errors for ResourceURI workDir: %v", errors)
	}
}

func TestValidate_HeaderInlineSecret(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp",
				Headers: map[string]MCPValueRef{
					"Authorization": {Value: "Bearer plaintext-token"},
				},
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_secret_inline_forbidden" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_secret_inline_forbidden, got: %v", errors)
	}
}

func TestValidate_RestrictedHeader(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp",
				Headers: map[string]MCPValueRef{
					"Host": {Value: "evil.com"},
				},
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_remote_header_restricted" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_remote_header_restricted, got: %v", errors)
	}
}

func TestValidate_CRLFHeader(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp",
				Headers: map[string]MCPValueRef{
					"X-Evil": {Value: "value\r\nInjection: attack"},
				},
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_remote_header_value_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_remote_header_value_invalid for CRLF, got: %v", errors)
	}
}

func TestValidate_AuthNoneWithSecretRef(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp",
				Auth: &MCPAuthSpec{
					Type:      MCPAuthTypeNone,
					SecretRef: "secret://mcp/token",
				},
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_auth_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_auth_invalid for none+secretRef, got: %v", errors)
	}
}

func TestValidate_AuthBearerWithoutSecretRef(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp",
				Auth: &MCPAuthSpec{
					Type: MCPAuthTypeBearerToken,
				},
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_auth_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_auth_invalid for bearer without secretRef, got: %v", errors)
	}
}

func TestValidate_UnknownCapabilityState(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
		Capabilities: MCPCapabilityPolicy{
			Server: MCPServerFeaturePolicy{
				Tools: "invalid_state",
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_capability_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_capability_invalid, got: %v", errors)
	}
}

func TestValidate_InvalidRestartPolicy(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
		Lifecycle: MCPLifecyclePolicy{
			RestartPolicy: "invalid_policy",
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_lifecycle_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_lifecycle_invalid, got: %v", errors)
	}
}

func TestValidate_NegativeMaxRestartAttempts(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
		Lifecycle: MCPLifecyclePolicy{
			MaxRestartAttempts: -1,
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_lifecycle_invalid" && strings.Contains(e.Path, "maxRestartAttempts") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_lifecycle_invalid for negative attempts, got: %v", errors)
	}
}

func TestValidate_HugeStartupTimeout(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
		Lifecycle: MCPLifecyclePolicy{
			StartupTimeout: "999h",
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_lifecycle_invalid" && strings.Contains(e.Path, "startupTimeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_lifecycle_invalid for huge timeout, got: %v", errors)
	}
}

func TestValidate_ValidLifecycle(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
		Lifecycle: MCPLifecyclePolicy{
			AutoStart:          false,
			RestartPolicy:      MCPRestartPolicyOnFailure,
			MaxRestartAttempts: 3,
			StartupTimeout:     "10s",
			ShutdownTimeout:    "3s",
			ReconnectPolicy: MCPReconnectPolicy{
				Enabled:     true,
				MaxAttempts: 6,
				Backoff:     "exponential",
			},
			RefreshOnReconnect: true,
		},
	}
	errors := Validate(spec, "test")
	if len(errors) > 0 {
		t.Fatalf("unexpected validation errors for valid lifecycle: %v", errors)
	}
}

func TestValidate_CapabilityPolicy(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStdio,
			Stdio: &MCPStdioTransportSpec{
				Command: "npx",
			},
		},
		Capabilities: MCPCapabilityPolicy{
			Server: MCPServerFeaturePolicy{
				Tools:     MCPCapabilityStateRequired,
				Resources: MCPCapabilityStateOptional,
				Prompts:   MCPCapabilityStateDisabled,
			},
			Client: MCPClientFeaturePolicy{
				Roots:    MCPCapabilityStateOptional,
				Sampling: MCPCapabilityStateDisabled,
			},
		},
	}
	errors := Validate(spec, "test")
	if len(errors) > 0 {
		t.Fatalf("unexpected validation errors for valid capabilities: %v", errors)
	}
}

func TestValidate_PrivateNetwork(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL:                 "https://example.com/mcp",
				AllowPrivateNetwork: true,
			},
		},
	}
	errors := Validate(spec, "test")
	if len(errors) > 0 {
		t.Fatalf("unexpected validation errors: %v", errors)
	}
}

func TestValidate_TopLevelSpec(t *testing.T) {
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
		"capabilities": map[string]any{
			"server": map[string]any{
				"tools":     "required",
				"resources": "optional",
				"prompts":   "disabled",
			},
			"client": map[string]any{
				"roots":    "optional",
				"sampling": "disabled",
			},
		},
		"lifecycle": map[string]any{
			"autoStart":     false,
			"restartPolicy": "on_failure",
			"reconnectPolicy": map[string]any{
				"enabled":     true,
				"maxAttempts": 6,
			},
		},
	}
	spec, err := ParseSpec(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	errors := Validate(spec, "test")
	if len(errors) > 0 {
		t.Fatalf("unexpected validation errors for top-level spec: %v", errors)
	}
}

func TestValidate_OAuthInlineToken(t *testing.T) {
	spec := MCPServerSpec{
		SchemaVersion: MCPServerSpecVersion,
		Transport: MCPTransportSpec{
			Type: MCPTransportTypeStreamableHTTP,
			Remote: &MCPRemoteTransportSpec{
				URL: "https://example.com/mcp",
				Auth: &MCPAuthSpec{
					Type:      MCPAuthTypeOAuth,
					SecretRef: "secret://mcp/token",
				},
			},
		},
	}
	errors := Validate(spec, "test")
	found := false
	for _, e := range errors {
		if e.Code == "mcp_auth_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected mcp_auth_invalid for oauth+secretRef, got: %v", errors)
	}
}
