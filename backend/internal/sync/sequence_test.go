package sync

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSequenceGeneratorAdvancesPastLegacyChangeLog(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:sync-sequence-legacy?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE sync_changes (seq INTEGER NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE sync_sequence (id INTEGER PRIMARY KEY, seq INTEGER NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sync_changes (seq) VALUES (8)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sync_sequence (id, seq) VALUES (1, 0)").Error; err != nil {
		t.Fatal(err)
	}

	seq, err := NewSequenceGenerator(db).NextSequence()
	if err != nil {
		t.Fatal(err)
	}
	if seq != 9 {
		t.Fatalf("sequence = %d, want 9", seq)
	}
}

func TestSequenceGeneratorInitializesMissingCounterPastLegacyChangeLog(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:sync-sequence-missing?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE sync_changes (seq INTEGER NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE sync_sequence (id INTEGER PRIMARY KEY, seq INTEGER NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sync_changes (seq) VALUES (12)").Error; err != nil {
		t.Fatal(err)
	}

	seq, err := NewSequenceGenerator(db).NextSequence()
	if err != nil {
		t.Fatal(err)
	}
	if seq != 13 {
		t.Fatalf("sequence = %d, want 13", seq)
	}
}
