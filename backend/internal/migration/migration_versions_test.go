package migration

import "testing"

func TestDefaultMigrationVersionsAreUnique(t *testing.T) {
	seen := make(map[string]string)
	for _, item := range DefaultMigrations() {
		if item.Version == "" {
			t.Fatalf("migration %q has an empty version", item.Name)
		}
		if previousName, exists := seen[item.Version]; exists {
			t.Fatalf("duplicate migration version %s: %q and %q", item.Version, previousName, item.Name)
		}
		seen[item.Version] = item.Name
	}
}

func TestDesktopPetFinalizationMigrationVersionsUseCanonicalFormat(t *testing.T) {
	want := map[string]string{
		"desktop_pet_action_revision_source_index_fix":            "202608290001",
		"add_app_settings_deleted_at_column":                      "202608290002",
		"finalize_desktop_pet_quality_inbox_idempotency":          "202608290003",
		"finalize_desktop_pet_editing_canonical_lineage":          "202608290004",
		"finalize_desktop_pet_schema_model_alignment":             "202608290005",
		"finalize_desktop_pet_behavior_decision_recovery":         "202608290006",
		"desktop_pet_processing_ownership_backfill":               "202608300001",
		"finalize_desktop_pet_runtime_geometry_and_behavior_mesh": "202608300002",
		"finalize_desktop_pet_behavior_reducer_dedup":             "202608310001",
		"finalize_desktop_pet_behavior_inbox_tenant_dedup":        "202608310002",
		"repair_desktop_pet_behavior_v2_columns":                  "202608310003",
		"repair_legacy_action_revision_data_to_stream":            "202608310004",
	}

	seen := make(map[string]string)
	for _, item := range DefaultMigrations() {
		if expected, ok := want[item.Name]; ok {
			if item.Version != expected {
				t.Fatalf("migration %q version = %q, want %q", item.Name, item.Version, expected)
			}
			seen[item.Name] = item.Version
		}
	}
	for name := range want {
		if _, ok := seen[name]; !ok {
			t.Fatalf("migration %q is not registered in DefaultMigrations", name)
		}
	}
}
