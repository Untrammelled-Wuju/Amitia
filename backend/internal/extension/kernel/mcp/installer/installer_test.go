// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package installer

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/mcp"
)

func TestNPXInstaller_Install_ValidPlan(t *testing.T) {
	inst := NewNPXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "npx", Command: "mcp-server"},
	}

	rev, err := inst.Install(context.Background(), plan, binding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev == nil {
		t.Fatal("expected non-nil revision")
	}
	if rev.BindingID != "b1" {
		t.Errorf("expected binding ID 'b1', got %q", rev.BindingID)
	}
	if rev.LauncherKind != "npx" {
		t.Errorf("expected launcher kind 'npx', got %q", rev.LauncherKind)
	}
	if !strings.HasPrefix(rev.RevisionID, "rev_") {
		t.Errorf("expected revision ID to start with 'rev_', got %q", rev.RevisionID)
	}
	if !strings.HasPrefix(rev.InstallRootURI, "extensions/mcp/b1/revisions/") {
		t.Errorf("unexpected install root URI: %q", rev.InstallRootURI)
	}
}

func TestNPXInstaller_Install_PlanDigestMismatch(t *testing.T) {
	inst := NewNPXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
		PlanDigest:       "tampered",
	}

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "npx", Command: "mcp-server"},
	}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for tampered plan digest")
	}
}

func TestNPXInstaller_Install_InvalidVersion(t *testing.T) {
	inst := NewNPXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "^1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "npx", Command: "mcp-server"},
	}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for unpinned version with ^")
	}
}

func TestNPXInstaller_Install_EmptyPackage(t *testing.T) {
	inst := NewNPXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "npx"},
	}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for empty package name")
	}
}

func TestNPXInstaller_Install_WildcardVersion(t *testing.T) {
	inst := NewNPXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "*",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "npx"},
	}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for wildcard version")
	}
}

func TestUVXInstaller_Install_ValidPlan(t *testing.T) {
	inst := NewUVXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "uvx", Command: "mcp-server"},
	}

	rev, err := inst.Install(context.Background(), plan, binding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev.LauncherKind != "uvx" {
		t.Errorf("expected launcher kind 'uvx', got %q", rev.LauncherKind)
	}
	if rev.PackageManager != "uv" {
		t.Errorf("expected package manager 'uv', got %q", rev.PackageManager)
	}
	if !strings.HasPrefix(rev.InstallRootURI, "extensions/mcp/b1/revisions/") {
		t.Errorf("unexpected install root URI: %q", rev.InstallRootURI)
	}
}

func TestUVXInstaller_Install_RangeVersion(t *testing.T) {
	inst := NewUVXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: ">=1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "uvx"},
	}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for range version")
	}
}

func TestUVXInstaller_Install_PlanDigestMismatch(t *testing.T) {
	inst := NewUVXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
		PlanDigest:       "wrong",
	}

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "uvx"},
	}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for tampered plan digest")
	}
}

func TestExecutableInstaller_Install_ValidPlan(t *testing.T) {
	inst := NewExecutableInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "",
		RequestedVersion: "",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "executable", Command: "/usr/bin/my-server"},
	}

	rev, err := inst.Install(context.Background(), plan, binding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev.LauncherKind != "executable" {
		t.Errorf("expected launcher kind 'executable', got %q", rev.LauncherKind)
	}
	if rev.EntryPoint != "/usr/bin/my-server" {
		t.Errorf("expected entry point '/usr/bin/my-server', got %q", rev.EntryPoint)
	}
	if rev.PackageManager != "" {
		t.Errorf("expected empty package manager for executable, got %q", rev.PackageManager)
	}
	if rev.InstallRootURI != "" {
		t.Errorf("expected empty install root URI for executable, got %q", rev.InstallRootURI)
	}
}

func TestExecutableInstaller_Install_MissingCommand(t *testing.T) {
	inst := NewExecutableInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:    "plan-1",
		BindingID: "b1",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "executable", Command: ""},
	}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestExecutableInstaller_Install_NilLauncher(t *testing.T) {
	inst := NewExecutableInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:    "plan-1",
		BindingID: "b1",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{ID: "b1"}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for nil launcher")
	}
}

func TestExecutableInstaller_Install_PlanDigestMismatch(t *testing.T) {
	inst := NewExecutableInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:     "plan-1",
		BindingID:  "b1",
		PlanDigest: "wrong",
	}

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "executable", Command: "/usr/bin/server"},
	}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for tampered plan digest")
	}
}

func TestRemoteInstaller_Install_ValidPlan(t *testing.T) {
	inst := NewRemoteInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:    "plan-1",
		BindingID: "b1",
		Source:    "https://mcp.example.com/sse",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{ID: "b1"}

	rev, err := inst.Install(context.Background(), plan, binding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rev.LauncherKind != "remote" {
		t.Errorf("expected launcher kind 'remote', got %q", rev.LauncherKind)
	}
	if rev.PackageManager != "" {
		t.Errorf("expected empty package manager for remote, got %q", rev.PackageManager)
	}
	if rev.EntryPoint != "" {
		t.Errorf("expected empty entry point for remote, got %q", rev.EntryPoint)
	}
}

func TestRemoteInstaller_Install_PlanDigestMismatch(t *testing.T) {
	inst := NewRemoteInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:     "plan-1",
		BindingID:  "b1",
		PlanDigest: "wrong",
	}

	binding := mcp.MCPBinding{ID: "b1"}

	_, err := inst.Install(context.Background(), plan, binding)
	if err == nil {
		t.Error("expected error for tampered plan digest")
	}
}

func TestValidateNPXPlan_Valid(t *testing.T) {
	plan := mcp.MCPInstallPlan{
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.2.3",
	}
	if err := ValidateNPXPlan(plan); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateNPXPlan_EmptyPackage(t *testing.T) {
	plan := mcp.MCPInstallPlan{
		RequestedPackage: "",
		RequestedVersion: "1.0.0",
	}
	if err := ValidateNPXPlan(plan); err == nil {
		t.Error("expected error for empty package")
	}
}

func TestValidateNPXPlan_UnpinnedVersions(t *testing.T) {
	versions := []string{"*", "^1.0.0", "~1.0.0", "latest"}
	for _, ver := range versions {
		plan := mcp.MCPInstallPlan{
			RequestedPackage: "mcp-server",
			RequestedVersion: ver,
		}
		if err := ValidateNPXPlan(plan); err == nil {
			t.Errorf("expected error for version %q", ver)
		}
	}
}

func TestValidateUVXPlan_Valid(t *testing.T) {
	plan := mcp.MCPInstallPlan{
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.2.3",
	}
	if err := ValidateUVXPlan(plan); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateUVXPlan_RangeVersions(t *testing.T) {
	versions := []string{"*", "^1.0.0", "~1.0.0", ">=1.0.0", "<2.0.0", ">1.0.0"}
	for _, ver := range versions {
		plan := mcp.MCPInstallPlan{
			RequestedPackage: "mcp-server",
			RequestedVersion: ver,
		}
		if err := ValidateUVXPlan(plan); err == nil {
			t.Errorf("expected error for version %q", ver)
		}
	}
}

func TestValidateUVXPlan_EmptyVersion(t *testing.T) {
	plan := mcp.MCPInstallPlan{
		RequestedPackage: "mcp-server",
		RequestedVersion: "",
	}
	if err := ValidateUVXPlan(plan); err == nil {
		t.Error("expected error for empty version")
	}
}

func TestGenerateRevisionID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateRevisionID()
		if ids[id] {
			t.Errorf("duplicate revision ID: %s", id)
		}
		ids[id] = true
		if !strings.HasPrefix(id, "rev_") {
			t.Errorf("expected prefix 'rev_', got %q", id)
		}
	}
}

func TestGenerateRevisionID_DifferentInstances(t *testing.T) {
	id1 := GenerateRevisionID()
	id2 := GenerateRevisionID()
	if id1 == id2 {
		t.Error("expected different revision IDs")
	}
}

func TestNPXInstaller_ThreadSafety(t *testing.T) {
	inst := NewNPXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "npx", Command: "mcp-server"},
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = inst.Install(context.Background(), plan, binding)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestUVXInstaller_ThreadSafety(t *testing.T) {
	inst := NewUVXInstaller()
	plan := mcp.MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	binding := mcp.MCPBinding{
		ID:       "b1",
		Launcher: &mcp.MCPLauncherSpec{Kind: "uvx", Command: "mcp-server"},
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = inst.Install(context.Background(), plan, binding)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
