package host_api

import (
	"reflect"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestRegisterPermissionDefinitionsPreservesCanonicalMetadata(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	before, ok := registry.Get("character.read")
	if !ok {
		t.Fatal("character.read builtin missing")
	}
	RegisterPermissionDefinitions(registry)
	after, ok := registry.Get("character.read")
	if !ok {
		t.Fatal("character.read missing after Host API registration")
	}
	if before.Name != after.Name || before.Description != after.Description || before.Category != after.Category || before.RiskLevel != after.RiskLevel || before.DefaultApproval != after.DefaultApproval || before.ChildInvocation != after.ChildInvocation {
		t.Fatalf("canonical metadata was overwritten: before=%+v after=%+v", before, after)
	}
	if !reflect.DeepEqual(before.AllowedScopes, after.AllowedScopes) {
		t.Fatalf("canonical scopes unexpectedly narrowed/changed: before=%v after=%v", before.AllowedScopes, after.AllowedScopes)
	}
}

func TestRegisterPermissionDefinitionsNeverCreatesSparseDefinitions(t *testing.T) {
	registry := permission.NewPermissionDefinitionRegistry()
	RegisterPermissionDefinitions(registry)
	seen := map[string]struct{}{}
	for _, entries := range DefaultPermissionMapping() {
		for _, entry := range entries {
			if entry.PermissionID == "" {
				continue
			}
			if _, done := seen[entry.PermissionID]; done {
				continue
			}
			seen[entry.PermissionID] = struct{}{}
			def, ok := registry.Get(entry.PermissionID)
			if !ok {
				t.Fatalf("mapped permission %q is not registered", entry.PermissionID)
			}
			if def.Name == "" || def.Description == "" || def.Category == "" || def.RiskLevel == "" || def.DefaultApproval == "" || def.ChildInvocation == "" || len(def.AllowedScopes) == 0 {
				t.Fatalf("mapped permission %q has incomplete governance metadata: %+v", entry.PermissionID, def)
			}
		}
	}
}
