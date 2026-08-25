package kernel

import "testing"

func TestCanonicalGameHostToolIDNamespacesThirdPartyTools(t *testing.T) {
	if got := canonicalGameHostToolID("vendor/game-a", "move"); got != "vendor/game-a/move" {
		t.Fatalf("unexpected canonical id %q", got)
	}
	if got := canonicalGameHostToolID("vendor/game-a", "vendor/game-a/move"); got != "vendor/game-a/move" {
		t.Fatalf("already canonical id changed: %q", got)
	}
	if a, b := canonicalGameHostToolID("vendor/game-a", "move"), canonicalGameHostToolID("vendor/game-b", "move"); a == b {
		t.Fatalf("independent extensions collided: %q", a)
	}
}
