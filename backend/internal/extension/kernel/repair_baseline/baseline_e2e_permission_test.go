package repair_baseline

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestBaseline_E2E_Permission_UngrantedMustDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E permission test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	subject := permission.SubjectForExtension("com.amitia.repair/tool-permission-denied")
	req := permission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []permission.PermissionRequirement{
			{PermissionID: "files.read"},
		},
	}
	result := container.PermissionBroker.Evaluate(ctx, req)
	if result.Decision == permission.DecisionAllow || result.Decision == permission.DecisionAllowOnce || result.Decision == permission.DecisionAllowSession || result.Decision == permission.DecisionAllowPersistent {
		t.Fatalf("ungranted permission must not be allowed (Phase 10 section 19.3.1-19.3.2), got %s; missing=%v", result.Decision, result.Missing)
	}
}

func TestBaseline_E2E_Permission_GrantThenAllow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E permission test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	subject := permission.SubjectForExtension("com.amitia.repair/tool-permission-denied")
	grant, err := container.PermissionBroker.Grant(ctx, permission.PermissionGrantRequest{
		Subject:      subject,
		PermissionID: "files.read",
		Scope:        permission.ScopeGlobalOnly(),
		IssuedBy:     permission.IssuerUser,
		Decision:     permission.DecisionAllowPersistent,
	})
	if err != nil {
		t.Fatalf("grant must succeed: %v", err)
	}

	req := permission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []permission.PermissionRequirement{
			{PermissionID: "files.read"},
		},
	}
	result := container.PermissionBroker.Evaluate(ctx, req)
	if result.Decision == permission.DecisionDeny {
		t.Fatalf("granted permission must not be denied (Phase 10 section 19.3.3-19.3.6), got %s; grant=%v", result.Decision, grant)
	}
}

func TestBaseline_E2E_Permission_RevokeThenDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E permission test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	subject := permission.SubjectForExtension("com.amitia.repair/tool-permission-denied")
	grant, err := container.PermissionBroker.Grant(ctx, permission.PermissionGrantRequest{
		Subject:      subject,
		PermissionID: "files.read",
		Scope:        permission.ScopeGlobalOnly(),
		IssuedBy:     permission.IssuerUser,
		Decision:     permission.DecisionAllowPersistent,
	})
	if err != nil {
		t.Fatalf("grant must succeed: %v", err)
	}

	if err := container.PermissionBroker.Revoke(ctx, grant.GrantID); err != nil {
		t.Fatalf("revoke must succeed: %v", err)
	}

	req := permission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []permission.PermissionRequirement{
			{PermissionID: "files.read"},
		},
	}
	result := container.PermissionBroker.Evaluate(ctx, req)
	if result.Decision == permission.DecisionAllow || result.Decision == permission.DecisionAllowOnce || result.Decision == permission.DecisionAllowSession || result.Decision == permission.DecisionAllowPersistent {
		t.Fatalf("revoked permission must not be allowed (Phase 10 section 19.3.8-19.3.9), got %s", result.Decision)
	}
}

func TestBaseline_E2E_Permission_UnknownPermissionFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E permission test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	subject := permission.SubjectForExtension("com.amitia.repair/tool-permission-denied")
	req := permission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []permission.PermissionRequirement{
			{PermissionID: "nonexistent.permission"},
		},
	}
	result := container.PermissionBroker.Evaluate(ctx, req)
	if result.Decision != permission.DecisionDeny {
		t.Fatalf("unknown permission must be denied (fail closed), got %s", result.Decision)
	}
}

func TestBaseline_E2E_Permission_NoRequirementsAllows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E permission test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	subject := permission.SubjectForExtension("com.amitia.repair/tool-basic")
	req := permission.PermissionEvaluationRequest{
		Subject:      subject,
		Requirements: []permission.PermissionRequirement{},
	}
	result := container.PermissionBroker.Evaluate(ctx, req)
	if result.Decision != permission.DecisionAllow {
		t.Fatalf("no requirements must allow, got %s", result.Decision)
	}
}

var _ capability.RiskLevel
