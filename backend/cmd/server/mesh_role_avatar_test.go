package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/character"
)

func TestActualTLSRoleAvatarsFollowRoleOwnerAndClear(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "avatar-source", schemas), newThreeCoreFixture(t, "avatar-core", schemas)
	avatars := map[*threeCoreFixture]string{a: "data:image/png;base64,source", b: "data:image/png;base64,core"}
	for fixture, avatar := range avatars {
		if err := fixture.services.DB.Model(&character.Character{}).Where("id=?", "one").Update("avatar", avatar).Error; err != nil {
			t.Fatal(err)
		}
	}
	pairThreeCoreFixtures(t, a, b)
	assertAvatar := func(expected string) {
		t.Helper()
		var response struct {
			Roles []map[string]json.RawMessage `json:"roles"`
		}
		if err := json.Unmarshal(b.request(t, a, http.MethodGet, "/api/device-mesh/v1/business/roles", nil, http.StatusOK), &response); err != nil {
			t.Fatal(err)
		}
		for _, role := range response.Roles {
			var id, avatar string
			if err := json.Unmarshal(role["id"], &id); err != nil {
				t.Fatal(err)
			}
			if id != "one" {
				continue
			}
			if err := json.Unmarshal(role["avatar"], &avatar); err != nil || avatar != expected {
				t.Fatalf("role avatar=%q expected=%q err=%v", avatar, expected, err)
			}
			if _, exposed := role["profile"]; exposed {
				t.Fatal("role listing exposed private profile")
			}
			return
		}
		t.Fatal("role not found")
	}
	assertAvatar(avatars[a])
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, b, a, policy.ModeRevision)
	assertAvatar(avatars[b])
	if err := b.services.DB.Model(&character.Character{}).Where("id=?", "one").Update("avatar", "").Error; err != nil {
		t.Fatal(err)
	}
	assertAvatar("")
}
