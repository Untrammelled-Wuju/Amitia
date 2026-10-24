package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRuntimeBudgetMigrationPreservesLegacyAndBaselineStartup(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh", true: "legacy"}[legacy], func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "budget.db")), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			connection, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			if legacy {
				if err := db.Exec(`CREATE TABLE runtime_model_usage (sequence INTEGER PRIMARY KEY AUTOINCREMENT,id TEXT NOT NULL,space_id TEXT NOT NULL,device_id TEXT,owner_id TEXT,role_id TEXT,request_id TEXT,model TEXT,provider TEXT,source TEXT,status TEXT,started_at TEXT,completed_at TEXT,input_tokens INTEGER,output_tokens INTEGER,cached_tokens INTEGER,total_tokens INTEGER,usage_reported NUMERIC)`).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Exec(`INSERT INTO runtime_model_usage(id,space_id,total_tokens,status) VALUES ('legacy-call','core',77,'succeeded')`).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := ApplyBaseline(db); err != nil {
				t.Fatal(err)
			}
			if !legacy && !db.Migrator().HasIndex("runtime_model_usage", "idx_model_usage_coord") {
				t.Fatal("fresh baseline is missing budget index before marking migrations")
			}
			runner := Runner{DB: db, SkipBackup: true}
			migrations := []Migration{ModelBudgetRatesMigration(), RuntimeModelBudgetMigration()}
			for attempt := 0; attempt < 2; attempt++ {
				if err := runner.Apply(migrations); err != nil {
					t.Fatal(err)
				}
			}
			for _, column := range []string{"coordination_id", "reserved_tokens", "reserved_cost_micro_usd", "budget_overrun", "review_evidence_ref", "review_actor_id", "review_recorded_at"} {
				if !db.Migrator().HasColumn("runtime_model_usage", column) {
					t.Fatalf("missing ledger column %s", column)
				}
			}
			if legacy {
				var row struct {
					TotalTokens       int64
					ReservedTokens    int64
					ReviewEvidenceRef string
				}
				if err := db.Table("runtime_model_usage").Where("id=?", "legacy-call").Take(&row).Error; err != nil {
					t.Fatal(err)
				}
				if row.TotalTokens != 77 || row.ReservedTokens != 0 || row.ReviewEvidenceRef != "" {
					t.Fatalf("legacy metering changed: %+v", row)
				}
			}
			if !db.Migrator().HasIndex("runtime_model_usage", "idx_model_usage_coord") {
				t.Fatal("missing migrated budget index")
			}
			var count int64
			if err := db.Model(&Record{}).Where("version IN ?", []string{"20261024001", "20261024002"}).Count(&count).Error; err != nil || count != 2 {
				t.Fatalf("unexpected migration receipts %d %v", count, err)
			}
		})
	}
}
