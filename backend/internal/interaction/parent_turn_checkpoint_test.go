package interaction

import (
	"context"
	"testing"
)

type parentCheckpointCapture struct {
	tracker InteractionTracker
	seen    *RecoveryDescriptor
}

func (p *parentCheckpointCapture) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	record, found, err := p.tracker.Get(ctx, req.InteractionID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrInteractionCASConflict
	}
	p.seen = record.RecoveryDescriptor
	return &ProcessResponse{ConversationID: req.ConversationID, CharacterID: req.CharacterID,
		RequestID: req.RequestID, Reply: "ok"}, nil
}

func TestOrchestratorWritesGenericParentCheckpointBeforeProcessor(t *testing.T) {
	tracker := NewInMemoryTracker()
	proc := &parentCheckpointCapture{tracker: tracker}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), proc, tracker, nil)
	orch.SetReady(true)
	_, err := orch.Process(context.Background(), &ProcessRequest{
		SpaceID: "space", CharacterID: "character", ConversationID: "conversation",
		Channel: "web", Source: "web", RequestID: "req-1", Message: "verify and repair",
		PermissionMode: "request_approval", ModelConfigID: 19, ReasoningEffort: "medium",
		ThreadID: "original-thread", MessageStyle: "concise",
	})
	if err != nil {
		t.Fatal(err)
	}
	if proc.seen == nil || proc.seen.ParentTurn == nil {
		t.Fatalf("ordinary web agent turn did not persist a parent checkpoint: %+v", proc.seen)
	}
	if proc.seen.ParentTurn.ModelConfigID != 19 ||
		proc.seen.ParentTurn.PermissionMode != "request_approval" ||
		proc.seen.ParentTurn.ThreadID != "original-thread" ||
		proc.seen.ParentTurn.MessageStyle != "concise" ||
		proc.seen.Interaction.RequestID != "req-1" {
		t.Fatalf("parent checkpoint lost original scope/configuration: %+v", proc.seen)
	}
	verified := *proc.seen
	original := verified.Fingerprint
	verified.ComputeFingerprint()
	if original == "" || original != verified.Fingerprint {
		t.Fatal("persisted parent checkpoint fingerprint invalid")
	}
}
