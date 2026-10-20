package migration

import "testing"

func TestPhase22KnowledgeOutboxIndexFreshAndUpgradeMatch(t *testing.T) {
	checkIndex := func(t *testing.T, columns []string) {
		t.Helper()
		want := []string{"tenant_id", "aggregate_type", "aggregate_id", "source_revision"}
		if len(columns) != len(want) {
			t.Fatalf("outbox index columns = %v, want %v", columns, want)
		}
		for i := range want {
			if columns[i] != want[i] {
				t.Fatalf("outbox index columns = %v, want %v", columns, want)
			}
		}
	}
	for _, scenario := range []string{"fresh", "legacy"} {
		t.Run(scenario, func(t *testing.T) {
			db := openInitialSQLTestDB(t)
			if err := ApplyBaseline(db); err != nil {
				t.Fatal(err)
			}
			if scenario == "legacy" {
				if err := db.Exec("DROP INDEX IF EXISTS idx_knowledge_outbox_aggregate").Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Exec("CREATE INDEX idx_knowledge_outbox_aggregate ON knowledge_index_outbox(tenant_id, aggregate_type, aggregate_id)").Error; err != nil {
					t.Fatal(err)
				}
			}
			migrations := []Migration{KnowledgeSchemaContractRepairMigration()}
			runner := Runner{DB: db, SkipBackup: true}
			for i := 0; i < 2; i++ {
				if err := runner.Apply(migrations); err != nil {
					t.Fatalf("migration replay %d: %v", i, err)
				}
			}
			var rows []struct {
				Name string `gorm:"column:name"`
			}
			if err := db.Raw("PRAGMA index_info(idx_knowledge_outbox_aggregate)").Scan(&rows).Error; err != nil {
				t.Fatal(err)
			}
			columns := make([]string, 0, len(rows))
			for _, row := range rows {
				columns = append(columns, row.Name)
			}
			checkIndex(t, columns)
			var applied int64
			if err := db.Table("schema_migrations").Where("version = ? AND status = ?", migrations[0].Version, "applied").Count(&applied).Error; err != nil || applied != 1 {
				t.Fatalf("migration history count=%d err=%v", applied, err)
			}
		})
	}
}
