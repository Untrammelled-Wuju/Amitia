// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"testing"
	"time"
)

func TestMCPInstallPlan_ComputeDigest(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		Source:           "npm",
		Transport:        "stdio",
		Launcher:         "npx",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
		Risk:             "low",
		Permissions:      []string{"fs:read", "net:outbound"},
		RuntimeDependencies: []MCPRuntimeDependency{
			{Name: "node", Version: ">=18", Required: true},
		},
		NetworkEndpoints: []string{"registry.npmjs.org"},
	}

	digest1 := plan.ComputeDigest()
	digest2 := plan.ComputeDigest()
	if digest1 != digest2 {
		t.Errorf("digest should be deterministic: %s vs %s", digest1, digest2)
	}
	if len(digest1) != 64 {
		t.Errorf("expected SHA-256 hex digest (64 chars), got %d chars", len(digest1))
	}
}

func TestMCPInstallPlan_Digest_DetectsChanges(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}

	digest := plan.ComputeDigest()
	plan.RequestedVersion = "2.0.0"
	newDigest := plan.ComputeDigest()

	if digest == newDigest {
		t.Error("digest should change when plan changes")
	}
}

func TestMCPInstallPlan_VerifyDigest_Valid(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}
	plan.PlanDigest = plan.ComputeDigest()

	if !plan.VerifyDigest() {
		t.Error("expected digest to verify correctly")
	}
}

func TestMCPInstallPlan_VerifyDigest_Invalid(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
		PlanDigest:       "tampered-digest",
	}

	if plan.VerifyDigest() {
		t.Error("expected tampered digest to fail verification")
	}
}

func TestMCPInstallPlan_VerifyDigest_Empty(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:    "plan-1",
		BindingID: "b1",
	}

	if plan.VerifyDigest() {
		t.Error("expected empty digest to fail verification")
	}
}

func TestMCPInstallPlan_IsExpired(t *testing.T) {
	plan := MCPInstallPlan{
		PlanID:    "plan-1",
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	if !plan.IsExpired(time.Now()) {
		t.Error("expected expired plan")
	}

	plan.ExpiresAt = time.Now().Add(time.Hour)
	if plan.IsExpired(time.Now()) {
		t.Error("expected non-expired plan")
	}

	plan.ExpiresAt = time.Time{}
	if plan.IsExpired(time.Now()) {
		t.Error("expected non-expired plan with zero time")
	}
}

func TestMCPInstallPlan_Digest_WithScripts(t *testing.T) {
	planNoScripts := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}

	planWithScripts := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
		InstallScripts: MCPInstallScriptPreview{
			HasScripts:  true,
			ScriptRisk:  "medium",
			ScriptTypes: []string{"postinstall"},
		},
	}

	d1 := planNoScripts.ComputeDigest()
	d2 := planWithScripts.ComputeDigest()
	if d1 == d2 {
		t.Error("digest should differ when scripts are present")
	}
}

func TestMCPInstallPlan_Digest_IncludesPermissions(t *testing.T) {
	base := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}

	withPerms := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
		Permissions:      []string{"fs:read", "fs:write"},
	}

	if base.ComputeDigest() == withPerms.ComputeDigest() {
		t.Error("digest should differ when permissions are added")
	}
}

func TestMCPInstallPlan_Digest_IncludesEndpoints(t *testing.T) {
	base := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}

	withEndpoints := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
		NetworkEndpoints: []string{"registry.npmjs.org"},
	}

	if base.ComputeDigest() == withEndpoints.ComputeDigest() {
		t.Error("digest should differ when endpoints are added")
	}
}

func TestMCPInstallPlan_Digest_IncludesDependencies(t *testing.T) {
	base := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
	}

	withDeps := MCPInstallPlan{
		PlanID:           "plan-1",
		BindingID:        "b1",
		RequestedPackage: "mcp-server",
		RequestedVersion: "1.0.0",
		RuntimeDependencies: []MCPRuntimeDependency{
			{Name: "node", Version: ">=18", Required: true, Present: true},
		},
	}

	if base.ComputeDigest() == withDeps.ComputeDigest() {
		t.Error("digest should differ when runtime dependencies are added")
	}
}
