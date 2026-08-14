// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"fmt"
	"time"
)

type MCPInstallation struct {
	BindingID          string          `json:"bindingId"`
	InstallState       MCPInstallState `json:"installState"`
	ActiveRevisionID   string          `json:"activeRevisionId"`
	PreviousRevisionID string          `json:"previousRevisionId"`
	Enabled            bool            `json:"enabled"`
	RuntimeState       MCPRuntimeState `json:"runtimeState"`
	Generation         uint64          `json:"generation"`
	LastOperationID    string          `json:"lastOperationId"`
	LastErrorCode      string          `json:"lastErrorCode"`
	LastErrorSummary   string          `json:"lastErrorSummary"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
}

func (i MCPInstallation) IsInstalled() bool {
	return i.InstallState == MCPInstallInstalled
}

func (i MCPInstallation) IsReady() bool {
	return i.InstallState == MCPInstallInstalled && i.RuntimeState == MCPRuntimeReady
}

func (i MCPInstallation) IsEnabled() bool {
	return i.Enabled && i.InstallState == MCPInstallInstalled
}

func (i MCPInstallation) IsOperationInProgress() bool {
	switch i.InstallState {
	case MCPInstallInstalling, MCPInstallUpgrading, MCPInstallRollingBack, MCPInstallUninstalling:
		return true
	}
	return false
}

func (i MCPInstallation) CanStart() bool {
	return i.InstallState == MCPInstallInstalled && i.Enabled
}

func (i MCPInstallation) CanEnable() bool {
	return i.InstallState == MCPInstallInstalled && !i.Enabled
}

func (i MCPInstallation) CanDisable() bool {
	return i.Enabled && (i.RuntimeState == MCPRuntimeStopped || i.RuntimeState == MCPRuntimeReady || i.RuntimeState == MCPRuntimeDegraded)
}

func (i MCPInstallation) CanUpgrade() bool {
	return i.InstallState == MCPInstallInstalled && !i.IsOperationInProgress()
}

func (i MCPInstallation) CanRollback() bool {
	return i.InstallState == MCPInstallInstalled && i.PreviousRevisionID != "" && !i.IsOperationInProgress()
}

func (i MCPInstallation) CanUninstall() bool {
	return i.InstallState == MCPInstallInstalled && !i.IsOperationInProgress()
}

func (i MCPInstallation) ValidateTransition(newState MCPInstallState) error {
	if !CanTransitionInstall(i.InstallState, newState) {
		return &InvalidInstallTransitionError{From: i.InstallState, To: newState}
	}
	return nil
}

type MCPRevision struct {
	RevisionID         string    `json:"revisionId"`
	BindingID          string    `json:"bindingId"`
	LauncherKind       string    `json:"launcherKind"`
	RequestedSpecJSON  string    `json:"requestedSpecJson"`
	ResolvedSpecJSON   string    `json:"resolvedSpecJson"`
	PackageManager     string    `json:"packageManager"`
	InstallRootURI     string    `json:"installRootUri"`
	EntryPoint         string    `json:"entryPoint"`
	ContentHash        string    `json:"contentHash"`
	LockHash           string    `json:"lockHash"`
	RuntimeFingerprint string    `json:"runtimeFingerprint"`
	CreatedAt          time.Time `json:"createdAt"`
	Validated          bool      `json:"validated"`
}

func (r MCPRevision) IsActive(installation MCPInstallation) bool {
	return r.RevisionID == installation.ActiveRevisionID
}

func (r MCPRevision) IsValidated() bool {
	return r.Validated && r.RevisionID != ""
}

type MCPPreparedLauncher struct {
	Executable                   string            `json:"executable"`
	Args                         []string          `json:"args"`
	WorkDir                      string            `json:"workDir"`
	Environment                  map[string]string `json:"environment,omitempty"`
	RuntimeDependencyFingerprint string            `json:"runtimeDependencyFingerprint"`
	RevisionID                   string            `json:"revisionId"`
}

type PreparedTransportConfig struct {
	Kind          string            `json:"kind"`
	Endpoint      string            `json:"endpoint"`
	Headers       map[string]string `json:"headers,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	CredentialRef string            `json:"credentialRef,omitempty"`
	RevisionID    string            `json:"revisionId"`
}

type MCPBindingStatus struct {
	BindingID             string `json:"bindingId"`
	InstallState          string `json:"installState"`
	RuntimeState          string `json:"runtimeState"`
	Enabled               bool   `json:"enabled"`
	ActiveRevision        string `json:"activeRevision"`
	PreviousRevision      string `json:"previousRevision"`
	RequestedVersion      string `json:"requestedVersion"`
	ResolvedVersion       string `json:"resolvedVersion"`
	Launcher              string `json:"launcher"`
	Transport             string `json:"transport"`
	Generation            uint64 `json:"generation"`
	AuthorizationRequired bool   `json:"authorizationRequired"`
	LastErrorCode         string `json:"lastErrorCode"`
	LastErrorSummary      string `json:"lastErrorSummary"`
}

func BuildBindingStatus(installation MCPInstallation, binding MCPBinding, revision *MCPRevision) MCPBindingStatus {
	status := MCPBindingStatus{
		BindingID:        installation.BindingID,
		InstallState:     string(installation.InstallState),
		RuntimeState:     string(installation.RuntimeState),
		Enabled:          installation.Enabled,
		ActiveRevision:   installation.ActiveRevisionID,
		PreviousRevision: installation.PreviousRevisionID,
		Launcher:         "",
		Transport:        binding.Transport.Kind,
		Generation:       installation.Generation,
		LastErrorCode:    installation.LastErrorCode,
		LastErrorSummary: installation.LastErrorSummary,
	}
	if binding.Launcher != nil {
		status.Launcher = binding.Launcher.Kind
	}
	if revision != nil {
		status.RequestedVersion = ""
		status.ResolvedVersion = ""
	}
	return status
}

type OperationConflictError struct {
	BindingID    string
	OperationID  string
	CurrentState MCPInstallState
}

func (e *OperationConflictError) Error() string {
	return fmt.Errorf("MCP_OPERATION_CONFLICT: binding %s already has operation %s in state %s", e.BindingID, e.OperationID, e.CurrentState).Error()
}

type InvalidBindingError struct {
	BindingID string
	Reason    string
}

func (e *InvalidBindingError) Error() string {
	return fmt.Errorf("MCP_BINDING_INVALID: binding %s: %s", e.BindingID, e.Reason).Error()
}
