package migration

import "testing"

func TestWorldBookOrphanAuditPreservesSourceAndReplays(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, role string }{{"unowned", ""}, {"blank", " "}, {"owned", "role-1"}} {
		if err := db.Exec("INSERT INTO world_book(id,space_id,character_id,inject_content) VALUES (?, 'space-1', ?, 'original')", row.id, row.role).Error; err != nil {
			t.Fatal(err)
		}
	}
	runner := Runner{DB: db, SkipBackup: true}
	for i := 0; i < 2; i++ {
		if err := runner.Apply([]Migration{WorldBookOrphanAuditLatestMigration()}); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Table("world_book_orphans").Where("space_id = ? AND inject_content = ?", "space-1", "original").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("orphans=%d err=%v", count, err)
	}
	if err := db.Table("world_book").Where("inject_content = ?", "original").Count(&count).Error; err != nil || count != 3 {
		t.Fatalf("source rows=%d err=%v", count, err)
	}
	found := false
	for _, m := range DefaultMigrations() {
		if m.Name == WorldBookOrphanAuditLatestMigration().Name {
			found = true
		}
	}
	if !found {
		t.Fatal("migration not registered")
	}
}
