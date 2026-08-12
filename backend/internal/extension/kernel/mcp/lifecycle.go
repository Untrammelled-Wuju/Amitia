// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"context"
	"fmt"
	"time"
)

type MCPDependencyProvisioner interface {
	Preview(ctx context.Context, spec MCPBinding) (MCPInstallPlan, error)
	Prepare(ctx context.Context, plan MCPInstallPlan) error
}

type MCPInstaller interface {
	InstallNPX(ctx context.Context, plan MCPInstallPlan, binding MCPBinding) (*MCPRevision, error)
	InstallUVX(ctx context.Context, plan MCPInstallPlan, binding MCPBinding) (*MCPRevision, error)
	InstallExecutable(ctx context.Context, plan MCPInstallPlan, binding MCPBinding) (*MCPRevision, error)
	InstallRemote(ctx context.Context, plan MCPInstallPlan, binding MCPBinding) (*MCPRevision, error)
}

type MCPLifecycle struct {
	provisioner MCPDependencyProvisioner
	installer   MCPInstaller
	installations map[string]*MCPInstallation
	operations    map[string]string
	locks         map[string]bool
}

func NewMCPLifecycle(provisioner MCPDependencyProvisioner, installer MCPInstaller) *MCPLifecycle {
	return &MCPLifecycle{
		provisioner:   provisioner,
		installer:     installer,
		installations: make(map[string]*MCPInstallation),
		operations:    make(map[string]string),
		locks:         make(map[string]bool),
	}
}

func (lc *MCPLifecycle) GetInstallation(bindingID string) (*MCPInstallation, error) {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return nil, &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	return inst, nil
}

func (lc *MCPLifecycle) RegisterBinding(binding MCPBinding) (*MCPInstallation, error) {
	if binding.ID == "" {
		return nil, &InvalidBindingError{BindingID: binding.ID, Reason: "binding ID is required"}
	}
	if _, exists := lc.installations[binding.ID]; exists {
		return nil, &InvalidBindingError{BindingID: binding.ID, Reason: "binding already exists"}
	}
	now := time.Now()
	inst := &MCPInstallation{
		BindingID:    binding.ID,
		InstallState: MCPInstallAbsent,
		Enabled:      false,
		RuntimeState: MCPRuntimeDisabled,
		Generation:   0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	lc.installations[binding.ID] = inst
	lc.operations[binding.ID] = ""
	return inst, nil
}

func (lc *MCPLifecycle) PreviewInstall(ctx context.Context, binding MCPBinding) (MCPInstallPlan, error) {
	if lc.provisioner == nil {
		return MCPInstallPlan{}, fmt.Errorf("MCP_INSTALL_PLAN_INVALID: provisioner not configured")
	}
	return lc.provisioner.Preview(ctx, binding)
}

func (lc *MCPLifecycle) Install(ctx context.Context, binding MCPBinding, plan MCPInstallPlan) error {
	inst, ok := lc.installations[binding.ID]
	if !ok {
		return &InvalidBindingError{BindingID: binding.ID, Reason: "binding not registered"}
	}
	if lc.locks[binding.ID] {
		return &OperationConflictError{BindingID: binding.ID, CurrentState: inst.InstallState}
	}
	if err := inst.ValidateTransition(MCPInstallInstalling); err != nil {
		return err
	}
	if !plan.VerifyDigest() {
		return &PlanChangedError{PlanID: plan.PlanID}
	}

	lc.locks[binding.ID] = true
	defer func() { lc.locks[binding.ID] = false }()

	inst.InstallState = MCPInstallInstalling
	inst.UpdatedAt = time.Now()

	if lc.installer == nil {
		inst.InstallState = MCPInstallFailed
		inst.LastErrorCode = "MCP_INSTALL_FAILED"
		inst.LastErrorSummary = "installer not configured"
		inst.UpdatedAt = time.Now()
		return fmt.Errorf("MCP_INSTALL_FAILED: installer not configured")
	}

	var revision *MCPRevision
	var err error

	switch {
	case binding.Launcher == nil:
		revision, err = lc.installer.InstallRemote(ctx, plan, binding)
	case MCPLauncherKind(binding.Launcher.Kind) == MCPLauncherNPX:
		revision, err = lc.installer.InstallNPX(ctx, plan, binding)
	case MCPLauncherKind(binding.Launcher.Kind) == MCPLauncherUVX:
		revision, err = lc.installer.InstallUVX(ctx, plan, binding)
	case MCPLauncherKind(binding.Launcher.Kind) == MCPLauncherExecutable:
		revision, err = lc.installer.InstallExecutable(ctx, plan, binding)
	default:
		revision, err = lc.installer.InstallRemote(ctx, plan, binding)
	}

	if err != nil {
		inst.InstallState = MCPInstallFailed
		inst.LastErrorCode = "MCP_INSTALL_FAILED"
		inst.LastErrorSummary = err.Error()
		inst.UpdatedAt = time.Now()
		return err
	}

	inst.ActiveRevisionID = revision.RevisionID
	inst.InstallState = MCPInstallInstalled
	inst.RuntimeState = MCPRuntimeStopped
	inst.Generation++
	inst.LastOperationID = plan.PlanID
	inst.LastErrorCode = ""
	inst.LastErrorSummary = ""
	inst.UpdatedAt = time.Now()

	return nil
}

func (lc *MCPLifecycle) Enable(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !inst.CanEnable() {
		return &InvalidInstallTransitionError{From: inst.InstallState, To: inst.InstallState}
	}
	inst.Enabled = true
	inst.RuntimeState = MCPRuntimeStopped
	inst.UpdatedAt = time.Now()
	return nil
}

func (lc *MCPLifecycle) Disable(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !inst.CanDisable() {
		return fmt.Errorf("MCP_BINDING_DISABLED: cannot disable binding in current state")
	}
	inst.Enabled = false
	inst.RuntimeState = MCPRuntimeDisabled
	inst.UpdatedAt = time.Now()
	return nil
}

func (lc *MCPLifecycle) Start(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !inst.CanStart() {
		return fmt.Errorf("MCP_START_FAILED: binding not in startable state")
	}
	if !CanTransitionRuntime(inst.RuntimeState, MCPRuntimeStarting) {
		return &InvalidRuntimeTransitionError{From: inst.RuntimeState, To: MCPRuntimeStarting}
	}
	inst.RuntimeState = MCPRuntimeStarting
	inst.UpdatedAt = time.Now()
	return nil
}

func (lc *MCPLifecycle) MarkReady(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !CanTransitionRuntime(inst.RuntimeState, MCPRuntimeReady) {
		return &InvalidRuntimeTransitionError{From: inst.RuntimeState, To: MCPRuntimeReady}
	}
	inst.RuntimeState = MCPRuntimeReady
	inst.UpdatedAt = time.Now()
	return nil
}

func (lc *MCPLifecycle) Stop(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !CanTransitionRuntime(inst.RuntimeState, MCPRuntimeStopping) {
		return &InvalidRuntimeTransitionError{From: inst.RuntimeState, To: MCPRuntimeStopping}
	}
	inst.RuntimeState = MCPRuntimeStopping
	inst.UpdatedAt = time.Now()
	return nil
}

func (lc *MCPLifecycle) MarkStopped(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !CanTransitionRuntime(inst.RuntimeState, MCPRuntimeStopped) {
		return &InvalidRuntimeTransitionError{From: inst.RuntimeState, To: MCPRuntimeStopped}
	}
	inst.RuntimeState = MCPRuntimeStopped
	inst.UpdatedAt = time.Now()
	return nil
}

func (lc *MCPLifecycle) MarkRuntimeFailed(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !CanTransitionRuntime(inst.RuntimeState, MCPRuntimeFailed) {
		return &InvalidRuntimeTransitionError{From: inst.RuntimeState, To: MCPRuntimeFailed}
	}
	inst.RuntimeState = MCPRuntimeFailed
	inst.UpdatedAt = time.Now()
	return nil
}

func (lc *MCPLifecycle) Uninstall(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !inst.CanUninstall() {
		return &InvalidInstallTransitionError{From: inst.InstallState, To: MCPInstallUninstalling}
	}
	inst.InstallState = MCPInstallUninstalling
	inst.Enabled = false
	inst.RuntimeState = MCPRuntimeDisabled
	inst.UpdatedAt = time.Now()

	inst.InstallState = MCPInstallRemoved
	inst.ActiveRevisionID = ""
	inst.Generation++
	inst.UpdatedAt = time.Now()
	return nil
}

func (lc *MCPLifecycle) Rollback(bindingID string) error {
	inst, ok := lc.installations[bindingID]
	if !ok {
		return &InvalidBindingError{BindingID: bindingID, Reason: "not found"}
	}
	if !inst.CanRollback() {
		return fmt.Errorf("MCP_ROLLBACK_UNAVAILABLE: no previous revision available")
	}
	if err := inst.ValidateTransition(MCPInstallRollingBack); err != nil {
		return err
	}
	inst.InstallState = MCPInstallRollingBack
	inst.UpdatedAt = time.Now()

	oldActive := inst.ActiveRevisionID
	inst.ActiveRevisionID = inst.PreviousRevisionID
	inst.PreviousRevisionID = oldActive
	inst.InstallState = MCPInstallInstalled
	inst.RuntimeState = MCPRuntimeStopped
	inst.Generation++
	inst.UpdatedAt = time.Now()

	return nil
}

func (lc *MCPLifecycle) RequireOperationLock(bindingID string) error {
	if lc.locks[bindingID] {
		return &OperationConflictError{BindingID: bindingID, CurrentState: lc.installations[bindingID].InstallState}
	}
	return nil
}

func (lc *MCPLifecycle) AcquireOperationLock(bindingID string, operationID string) error {
	if lc.locks[bindingID] {
		return &OperationConflictError{BindingID: bindingID, OperationID: operationID, CurrentState: lc.installations[bindingID].InstallState}
	}
	lc.locks[bindingID] = true
	lc.operations[bindingID] = operationID
	return nil
}

func (lc *MCPLifecycle) ReleaseOperationLock(bindingID string) {
	lc.locks[bindingID] = false
	lc.operations[bindingID] = ""
}
