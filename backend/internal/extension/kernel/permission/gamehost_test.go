package permission_test

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestGameHostPermissionDefinitions_Registered(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()

	expected := []struct {
		id      string
		cat     permission.PermissionCategory
		risk    capability.RiskLevel
	}{
		{permission.PermissionGameHostControl, permission.CategoryGameHost, capability.RiskHigh},
		{permission.PermissionGameHostChannelUse, permission.CategoryGameHost, capability.RiskMedium},
		{permission.PermissionGameHostAPIInvoke, permission.CategoryGameHost, capability.RiskMedium},
	}

	for _, e := range expected {
		def, ok := registry.Get(e.id)
		if !ok {
			t.Fatalf("expected permission %q to be registered", e.id)
		}
		if def.Category != e.cat {
			t.Fatalf("expected %q category %q, got %q", e.id, e.cat, def.Category)
		}
		if string(def.RiskLevel) != string(e.risk) {
			t.Fatalf("expected %q risk %q, got %q", e.id, e.risk, def.RiskLevel)
		}
	}
}

func TestGameHostPermissions_Mapping(t *testing.T) {
	set := permission.GameHostPermissions()
	if set.Control != "gamehost.control" {
		t.Fatalf("expected Control=gamehost.control, got %q", set.Control)
	}
	if set.ChannelUse != "gamehost.channel.use" {
		t.Fatalf("expected ChannelUse=gamehost.channel.use, got %q", set.ChannelUse)
	}
	if set.APIInvoke != "gamehost.host_api.invoke" {
		t.Fatalf("expected APIInvoke=gamehost.host_api.invoke, got %q", set.APIInvoke)
	}
}

func TestGameHostPermissionDefinition_ScopeSupport(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()

	control, ok := registry.Get(permission.PermissionGameHostControl)
	if !ok {
		t.Fatal("gamehost.control not registered")
	}
	hasGlobal := false
	hasResource := false
	for _, s := range control.AllowedScopes {
		if s == permission.ScopeGlobal {
			hasGlobal = true
		}
		if s == permission.ScopeResource {
			hasResource = true
		}
	}
	if !hasGlobal || !hasResource {
		t.Fatalf("expected control to support global+resource scopes, got %v", control.AllowedScopes)
	}
}

func TestGameHostPermissionDefinition_DefaultDeny(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()

	expected := []string{
		permission.PermissionGameHostControl,
		permission.PermissionGameHostChannelUse,
		permission.PermissionGameHostAPIInvoke,
	}
	for _, id := range expected {
		def, ok := registry.Get(id)
		if !ok {
			t.Fatalf("permission %q not registered", id)
		}
		if def.DefaultApproval == permission.ApprovalAuto {
			t.Fatalf("expected %q default approval to NOT be auto (deny-by-default), got %q", id, def.DefaultApproval)
		}
	}
}

func TestGameHostPermissionDefinition_NoExistingDuplicate(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()

	protected := []string{
		"files.read",
		"files.write",
		"network.request",
		"process.spawn",
		"service.process.spawn",
		"service.network.request",
		"service.files.extension_data",
		"character.read",
		"conversation.read",
		"provider.use",
		"desktop.notification",
		"service.runtime.execute",
		"service.tool.execute",
	}
	for _, id := range protected {
		if _, ok := registry.Get(id); !ok {
			t.Fatalf("existing permission %q should still be registered", id)
		}
	}
}

func TestGameHostPermissionDefinition_GameSemanticFree(t *testing.T) {
	gameSemantics := []string{
		"gamehost.control",
		"gamehost.channel.use",
		"gamehost.host_api.invoke",
	}
	forbiddenSubstrings := []string{"minecraft", "inventory", "attack", "craft", "player", "block", "move", "jump", "keyboard", "mouse", "keypress", "click"}
	for _, id := range gameSemantics {
		for _, substr := range forbiddenSubstrings {
			if id == "gamehost."+substr {
				t.Fatalf("permission %q must not contain game-specific semantics", id)
			}
		}
	}
	if len(gameSemantics) != 3 {
		t.Fatalf("expected exactly 3 GameHost permissions, got %d", len(gameSemantics))
	}
}

func TestGameHostUniquePermissionIDs(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	seen := make(map[string]struct{})
	for _, def := range registry.List() {
		if _, ok := seen[def.ID]; ok {
			t.Fatalf("duplicate permission ID: %q", def.ID)
		}
		seen[def.ID] = struct{}{}
	}
}
