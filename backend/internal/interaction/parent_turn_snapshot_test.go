package interaction

import (
	"context"
	"testing"
)

func TestGenericParentCheckpointCapturesEffectiveConversationDefaults(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	if err := tracker.db.Exec(`CREATE TABLE conversations (
		id TEXT PRIMARY KEY, space_id TEXT, deleted_at TEXT,
		model_config_id INTEGER, reasoning_effort TEXT, reasoning_enabled INTEGER,
		permission_mode TEXT, workspace_id TEXT, workspace_device_id TEXT
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := tracker.db.Exec(`INSERT INTO conversations
		(id,space_id,model_config_id,reasoning_effort,reasoning_enabled,permission_mode,workspace_id,workspace_device_id)
		VALUES ('conversation','space',42,'medium',1,'request_approval','workspace-1','device-1')`).Error; err != nil {
		t.Fatal(err)
	}
	proc := &parentCheckpointCapture{tracker: tracker}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), proc, tracker, nil)
	orch.SetReady(true)
	_, err := orch.Process(context.Background(), &ProcessRequest{
		SpaceID: "space", CharacterID: "character", ConversationID: "conversation",
		Channel: "web", Source: "web", RequestID: "req-defaults", Message: "continue",
	})
	if err != nil {
		t.Fatal(err)
	}
	if proc.seen == nil || proc.seen.ParentTurn == nil || proc.seen.ParentTurn.ConversationSnapshot == nil {
		t.Fatalf("default session configuration was not journaled: %+v", proc.seen)
	}
	ref := proc.seen.ParentTurn
	if ref.ModelConfigID != 42 || ref.ReasoningEffort != "medium" ||
		ref.PermissionMode != "request_approval" || ref.WorkspaceID != "workspace-1" ||
		ref.WorkspaceDeviceID != "device-1" || ref.ConversationSnapshot.ReasoningEnabled != 1 {
		t.Fatalf("generic parent lost the original effective model or device binding: %+v", ref)
	}
	recheck := *proc.seen
	fingerprint := recheck.Fingerprint
	recheck.ComputeFingerprint()
	if fingerprint == "" || fingerprint != recheck.Fingerprint {
		t.Fatal("conversation baseline not protected by descriptor fingerprint")
	}
}
