package permission

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGameHostPermissionIDs_ExactlyThree(t *testing.T) {
	ids := GameHostPermissionIDs()
	if len(ids) != 3 {
		t.Fatalf("expected exactly 3 GameHost permission IDs, got %d: %v", len(ids), ids)
	}
}

func TestGameHostPermissionIDs_CanonicalValues(t *testing.T) {
	ids := GameHostPermissionIDs()
	expected := map[string]bool{
		"gamehost.control":         false,
		"gamehost.channel.use":     false,
		"gamehost.host_api.invoke": false,
	}
	for _, id := range ids {
		if _, ok := expected[id]; !ok {
			t.Fatalf("unexpected GameHost permission ID: %s", id)
		}
		expected[id] = true
	}
	for id, found := range expected {
		if !found {
			t.Fatalf("missing canonical GameHost permission ID: %s", id)
		}
	}
}

func TestGameHostPermissionIDs_MatchRegistry(t *testing.T) {
	registry := NewPermissionDefinitionRegistry()
	ids := GameHostPermissionIDs()
	for _, id := range ids {
		if _, ok := registry.Get(id); !ok {
			t.Fatalf("GameHost permission ID %s not found in PermissionDefinitionRegistry", id)
		}
	}
}

func TestGameHostPermissionIDs_MatchGoldenFixture(t *testing.T) {
	data, err := os.ReadFile("../../../../pkg/gameplugin/conformance/fixtures/permission_contract.json")
	if err != nil {
		t.Fatalf("read golden fixture: %v", err)
	}
	var fixture struct {
		GameHostPermissions []string `json:"gameHostPermissions"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse golden fixture: %v", err)
	}
	ids := GameHostPermissionIDs()
	if len(ids) != len(fixture.GameHostPermissions) {
		t.Fatalf("GameHost permission count mismatch: got %d, golden %d", len(ids), len(fixture.GameHostPermissions))
	}
	for i, id := range ids {
		if id != fixture.GameHostPermissions[i] {
			t.Fatalf("GameHost permission mismatch at index %d: got %s, golden %s", i, id, fixture.GameHostPermissions[i])
		}
	}
}

func TestGameHostPermission_OldIDsNotExist(t *testing.T) {
	registry := NewPermissionDefinitionRegistry()
	oldIDs := []string{
		"gamehost.control.request",
		"gamehost.control.output",
		"gamehost.channel.register",
	}
	for _, id := range oldIDs {
		if _, ok := registry.Get(id); ok {
			t.Fatalf("old invalid GameHost permission ID %s should not exist in registry", id)
		}
	}
}
