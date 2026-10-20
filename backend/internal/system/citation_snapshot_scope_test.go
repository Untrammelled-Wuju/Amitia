package system

import (
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"testing"
)

func TestCitationSnapshotRejectsForeignOwner(t *testing.T) {
	scope := coordination.ExecutionScope{ResourceOwnerID: "owner", RoleID: "role", RoleRevision: 1}
	snapshot := coordination.DataSnapshot{OwnerID: "owner", Role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}, Resources: []coordination.Resource{{OwnerID: "owner", Kind: "message", ID: "message", RoleID: "role", Revision: 1, Body: json.RawMessage(`{"citationSnapshot":[{"id":"K1","title":"private"}]}`)}}}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.OwnerID = "foreign"
	if err := coordination.ValidateSnapshot(scope, snapshot); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign snapshot accepted: %v", err)
	}
	snapshot.OwnerID = "owner"
	snapshot.Resources[0].OwnerID = "foreign"
	if err := coordination.ValidateSnapshot(scope, snapshot); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign cited message accepted: %v", err)
	}
}
