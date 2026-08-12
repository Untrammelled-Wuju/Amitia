// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"testing"
	"time"
)

func TestMCPRevision_IsActive(t *testing.T) {
	rev := MCPRevision{RevisionID: "rev-1"}
	inst := MCPInstallation{ActiveRevisionID: "rev-1"}

	if !rev.IsActive(inst) {
		t.Error("expected revision to be active")
	}

	inst.ActiveRevisionID = "rev-2"
	if rev.IsActive(inst) {
		t.Error("expected revision to not be active")
	}
}

func TestMCPRevision_IsValidated(t *testing.T) {
	tests := []struct {
		rev  MCPRevision
		want bool
	}{
		{MCPRevision{RevisionID: "rev-1", Validated: true}, true},
		{MCPRevision{RevisionID: "rev-1", Validated: false}, false},
		{MCPRevision{RevisionID: "", Validated: true}, false},
	}

	for _, tt := range tests {
		if got := tt.rev.IsValidated(); got != tt.want {
			t.Errorf("IsValidated(%+v)=%v, want %v", tt.rev, got, tt.want)
		}
	}
}

func TestBuildBindingStatus(t *testing.T) {
	inst := MCPInstallation{
		BindingID:          "b1",
		InstallState:       MCPInstallInstalled,
		RuntimeState:       MCPRuntimeReady,
		Enabled:            true,
		ActiveRevisionID:   "rev-1",
		PreviousRevisionID: "rev-0",
		Generation:         2,
	}

	binding := MCPBinding{
		ID:        "b1",
		Launcher:  &MCPLauncherSpec{Kind: "npx"},
		Transport: MCPTransportSpec{Kind: "stdio"},
	}

	status := BuildBindingStatus(inst, binding, nil)

	if status.BindingID != "b1" {
		t.Errorf("expected binding ID 'b1', got %q", status.BindingID)
	}
	if status.InstallState != "installed" {
		t.Errorf("expected install state 'installed', got %q", status.InstallState)
	}
	if status.RuntimeState != "ready" {
		t.Errorf("expected runtime state 'ready', got %q", status.RuntimeState)
	}
	if !status.Enabled {
		t.Error("expected enabled=true")
	}
	if status.ActiveRevision != "rev-1" {
		t.Errorf("expected active revision 'rev-1', got %q", status.ActiveRevision)
	}
	if status.PreviousRevision != "rev-0" {
		t.Errorf("expected previous revision 'rev-0', got %q", status.PreviousRevision)
	}
	if status.Launcher != "npx" {
		t.Errorf("expected launcher 'npx', got %q", status.Launcher)
	}
	if status.Transport != "stdio" {
		t.Errorf("expected transport 'stdio', got %q", status.Transport)
	}
	if status.Generation != 2 {
		t.Errorf("expected generation 2, got %d", status.Generation)
	}
}

func TestBuildBindingStatus_NilLauncher(t *testing.T) {
	inst := MCPInstallation{BindingID: "b1"}
	binding := MCPBinding{ID: "b1"}

	status := BuildBindingStatus(inst, binding, nil)
	if status.Launcher != "" {
		t.Errorf("expected empty launcher for nil launcher, got %q", status.Launcher)
	}
}

func TestOperationConflictError(t *testing.T) {
	err := &OperationConflictError{BindingID: "b1", OperationID: "op-1", CurrentState: MCPInstallInstalling}
	want := "MCP_OPERATION_CONFLICT: binding b1 already has operation op-1 in state installing"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestInvalidBindingError(t *testing.T) {
	err := &InvalidBindingError{BindingID: "b1", Reason: "not found"}
	want := "MCP_BINDING_INVALID: binding b1: not found"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestMCPInstallation_IsEnabled(t *testing.T) {
	tests := []struct {
		inst MCPInstallation
		want bool
	}{
		{MCPInstallation{InstallState: MCPInstallInstalled, Enabled: true}, true},
		{MCPInstallation{InstallState: MCPInstallInstalled, Enabled: false}, false},
		{MCPInstallation{InstallState: MCPInstallAbsent, Enabled: true}, false},
		{MCPInstallation{InstallState: MCPInstallFailed, Enabled: true}, false},
	}

	for _, tt := range tests {
		if got := tt.inst.IsEnabled(); got != tt.want {
			t.Errorf("IsEnabled(%+v)=%v, want %v", tt.inst, got, tt.want)
		}
	}
}

func TestMCPInstallation_CanUpgrade(t *testing.T) {
	tests := []struct {
		inst MCPInstallation
		want bool
	}{
		{MCPInstallation{InstallState: MCPInstallInstalled}, true},
		{MCPInstallation{InstallState: MCPInstallUpgrading}, false},
		{MCPInstallation{InstallState: MCPInstallFailed}, false},
		{MCPInstallation{InstallState: MCPInstallAbsent}, false},
	}

	for _, tt := range tests {
		if got := tt.inst.CanUpgrade(); got != tt.want {
			t.Errorf("CanUpgrade(%s)=%v, want %v", tt.inst.InstallState, got, tt.want)
		}
	}
}

func TestMCPInstallation_CanUninstall(t *testing.T) {
	tests := []struct {
		inst MCPInstallation
		want bool
	}{
		{MCPInstallation{InstallState: MCPInstallInstalled}, true},
		{MCPInstallation{InstallState: MCPInstallUpgrading}, false},
		{MCPInstallation{InstallState: MCPInstallFailed}, false},
	}

	for _, tt := range tests {
		if got := tt.inst.CanUninstall(); got != tt.want {
			t.Errorf("CanUninstall(%s)=%v, want %v", tt.inst.InstallState, got, tt.want)
		}
	}
}

func TestMCPPreparedLauncher_Fields(t *testing.T) {
	launcher := MCPPreparedLauncher{
		Executable:                  "node",
		Args:                        []string{"server.js"},
		WorkDir:                     "/tmp",
		Environment:                 map[string]string{"NODE_ENV": "production"},
		RuntimeDependencyFingerprint: "abc123",
		RevisionID:                  "rev-1",
	}
	if launcher.Executable != "node" {
		t.Errorf("unexpected executable: %q", launcher.Executable)
	}
	if launcher.RevisionID != "rev-1" {
		t.Errorf("unexpected revision ID: %q", launcher.RevisionID)
	}
}

func TestPreparedTransportConfig_Fields(t *testing.T) {
	cfg := PreparedTransportConfig{
		Kind:        "streamable_http",
		Endpoint:    "https://mcp.example.com/sse",
		Headers:     map[string]string{"Authorization": "Bearer token"},
		CredentialRef: "cred-1",
		RevisionID:  "rev-1",
	}
	if cfg.Kind != "streamable_http" {
		t.Errorf("unexpected kind: %q", cfg.Kind)
	}
	if cfg.Endpoint != "https://mcp.example.com/sse" {
		t.Errorf("unexpected endpoint: %q", cfg.Endpoint)
	}
	if cfg.RevisionID != "rev-1" {
		t.Errorf("unexpected revision ID: %q", cfg.RevisionID)
	}
}

func TestMCPRevision_CreationTimestamp(t *testing.T) {
	rev := MCPRevision{
		RevisionID: "rev-1",
		CreatedAt:  time.Now(),
	}
	if rev.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}
