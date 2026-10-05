package business

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type projectionTestPort struct {
	testDataPort
	rebuilds int
}

func (p *projectionTestPort) ProjectionState(ctx context.Context, scope coordination.ExecutionScope, rebuild bool) (coordination.ProjectionStatus, error) {
	store := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID)
	if rebuild {
		p.rebuilds++
		if err := store.RebuildProjections(ctx, scope.RoleID); err != nil {
			return coordination.ProjectionStatus{}, err
		}
	}
	return store.ProjectionStatus(ctx, scope.RoleID)
}

func TestProjectionRebuildRequiresDisplayedAuthority(t *testing.T) {
	engine, _, service, _ := engineHarness(t)
	port := &projectionTestPort{testDataPort: engine.data.(testDataPort)}
	engine.data = port
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "status", RoleID: "role"}
	status, scope, err := engine.Projections(t.Context(), request, false, nil)
	if err != nil || status.OwnerID != "a" || len(status.Layers) != 2 {
		t.Fatalf("status: %+v %v", status, err)
	}
	if _, _, err := engine.Projections(t.Context(), request, true, nil); !errors.Is(err, coordination.ErrScopeExpired) || port.rebuilds != 0 {
		t.Fatalf("unversioned rebuild: %d %v", port.rebuilds, err)
	}
	if _, _, err := engine.Projections(t.Context(), request, true, &scope); err != nil || port.rebuilds != 1 {
		t.Fatalf("valid rebuild: %d %v", port.rebuilds, err)
	}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.Projections(t.Context(), request, true, &scope); !errors.Is(err, coordination.ErrScopeExpired) || port.rebuilds != 1 {
		t.Fatalf("old owner rebuild: %d %v", port.rebuilds, err)
	}
}
