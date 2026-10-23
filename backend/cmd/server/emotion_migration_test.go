package main

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type emotionMigrationModel struct{}

func (emotionMigrationModel) GenerateOwnedReply(context.Context, business.Inference) (business.Generation, error) {
	return business.Generation{Text: "ok"}, nil
}

func (emotionMigrationModel) ExtractOwnedMemory(context.Context, business.Inference, business.Generation) ([]business.DerivedMemory, error) {
	return nil, nil
}

func TestLegacyEmotionHistoryMigratesToOwnerIdempotently(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device-a','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(coordination.NewService(db), p, emotionMigrationModel{})
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := p.services.DB.Exec(`INSERT INTO moods(id,character_id,mood,mood_value,level,created_at) VALUES(1,'one','calm','0.65',65,?)`, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Exec(`INSERT INTO emotion_states(space_id,character_id,user_affect_json,relationship_emotion_json,signals_json,baseline_json,version,updated_at) VALUES('core','one','{"primary_emotion":"sadness","intensity":0.8,"stress":0.7,"confidence":0.9}','{}','{}','{}',1,?)`, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Exec(`ALTER TABLE messages ADD COLUMN mood TEXT DEFAULT ''`).Error; err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Exec(`INSERT INTO messages(id,conversation_id,character_id,role,content,mood,created_at) VALUES('legacy-message','chat','one','assistant','reply','happy',?)`, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyEmotionHistory(t.Context(), p.services.DB, engine, "core", "device-a", "core"); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyEmotionHistory(t.Context(), p.services.DB, engine, "core", "device-a", "core"); err != nil {
		t.Fatal(err)
	}
	request := business.Request{SpaceID: "core", DeviceID: "device-a", CoreID: "core", RoleID: "one", RequestID: "read-emotions"}
	result, err := engine.Query(t.Context(), request, coordination.DataQuery{ResourceKind: business.EmotionSnapshotKind, Management: true, Limit: 20})
	if err != nil || len(result.Snapshot.Resources) != 3 {
		t.Fatalf("resources=%+v err=%v", result.Snapshot.Resources, err)
	}
}

func TestOwnedEmotionSnapshotUsesConfiguredThresholdAndRecovery(t *testing.T) {
	scope := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", RoleID: "one", RoleRevision: 1, ResourceOwnerID: "device"}
	request := coordination.OwnedChatEffectsRequest{ConversationID: "chat", MessageID: "reply", RequestID: "request", Source: "chat"}
	state := interaction.PsycheState{Valence: 0.8, Arousal: 0.7, Dominance: 0.6, Stress: 0.2, Fatigue: 0.3, MoodValence: 0.2, MoodArousal: 0.4, MoodPressure: 0.4}
	snapshot := buildOwnedEmotionSnapshot(scope, request, time.Now().Add(-48*time.Hour), time.Now(), state, 0.5)
	if !snapshot.RecoveryApplied || snapshot.RecoveredMood == "" || snapshot.Confidence < 0.5 || snapshot.AffectLabel == "" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestOwnedMoodDetectionSettingsAreConsumedFromAppSettings(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	if err := p.services.DB.Exec(`INSERT INTO app_settings(key,value,revision,updated_at) VALUES('mood_detection_enabled','true',1,'now'),('mood_detection_threshold','0.72',1,'now')`).Error; err != nil {
		t.Fatal(err)
	}
	enabled, threshold, err := ownedMoodDetectionSettings(t.Context(), p.services.DB)
	if err != nil || !enabled || threshold != 0.72 {
		t.Fatalf("enabled=%v threshold=%v err=%v", enabled, threshold, err)
	}
}
