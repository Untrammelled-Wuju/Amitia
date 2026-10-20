package migration

import "testing"

func TestCitationSnapshotFreshBaselineAndRegistration(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"messages", "knowledge_sources", "knowledge_units", "knowledge_chunks", "knowledge_source_revisions", "world_book"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("fresh database lacks table %s", table)
		}
	}
	if !db.Migrator().HasColumn("messages", "citation_snapshot_json") {
		t.Fatal("fresh baseline has no immutable citation field")
	}
	migrations := []Migration{MessageCitationSnapshotMigration()}
	if err := MarkAllMigrationsApplied(db, migrations); err != nil {
		t.Fatal(err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply(migrations); err != nil {
		t.Fatalf("registered baseline migration checksum mismatch: %v", err)
	}
	var count int64
	if err := db.Table("schema_migrations").Where("version = ? AND status = ?", migrations[0].Version, "applied").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("citation migration not marked once: count=%d err=%v", count, err)
	}
}

func TestCitationSnapshotUpgradeLegacyMessageIsNonDestructive(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := db.Exec("CREATE TABLE messages (id TEXT PRIMARY KEY, content TEXT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, content) VALUES ('legacy-message', 'historic content')").Error; err != nil {
		t.Fatal(err)
	}
	migrations := []Migration{MessageCitationSnapshotMigration()}
	runner := Runner{DB: db, SkipBackup: true}
	for i := 0; i < 2; i++ {
		if err := runner.Apply(migrations); err != nil {
			t.Fatalf("upgrade iteration %d: %v", i, err)
		}
	}
	var row struct {
		Content  string `gorm:"column:content"`
		Snapshot string `gorm:"column:citation_snapshot_json"`
	}
	if err := db.Table("messages").Select("content", "citation_snapshot_json").Where("id = ?", "legacy-message").Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Content != "historic content" || row.Snapshot != "[]" {
		t.Fatalf("upgrade lost legacy message: %+v", row)
	}
	var count int64
	if err := db.Raw("SELECT count(*) FROM pragma_table_info('messages') WHERE name = 'citation_snapshot_json'").Scan(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicate or missing column: count=%d err=%v", count, err)
	}
}

func TestKnowledgeBootstrapDoesNotRebuildWorldBook(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := db.Exec("CREATE TABLE world_book (id TEXT PRIMARY KEY, character_id TEXT, content TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO world_book (id, character_id, content) VALUES ('rule-1', 'character-a', 'retain-original')").Error; err != nil {
		t.Fatal(err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{KnowledgeBootstrapLatestMigration()}); err != nil {
		t.Fatal(err)
	}
	var row struct {
		Content string `gorm:"column:content"`
	}
	if err := db.Table("world_book").Select("content").Where("id = ?", "rule-1").Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Content != "retain-original" {
		t.Fatalf("knowledge migration replaced world book: %q", row.Content)
	}
}
