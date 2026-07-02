package psyche

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newSQLitePsycheStoreForTest(t *testing.T) *SQLitePsycheStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "psyche.db")), &gorm.Config{})
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
	store := NewSQLitePsycheStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSQLitePsycheStoreSaveStateInsertsMissingState(t *testing.T) {
	store := newSQLitePsycheStoreForTest(t)
	state := NewPsycheState("char-sqlite-insert")
	state.Stress = 0.25

	if err := store.SaveState(&state); err != nil {
		t.Fatalf("save insert failed: %v", err)
	}
	loaded, err := store.LoadState("char-sqlite-insert")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.StateVersion != 1 {
		t.Fatalf("expected initial version 1, got %d", loaded.StateVersion)
	}
	if loaded.Stress != 0.25 {
		t.Fatalf("stress not persisted: %f", loaded.Stress)
	}
	if loaded.Emotion.Valence != state.Emotion.Valence || loaded.Mood.MoodValence != state.Mood.MoodValence {
		t.Fatalf("json dimensions not persisted: %#v %#v", loaded.Emotion, loaded.Mood)
	}
}

func TestSQLitePsycheStoreSaveStateUsesCASVersion(t *testing.T) {
	store := newSQLitePsycheStoreForTest(t)
	state := NewPsycheState("char-sqlite-cas")
	if err := store.SaveState(&state); err != nil {
		t.Fatal(err)
	}

	first, err := store.LoadState("char-sqlite-cas")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.LoadState("char-sqlite-cas")
	if err != nil {
		t.Fatal(err)
	}
	first.Stress = 0.4
	if err := store.SaveState(first); err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	second.Stress = 0.8
	err = store.SaveState(second)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	loaded, err := store.LoadState("char-sqlite-cas")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.StateVersion != 2 || loaded.Stress != 0.4 {
		t.Fatalf("unexpected final state: version=%d stress=%f", loaded.StateVersion, loaded.Stress)
	}
}
