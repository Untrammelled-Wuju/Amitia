package interaction

import (
	"testing"
)

func TestReconciledChildMustMatchDurableAssignmentScope(t *testing.T) {
	assignment := AgentAssignment{ID: "assignment", ChildInteractionID: "child", WorkerRef: AgentWorkerRef{CharacterID: "worker-char"}}
	scope := InteractionScope{SpaceID: "space-a", CharacterID: "worker-char", ConversationID: "multi-agent-worker-assignment", RequestID: "ma-assignment"}
	record := &InteractionRecord{ID: "child", Scope: scope}
	if err := validateReconciledChildScope(record, assignment, "space-a", "parent-conversation"); err != nil {
		t.Fatalf("valid durable child rejected: %v", err)
	}
	cases := []struct {
		name   string
		modify func(*InteractionRecord)
	}{
		{"wrong space", func(r *InteractionRecord) { r.Scope.SpaceID = "space-b" }},
		{"wrong conversation", func(r *InteractionRecord) { r.Scope.ConversationID = "parent-conversation" }},
		{"wrong character", func(r *InteractionRecord) { r.Scope.CharacterID = "intruder" }},
		{"wrong request", func(r *InteractionRecord) { r.Scope.RequestID = "ma-another" }},
		{"wrong child id", func(r *InteractionRecord) { r.ID = "different" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			altered := *record
			c.modify(&altered)
			if err := validateReconciledChildScope(&altered, assignment, "space-a", "parent-conversation"); err == nil {
				t.Fatal("reconciliation accepted a child from different identity")
			}
		})
	}
}
