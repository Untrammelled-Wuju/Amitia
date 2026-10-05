package business

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type omittedMemoryPort struct{ testDataPort }

func (p omittedMemoryPort) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	snapshot, err := p.testDataPort.Snapshot(ctx, scope, query)
	if err != nil {
		return snapshot, err
	}
	filtered := snapshot.Resources[:0]
	for _, resource := range snapshot.Resources {
		if resource.Kind == "conversation" || resource.Kind == "message" || resource.Kind == "checkpoint" {
			filtered = append(filtered, resource)
		}
	}
	snapshot.Resources = filtered
	return snapshot, nil
}

func TestMemoryUpdateUsesOwnerVersionBeyondContextWindowAndNeverRevivesDeletion(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
		return []DerivedMemory{{Kind: "fact", Key: "drink", Body: json.RawMessage(`{"value":"tea"}`)}}, nil
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", ConversationID: "chat", RequestID: "first", Message: "tea"}
	if response, err := engine.Run(t.Context(), request); err != nil || response.MemoryStatus != "saved" {
		t.Fatalf("first=%+v err=%v", response, err)
	}
	engine.data = omittedMemoryPort{engine.data.(testDataPort)}
	request.RequestID = "second"
	if response, err := engine.Run(t.Context(), request); err != nil || response.MemoryStatus != "saved" {
		t.Fatalf("old memory update=%+v err=%v", response, err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	resources, err := store.List(t.Context(), "memory", "role", false)
	if err != nil || len(resources) != 1 || resources[0].Revision != 2 {
		t.Fatalf("updated memory=%v err=%v", resources, err)
	}
	if _, err := engine.Edit(t.Context(), request, EditRequest{RoleID: "role", RequestID: "forget", Kind: "memory", ID: resources[0].ID, ExpectedRevision: 2, Deleted: true}); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "third"
	response, err := engine.Run(t.Context(), request)
	if err != nil || !response.Saved || response.MemoryStatus != "paused" {
		t.Fatalf("forgotten memory=%+v err=%v", response, err)
	}
	if resources, err := store.List(t.Context(), "memory", "role", false); err != nil || len(resources) != 0 {
		t.Fatal("forgotten memory was recreated", err)
	}
}
