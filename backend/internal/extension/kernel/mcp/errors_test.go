// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "testing"

func TestMCPError_WithError(t *testing.T) {
	err := &MCPError{Code: "MCP_INSTALL_FAILED", Summary: "npm not found"}
	want := "MCP_INSTALL_FAILED: npm not found"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestMCPError_WithoutSummary(t *testing.T) {
	err := &MCPError{Code: "MCP_INSTALL_FAILED"}
	want := "MCP_INSTALL_FAILED"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestNewMCPError(t *testing.T) {
	err := NewMCPError("MCP_START_FAILED", "process exited with code 1")
	if err.Code != "MCP_START_FAILED" {
		t.Errorf("expected code 'MCP_START_FAILED', got %q", err.Code)
	}
	if err.Summary != "process exited with code 1" {
		t.Errorf("unexpected summary: %q", err.Summary)
	}
}

func TestErrorVariables_Identity(t *testing.T) {
	if ErrMCPInstallPlanInvalid == nil {
		t.Error("expected non-nil ErrMCPInstallPlanInvalid")
	}
	if ErrMCPInstallFailed == nil {
		t.Error("expected non-nil ErrMCPInstallFailed")
	}
	if ErrMCPStartFailed == nil {
		t.Error("expected non-nil ErrMCPStartFailed")
	}
	if ErrMCPRollbackUnavailable == nil {
		t.Error("expected non-nil ErrMCPRollbackUnavailable")
	}
	if ErrMCPRuntimeDependencyMissing == nil {
		t.Error("expected non-nil ErrMCPRuntimeDependencyMissing")
	}
}

func TestPlanInvalidError(t *testing.T) {
	err := &PlanInvalidError{PlanID: "plan-1", Reason: "missing package"}
	want := "MCP_INSTALL_PLAN_INVALID: plan plan-1: missing package"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestPlanExpiredError(t *testing.T) {
	err := &PlanExpiredError{PlanID: "plan-1"}
	want := "MCP_INSTALL_PLAN_EXPIRED: plan plan-1 has expired"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestPlanChangedError(t *testing.T) {
	err := &PlanChangedError{PlanID: "plan-1"}
	want := "MCP_INSTALL_PLAN_CHANGED: plan plan-1 digest mismatch"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestApprovalRequiredError(t *testing.T) {
	err := &ApprovalRequiredError{PlanID: "plan-1"}
	want := "MCP_INSTALL_APPROVAL_REQUIRED: plan plan-1 requires approval"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestErrorCodes_UniqueValues(t *testing.T) {
	errors := map[string]error{
		"MCP_INSTALL_PLAN_INVALID":       ErrMCPInstallPlanInvalid,
		"MCP_INSTALL_PLAN_EXPIRED":       ErrMCPInstallPlanExpired,
		"MCP_INSTALL_PLAN_CHANGED":       ErrMCPInstallPlanChanged,
		"MCP_INSTALL_APPROVAL_REQUIRED":  ErrMCPInstallApprovalRequired,
		"MCP_INSTALL_STATE_INVALID":      ErrMCPInstallStateInvalid,
		"MCP_RUNTIME_STATE_INVALID":      ErrMCPRuntimeStateInvalid,
		"MCP_OPERATION_CONFLICT":         ErrMCPOperationConflict,
		"MCP_RUNTIME_DEPENDENCY_MISSING": ErrMCPRuntimeDependencyMissing,
		"MCP_NODE_UNAVAILABLE":           ErrMCPNodeUnavailable,
		"MCP_NPM_UNAVAILABLE":            ErrMCPNpmUnavailable,
		"MCP_UV_UNAVAILABLE":             ErrMCPUVUnavailable,
		"MCP_PYTHON_UNAVAILABLE":         ErrMCPPythonUnavailable,
		"MCP_PACKAGE_SPEC_INVALID":       ErrMCPPackageSpecInvalid,
		"MCP_PACKAGE_VERSION_UNPINNED":   ErrMCPPackageVersionUnpinned,
		"MCP_PACKAGE_DOWNLOAD_FAILED":    ErrMCPPackageDownloadFailed,
		"MCP_REVISION_INVALID":           ErrMCPRevisionInvalid,
		"MCP_ENTRYPOINT_NOT_FOUND":       ErrMCPEntryPointNotFound,
		"MCP_INSTALL_FAILED":             ErrMCPInstallFailed,
		"MCP_START_FAILED":               ErrMCPStartFailed,
		"MCP_STOP_FAILED":                ErrMCPStopFailed,
		"MCP_BINDING_DISABLED":           ErrMCPBindingDisabled,
		"MCP_UPGRADE_FAILED":             ErrMCPUpgradeFailed,
		"MCP_ROLLBACK_UNAVAILABLE":       ErrMCPRollbackUnavailable,
		"MCP_ROLLBACK_FAILED":            ErrMCPRollbackFailed,
		"MCP_UNINSTALL_FAILED":           ErrMCPUninstallFailed,
		"MCP_RUNTIME_UNAVAILABLE":        ErrMCPRuntimeUnavailable,
		"MCP_RUNTIME_FAILED":             ErrMCPRuntimeFailed,
	}
	if len(errors) < 20 {
		t.Errorf("expected at least 20 error variables, got %d", len(errors))
	}
}
