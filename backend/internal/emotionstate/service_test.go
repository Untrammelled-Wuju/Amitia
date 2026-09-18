package emotionstate

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newEmotionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "emotion.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&Record{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestServiceCommitAndLoadUserAffect(t *testing.T) {
	service := NewService(newEmotionTestDB(t))
	state, err := service.Commit(context.Background(), "user-1", "char-1", UserAffect{
		PrimaryEmotion: "sadness",
		Intensity:      0.8,
		Stress:         0.7,
		Need:           "comfort",
		Severity:       0.6,
		Confidence:     0.82,
		Evidence:       []string{"今天很累"},
	}, RelationshipEmotionDelta{Concern: 0.12, Warmth: 0.03}, Signals{
		VolumeRMSMean:         0.02,
		SpeechRateCharsPerSec: 3.2,
		UtteranceDurationMS:   2000,
	})
	if err != nil {
		t.Fatalf("commit emotion: %v", err)
	}
	if state.UserAffect.PrimaryEmotion != "sadness" || state.RelationshipEmotion.Concern != 0.12 {
		t.Fatalf("unexpected committed state: %+v", state)
	}
	loaded, err := service.Load(context.Background(), "user-1", "char-1")
	if err != nil {
		t.Fatalf("load emotion: %v", err)
	}
	if loaded.UserAffect.Need != "comfort" || loaded.Baseline.SampleCount != 1 {
		t.Fatalf("unexpected loaded state: %+v", loaded)
	}
}

func TestServiceExpiresStaleUserAffect(t *testing.T) {
	db := newEmotionTestDB(t)
	service := NewService(db)
	state, err := service.Commit(context.Background(), "user-1", "char-1", UserAffect{
		PrimaryEmotion: "anger",
		Intensity:      0.7,
		Need:           "space",
		Confidence:     0.8,
	}, RelationshipEmotionDelta{}, Signals{})
	if err != nil {
		t.Fatalf("commit emotion: %v", err)
	}
	state.UserAffect.UpdatedAt = time.Now().UTC().Add(-UserAffectTTL - time.Minute)
	record, err := stateRecord(state)
	if err != nil {
		t.Fatalf("build state record: %v", err)
	}
	if err := db.Save(record).Error; err != nil {
		t.Fatalf("age state: %v", err)
	}
	loaded, err := service.Load(context.Background(), "user-1", "char-1")
	if err != nil {
		t.Fatalf("load emotion: %v", err)
	}
	if loaded.UserAffect.PrimaryEmotion != "neutral" {
		t.Fatalf("stale affect was not expired: %+v", loaded.UserAffect)
	}
}

func TestRelationshipEmotionDecayUsesDifferentHalfLives(t *testing.T) {
	now := time.Now().UTC()
	state := RelationshipEmotion{
		Anger:          0.8,
		Hurt:           0.8,
		Concern:        0.8,
		Disappointment: 0.8,
		UpdatedAt:      now.Add(-24 * time.Hour),
	}
	decayed := DecayRelationshipEmotion(state, now)
	if decayed.Concern >= decayed.Hurt || decayed.Anger >= decayed.Hurt {
		t.Fatalf("unexpected decay ordering: %+v", decayed)
	}
	if decayed.Disappointment <= decayed.Hurt {
		t.Fatalf("disappointment should decay slower than hurt: %+v", decayed)
	}
}
