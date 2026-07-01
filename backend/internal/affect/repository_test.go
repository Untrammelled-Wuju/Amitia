package affect

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := AutoMigrateAffect(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestSQLiteRepositorySaveAndLoadRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	repo := &sqliteRepository{db: db}
	now := time.Date(2026, 7, 1, 14, 0, 0, 0, time.UTC)

	state := AffectState{
		Version: StateVersionV1,
		Emotion: EmotionState{
			Positive:    0.35,
			Negative:    0.12,
			Arousal:     0.28,
			Dominance:   0.45,
			LastEventID: "evt-test",
			UpdatedAt:   now,
		},
		Mood: MoodState{
			PAD:       "relaxed",
			Valence:   0.22,
			Tension:   0.08,
			UpdatedAt: now,
		},
		Stress:    0.15,
		UpdatedAt: now,
	}

	if err := repo.SaveState("char-test-1", state); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := repo.LoadState("char-test-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil state")
	}
	if loaded.Version != state.Version {
		t.Fatalf("version: expected %s got %s", state.Version, loaded.Version)
	}
	if loaded.Emotion.Positive != state.Emotion.Positive {
		t.Fatalf("emotion.positive: expected %f got %f", state.Emotion.Positive, loaded.Emotion.Positive)
	}
	if loaded.Emotion.Negative != state.Emotion.Negative {
		t.Fatalf("emotion.negative: expected %f got %f", state.Emotion.Negative, loaded.Emotion.Negative)
	}
	if loaded.Emotion.Arousal != state.Emotion.Arousal {
		t.Fatalf("emotion.arousal: expected %f got %f", state.Emotion.Arousal, loaded.Emotion.Arousal)
	}
	if loaded.Emotion.Dominance != state.Emotion.Dominance {
		t.Fatalf("emotion.dominance: expected %f got %f", state.Emotion.Dominance, loaded.Emotion.Dominance)
	}
	if loaded.Emotion.LastEventID != state.Emotion.LastEventID {
		t.Fatalf("emotion.lastEventID: expected %s got %s", state.Emotion.LastEventID, loaded.Emotion.LastEventID)
	}
	if loaded.Mood.PAD != state.Mood.PAD {
		t.Fatalf("mood.pad: expected %s got %s", state.Mood.PAD, loaded.Mood.PAD)
	}
	if loaded.Mood.Valence != state.Mood.Valence {
		t.Fatalf("mood.valence: expected %f got %f", state.Mood.Valence, loaded.Mood.Valence)
	}
	if loaded.Mood.Tension != state.Mood.Tension {
		t.Fatalf("mood.tension: expected %f got %f", state.Mood.Tension, loaded.Mood.Tension)
	}
	if loaded.Stress != state.Stress {
		t.Fatalf("stress: expected %f got %f", state.Stress, loaded.Stress)
	}
}

func TestSQLiteRepositoryLoadMissingReturnsNil(t *testing.T) {
	db := setupTestDB(t)
	repo := &sqliteRepository{db: db}

	loaded, err := repo.LoadState("nonexistent")
	if err != nil {
		t.Fatalf("load nonexistent: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil for missing character")
	}
}

func TestSQLiteRepositoryOverwriteUpdatesExisting(t *testing.T) {
	db := setupTestDB(t)
	repo := &sqliteRepository{db: db}
	now := time.Date(2026, 7, 1, 15, 0, 0, 0, time.UTC)

	first := AffectState{
		Version: StateVersionV1,
		Emotion: EmotionState{Positive: 0.5, Negative: 0.1, UpdatedAt: now},
		Mood:    MoodState{Valence: 0.3, UpdatedAt: now},
		Stress:  0.2,
		UpdatedAt: now,
	}
	if err := repo.SaveState("char-overwrite", first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	second := AffectState{
		Version: StateVersionV1,
		Emotion: EmotionState{Positive: 0.8, Negative: 0.05, UpdatedAt: now},
		Mood:    MoodState{Valence: 0.6, UpdatedAt: now},
		Stress:  0.1,
		UpdatedAt: now,
	}
	if err := repo.SaveState("char-overwrite", second); err != nil {
		t.Fatalf("second save: %v", err)
	}

	loaded, err := repo.LoadState("char-overwrite")
	if err != nil {
		t.Fatalf("load after overwrite: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil after overwrite")
	}
	if loaded.Emotion.Positive != 0.8 {
		t.Fatalf("expected updated positive=0.8, got %f", loaded.Emotion.Positive)
	}
}

func TestSQLiteRepositoryMultipleCharactersIsolated(t *testing.T) {
	db := setupTestDB(t)
	repo := &sqliteRepository{db: db}
	now := time.Date(2026, 7, 1, 16, 0, 0, 0, time.UTC)

	if err := repo.SaveState("char-a", AffectState{
		Version: StateVersionV1,
		Emotion: EmotionState{Positive: 0.9, UpdatedAt: now},
		Mood:    MoodState{UpdatedAt: now},
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save char-a: %v", err)
	}
	if err := repo.SaveState("char-b", AffectState{
		Version: StateVersionV1,
		Emotion: EmotionState{Positive: 0.1, UpdatedAt: now},
		Mood:    MoodState{UpdatedAt: now},
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("save char-b: %v", err)
	}

	loadedA, _ := repo.LoadState("char-a")
	loadedB, _ := repo.LoadState("char-b")
	if loadedA.Emotion.Positive != 0.9 || loadedB.Emotion.Positive != 0.1 {
		t.Fatalf("isolation failed: a=%f b=%f", loadedA.Emotion.Positive, loadedB.Emotion.Positive)
	}
}
