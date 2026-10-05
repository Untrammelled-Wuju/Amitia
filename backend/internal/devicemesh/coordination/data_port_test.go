package coordination_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestResolvedRoleUsesStableIdentityAndNeverFallsBack(t *testing.T) {
	roles := []coordination.Role{{ID: "one", Name: "same-name", Revision: 3, Profile: json.RawMessage(`{"name":"one"}`)}}
	got, err := coordination.ResolveRole("", roles)
	if err != nil || got.ID != "one" || got.Revision != 3 {
		t.Fatalf("one role: %+v %v", got, err)
	}
	if _, err := coordination.ResolveRole("deleted-id", roles); !errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("deleted role fell back: %v", err)
	}
	roles = append(roles, coordination.Role{ID: "two", Name: "same-name", Revision: 1, Profile: json.RawMessage(`{}`)})
	if _, err := coordination.ResolveRole("", roles); !errors.Is(err, coordination.ErrRoleSelection) {
		t.Fatalf("ambiguous role auto-selected: %v", err)
	}
}

func TestSnapshotRejectsCrossOwnerCrossRoleAndDuplicateData(t *testing.T) {
	scope := coordination.ExecutionScope{ResourceOwnerID: "a", RoleID: "role", RoleRevision: 3}
	snapshot := coordination.DataSnapshot{OwnerID: "a", Role: coordination.Role{ID: "role", Revision: 3, Profile: json.RawMessage(`{}`)}, Resources: []coordination.Resource{{OwnerID: "a", RoleID: "role", Kind: "memory", ID: "one", Revision: 1, Body: json.RawMessage(`{}`)}}}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.OwnerID = "b"
	if err := coordination.ValidateSnapshot(scope, snapshot); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatal(err)
	}
	snapshot.OwnerID = "a"
	snapshot.Resources[0].RoleID = "other"
	if err := coordination.ValidateSnapshot(scope, snapshot); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatal(err)
	}
	snapshot.Resources[0].RoleID = "role"
	snapshot.Resources = append(snapshot.Resources, snapshot.Resources[0])
	if err := coordination.ValidateSnapshot(scope, snapshot); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatal(err)
	}
}
