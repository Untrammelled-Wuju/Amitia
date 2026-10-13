package business

import (
	"context"
	"encoding/json"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
	"testing"
	"time"
)

func TestObservedChatUsesCanonicalMemoryAndReinforcesOnlyReadVersions(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "observed-seed"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &view.Scope
	ids := []string{}
	for _, key := range []string{"used", "unused"} {
		request.RequestID = "seed-" + key
		created, err := engine.ManageOwnedMemory(t.Context(), request, MemoryManagementInput{Action: "create", Key: key, Value: key, RetentionLevel: 3, Importance: 7})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, created.Resources[0].ID)
	}
	request.RequestID, request.ConversationID, request.Message, request.ExpectedScope = "observed-turn", "chat", "used", nil
	request.RecallEvidence = []coordination.ResourceVersion{{Kind: "memory", ID: ids[0], Revision: 1}}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := engine.ObserveMemory(t.Context(), request, Generation{Text: "reply"}); err != nil {
			t.Fatal(err)
		}
	}
	for index, id := range ids {
		row, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "memory", id)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		_ = json.Unmarshal(row.Body, &document)
		state := memorypolicy.ReadLifecycle(document, time.Now())
		if state.ReinforceCount != 1-index {
			t.Fatalf("memory %d reinforced incorrectly: %+v", index, state)
		}
	}
	if model.calls.Load() != 0 || model.extractions.Load() != 1 {
		t.Fatal("observation regenerated reply or repeated extraction")
	}
	request.RequestID = "stale-observation"
	if _, err := engine.ObserveMemory(t.Context(), request, Generation{Text: "reply"}); err == nil {
		t.Fatal("stale recall version accepted")
	}
}

func TestReflectionCandidateUsesOwnedReviewAndReplayLedger(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "conversation-seed", ConversationID: "chat", Message: "喜欢茶"}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "reflection-submit"
	input := MemoryCandidateInput{Key: "饮品", Value: "喜欢茶", MemoryType: "preference", Importance: 7}
	first, err := engine.SubmitMemoryCandidate(t.Context(), request, input)
	if err != nil || !first.Saved || len(first.Resources) != 1 {
		t.Fatalf("submitted=%+v err=%v", first, err)
	}
	second, err := engine.SubmitMemoryCandidate(t.Context(), request, input)
	if err != nil || second.Resources[0].ID != first.Resources[0].ID {
		t.Fatalf("replay=%+v err=%v", second, err)
	}
	request.RequestID, request.ExpectedScope = "reflection-accept", &first.Scope
	accepted, err := engine.ManageOwnedMemoryCandidates(t.Context(), request, MemoryCandidateInput{Action: "accept", ID: first.Resources[0].ID, ExpectedRevision: 1})
	if err != nil || !accepted.Saved {
		t.Fatalf("accepted=%+v err=%v", accepted, err)
	}
	rows, err := coordination.NewOwnershipStore(db, "a").List(t.Context(), "memory", "role", false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("owned memories=%v err=%v", rows, err)
	}
}
