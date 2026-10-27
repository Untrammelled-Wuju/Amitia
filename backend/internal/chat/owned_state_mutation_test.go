package chat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func TestOwnedStateMutationsUseRevisionedOwnerResources(t *testing.T) {
	svc := &service{}
	runtime := interaction.RuntimeAssembly{Appraisal: &interaction.AppraisalResult{EventType: "praise", PsycheDelta: 0.2, RelationshipDelta: 0.3, Severity: 0.4, NeedDeltas: map[string]float64{"connection": 0.1}}}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{SpaceID: "space", CoreID: "core", ResourceOwnerID: "device", RoleOwnerID: "device", RoleID: "role", RequestID: "request", TurnID: "turn"}
	ctx := coordination.WithScope(t.Context(), scope)
	inference := business.Inference{Scope: scope, ConversationID: "conversation", Snapshot: coordination.DataSnapshot{Role: coordination.Role{ID: "role", Revision: 1}, Resources: []coordination.Resource{
		{OwnerID: "device", Kind: "psyche", ID: coordination.RoleRuntimeStateID("psyche", "role"), RoleID: "role", Revision: 3, Body: json.RawMessage(`{"valence":0.6,"dominance":0.5,"arousal":0.5,"moodValence":0.5,"moodArousal":0.5,"stress":0.2,"fatigue":0.3}`)},
		{OwnerID: "device", Kind: "relationship", ID: coordination.RoleRuntimeStateID("relationship", "role"), RoleID: "role", Revision: 4, Body: json.RawMessage(`{"trust":0.6,"familiarity":0.2,"security":0.5,"tension":0.1,"repairConfidence":0.5,"boundary":0.5}`)},
		{OwnerID: "device", Kind: "needs", ID: coordination.RoleRuntimeStateID("needs", "role"), RoleID: "role", Revision: 5, Body: json.RawMessage(`{"needs":[{"kind":"connection","level":0.4,"baseline":0.34,"updatedAt":"2026-10-21T00:00:00Z"}],"count":1}`)},
	}}}
	mutations, err := svc.OwnedStateMutations(ctx, inference, business.Generation{StateEffects: encoded})
	if err != nil {
		t.Fatal(err)
	}
	if len(mutations) != 3 {
		t.Fatalf("mutations=%d", len(mutations))
	}
	revisions := map[string]int64{}
	for _, mutation := range mutations {
		revisions[mutation.Kind] = mutation.ExpectedRevision
		expectedID := coordination.RoleRuntimeStateID(mutation.Kind, "role")
		if mutation.RoleID != "role" || mutation.ID != expectedID || !json.Valid(mutation.Body) {
			t.Fatalf("invalid mutation: %+v", mutation)
		}
	}
	if revisions["psyche"] != 3 || revisions["relationship"] != 4 || revisions["needs"] != 5 {
		t.Fatalf("revisions=%v", revisions)
	}
}

func TestOwnedStateMutationsRejectCancelledAuthority(t *testing.T) {
	svc := &service{}
	runtime := interaction.RuntimeAssembly{Appraisal: &interaction.AppraisalResult{EventType: "chat"}}
	encoded, _ := json.Marshal(runtime)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := svc.OwnedStateMutations(ctx, business.Inference{}, business.Generation{StateEffects: encoded})
	if err == nil {
		t.Fatal("cancelled authority accepted")
	}
}

func TestOwnedStateMutationsIgnoreOtherGroupMemberState(t *testing.T) {
	svc := &service{}
	runtime := interaction.RuntimeAssembly{Appraisal: &interaction.AppraisalResult{EventType: "chat"}}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{SpaceID: "space", CoreID: "core", ResourceOwnerID: "device", RoleOwnerID: "device", RoleID: "role-b", RequestID: "request", TurnID: "turn", ConversationRoleID: "role-a"}
	ctx := coordination.WithScope(t.Context(), scope)
	inference := business.Inference{Scope: scope, ConversationID: "conversation", Snapshot: coordination.DataSnapshot{Role: coordination.Role{ID: "role-b", Revision: 1}, Resources: []coordination.Resource{
		{OwnerID: "device", Kind: "psyche", ID: "role", RoleID: "role-a", Revision: 9, Body: json.RawMessage(`{"valence":0.6,"dominance":0.5,"arousal":0.5}`)},
		{OwnerID: "device", Kind: "relationship", ID: "user_character", RoleID: "role-a", Revision: 9, Body: json.RawMessage(`{"trust":0.6}`)},
		{OwnerID: "device", Kind: "needs", ID: "role", RoleID: "role-a", Revision: 9, Body: json.RawMessage(`{"needs":[],"count":0}`)},
		{OwnerID: "device", Kind: "psyche", ID: "role/role-b", RoleID: "role-b", Revision: 2, Body: json.RawMessage(`{"valence":0.4,"dominance":0.5,"arousal":0.5}`)},
	}}}
	mutations, err := svc.OwnedStateMutations(ctx, inference, business.Generation{StateEffects: encoded})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range mutations {
		if mutation.RoleID != "role-b" {
			t.Fatalf("group member state mutation escaped role scope: %+v", mutation)
		}
		if mutation.Kind == "psyche" && mutation.ExpectedRevision != 2 {
			t.Fatalf("psyche revision=%d", mutation.ExpectedRevision)
		}
		if mutation.Kind != "psyche" && mutation.ExpectedRevision != 0 {
			t.Fatalf("%s revision=%d", mutation.Kind, mutation.ExpectedRevision)
		}
	}
}

func TestOwnedStateMutationsMigrateLegacyStateWithoutOverwritingOtherRoles(t *testing.T) {
	svc := &service{}
	runtime := interaction.RuntimeAssembly{Appraisal: &interaction.AppraisalResult{EventType: "chat"}}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{SpaceID: "space", CoreID: "core", ResourceOwnerID: "device", RoleOwnerID: "device", RoleID: "role", RequestID: "request", TurnID: "turn"}
	ctx := coordination.WithScope(t.Context(), scope)
	inference := business.Inference{Scope: scope, ConversationID: "conversation", Snapshot: coordination.DataSnapshot{Role: coordination.Role{ID: "role", Revision: 1}, Resources: []coordination.Resource{
		{OwnerID: "device", Kind: "psyche", ID: "role", RoleID: "role", Revision: 7, Body: json.RawMessage(`{"valence":0.4,"dominance":0.5,"arousal":0.5}`)},
		{OwnerID: "device", Kind: "relationship", ID: "user_character", RoleID: "role", Revision: 8, Body: json.RawMessage(`{"trust":0.4}`)},
		{OwnerID: "device", Kind: "needs", ID: "role", RoleID: "role", Revision: 9, Body: json.RawMessage(`{"needs":[],"count":0}`)},
	}}}
	mutations, err := svc.OwnedStateMutations(ctx, inference, business.Generation{StateEffects: encoded})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range mutations {
		if mutation.ID != coordination.RoleRuntimeStateID(mutation.Kind, "role") || mutation.ExpectedRevision != 0 {
			t.Fatalf("legacy state was not migrated by role-scoped id: %+v", mutation)
		}
	}
}
