package chat

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/psyche"
	"gorm.io/gorm"
)

func TestMoodRecoveryCheckInsertsMoodAfterIdleReturn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.Exec(`CREATE TABLE messages (
id TEXT PRIMARY KEY,
conversation_id TEXT,
role TEXT,
created_at TEXT DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE moods (
id INTEGER PRIMARY KEY AUTOINCREMENT,
character_id TEXT DEFAULT '',
mood TEXT DEFAULT '',
level INTEGER DEFAULT 50,
created_at TEXT DEFAULT (datetime('now'))
)`).Error; err != nil {
		t.Fatal(err)
	}
	previousAt := time.Now().Add(-15 * time.Hour).Format("2006-01-02 15:04:05")
	currentAt := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Exec("INSERT INTO messages (id, conversation_id, role, created_at) VALUES ('u1', 'c1', 'user', ?)", previousAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, role, created_at) VALUES ('u2', 'c1', 'user', ?)", currentAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO moods (character_id, mood, level, created_at) VALUES ('char-1', 'anxious', 20, ?)", previousAt).Error; err != nil {
		t.Fatal(err)
	}
	svc := &service{db: db}
	svc.moodRecoveryCheck(context.Background(), "c1", "char-1", "manual")
	var got struct {
		CharacterID string
		Mood        string
		Level       int
		CreatedAt   string
	}
	if err := db.Table("moods").Select("character_id, mood, level, created_at").Order("id DESC").Limit(1).Row().Scan(&got.CharacterID, &got.Mood, &got.Level, &got.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if got.CharacterID != "char-1" || got.Mood != "anxious" || got.Level != 35 {
		t.Fatalf("unexpected mood row: %#v", got)
	}
	if _, err := time.ParseInLocation(moodRecoveryTimeLayout, got.CreatedAt, time.Local); err != nil {
		t.Fatalf("created_at should use mood recovery layout, got %q: %v", got.CreatedAt, err)
	}
}

func TestMoodRecoveryCheckDerivesMoodFromPsycheState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.Exec(`CREATE TABLE messages (
id TEXT PRIMARY KEY,
conversation_id TEXT,
role TEXT,
created_at TEXT DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE moods (
id INTEGER PRIMARY KEY AUTOINCREMENT,
character_id TEXT DEFAULT '',
mood TEXT DEFAULT '',
level INTEGER DEFAULT 50,
created_at TEXT DEFAULT (datetime('now'))
)`).Error; err != nil {
		t.Fatal(err)
	}
	previousAt := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05")
	currentAt := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Exec("INSERT INTO messages (id, conversation_id, role, created_at) VALUES ('u1', 'c1', 'user', ?)", previousAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, role, created_at) VALUES ('u2', 'c1', 'user', ?)", currentAt).Error; err != nil {
		t.Fatal(err)
	}
	store := psyche.NewInMemoryPsycheStore()
	state := psyche.NewPsycheState("char-1")
	state.Mood.MoodValence = 0.8
	if err := store.SaveState(&state); err != nil {
		t.Fatal(err)
	}
	svc := &service{db: db, psycheStore: store}
	svc.moodRecoveryCheck(context.Background(), "c1", "char-1", "manual")
	var got struct {
		CharacterID string
		Mood        string
		Level       int
	}
	if err := db.Table("moods").Select("character_id, mood, level").Order("id DESC").Limit(1).Row().Scan(&got.CharacterID, &got.Mood, &got.Level); err != nil {
		t.Fatal(err)
	}
	if got.CharacterID != "char-1" || got.Mood != "neutral" || got.Level != 50 {
		t.Fatalf("unexpected mood row: %#v", got)
	}
}

func TestMoodRecoveryCheckSkipsWhenContextCancelled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.Exec(`CREATE TABLE messages (
id TEXT PRIMARY KEY,
conversation_id TEXT,
role TEXT,
created_at TEXT DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE moods (
id INTEGER PRIMARY KEY AUTOINCREMENT,
character_id TEXT DEFAULT '',
mood TEXT DEFAULT '',
level INTEGER DEFAULT 50,
created_at TEXT DEFAULT (datetime('now'))
)`).Error; err != nil {
		t.Fatal(err)
	}
	previousAt := time.Now().Add(-7 * time.Hour).Format("2006-01-02 15:04:05")
	currentAt := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Exec("INSERT INTO messages (id, conversation_id, role, created_at) VALUES ('u1', 'c1', 'user', ?)", previousAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, role, created_at) VALUES ('u2', 'c1', 'user', ?)", currentAt).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := &service{db: db}
	svc.moodRecoveryCheck(ctx, "c1", "char-1", "manual")
	var count int64
	if err := db.Table("moods").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no mood rows after cancellation, got %d", count)
	}
}
