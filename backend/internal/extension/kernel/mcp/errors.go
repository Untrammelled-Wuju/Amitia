// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "fmt"

var (
	ErrMCPInstallPlanInvalid         = fmt.Errorf("MCP_INSTALL_PLAN_INVALID")
	ErrMCPInstallPlanExpired         = fmt.Errorf("MCP_INSTALL_PLAN_EXPIRED")
	ErrMCPInstallPlanChanged         = fmt.Errorf("MCP_INSTALL_PLAN_CHANGED")
	ErrMCPInstallApprovalRequired    = fmt.Errorf("MCP_INSTALL_APPROVAL_REQUIRED")

	ErrMCPInstallStateInvalid        = fmt.Errorf("MCP_INSTALL_STATE_INVALID")
	ErrMCPRuntimeStateInvalid        = fmt.Errorf("MCP_RUNTIME_STATE_INVALID")
	ErrMCPOperationConflict          = fmt.Errorf("MCP_OPERATION_CONFLICT")

	ErrMCPRuntimeDependencyMissing   = fmt.Errorf("MCP_RUNTIME_DEPENDENCY_MISSING")
	ErrMCPRuntimeDependencyUnsupported = fmt.Errorf("MCP_RUNTIME_DEPENDENCY_UNSUPPORTED")

	ErrMCPNodeUnavailable            = fmt.Errorf("MCP_NODE_UNAVAILABLE")
	ErrMCPNpmUnavailable             = fmt.Errorf("MCP_NPM_UNAVAILABLE")
	ErrMCPUVUnavailable              = fmt.Errorf("MCP_UV_UNAVAILABLE")
	ErrMCPPythonUnavailable          = fmt.Errorf("MCP_PYTHON_UNAVAILABLE")

	ErrMCPPackageSpecInvalid         = fmt.Errorf("MCP_PACKAGE_SPEC_INVALID")
	ErrMCPPackageVersionUnpinned     = fmt.Errorf("MCP_PACKAGE_VERSION_UNPINNED")
	ErrMCPPackageDownloadFailed      = fmt.Errorf("MCP_PACKAGE_DOWNLOAD_FAILED")
	ErrMCPPackageResolutionFailed    = fmt.Errorf("MCP_PACKAGE_RESOLUTION_FAILED")
	ErrMCPPackageTooLarge            = fmt.Errorf("MCP_PACKAGE_TOO_LARGE")

	ErrMCPInstallScriptRequired      = fmt.Errorf("MCP_INSTALL_SCRIPT_REQUIRED")
	ErrMCPInstallScriptDenied        = fmt.Errorf("MCP_INSTALL_SCRIPT_DENIED")
	ErrMCPInstallScriptApprovalRequired = fmt.Errorf("MCP_INSTALL_SCRIPT_APPROVAL_REQUIRED")
	ErrMCPInstallScriptFailed        = fmt.Errorf("MCP_INSTALL_SCRIPT_FAILED")

	ErrMCPRevisionInvalid            = fmt.Errorf("MCP_REVISION_INVALID")
	ErrMCPRevisionHashMismatch       = fmt.Errorf("MCP_REVISION_HASH_MISMATCH")
	ErrMCPEntryPointNotFound         = fmt.Errorf("MCP_ENTRYPOINT_NOT_FOUND")
	ErrMCPEntryPointInvalid          = fmt.Errorf("MCP_ENTRYPOINT_INVALID")

	ErrMCPInstallFailed              = fmt.Errorf("MCP_INSTALL_FAILED")
	ErrMCPInstallCancelled           = fmt.Errorf("MCP_INSTALL_CANCELLED")

	ErrMCPStartFailed                = fmt.Errorf("MCP_START_FAILED")
	ErrMCPStopFailed                 = fmt.Errorf("MCP_STOP_FAILED")
	ErrMCPBindingDisabled            = fmt.Errorf("MCP_BINDING_DISABLED")
	ErrMCPBindingStale               = fmt.Errorf("MCP_BINDING_STALE")

	ErrMCPUpgradeFailed              = fmt.Errorf("MCP_UPGRADE_FAILED")
	ErrMCPUpgradeRequiresApproval    = fmt.Errorf("MCP_UPGRADE_REQUIRES_APPROVAL")
	ErrMCPUpgradeCandidateInvalid    = fmt.Errorf("MCP_UPGRADE_CANDIDATE_INVALID")

	ErrMCPRollbackUnavailable        = fmt.Errorf("MCP_ROLLBACK_UNAVAILABLE")
	ErrMCPRollbackRevisionInvalid    = fmt.Errorf("MCP_ROLLBACK_REVISION_INVALID")
	ErrMCPRollbackFailed             = fmt.Errorf("MCP_ROLLBACK_FAILED")

	ErrMCPUninstallFailed            = fmt.Errorf("MCP_UNINSTALL_FAILED")
	ErrMCPCleanupIncomplete          = fmt.Errorf("MCP_CLEANUP_INCOMPLETE")

	ErrMCPRuntimeUnavailable         = fmt.Errorf("MCP_RUNTIME_UNAVAILABLE")
	ErrMCPRuntimeFailed              = fmt.Errorf("MCP_RUNTIME_FAILED")

	ErrMCPExecutableChanged          = fmt.Errorf("MCP_EXECUTABLE_CHANGED")
	ErrMCPLocalProcessUnavailable    = fmt.Errorf("MCP_LOCAL_PROCESS_UNAVAILABLE")

	ErrMCPOAuthDiscoveryFailed       = fmt.Errorf("MCP_OAUTH_DISCOVERY_FAILED")
	ErrMCPOAuthPKCEUnsupported       = fmt.Errorf("MCP_OAUTH_PKCE_UNSUPPORTED")
	ErrMCPOAuthIssuerMismatch        = fmt.Errorf("MCP_OAUTH_ISSUER_MISMATCH")
	ErrMCPOAuthStateInvalid          = fmt.Errorf("MCP_OAUTH_STATE_INVALID")
	ErrMCPOAuthCallbackFailed        = fmt.Errorf("MCP_OAUTH_CALLBACK_FAILED")
	ErrMCPOAuthTokenExchangeFailed   = fmt.Errorf("MCP_OAUTH_TOKEN_EXCHANGE_FAILED")
	ErrMCPOAuthTokenRefreshFailed    = fmt.Errorf("MCP_TOKEN_REFRESH_FAILED")
	ErrMCPOAuthScopeRequired         = fmt.Errorf("MCP_AUTH_SCOPE_REQUIRED")
	ErrMCPOAuthRegistrationUnavailable = fmt.Errorf("MCP_OAUTH_REGISTRATION_UNAVAILABLE")
	ErrMCPOAuthCIMDInvalid           = fmt.Errorf("MCP_OAUTH_CIMD_INVALID")
	ErrMCPOAuthDCRFailed             = fmt.Errorf("MCP_OAUTH_DCR_FAILED")
	ErrMCPOAuthRevokeFailed          = fmt.Errorf("MCP_OAUTH_REVOKE_FAILED")
	ErrMCPOAuthInsufficientScope     = fmt.Errorf("MCP_OAUTH_INSUFFICIENT_SCOPE")

	ErrMCPHealthProbeFailed          = fmt.Errorf("MCP_HEALTH_PROBE_FAILED")
	ErrMCPHealthUnavailable          = fmt.Errorf("MCP_HEALTH_UNAVAILABLE")
	ErrMCPHealthIncompatible         = fmt.Errorf("MCP_HEALTH_INCOMPATIBLE")
)

type MCPError struct {
	Code    string
	Summary string
}

func (e *MCPError) Error() string {
	if e.Summary != "" {
		return fmt.Errorf("%s: %s", e.Code, e.Summary).Error()
	}
	return e.Code
}

func NewMCPError(code, summary string) *MCPError {
	return &MCPError{Code: code, Summary: summary}
}
