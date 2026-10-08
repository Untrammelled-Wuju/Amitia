package business

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type guardedResourceTestPort struct {
	testDataPort
	revision   atomic.Int64
	resource   coordination.Resource
	changeRole bool
}

func (p *guardedResourceTestPort) Roles(context.Context, coordination.ExecutionScope) ([]coordination.Role, error) {
	role := p.role
	role.Revision = p.revision.Load()
	return []coordination.Role{role}, nil
}

func (p *guardedResourceTestPort) Resource(context.Context, coordination.ExecutionScope, string, string) (*coordination.Resource, error) {
	if p.changeRole {
		p.revision.Add(1)
	}
	return &p.resource, nil
}

func TestOwnedResourceReadRejectsWrongReturnedIdentityAndChangedRole(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	original := engine.data.(testDataPort)
	for _, change := range []string{"owner", "role", "kind", "id", "revision", "body", "changed-role"} {
		port := &guardedResourceTestPort{testDataPort: original, resource: coordination.Resource{OwnerID: "a", RoleID: "role", Kind: "project", ID: "expected-id", Revision: 1, Body: json.RawMessage(`{"title":"项目"}`)}}
		port.revision.Store(1)
		switch change {
		case "owner":
			port.resource.OwnerID = "wrong-owner"
		case "role":
			port.resource.RoleID = "wrong-role"
		case "kind":
			port.resource.Kind = "memory"
		case "id":
			port.resource.ID = "wrong-id"
		case "revision":
			port.resource.Revision = 0
		case "body":
			port.resource.Body = json.RawMessage(`invalid`)
		case "changed-role":
			port.changeRole = true
		}
		engine.data = port
		row, _, err := engine.ReadResource(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "read-" + change}, "project", "expected-id")
		if err == nil || row != nil {
			t.Fatalf("unsafe %s resource escaped: %v", change, err)
		}
	}
}
