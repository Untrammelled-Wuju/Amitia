package sdk

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGameHostPermissionConstants_CanonicalValues(t *testing.T) {
	if PermGameHostControl != "gamehost.control" {
		t.Fatalf("PermGameHostControl = %q, want %q", PermGameHostControl, "gamehost.control")
	}
	if PermGameHostChannelUse != "gamehost.channel.use" {
		t.Fatalf("PermGameHostChannelUse = %q, want %q", PermGameHostChannelUse, "gamehost.channel.use")
	}
	if PermGameHostHostAPIInvoke != "gamehost.host_api.invoke" {
		t.Fatalf("PermGameHostHostAPIInvoke = %q, want %q", PermGameHostHostAPIInvoke, "gamehost.host_api.invoke")
	}
	if PermGameHostArtifactDeploy != "gamehost.artifact.deploy" {
		t.Fatalf("PermGameHostArtifactDeploy = %q, want %q", PermGameHostArtifactDeploy, "gamehost.artifact.deploy")
	}
}

func TestGameHostPermissionConstants_MatchGoldenFixture(t *testing.T) {
	data, err := os.ReadFile("../../conformance/fixtures/permission_contract.json")
	if err != nil {
		t.Fatalf("read golden fixture: %v", err)
	}
	var fixture struct {
		GameHostPermissions []string `json:"gameHostPermissions"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse golden fixture: %v", err)
	}
	sdkPerms := []string{PermGameHostControl, PermGameHostChannelUse, PermGameHostHostAPIInvoke, PermGameHostArtifactDeploy}
	if len(sdkPerms) != len(fixture.GameHostPermissions) {
		t.Fatalf("SDK GameHost permission count mismatch: got %d, golden %d", len(sdkPerms), len(fixture.GameHostPermissions))
	}
	for i, perm := range sdkPerms {
		if perm != fixture.GameHostPermissions[i] {
			t.Fatalf("SDK GameHost permission mismatch at index %d: got %s, golden %s", i, perm, fixture.GameHostPermissions[i])
		}
	}
}

func TestGameHostPermissionConstants_ExactlyFour(t *testing.T) {
	perms := map[string]bool{
		PermGameHostControl:        true,
		PermGameHostChannelUse:     true,
		PermGameHostHostAPIInvoke:  true,
		PermGameHostArtifactDeploy: true,
	}
	if len(perms) != 4 {
		t.Fatalf("expected exactly 4 unique GameHost permissions, got %d", len(perms))
	}
}
