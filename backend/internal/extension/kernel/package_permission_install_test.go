package kernel

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
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
	if err := runtime.syncInstalledPackagePermissions(ctx, extID, []plugincontract.Permission{{
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

func TestInstallationPermissionPolicyUsesBuiltInInstallGrant(t *testing.T) {
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

	extID := domain.ExtensionID("com.amitia.builtin.test-permission")
	if err := container.PermissionRepository.PutRequirement(ctx, sqlite.PermissionRequirement{
		ExtensionID:    extID,
		PermissionName: "tool.invoke",
		Required:       true,
		Scope:          string(permission.ScopeExtension),
	}); err != nil {
		t.Fatalf("PutRequirement: %v", err)
	}
	if err := container.PermissionRepository.PutGrant(ctx, sqlite.PermissionGrant{
		ExtensionID:    extID,
		PermissionName: "tool.invoke",
		State:          "granted",
	}); err != nil {
		t.Fatalf("PutGrant: %v", err)
	}
	granted, err := container.PermissionRepository.IsGranted(ctx, extID, "tool.invoke")
	if err != nil {
		t.Fatalf("IsGranted: %v", err)
	}
	if !granted {
		t.Fatal("built-in tool.invoke grant was not persisted")
	}
	request := permission.PermissionEvaluationRequest{
		Subject: permission.SubjectForTool(string(extID), "tool.invoke"),
		Requirements: []permission.PermissionRequirement{{
			PermissionID: "tool.invoke",
			Scope:        permission.ScopeForExtension(string(extID)),
		}},
	}
	result := container.PermissionBroker.Evaluate(ctx, request)
	if result.Decision != permission.DecisionAllow {
		t.Fatalf("expected built-in install grant to allow, got %s missing=%v", result.Decision, result.Missing)
	}
}

func TestInstallationPermissionPolicySkipsUndeclaredBuiltInToolPermission(t *testing.T) {
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

	extID := domain.ExtensionID("com.amitia.builtin.workspace")
	request := permission.PermissionEvaluationRequest{
		Subject:      permission.SubjectForTool(string(extID), "workspace_write"),
		ApprovalMode: string(permission.ApprovalFullControl),
		Requirements: []permission.PermissionRequirement{{
			PermissionID: "workspace.write",
			Scope:        permission.ScopeGlobalOnly(),
		}},
	}
	result := container.PermissionBroker.Evaluate(ctx, request)
	if result.Decision != permission.DecisionAllow {
		t.Fatalf("undeclared built-in tool permission should use normal policy, got %s reasons=%v", result.Decision, result.Reasons)
	}
}

func TestInstallationPermissionPolicyRejectsBuiltInNamespaceSpoof(t *testing.T) {
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

	extID := domain.ExtensionID("com.amitia.builtin.namespace-spoof")
	version := domain.SemanticVersion{Major: 1}
	if err := container.DefinitionRepository.PutExtension(ctx, domain.ExtensionDefinition{
		ID:              extID,
		Name:            domain.LocalizedText{Default: "Spoof"},
		Version:         version,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Metadata:        map[string]any{},
		Modules: []domain.ModuleDefinition{{
			ID:          "main",
			ExtensionID: extID,
			Name:        domain.LocalizedText{Default: "Main"},
			Type:        domain.ModuleTypeService,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := container.InstallationRepository.PutInstallation(ctx, domain.ExtensionInstallation{
		InstallationID:    "spoof-install",
		ExtensionID:       extID,
		InstalledVersion:  version,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementDisabled,
	}); err != nil {
		t.Fatal(err)
	}
	request := permission.PermissionEvaluationRequest{
		Subject:      permission.SubjectForTool(string(extID), "spoof-tool"),
		ApprovalMode: string(permission.ApprovalFullControl),
		Requirements: []permission.PermissionRequirement{{
			PermissionID: "undeclared.spoof",
			Scope:        permission.ScopeForExtension(string(extID)),
		}},
	}
	if result := container.PermissionBroker.Evaluate(ctx, request); result.Decision != permission.DecisionDeny {
		t.Fatalf("builtin namespace spoof bypassed installation permission policy: %s", result.Decision)
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
	if err := runtime.syncInstalledPackagePermissions(ctx, extID, []plugincontract.Permission{{
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

func TestRestorePackagePermissionsAddsServiceRuntimeExecute(t *testing.T) {
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

	extID := domain.ExtensionID("com.amitiax/runtime-permission")
	if err := container.PermissionRepository.PutRequirement(ctx, sqlite.PermissionRequirement{
		ExtensionID:    extID,
		PermissionName: permission.PermissionServiceNetworkRequest,
		Required:       true,
		Scope:          string(permission.ScopeExtension),
	}); err != nil {
		t.Fatal(err)
	}
	definition := domain.ExtensionDefinition{
		ID: extID,
		Modules: []domain.ModuleDefinition{{
			ID:      "runtime",
			Runtime: &domain.RuntimeDefinition{Type: domain.RuntimeTypeJavaScript},
		}},
	}
	if err := restorePackagePermissionsFromDefinition(ctx, container.PermissionRepository, extID, definition); err != nil {
		t.Fatal(err)
	}
	requirements, err := container.PermissionRepository.ListRequirements(ctx, extID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, requirement := range requirements {
		if requirement.PermissionName == permission.PermissionServiceRuntimeExecute {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("runtime execute requirement not restored: %+v", requirements)
	}
	granted, err := container.PermissionRepository.IsGranted(ctx, extID, permission.PermissionServiceRuntimeExecute)
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("runtime execute permission was not granted during restore")
	}
}

func TestRestorePackagePermissionsAddsUIToolInvoke(t *testing.T) {
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

	extID := domain.ExtensionID("com.amitiax/ui-tool-permission")
	definition := domain.ExtensionDefinition{
		ID: extID,
		Modules: []domain.ModuleDefinition{{
			ID: "ui",
			Contributions: []domain.ContributionDefinition{{
				ID:              "dashboard",
				Kind:            domain.ContributionKindUIPage,
				Adapter:         domain.ContributionAdapterUI,
				ContractVersion: 1,
				Definition: map[string]any{
					"actions": []any{
						map[string]any{
							"target": map[string]any{"type": "tool"},
						},
					},
				},
			}},
		}},
	}
	if err := restorePackagePermissionsFromDefinition(ctx, container.PermissionRepository, extID, definition); err != nil {
		t.Fatal(err)
	}
	granted, err := container.PermissionRepository.IsGranted(ctx, extID, "tool.invoke")
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("tool invoke permission was not granted during restore")
	}
}
