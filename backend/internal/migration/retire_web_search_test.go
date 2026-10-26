package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRetireWebSearchDropsOnlySearchStorage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "search-retire.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, table := range []string{
		"web_references",
		"web_pages",
		"web_page_versions",
		"web_evidence",
		"web_turn_citations",
		"web_turn_citation_evidence",
		"search_api_keys",
		"search_credential_cleanup",
		"messages",
	} {
		if err := db.Exec("CREATE TABLE " + table + "(id TEXT PRIMARY KEY)").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("DROP TABLE search_api_keys").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE search_api_keys(engine_id TEXT PRIMARY KEY, api_key TEXT NOT NULL, updated_at DATETIME NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO search_api_keys(engine_id, api_key, updated_at) VALUES('serper','secret://search/serper',CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP TABLE search_credential_cleanup").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE search_credential_cleanup(secret_ref TEXT PRIMARY KEY, engine_id TEXT NOT NULL, created_at DATETIME NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO search_credential_cleanup(secret_ref, engine_id, created_at) VALUES('mcp-secret://search/youtube','youtube',CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{RetireWebSearchMigration()}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{
		"web_references",
		"web_pages",
		"web_page_versions",
		"web_evidence",
		"web_turn_citations",
		"web_turn_citation_evidence",
		"search_api_keys",
		"search_credential_cleanup",
	} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("retired table retained: %s", table)
		}
	}
	if !db.Migrator().HasTable("messages") {
		t.Fatal("retirement removed unrelated chat storage")
	}
	var refs int64
	if err := db.Table("retired_secret_cleanup").Count(&refs).Error; err != nil || refs != 2 {
		t.Fatalf("retired secret refs = %d, err=%v", refs, err)
	}
}
