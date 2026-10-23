package migration

import "testing"

func TestSafetyGovernanceFreshBaselineHasNoLegacyEventsTable(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable("safety_events") {
		t.Fatal("fresh baseline recreated the retired safety events table")
	}
	step := &Step{db: db}
	if err := SafetyGovernanceMigration().Up(step); err != nil {
		t.Fatal(err)
	}
	for _, command := range step.commands {
		if err := db.Exec(command).Error; err != nil {
			t.Fatal(err)
		}
	}
	if !db.Migrator().HasTable("safety_policies") || !db.Migrator().HasTable("safety_incidents") {
		t.Fatal("scoped safety governance tables were not created")
	}
}
