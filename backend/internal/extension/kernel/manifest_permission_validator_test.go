package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestManifestPermissionValidator_CanonicalPass(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "gamehost.control", Required: true},
			{Name: "gamehost.channel.use", Required: true},
			{Name: "gamehost.host_api.invoke", Required: true},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %d: %v", len(issues), issues)
	}
}

func TestManifestPermissionValidator_OldControlOutputFail(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "gamehost.control.output", Required: true},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Code != "unknown_permission" {
		t.Fatalf("expected unknown_permission, got %s", issues[0].Code)
	}
}

func TestManifestPermissionValidator_OldControlRequestFail(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "gamehost.control.request", Required: true},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Code != "unknown_permission" {
		t.Fatalf("expected unknown_permission, got %s", issues[0].Code)
	}
}

func TestManifestPermissionValidator_OldChannelRegisterFail(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "gamehost.channel.register", Required: true},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Code != "unknown_permission" {
		t.Fatalf("expected unknown_permission, got %s", issues[0].Code)
	}
}

func TestManifestPermissionValidator_RuntimeUndeclaredPermissionFail(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "gamehost.channel.use", Required: true},
		},
		Modules: []manifest_v2.ModuleMeta{
			{
				ID:   "mod-1",
				Type: "javascript",
				Runtime: &manifest_v2.RuntimeMeta{
					Type:        "javascript",
					Permissions: []string{"gamehost.control"},
				},
			},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %v", len(issues), issues)
	}
	if issues[0].Code != "permission_not_declared" {
		t.Fatalf("expected permission_not_declared, got %s", issues[0].Code)
	}
}

func TestManifestPermissionValidator_ContributionUndeclaredFail(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "gamehost.host_api.invoke", Required: true},
		},
		Modules: []manifest_v2.ModuleMeta{
			{
				ID:   "mod-1",
				Type: "javascript",
				Contributions: []manifest_v2.ContributionMeta{
					{
						ID:                  "contrib-1",
						Kind:                "tool",
						RequiredPermissions: []string{"gamehost.control"},
					},
				},
			},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %v", len(issues), issues)
	}
	if issues[0].Code != "permission_not_declared" {
		t.Fatalf("expected permission_not_declared, got %s", issues[0].Code)
	}
}

func TestManifestPermissionValidator_ScopeValidation(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "gamehost.control", Required: true, Scope: "resource"},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 0 {
		t.Fatalf("expected no issues for valid scope, got %d: %v", len(issues), issues)
	}
}

func TestManifestPermissionValidator_InvalidScopeFail(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "gamehost.channel.use", Required: true, Scope: "conversation"},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %v", len(issues), issues)
	}
	if issues[0].Code != "invalid_permission_scope" {
		t.Fatalf("expected invalid_permission_scope, got %s", issues[0].Code)
	}
}

func TestManifestPermissionValidator_UnknownPermissionFail(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "whatever.foo", Required: true},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Code != "unknown_permission" {
		t.Fatalf("expected unknown_permission, got %s", issues[0].Code)
	}
}

func TestManifestPermissionValidator_ExistingNonGameHostPass(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	v := NewManifestPermissionValidator(registry)

	manifest := manifest_v2.Manifest{
		Permissions: []manifest_v2.PermissionReq{
			{Name: "character.read", Required: true},
			{Name: "provider.use", Required: true},
		},
	}

	issues := v.Validate(manifest)
	if len(issues) != 0 {
		t.Fatalf("expected no issues for valid existing permissions, got %d: %v", len(issues), issues)
	}
}
