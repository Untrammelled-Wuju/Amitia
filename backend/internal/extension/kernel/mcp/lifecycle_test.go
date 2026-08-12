// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"context"
	"testing"
	"time"
)

type mockProvisioner struct {
	plan MCPInstallPlan
	err  error
}

func (m *mockProvisioner) Preview(ctx context.Context, spec MCPBinding) (MCPInstallPlan, error) {
	return m.plan, m.err
}

func (m *mockProvisioner) Prepare(ctx context.Context, plan MCPInstallPlan) error {
	return m.err
}

type mockInstaller struct {
	npxRevision        *MCPRevision
	uvxRevision        *MCPRevision
	executableRevision *MCPRevision
	remoteRevision     *MCPRevision
	npxErr             error
	uvxErr             error
	executableErr      error
	remoteErr          error
}

func (m *mockInstaller) InstallNPX(ctx context.Context, plan MCPInstallPlan, binding MCPBinding) (*MCPRevision, error) {
	return m.npxRevision, m.npxErr
}

func (m *mockInstaller) InstallUVX(ctx context.Context, plan MCPInstallPlan, binding MCPBinding) (*MCPRevision, error) {
	return m.uvxRevision, m.uvxErr
}

func (m *mockInstaller) InstallExecutable(ctx context.Context, plan MCPInstallPlan, binding MCPBinding) (*MCPRevision, error) {
	return m.executableRevision, m.executableErr
}

func (m *mockInstaller) InstallRemote(ctx context.Context, plan MCPInstallPlan, binding MCPBinding) (*MCPRevision, error) {
	return m.remoteRevision, m.remoteErr
}

func TestMCPLifecycle_RegisterBinding(t *testing.T) {
	lc := NewMCPLifecycle(nil, nil)
	binding := MCPBinding{
		ID: "test-binding",
	}

	inst, err := lc.RegisterBinding(binding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.BindingID != "test-binding" {
		t.Errorf("expected binding ID 'test-binding', got %q", inst.BindingID)
	}
	if inst.InstallState != MCPInstallAbsent {
		t.Errorf("expected initial state 'absent', got %q", inst.InstallState)
	}
	if inst.RuntimeState != MCPRuntimeDisabled {
		t.Errorf("expected initial runtime state 'disabled', got %q", inst.RuntimeState)
	}
}

func TestMCPLifecycle_RegisterBinding_Duplicate(t *testing.T) {
	lc := NewMCPLifecycle(nil, nil)
	binding := MCPBinding{ID: "dup"}

	_, err := lc.RegisterBinding(binding)
	if err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}

	_, err = lc.RegisterBinding(binding)
	if err == nil {
		t.Error("expected error on duplicate registration")
	}
}

func TestMCPLifecycle_RegisterBinding_EmptyID(t *testing.T) {
	lc := NewMCPLifecycle(nil, nil)
	binding := MCPBinding{ID: ""}

	_, err := lc.RegisterBinding(binding)
	if err == nil {
		t.Error("expected error on empty binding ID")
	}
}

func TestMCPLifecycle_GetInstallation_NotFound(t *testing.T) {
	lc := NewMCPLifecycle(nil, nil)

	_, err := lc.GetInstallation("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent binding")
	}
}

func TestMCPLifecycle_Install_NPX(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallAbsent,
		RuntimeState: MCPRuntimeDisabled,
	}

	lc := &MCPLifecycle{
		provisioner:   &mockProvisioner{},
		installer:     &mockInstaller{npxRevision: &MCPRevision{RevisionID: "rev-1"}},
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	binding := MCPBinding{
		ID:       "b1",
		Launcher: &MCPLauncherSpec{Kind: "npx"},
	}

	err := lc.Install(context.Background(), binding, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.InstallState != MCPInstallInstalled {
		t.Errorf("expected installed state, got %q", inst.InstallState)
	}
	if inst.ActiveRevisionID != "rev-1" {
		t.Errorf("expected active revision 'rev-1', got %q", inst.ActiveRevisionID)
	}
}

func TestMCPLifecycle_Install_PlanChanged(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:    "plan-1",
		BindingID: "b1",
	}
	plan.PlanDigest = "wrong-digest"

	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallAbsent,
		RuntimeState: MCPRuntimeDisabled,
	}

	lc := &MCPLifecycle{
		provisioner:   &mockProvisioner{},
		installer:     &mockInstaller{},
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	binding := MCPBinding{ID: "b1", Launcher: &MCPLauncherSpec{Kind: "npx"}}
	err := lc.Install(context.Background(), binding, plan)
	if err == nil {
		t.Error("expected error for changed plan digest")
	}
	if inst.InstallState != MCPInstallAbsent {
		t.Errorf("expected absent state after digest mismatch (before state change), got %q", inst.InstallState)
	}
}

func TestMCPLifecycle_Install_Locked(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:    "plan-1",
		BindingID: "b1",
	}
	plan.PlanDigest = plan.ComputeDigest()

	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallAbsent,
		RuntimeState: MCPRuntimeDisabled,
	}

	lc := &MCPLifecycle{
		provisioner:   &mockProvisioner{},
		installer:     &mockInstaller{},
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{"b1": true},
	}

	binding := MCPBinding{ID: "b1", Launcher: &MCPLauncherSpec{Kind: "npx"}}
	err := lc.Install(context.Background(), binding, plan)
	if err == nil {
		t.Error("expected error when operation locked")
	}
}

func TestMCPLifecycle_Install_InstallerError(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "bad-pkg",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallAbsent,
		RuntimeState: MCPRuntimeDisabled,
	}

	lc := &MCPLifecycle{
		provisioner:   &mockProvisioner{},
		installer:     &mockInstaller{npxErr: ErrMCPInstallFailed},
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	binding := MCPBinding{ID: "b1", Launcher: &MCPLauncherSpec{Kind: "npx"}}
	err := lc.Install(context.Background(), binding, plan)
	if err == nil {
		t.Error("expected error when installer fails")
	}
	if inst.InstallState != MCPInstallFailed {
		t.Errorf("expected failed state, got %q", inst.InstallState)
	}
	if inst.LastErrorCode != "MCP_INSTALL_FAILED" {
		t.Errorf("expected error code 'MCP_INSTALL_FAILED', got %q", inst.LastErrorCode)
	}
}

func TestMCPLifecycle_EnableDisable(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallInstalled,
		Enabled:      false,
		RuntimeState: MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	if err := lc.Enable("b1"); err != nil {
		t.Fatalf("enable should succeed: %v", err)
	}
	if !inst.Enabled {
		t.Error("expected enabled=true")
	}

	if err := lc.Disable("b1"); err != nil {
		t.Fatalf("disable should succeed: %v", err)
	}
	if inst.Enabled {
		t.Error("expected enabled=false")
	}
	if inst.RuntimeState != MCPRuntimeDisabled {
		t.Errorf("expected runtime state 'disabled', got %q", inst.RuntimeState)
	}
}

func TestMCPLifecycle_Disable_NotEnabled(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallInstalled,
		Enabled:      false,
		RuntimeState: MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	err := lc.Disable("b1")
	if err == nil {
		t.Error("expected error when disabling already-disabled binding")
	}
}

func TestMCPLifecycle_StartStop(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallInstalled,
		Enabled:      true,
		RuntimeState: MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	if err := lc.Start("b1"); err != nil {
		t.Fatalf("start should succeed: %v", err)
	}
	if inst.RuntimeState != MCPRuntimeStarting {
		t.Errorf("expected starting state, got %q", inst.RuntimeState)
	}

	if err := lc.MarkReady("b1"); err != nil {
		t.Fatalf("mark ready should succeed: %v", err)
	}
	if inst.RuntimeState != MCPRuntimeReady {
		t.Errorf("expected ready state, got %q", inst.RuntimeState)
	}

	if err := lc.Stop("b1"); err != nil {
		t.Fatalf("stop should succeed: %v", err)
	}
	if inst.RuntimeState != MCPRuntimeStopping {
		t.Errorf("expected stopping state, got %q", inst.RuntimeState)
	}

	if err := lc.MarkStopped("b1"); err != nil {
		t.Fatalf("mark stopped should succeed: %v", err)
	}
	if inst.RuntimeState != MCPRuntimeStopped {
		t.Errorf("expected stopped state, got %q", inst.RuntimeState)
	}
}

func TestMCPLifecycle_Start_NotEnabled(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallInstalled,
		Enabled:      false,
		RuntimeState: MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	err := lc.Start("b1")
	if err == nil {
		t.Error("expected error when starting disabled binding")
	}
}

func TestMCPLifecycle_MarkRuntimeFailed(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallInstalled,
		Enabled:      true,
		RuntimeState: MCPRuntimeReady,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	if err := lc.MarkRuntimeFailed("b1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.RuntimeState != MCPRuntimeFailed {
		t.Errorf("expected runtime_failed, got %q", inst.RuntimeState)
	}
}

func TestMCPLifecycle_Uninstall(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:        "b1",
		InstallState:     MCPInstallInstalled,
		ActiveRevisionID: "rev-1",
		Enabled:          true,
		RuntimeState:     MCPRuntimeStopped,
		Generation:       1,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	if err := lc.Uninstall("b1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.InstallState != MCPInstallRemoved {
		t.Errorf("expected removed state, got %q", inst.InstallState)
	}
	if inst.ActiveRevisionID != "" {
		t.Error("expected active revision cleared")
	}
	if inst.Generation != 2 {
		t.Errorf("expected generation 2, got %d", inst.Generation)
	}
}

func TestMCPLifecycle_Uninstall_InProgress(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallUpgrading,
		Enabled:      true,
		RuntimeState: MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	err := lc.Uninstall("b1")
	if err == nil {
		t.Error("expected error when uninstalling during in-progress operation")
	}
}

func TestMCPLifecycle_Rollback(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:          "b1",
		InstallState:       MCPInstallInstalled,
		ActiveRevisionID:   "rev-2",
		PreviousRevisionID: "rev-1",
		Enabled:            true,
		RuntimeState:       MCPRuntimeStopped,
		Generation:         2,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	if err := lc.Rollback("b1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.ActiveRevisionID != "rev-1" {
		t.Errorf("expected active revision 'rev-1', got %q", inst.ActiveRevisionID)
	}
	if inst.PreviousRevisionID != "rev-2" {
		t.Errorf("expected previous revision 'rev-2', got %q", inst.PreviousRevisionID)
	}
	if inst.Generation != 3 {
		t.Errorf("expected generation 3, got %d", inst.Generation)
	}
}

func TestMCPLifecycle_Rollback_NoPrevious(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:        "b1",
		InstallState:     MCPInstallInstalled,
		ActiveRevisionID: "rev-1",
		Enabled:          true,
		RuntimeState:     MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	err := lc.Rollback("b1")
	if err == nil {
		t.Error("expected error when no previous revision")
	}
}

func TestMCPLifecycle_OperationLock(t *testing.T) {
	lc := NewMCPLifecycle(nil, nil)
	_, _ = lc.RegisterBinding(MCPBinding{ID: "b1"})

	if err := lc.AcquireOperationLock("b1", "op-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := lc.RequireOperationLock("b1"); err == nil {
		t.Error("expected error when lock held")
	}

	lc.ReleaseOperationLock("b1")

	if err := lc.RequireOperationLock("b1"); err != nil {
		t.Errorf("expected no error after release: %v", err)
	}
}

func TestMCPLifecycle_PreviewInstall(t *testing.T) {
	expectedPlan := MCPInstallPlan{PlanID: "test-plan"}
	prov := &mockProvisioner{plan: expectedPlan}
	lc := NewMCPLifecycle(prov, nil)

	plan, err := lc.PreviewInstall(context.Background(), MCPBinding{ID: "b1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.PlanID != "test-plan" {
		t.Errorf("expected plan ID 'test-plan', got %q", plan.PlanID)
	}
}

func TestMCPLifecycle_Install_BindingNotRegistered(t *testing.T) {
	lc := NewMCPLifecycle(&mockProvisioner{}, &mockInstaller{})
	plan := MCPInstallPlan{PlanID: "p1", BindingID: "nonexistent"}
	plan.PlanDigest = plan.ComputeDigest()

	binding := MCPBinding{ID: "nonexistent", Launcher: &MCPLauncherSpec{Kind: "npx"}}
	err := lc.Install(context.Background(), binding, plan)
	if err == nil {
		t.Error("expected error for unregistered binding")
	}
}

func TestMCPLifecycle_Install_NilLauncher(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallAbsent,
		RuntimeState: MCPRuntimeDisabled,
	}

	lc := &MCPLifecycle{
		provisioner:   &mockProvisioner{},
		installer:     &mockInstaller{remoteRevision: &MCPRevision{RevisionID: "rev-remote"}},
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	binding := MCPBinding{ID: "b1"}
	err := lc.Install(context.Background(), binding, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.ActiveRevisionID != "rev-remote" {
		t.Errorf("expected remote revision, got %q", inst.ActiveRevisionID)
	}
}

func TestMCPInstallation_IsInstalled(t *testing.T) {
	inst := MCPInstallation{InstallState: MCPInstallInstalled}
	if !inst.IsInstalled() {
		t.Error("expected IsInstalled()=true")
	}
	inst.InstallState = MCPInstallFailed
	if inst.IsInstalled() {
		t.Error("expected IsInstalled()=false for failed state")
	}
}

func TestMCPInstallation_IsReady(t *testing.T) {
	inst := MCPInstallation{InstallState: MCPInstallInstalled, RuntimeState: MCPRuntimeReady}
	if !inst.IsReady() {
		t.Error("expected IsReady()=true")
	}
	inst.RuntimeState = MCPRuntimeStopped
	if inst.IsReady() {
		t.Error("expected IsReady()=false when not ready")
	}
}

func TestMCPInstallation_IsOperationInProgress(t *testing.T) {
	tests := []struct {
		state MCPInstallState
		want  bool
	}{
		{MCPInstallInstalling, true},
		{MCPInstallUpgrading, true},
		{MCPInstallRollingBack, true},
		{MCPInstallUninstalling, true},
		{MCPInstallInstalled, false},
		{MCPInstallFailed, false},
		{MCPInstallAbsent, false},
	}

	for _, tt := range tests {
		inst := MCPInstallation{InstallState: tt.state}
		if got := inst.IsOperationInProgress(); got != tt.want {
			t.Errorf("IsOperationInProgress(%s)=%v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestMCPInstallation_CanStart(t *testing.T) {
	inst := MCPInstallation{InstallState: MCPInstallInstalled, Enabled: true}
	if !inst.CanStart() {
		t.Error("expected CanStart()=true")
	}
	inst.Enabled = false
	if inst.CanStart() {
		t.Error("expected CanStart()=false when disabled")
	}
}

func TestMCPInstallation_CanEnable(t *testing.T) {
	inst := MCPInstallation{InstallState: MCPInstallInstalled, Enabled: false}
	if !inst.CanEnable() {
		t.Error("expected CanEnable()=true")
	}
	inst.Enabled = true
	if inst.CanEnable() {
		t.Error("expected CanEnable()=false when already enabled")
	}
}

func TestMCPInstallation_CanDisable(t *testing.T) {
	tests := []struct {
		runtime MCPRuntimeState
		enabled bool
		want    bool
	}{
		{MCPRuntimeReady, true, true},
		{MCPRuntimeStopped, true, true},
		{MCPRuntimeDegraded, true, true},
		{MCPRuntimeReady, false, false},
		{MCPRuntimeStarting, true, false},
	}

	for _, tt := range tests {
		inst := MCPInstallation{Enabled: tt.enabled, RuntimeState: tt.runtime}
		if got := inst.CanDisable(); got != tt.want {
			t.Errorf("CanDisable(enabled=%v, runtime=%s)=%v, want %v", tt.enabled, tt.runtime, got, tt.want)
		}
	}
}

func TestMCPInstallation_CanRollback(t *testing.T) {
	inst := MCPInstallation{InstallState: MCPInstallInstalled, PreviousRevisionID: "rev-1"}
	if !inst.CanRollback() {
		t.Error("expected CanRollback()=true")
	}
	inst.PreviousRevisionID = ""
	if inst.CanRollback() {
		t.Error("expected CanRollback()=false when no previous revision")
	}
	inst.PreviousRevisionID = "rev-1"
	inst.InstallState = MCPInstallUpgrading
	if inst.CanRollback() {
		t.Error("expected CanRollback()=false during upgrade")
	}
}

func TestMCPInstallation_ValidateTransition(t *testing.T) {
	inst := MCPInstallation{InstallState: MCPInstallInstalling}
	if err := inst.ValidateTransition(MCPInstallInstalled); err != nil {
		t.Errorf("expected installing->installed to be valid: %v", err)
	}
	if err := inst.ValidateTransition(MCPInstallUpgrading); err == nil {
		t.Error("expected installing->upgrading to be invalid")
	}
}

func TestMCPLifecycle_Stop_InvalidTransition(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallInstalled,
		Enabled:      true,
		RuntimeState: MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	err := lc.Stop("b1")
	if err == nil {
		t.Error("expected error when stopping already-stopped binding")
	}
}

func TestMCPLifecycle_MarkReady_InvalidTransition(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallInstalled,
		Enabled:      true,
		RuntimeState: MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	err := lc.MarkReady("b1")
	if err == nil {
		t.Error("expected error when marking ready from stopped state")
	}
}

func TestMCPLifecycle_Stop_MarkStopped_FullCycle(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallInstalled,
		Enabled:      true,
		RuntimeState: MCPRuntimeReady,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	if err := lc.Stop("b1"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if inst.RuntimeState != MCPRuntimeStopping {
		t.Errorf("expected stopping, got %q", inst.RuntimeState)
	}

	if err := lc.MarkStopped("b1"); err != nil {
		t.Fatalf("mark stopped: %v", err)
	}
	if inst.RuntimeState != MCPRuntimeStopped {
		t.Errorf("expected stopped, got %q", inst.RuntimeState)
	}
}

func TestMCPLifecycle_PreviewInstall_NilProvisioner(t *testing.T) {
	lc := NewMCPLifecycle(nil, nil)
	_, err := lc.PreviewInstall(context.Background(), MCPBinding{ID: "b1"})
	if err == nil {
		t.Error("expected error with nil provisioner")
	}
}

func TestMCPLifecycle_Rollback_InvalidatesLockedState(t *testing.T) {
	inst := &MCPInstallation{
		BindingID:          "b1",
		InstallState:       MCPInstallRollingBack,
		ActiveRevisionID:   "rev-2",
		PreviousRevisionID: "rev-1",
		Enabled:            true,
		RuntimeState:       MCPRuntimeStopped,
	}

	lc := &MCPLifecycle{
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	err := lc.Rollback("b1")
	if err == nil {
		t.Error("expected error when rolling back from rolling_back state")
	}
}

func TestMCPLifecycle_Install_NilInstaller(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:    "plan-1",
		BindingID: "b1",
	}
	plan.PlanDigest = plan.ComputeDigest()

	inst := &MCPInstallation{
		BindingID:    "b1",
		InstallState: MCPInstallAbsent,
		RuntimeState: MCPRuntimeDisabled,
	}

	lc := &MCPLifecycle{
		provisioner:   &mockProvisioner{},
		installer:     nil,
		installations: map[string]*MCPInstallation{"b1": inst},
		operations:    map[string]string{},
		locks:         map[string]bool{},
	}

	binding := MCPBinding{ID: "b1", Launcher: &MCPLauncherSpec{Kind: "npx"}}
	err := lc.Install(context.Background(), binding, plan)
	if err == nil {
		t.Error("expected error when installer is nil")
	}
	if inst.InstallState != MCPInstallFailed {
		t.Errorf("expected failed state, got %q", inst.InstallState)
	}
}

func TestMCPLifecycle_TimestampUpdates(t *testing.T) {
	before := time.Now().Add(-time.Second)
	lc := NewMCPLifecycle(nil, nil)
	inst, _ := lc.RegisterBinding(MCPBinding{ID: "b1"})

	if inst.CreatedAt.Before(before) {
		t.Error("expected CreatedAt to be set")
	}

	oldUpdated := inst.UpdatedAt
	time.Sleep(time.Millisecond)
	inst.Enabled = true
	inst.UpdatedAt = time.Now()
	if !inst.UpdatedAt.After(oldUpdated) {
		t.Error("expected UpdatedAt to change after manual update")
	}
}
