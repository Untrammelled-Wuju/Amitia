package kernel

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v1"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func TestInstallationPermissionPolicyUsesInstallGrant(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root := t.TempDir()
	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(root, "kernel.db")).
		WithExtensionRoot(filepath.Join(root, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer container.Close()

	extID := domain.ExtensionID("com.amitia.test/install-permission")
	subject := permission.SubjectForExtension(string(extID))
	request := permission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []permission.PermissionRequirement{{
			PermissionID: "message.send",
			Scope:        permission.ScopeForExtension(string(extID)),
		}},
	}
	before := container.PermissionBroker.Evaluate(ctx, request)
	if before.Decision != permission.DecisionDeny {
		t.Fatalf("expected deny before install grant, got %s", before.Decision)
	}

	runtime := &Runtime{container: container}
	if err := runtime.syncInstalledPackagePermissions(ctx, extID, []manifest_v1.PermissionReq{{
		Name:     "message.send",
		Required: true,
		Scope:    string(permission.ScopeExtension),
	}}); err != nil {
		t.Fatalf("syncInstalledPackagePermissions: %v", err)
	}

	after := container.PermissionBroker.Evaluate(ctx, request)
	if after.Decision != permission.DecisionAllow {
		t.Fatalf("expected allow after install grant, got %s missing=%v", after.Decision, after.Missing)
	}
}

func TestInstallationPermissionPolicyOverridesPerUseAndPreservesRevokedGrant(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root := t.TempDir()
	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(root, "kernel.db")).
		WithExtensionRoot(filepath.Join(root, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer container.Close()

	extID := domain.ExtensionID("com.amitia.test/permission-toggle")
	requirements := []sqlite.PermissionRequirement{{
		ExtensionID:    extID,
		PermissionName: "resource.write",
		Required:       true,
		Scope:          string(permission.ScopeExtension),
	}}
	request := permission.PermissionEvaluationRequest{
		Subject: permission.SubjectForExtension(string(extID)),
		Requirements: []permission.PermissionRequirement{{
			PermissionID: "resource.write",
			Scope:        permission.ScopeForExtension(string(extID)),
		}},
	}
	runtime := &Runtime{container: container}
	if err := runtime.syncInstalledPackagePermissions(ctx, extID, []manifest_v1.PermissionReq{{
		Name:     "resource.write",
		Required: true,
		Scope:    string(permission.ScopeExtension),
	}}); err != nil {
		t.Fatalf("syncInstalledPackagePermissions: %v", err)
	}
	if result := container.PermissionBroker.Evaluate(ctx, request); result.Decision != permission.DecisionAllow {
		t.Fatalf("expected install grant to override per-use approval, got %s missing=%v", result.Decision, result.Missing)
	}
	if err := container.PermissionRepository.PutGrant(ctx, sqlite.PermissionGrant{
		ExtensionID:    extID,
		PermissionName: "resource.write",
		State:          "revoked",
	}); err != nil {
		t.Fatalf("PutGrant revoked: %v", err)
	}
	if result := container.PermissionBroker.Evaluate(ctx, request); result.Decision != permission.DecisionDeny {
		t.Fatalf("expected revoked grant to deny, got %s", result.Decision)
	}
	if err := restoreInstalledPackagePermissions(ctx, container.PermissionRepository, extID, requirements); err != nil {
		t.Fatalf("restoreInstalledPackagePermissions: %v", err)
	}
	if result := container.PermissionBroker.Evaluate(ctx, request); result.Decision != permission.DecisionDeny {
		t.Fatalf("expected restored revoked grant to remain denied, got %s", result.Decision)
	}
	if err := container.PermissionRepository.PutGrant(ctx, sqlite.PermissionGrant{
		ExtensionID:    extID,
		PermissionName: "resource.write",
		State:          "granted",
	}); err != nil {
		t.Fatalf("PutGrant granted: %v", err)
	}
	if result := container.PermissionBroker.Evaluate(ctx, request); result.Decision != permission.DecisionAllow {
		t.Fatalf("expected re-enabled grant to allow, got %s", result.Decision)
	}
}
