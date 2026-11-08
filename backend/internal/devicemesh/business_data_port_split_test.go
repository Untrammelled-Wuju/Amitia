package devicemesh_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type splitOwnedCommitPort struct {
	owner    string
	commits  []coordination.Commit
	role     coordination.Role
	revision int64
}

func (p *splitOwnedCommitPort) Roles(context.Context, coordination.ExecutionScope) ([]coordination.Role, error) {
	return []coordination.Role{p.role}, nil
}

func (p *splitOwnedCommitPort) Snapshot(context.Context, coordination.ExecutionScope, coordination.DataQuery) (coordination.DataSnapshot, error) {
	return coordination.DataSnapshot{}, nil
}

func (p *splitOwnedCommitPort) Commit(_ context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	p.commits = append(p.commits, commit)
	versions := make(map[string]int64, len(commit.Mutations))
	for _, mutation := range commit.Mutations {
		revision := mutation.ExpectedRevision + 1
		if mutation.ExpectedRevision == coordination.RevisionCurrent {
			revision = 1
		}
		versions[mutation.Kind+"/"+mutation.ID] = revision
	}
	return coordination.Acknowledgement{RequestID: commit.Scope.RequestID, OwnerID: p.owner, Versions: versions}, nil
}

func TestCommitSplitsDeviceChatAndCoreRuntimeStateByOwner(t *testing.T) {
	db := meshAuthorityDB(t)
	registry := host_registry.NewRegistry(db)
	if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: "phone", Platform: runtimeidentity.PlatformAndroid, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	rt := newCloudRuntimeForTest(t, db, registry)
	rt.LocalDeviceID = "phone"
	if _, err := rt.Coordination.InitializeCoordinated(t.Context(), "core", "phone", 1, "role"); err != nil {
		t.Fatal(err)
	}
	devicePort := &splitOwnedCommitPort{owner: "phone", role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}}
	corePort := &splitOwnedCommitPort{owner: "core", role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{}`)}}
	rt.LocalDeviceDataPort = devicePort
	rt.CoreDataPort = corePort
	_, scope, finish, err := rt.Coordination.BeginWithOwners(t.Context(), "core", "phone", "phone", "core", "role", "request", "phone", "core")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	scope.RoleRevision = 1
	commit := coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{
		{Kind: "message", ID: "request/assistant", RoleID: "role", Body: json.RawMessage(`{"content":"reply"}`)},
		{Kind: "checkpoint", ID: "turn/request", RoleID: "role", ExpectedRevision: coordination.RevisionCurrent, Body: json.RawMessage(`{"status":"completed"}`)},
		{Kind: "psyche", ID: "role/role", RoleID: "role", ExpectedRevision: 24, Body: json.RawMessage(`{"valence":0.6}`)},
	}, Dependencies: []coordination.ResourceVersion{{Kind: "message", ID: "request/user", Revision: 1}, {Kind: "psyche", ID: "role/role", Revision: 24}}}
	ack, err := rt.Commit(t.Context(), commit)
	if err != nil {
		t.Fatal(err)
	}
	if ack.OwnerID != "phone" || ack.RequestID != "request" || ack.Versions["message/request/assistant"] != 1 || ack.Versions["psyche/role/role"] != 25 {
		t.Fatalf("combined acknowledgement=%+v", ack)
	}
	if len(devicePort.commits) != 1 || len(devicePort.commits[0].Mutations) != 2 || len(devicePort.commits[0].Dependencies) != 1 {
		t.Fatalf("device commit was not isolated: %+v", devicePort.commits)
	}
	if len(corePort.commits) != 1 || len(corePort.commits[0].Mutations) != 1 || corePort.commits[0].Mutations[0].Kind != "psyche" || len(corePort.commits[0].Dependencies) != 1 || corePort.commits[0].Scope.ResourceOwnerID != "core" || corePort.commits[0].Scope.RoleOwnerID != "core" {
		t.Fatalf("core commit was not isolated: %+v", corePort.commits)
	}
}
