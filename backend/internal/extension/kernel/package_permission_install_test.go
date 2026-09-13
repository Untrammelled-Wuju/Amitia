package kernel

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v1"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
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
			PermissionID: "proactive.dispatch",
			Scope:        permission.ScopeForExtension(string(extID)),
		}},
	}
	before := container.PermissionBroker.Evaluate(ctx, request)
	if before.Decision != permission.DecisionDeny {
		t.Fatalf("expected deny before install grant, got %s", before.Decision)
	}

	runtime := &Runtime{container: container}
	if err := runtime.syncInstalledPackagePermissions(ctx, extID, []manifest_v1.PermissionReq{{
		Name:     "proactive.dispatch",
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
