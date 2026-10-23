package business

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestEmotionSnapshotImportsToOwnerAndIsIdempotent(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "emotion-request"}
	snapshot := EmotionSnapshot{ID: "emotion/emotion-request", SpaceID: "core", RoleID: "role", ConversationID: "chat", MessageID: "reply", RequestID: "emotion-request", Source: "chat", DetectionEnabled: true, DetectionThreshold: 0.5, Confidence: 0.8, MoodValence: 0.7, MoodArousal: 0.4, MoodLabel: "calm", AffectLabel: "积极", CreatedAt: "2026-10-23T07:00:00Z"}
	if err := engine.ImportEmotionSnapshot(t.Context(), request, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := engine.ImportEmotionSnapshot(t.Context(), request, snapshot); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: EmotionSnapshotKind, Management: true, Limit: 10})
	if err != nil || len(result.Snapshot.Resources) != 1 || result.Snapshot.Resources[0].ID != snapshot.ID || result.Snapshot.OwnerID != "a" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestEmotionSnapshotUsesCoordinatedCoreOwner(t *testing.T) {
	engine, _, service, _ := engineHarness(t)
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "emotion-cloud"}
	snapshot := EmotionSnapshot{ID: "emotion/emotion-cloud", SpaceID: "core", RoleID: "role", RequestID: "emotion-cloud", Source: "chat", DetectionEnabled: true, DetectionThreshold: 0.2, Confidence: 0.8, MoodValence: 0.7, MoodArousal: 0.4, MoodLabel: "calm", AffectLabel: "积极", CreatedAt: "2026-10-23T08:00:00Z"}
	if err := engine.ImportEmotionSnapshot(t.Context(), request, snapshot); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: EmotionSnapshotKind, Management: true, Limit: 10})
	if err != nil || result.Snapshot.OwnerID != "core" || len(result.Snapshot.Resources) != 1 || result.Snapshot.Resources[0].ID != snapshot.ID {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestEmotionHistoryDeletionIsOwnerScopedAndRevisionChecked(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "emotion-delete"}
	snapshot := EmotionSnapshot{ID: "emotion/emotion-delete", SpaceID: "core", RoleID: "role", ConversationID: "chat", RequestID: "emotion-delete", Source: "chat", MoodValence: 0.7, MoodLabel: "calm", CreatedAt: "2026-10-23T08:10:00Z"}
	if err := engine.ImportEmotionSnapshot(t.Context(), request, snapshot); err != nil {
		t.Fatal(err)
	}
	before, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: EmotionSnapshotKind, Management: true, Limit: 10})
	if err != nil || len(before.Snapshot.Resources) != 1 {
		t.Fatalf("before=%+v err=%v", before, err)
	}
	resource := before.Snapshot.Resources[0]
	if _, err := engine.Edit(t.Context(), request, EditRequest{RoleID: "role", RequestID: "emotion-delete-request", Kind: EmotionSnapshotKind, ID: resource.ID, ExpectedRevision: resource.Revision, ExpectedScope: &before.Scope, Deleted: true}); err != nil {
		t.Fatal(err)
	}
	if err := engine.ImportEmotionSnapshot(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "emotion-reimport"}, snapshot); err != nil {
		t.Fatal(err)
	}
	after, err := engine.Query(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role"}, coordination.DataQuery{ResourceKind: EmotionSnapshotKind, Management: true, Limit: 10})
	emotions := 0
	for _, item := range after.Snapshot.Resources {
		if item.Kind == EmotionSnapshotKind {
			emotions++
		}
	}
	if err != nil || emotions != 0 {
		t.Fatalf("after=%+v err=%v", after, err)
	}
	if _, err := engine.Edit(t.Context(), request, EditRequest{RoleID: "role", RequestID: "emotion-edit-request", Kind: EmotionSnapshotKind, ID: resource.ID, ExpectedRevision: resource.Revision, Changes: map[string]json.RawMessage{"moodLabel": json.RawMessage(`"happy"`)}}); err == nil {
		t.Fatal("emotion history content mutation succeeded")
	}
}
