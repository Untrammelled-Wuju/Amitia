package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite"
)

func TestWebResearchEvidenceSchemaMigration(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "webresearch-migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"web_page_versions", "web_turn_citation_evidence"} {
		var count int
		if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %s was not created", table)
		}
	}
	var pageContentHashCount int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM pragma_table_info('web_evidence') WHERE name = 'page_content_hash'").Scan(&pageContentHashCount); err != nil {
		t.Fatal(err)
	}
	if pageContentHashCount != 1 {
		t.Fatal("web_evidence.page_content_hash was not created")
	}
}
