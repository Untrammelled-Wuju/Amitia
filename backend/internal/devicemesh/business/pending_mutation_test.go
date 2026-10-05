package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

var errPendingOffline = errors.New("device disconnected before save acknowledgement")

type queuedMutationPort struct {
	testDataPort
	service *coordination.Service
	offline bool
}

func (p *queuedMutationPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if err := p.service.Enqueue(ctx, commit); err != nil {
		return coordination.Acknowledgement{}, err
	}
	if p.offline {
		return coordination.Acknowledgement{}, errPendingOffline
	}
	pending, err := p.service.PendingRequest(ctx, commit.Scope.ResourceOwnerID, commit.Scope.RequestID)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	ack, err := p.testDataPort.Commit(ctx, commit)
	if err == nil {
		err = p.service.Acknowledge(ctx, ack, pending.Hash)
	}
	return ack, err
}

func TestQueuedEditRetriesOriginalPayloadAndRejectsChangedRequest(t *testing.T) {
	engine, db, service, _ := engineHarness(t)
	authority := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "chat", RequestID: "turn", Message: "tea"}
	if _, err := engine.Run(t.Context(), authority); err != nil {
		t.Fatal(err)
	}
	port := &queuedMutationPort{testDataPort: engine.data.(testDataPort), service: service, offline: true}
	engine.data = port
	edit := EditRequest{RoleID: "role", RequestID: "edit-pending", Kind: "message", ID: "turn/user", ExpectedRevision: 1, Changes: map[string]json.RawMessage{"content": json.RawMessage(`"coffee"`)}}
	if _, err := engine.Edit(t.Context(), authority, edit); !errors.Is(err, errPendingOffline) {
		t.Fatalf("offline edit=%v", err)
	}
	changed := edit
	changed.Changes = map[string]json.RawMessage{"content": json.RawMessage(`"other"`)}
	if _, err := engine.Edit(t.Context(), authority, changed); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("conflicting pending edit accepted: %v", err)
	}
	port.offline = false
	ack, err := engine.Edit(t.Context(), authority, edit)
	if err != nil || ack.Versions["message/turn/user"] != 2 {
		t.Fatalf("original edit retry=%+v %v", ack, err)
	}
	resource, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "message", "turn/user")
	if err != nil || resource.Revision != 2 {
		t.Fatalf("resource=%+v %v", resource, err)
	}
	if pending, err := service.PendingRequest(t.Context(), "a", "edit-pending"); err != nil || pending != nil {
		t.Fatalf("acknowledged body retained: %v %v", pending, err)
	}
}

func TestQueuedContinuityRetriesOriginalLeaseDocument(t *testing.T) {
	engine, _, service, _ := engineHarness(t)
	port := &queuedMutationPort{testDataPort: engine.data.(testDataPort), service: service, offline: true}
	engine.data = port
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "create-pending"}
	mutation := ContinuityMutation{Action: "create", Title: "检查进展"}
	first, _, err := engine.Continuity(t.Context(), request, mutation)
	if !errors.Is(err, errPendingOffline) {
		t.Fatal(err)
	}
	port.offline = false
	resumed, ack, err := engine.Continuity(t.Context(), request, mutation)
	if err != nil || resumed.Thread.CreatedAt != first.Thread.CreatedAt || ack.Versions["continuity/"+first.Thread.ID] != 1 {
		t.Fatalf("queued continuity regenerated: %+v %v", resumed, err)
	}
}
