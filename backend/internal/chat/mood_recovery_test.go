package chat

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
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
	previousAt := time.Now().Add(-7 * time.Hour).Format("2006-01-02 15:04:05")
	currentAt := time.Now().Format("2006-01-02 15:04:05")
	if err := db.Exec("INSERT INTO messages (id, conversation_id, role, created_at) VALUES ('u1', 'c1', 'user', ?)", previousAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, role, created_at) VALUES ('u2', 'c1', 'user', ?)", currentAt).Error; err != nil {
		t.Fatal(err)
	}
	svc := &service{db: db}
	svc.moodRecoveryCheck("c1", "char-1", "manual")
	var got struct {
		CharacterID string
		Mood        string
		Level       int
	}
	if err := db.Table("moods").Select("character_id, mood, level").Row().Scan(&got.CharacterID, &got.Mood, &got.Level); err != nil {
		t.Fatal(err)
	}
	if got.CharacterID != "char-1" || got.Mood != "happy" || got.Level != 7 {
		t.Fatalf("unexpected mood row: %#v", got)
	}
}
