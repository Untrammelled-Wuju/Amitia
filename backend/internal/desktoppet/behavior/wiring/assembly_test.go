package wiring

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/behavior/persistence"
	"github.com/u-ai/backend/internal/psyche"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	for _, sql := range persistence.DesktopPetBehaviorTableSQL {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("failed to execute table SQL: %v", err)
		}
	}
	return db
}

func closeTestDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func TestAssembleBehavior_NilDeps(t *testing.T) {
	_, err := AssembleBehavior(AssemblyDeps{})
	if err == nil {
		t.Fatal("expected error for nil deps")
	}
}

func TestAssembleBehavior_ValidDeps(t *testing.T) {
	db := setupTestDB(t)
	defer closeTestDB(db)

	store := psyche.NewInMemoryPsycheStore()

	deps := AssemblyDeps{
		DB:           db,
		PsycheStore:  store,
		DataDir:      "",
		ShadowMode:   true,
		RuntimeCmdOn: false,
	}

	result, err := AssembleBehavior(deps)
	if err != nil {
		t.Fatalf("AssembleBehavior failed: %v", err)
	}

	if result.Engine == nil {
		t.Fatal("Engine is nil")
	}
	if result.Service == nil {
		t.Fatal("Service is nil")
	}
	if result.Repo == nil {
		t.Fatal("Repo is nil")
	}
	if result.BindingRepo == nil {
		t.Fatal("BindingRepo is nil")
	}
	if result.Reconciler == nil {
		t.Fatal("Reconciler is nil")
	}
}

func TestAssembledEngine_StartStop(t *testing.T) {
	db := setupTestDB(t)
	defer closeTestDB(db)

	store := psyche.NewInMemoryPsycheStore()

	deps := AssemblyDeps{
		DB:           db,
		PsycheStore:  store,
		ShadowMode:   true,
		RuntimeCmdOn: false,
	}

	result, err := AssembleBehavior(deps)
	if err != nil {
		t.Fatalf("AssembleBehavior failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := result.Engine.Start(ctx); err != nil {
		t.Fatalf("Engine.Start failed: %v", err)
	}

	if !result.Engine.IsRunning() {
		t.Fatal("Engine should be running after Start")
	}

	if err := result.Engine.Stop(); err != nil {
		t.Fatalf("Engine.Stop failed: %v", err)
	}

	if result.Engine.IsRunning() {
		t.Fatal("Engine should not be running after Stop")
	}
}

func TestAssembledEngine_SubmitEvent(t *testing.T) {
	db := setupTestDB(t)
	defer closeTestDB(db)

	store := psyche.NewInMemoryPsycheStore()

	deps := AssemblyDeps{
		DB:           db,
		PsycheStore:  store,
		ShadowMode:   true,
		RuntimeCmdOn: false,
	}

	result, err := AssembleBehavior(deps)
	if err != nil {
		t.Fatalf("AssembleBehavior failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := result.Engine.Start(ctx); err != nil {
		t.Fatalf("Engine.Start failed: %v", err)
	}
	defer result.Engine.Stop()

	payload, _ := json.Marshal(map[string]interface{}{
		"interactionId": "test-interaction-1",
		"phase":         "started",
	})

	event := behavior.BehaviorEventEnvelope{
		EventID:       "evt-test-001",
		EventType:     "chat.response.started",
		SchemaVersion: 1,
		OccurredAt:    time.Now(),
		ReceivedAt:    time.Now(),
		UserID:        "test-user-1",
		CharacterID:   "test-char-1",
		Origin:        behavior.OriginInteraction,
		DedupKey:      "test:user-1:char-1:001",
		Payload:       payload,
	}

	err = result.Engine.SubmitEvent(ctx, event)
	if err != nil {
		t.Logf("SubmitEvent returned error (expected in shadow mode without active installation): %v", err)
	}

	metrics := result.Engine.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}
}

func TestActivityAdapter_GetActivitySnapshot(t *testing.T) {
	adapter := NewActivityAdapter(nil)

	ctx := context.Background()
	snapshot, err := adapter.GetActivitySnapshot(ctx, "user-1", "char-1")
	if err != nil {
		t.Fatalf("GetActivitySnapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	if snapshot.ActivityKey == "" {
		t.Fatal("ActivityKey is empty")
	}
	if snapshot.Source != "time_inference" {
		t.Fatalf("expected source time_inference, got %s", snapshot.Source)
	}
	if snapshot.Version != "time-v1" {
		t.Fatalf("expected version time-v1, got %s", snapshot.Version)
	}
}

func TestAffectAdapter_NoState(t *testing.T) {
	store := psyche.NewInMemoryPsycheStore()
	adapter := NewAffectAdapter(store)

	ctx := context.Background()
	snapshot, err := adapter.GetAffectSnapshot(ctx, "user-1", "char-1")
	if err != nil {
		t.Fatalf("GetAffectSnapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	if snapshot.Label != "neutral" {
		t.Fatalf("expected label neutral for no state, got %s", snapshot.Label)
	}
	if snapshot.Confidence != 0.5 {
		t.Fatalf("expected confidence 0.5 for no state, got %f", snapshot.Confidence)
	}
}

func TestAffectAdapter_WithState(t *testing.T) {
	store := psyche.NewInMemoryPsycheStore()

	state := &psyche.PsycheState{
		CharacterID: "char-1",
		Version:     "v1",
		Emotion: psyche.EmotionDimensions{
			Valence:   0.8,
			Arousal:   0.7,
			Dominance: 0.5,
		},
		Stress:    0.2,
		UpdatedAt: time.Now(),
	}
	if err := store.SaveState(state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	adapter := NewAffectAdapter(store)
	ctx := context.Background()
	snapshot, err := adapter.GetAffectSnapshot(ctx, "user-1", "char-1")
	if err != nil {
		t.Fatalf("GetAffectSnapshot failed: %v", err)
	}

	if snapshot.Valence != 0.8 {
		t.Fatalf("expected valence 0.8, got %f", snapshot.Valence)
	}
	if snapshot.Arousal != 0.7 {
		t.Fatalf("expected arousal 0.7, got %f", snapshot.Arousal)
	}
	if snapshot.Label != "happy" {
		t.Fatalf("expected label happy for valence>0.3 and arousal>0.3, got %s", snapshot.Label)
	}
}

func TestMapEmotionLabel(t *testing.T) {
	tests := []struct {
		valence float64
		arousal float64
		want    string
	}{
		{0.5, 0.5, "happy"},
		{0.5, 0.1, "calm"},
		{-0.5, 0.5, "angry"},
		{-0.5, 0.1, "sad"},
		{0.0, 0.7, "excited"},
		{0.0, 0.3, "neutral"},
	}

	for _, tt := range tests {
		got := mapEmotionLabel(tt.valence, tt.arousal)
		if got != tt.want {
			t.Errorf("mapEmotionLabel(%.1f, %.1f) = %s, want %s", tt.valence, tt.arousal, got, tt.want)
		}
	}
}

func TestInferTimePeriodActivity(t *testing.T) {
	tests := []struct {
		hour    int
		wantKey string
		wantSrc string
	}{
		{2, "sleeping", "time_inference"},
		{7, "morning_routine", "time_inference"},
		{10, "working", "time_inference"},
		{13, "lunch", "time_inference"},
		{16, "working", "time_inference"},
		{20, "leisure", "time_inference"},
		{23, "relaxing", "time_inference"},
	}

	for _, tt := range tests {
		key, src := inferTimePeriodActivity(tt.hour)
		if key != tt.wantKey {
			t.Errorf("inferTimePeriodActivity(%d) key = %s, want %s", tt.hour, key, tt.wantKey)
		}
		if src != tt.wantSrc {
			t.Errorf("inferTimePeriodActivity(%d) src = %s, want %s", tt.hour, src, tt.wantSrc)
		}
	}
}

func TestInferCategoryFromKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"idle_normal", "idle"},
		{"idle_blink", "idle"},
		{"walk_left", "movement"},
		{"wave", "emotion"},
		{"happy", "emotion"},
		{"speaking", "dialogue"},
		{"sleeping", "life"},
		{"custom_action", "interaction"},
	}

	for _, tt := range tests {
		got := inferCategoryFromKey(tt.key)
		if got != tt.want {
			t.Errorf("inferCategoryFromKey(%s) = %s, want %s", tt.key, got, tt.want)
		}
	}
}
