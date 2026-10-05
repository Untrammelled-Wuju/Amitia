package business

import (
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestChatRejectsChangedDisplayedScopeBeforeSavingInput(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "prepare"}
	result, err := engine.Query(t.Context(), request, coordination.DataQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"core", "owner", "mode", "permissions", "target-epoch", "role-revision", "realm"} {
		scope := result.Scope
		switch scenario {
		case "core":
			scope.CoreID = "new-core"
		case "owner":
			scope.ResourceOwnerID = "new-owner"
		case "mode":
			scope.ModeRevision++
		case "permissions":
			scope.TargetPermissionRevision++
		case "target-epoch":
			scope.TargetProviderEpoch++
		case "role-revision":
			scope.RoleRevision++
		case "realm":
			scope.AuthorizationRealm = "new-realm"
		}
		request.RequestID, request.Message, request.ExpectedScope = "request-"+scenario, "execute", &scope
		if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrScopeExpired) {
			t.Fatalf("%s scope accepted: %v", scenario, err)
		}
	}
	var count int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind IN ('message','checkpoint','conversation')`).Scan(&count); err != nil || count != 0 || model.calls.Load() != 0 {
		t.Fatalf("expired request saved or computed: rows=%d calls=%d err=%v", count, model.calls.Load(), err)
	}
	request.RequestID, request.ExpectedScope = "valid-call", &result.Scope
	if response, err := engine.Run(t.Context(), request); err != nil || !response.Saved || model.calls.Load() != 1 {
		t.Fatalf("valid displayed scope rejected: %+v %v", response, err)
	}
}
