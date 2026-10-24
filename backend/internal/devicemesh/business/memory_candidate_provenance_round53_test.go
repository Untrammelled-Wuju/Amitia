package business

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound53ReviewedCandidatePreservesSourceAndConflictEvidence(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) { return nil, nil }
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "chat", RequestID: "seed", Message: "我喜欢茶"}
	chat, err := engine.Run(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedScope = &chat.Scope
	model.extract = func(context.Context, Inference, Generation) ([]DerivedMemory, error) {
		return []DerivedMemory{{Kind: "fact", Key: "饮品", Body: json.RawMessage(`{"value":"茶","importance":7}`)}}, nil
	}
	request.RequestID = "first-candidate"
	first, err := engine.ManageOwnedMemoryCandidates(t.Context(), request, MemoryCandidateInput{Action: "generate", ConversationID: "chat"})
	if err != nil || len(first.Resources) != 1 {
		t.Fatalf("first candidate: %+v %v", first, err)
	}
	var candidate OwnedMemoryCandidate
	if err := json.Unmarshal(first.Resources[0].Body, &candidate); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "accept-first"
	if _, err := engine.ManageOwnedMemoryCandidates(t.Context(), request, MemoryCandidateInput{Action: "accept", ID: candidate.ID, ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	store := coordination.NewOwnershipStore(db, chat.Scope.ResourceOwnerID)
	assertMemory := func() (coordination.Resource, map[string]json.RawMessage) {
		rows, err := store.List(t.Context(), "memory", "role", false)
		if err != nil || len(rows) != 1 {
			t.Fatalf("memories: %+v %v", rows, err)
		}
		var doc struct {
			Content map[string]json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(rows[0].Body, &doc); err != nil {
			t.Fatal(err)
		}
		return rows[0], doc.Content
	}
	firstMemory, content := assertMemory()
	if MemoryText(content, "source") != "reviewed" || MemoryText(content, "sourceCandidateId") != candidate.ID || MemoryText(content, "sourceConversationId") != "chat" || MemoryText(content, "sourceProvenanceHash") != candidate.ProvenanceHash || MemoryText(content, "sourceOwnerId") != chat.Scope.ResourceOwnerID {
		t.Fatalf("source evidence missing: %+v", content)
	}
	request.RequestID = "second-candidate"
	second, err := engine.ManageOwnedMemoryCandidates(t.Context(), request, MemoryCandidateInput{Action: "generate", ConversationID: "chat"})
	if err != nil || len(second.Resources) != 1 {
		t.Fatalf("second candidate: %+v %v", second, err)
	}
	request.RequestID = "accept-second"
	_, err = engine.ManageOwnedMemoryCandidates(t.Context(), request, MemoryCandidateInput{Action: "accept", ID: second.Resources[0].ID, ExpectedRevision: 1, ConflictID: firstMemory.ID, ExpectedConflictRevision: firstMemory.Revision, Resolution: "merge"})
	if err != nil {
		t.Fatal(err)
	}
	merged, content := assertMemory()
	if merged.Revision != firstMemory.Revision+1 || MemoryText(content, "supersedesMemoryId") != firstMemory.ID || MemoryText(content, "supersedesMemoryHash") != memoryManagementHash(firstMemory.Body) {
		t.Fatalf("conflict evidence lost: %+v", content)
	}
}
