package business

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestContinuityOriginalIntentRejectsModeABAAndForeignRealm(t *testing.T) {
	engine, db, service, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "create-original"}
	query, err := engine.Query(t.Context(), request, coordination.DataQuery{})
	if err != nil {
		t.Fatal(err)
	}
	original := query.Scope
	created, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "原设备事项", ExpectedScope: &original})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 2, false, "role"); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "old-resume"
	if _, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: created.Thread.ID, Action: "resume", ExpectedRevision: 1, ExpectedScope: &original}); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("old form after mode ABA admitted: %v", err)
	}
	query, err = engine.Query(t.Context(), request, coordination.DataQuery{})
	if err != nil {
		t.Fatal(err)
	}
	foreign := query.Scope
	foreign.AuthorizationRealm = "other-pairing-realm"
	if _, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: created.Thread.ID, Action: "pause", ExpectedRevision: 1, ExpectedScope: &foreign}); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("foreign realm intent admitted: %v", err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "continuity", "role", false)
	if err != nil || len(rows) != 1 || rows[0].Revision != 1 {
		t.Fatalf("rejected intent changed owner data: %+v %v", rows, err)
	}
	request.RequestID = "explicit-current-resume"
	resumed, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{ID: created.Thread.ID, Action: "resume", ExpectedRevision: 1, ExpectedScope: &query.Scope})
	if err != nil || resumed.Thread.Revision != 2 {
		t.Fatalf("fresh explicit resume failed: %+v %v", resumed, err)
	}
}
