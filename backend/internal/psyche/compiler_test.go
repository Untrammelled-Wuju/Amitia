package psyche

import (
	"reflect"
	"testing"
)

func TestCompilePersonalityDefaults(t *testing.T) {
	profile := CompilePersonality(DefaultConfig())

	if profile.CompilerVersion != DefaultCompilerVersion {
		t.Fatalf("unexpected compiler version: %s", profile.CompilerVersion)
	}
	if profile.Resolved.SchemaVersion != DefaultSchemaVersion {
		t.Fatalf("unexpected schema version: %s", profile.Resolved.SchemaVersion)
	}
	if profile.Resolved.Initiative != 50 || profile.Resolved.Warmth != 65 || profile.Resolved.Stability != 60 {
		t.Fatalf("unexpected defaults: %#v", profile.Resolved)
	}
	if profile.Sources["initiative"] != "default" || profile.Sources["schemaVersion"] != "default" {
		t.Fatalf("unexpected sources: %#v", profile.Sources)
	}
	if len(profile.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", profile.Diagnostics)
	}
	if profile.Internal.StableCore.SocialInitiative != 0.5 {
		t.Fatalf("unexpected internal initiative: %#v", profile.Internal.StableCore)
	}
	if profile.Expression.MinReplyChars < 18 || profile.Expression.MaxReplyChars > 240 {
		t.Fatalf("unexpected expression bounds: %#v", profile.Expression)
	}
	if profile.Behavior.InitiationThreshold < 0.15 || profile.Behavior.InitiationThreshold > 0.85 {
		t.Fatalf("unexpected initiation threshold: %#v", profile.Behavior)
	}
}

func TestCompilePersonalityClampsBoundaryValues(t *testing.T) {
	low := -20.0
	high := 180.0
	profile := CompilePersonality(PersonalityConfig{
		Initiative:        &low,
		Sensitivity:       &high,
		Tolerance:         &low,
		Stability:         &high,
		Boundary:          &high,
		Warmth:            &low,
		Directness:        &high,
		Humor:             &low,
		Affection:         &high,
		Verbosity:         &high,
		ConflictAvoidance: &low,
	})

	if profile.Resolved.Initiative != 0 || profile.Resolved.Sensitivity != 100 || profile.Resolved.Warmth != 0 {
		t.Fatalf("unexpected resolved clamp values: %#v", profile.Resolved)
	}
	if profile.Sources["initiative"] != "user_clamped" || profile.Sources["sensitivity"] != "user_clamped" {
		t.Fatalf("unexpected source tracking: %#v", profile.Sources)
	}
	if len(profile.Diagnostics) != 11 {
		t.Fatalf("expected 11 clamp diagnostics, got %#v", profile.Diagnostics)
	}
	if profile.Appraisal.AmplificationCap < 1.15 || profile.Appraisal.AmplificationCap > 2.2 {
		t.Fatalf("unexpected amplification cap: %#v", profile.Appraisal)
	}
	if profile.Recovery.EmotionHalfLifeHours < 6 || profile.Recovery.EmotionHalfLifeHours > 24 {
		t.Fatalf("unexpected emotion half life: %#v", profile.Recovery)
	}
	if profile.Recovery.MaxRecoveryRate < profile.Recovery.MinRecoveryRate {
		t.Fatalf("unexpected recovery rates: %#v", profile.Recovery)
	}
	if profile.Expression.MinReplyChars > profile.Expression.MaxReplyChars {
		t.Fatalf("unexpected expression chars: %#v", profile.Expression)
	}
}

func TestCompilePersonalityStability(t *testing.T) {
	initiative := 72.0
	sensitivity := 68.0
	tolerance := 20.0
	stability := 31.0
	boundary := 77.0
	warmth := 84.0
	directness := 58.0
	humor := 41.0
	affection := 63.0
	verbosity := 37.0
	conflictAvoidance := 66.0

	cfg := PersonalityConfig{
		SchemaVersion:     "personality-config-v2-preview",
		Initiative:        &initiative,
		Sensitivity:       &sensitivity,
		Tolerance:         &tolerance,
		Stability:         &stability,
		Boundary:          &boundary,
		Warmth:            &warmth,
		Directness:        &directness,
		Humor:             &humor,
		Affection:         &affection,
		Verbosity:         &verbosity,
		ConflictAvoidance: &conflictAvoidance,
	}

	first := CompilePersonality(cfg)
	second := CompilePersonality(cfg)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("compile result is not stable\nfirst=%#v\nsecond=%#v", first, second)
	}
	if first.Sources["schemaVersion"] != "user" {
		t.Fatalf("unexpected schema source: %#v", first.Sources)
	}
}

func TestCompilePersonalityTraitInfluence(t *testing.T) {
	lowInitiative := 10.0
	highInitiative := 90.0
	lowStability := 20.0
	highStability := 90.0

	passive := CompilePersonality(PersonalityConfig{
		Initiative: &lowInitiative,
		Stability:  &lowStability,
	})
	active := CompilePersonality(PersonalityConfig{
		Initiative: &highInitiative,
		Stability:  &highStability,
	})

	if active.Behavior.InitiateWeight <= passive.Behavior.InitiateWeight {
		t.Fatalf("initiative weight did not increase: passive=%#v active=%#v", passive.Behavior, active.Behavior)
	}
	if active.Behavior.InitiationThreshold >= passive.Behavior.InitiationThreshold {
		t.Fatalf("initiation threshold did not decrease: passive=%#v active=%#v", passive.Behavior, active.Behavior)
	}
	if active.Recovery.EmotionHalfLifeHours >= passive.Recovery.EmotionHalfLifeHours {
		t.Fatalf("recovery did not improve with stability: passive=%#v active=%#v", passive.Recovery, active.Recovery)
	}
}

func TestMigratePersonalityConfigFillsMissingWithoutOverwritingUserValues(t *testing.T) {
	initiative := 73.0
	cfg := PersonalityConfig{
		SchemaVersion: "legacy-personality-v0",
		Initiative:    &initiative,
	}

	migrated, migration := MigratePersonalityConfig(cfg)

	if migrated.SchemaVersion != DefaultSchemaVersion || migrated.PersonalitySchemaVersion != DefaultSchemaVersion {
		t.Fatalf("schema version not migrated: %#v", migrated)
	}
	if migrated.Initiative == nil || *migrated.Initiative != initiative {
		t.Fatalf("user initiative was overwritten: %#v", migrated.Initiative)
	}
	if migrated.Warmth == nil || *migrated.Warmth != 65 {
		t.Fatalf("missing warmth was not defaulted: %#v", migrated.Warmth)
	}
	if migration.FromSchema != "legacy-personality-v0" || migration.ToSchema != DefaultSchemaVersion {
		t.Fatalf("unexpected migration schema: %#v", migration)
	}
	if migration.Snapshot.SchemaVersion != "legacy-personality-v0" || migration.Snapshot.Initiative == nil || *migration.Snapshot.Initiative != initiative {
		t.Fatalf("migration snapshot not preserved: %#v", migration.Snapshot)
	}
	if migration.Sources["initiative"] != "user" || migration.Sources["warmth"] != "default:"+DefaultSchemaVersion {
		t.Fatalf("unexpected migration sources: %#v", migration.Sources)
	}
	if len(migration.Diagnostics) != 1 || migration.Diagnostics[0] != "personality_schema_migrated" {
		t.Fatalf("unexpected diagnostics: %#v", migration.Diagnostics)
	}
}

func TestCompilePersonalityUsesMigrationSources(t *testing.T) {
	verbosity := 88.0
	profile := CompilePersonality(PersonalityConfig{
		PersonalitySchemaVersion: "legacy-personality-v0",
		Verbosity:                &verbosity,
	})

	if profile.Resolved.SchemaVersion != DefaultSchemaVersion {
		t.Fatalf("unexpected resolved schema: %#v", profile.Resolved)
	}
	if profile.Sources["verbosity"] != "user" || profile.Sources["warmth"] != "default" {
		t.Fatalf("unexpected compile sources: %#v", profile.Sources)
	}
	if profile.Migration.Snapshot.PersonalitySchemaVersion != "legacy-personality-v0" {
		t.Fatalf("missing migration snapshot: %#v", profile.Migration)
	}
	if profile.Migration.Sources["warmth"] != "default:"+DefaultSchemaVersion {
		t.Fatalf("missing migration source tracking: %#v", profile.Migration.Sources)
	}
}

func TestMigratePersonalityConfigJSONPreservesUnknownFields(t *testing.T) {
	raw := []byte(`{"personality_schema_version":"legacy-personality-v0","initiative":62,"futureTrait":{"weight":3},"dailyLimit":5}`)

	migrated, migration, err := MigratePersonalityConfigJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated.Initiative == nil || *migrated.Initiative != 62 {
		t.Fatalf("known field not migrated: %#v", migrated)
	}
	if len(migration.UnknownFields) != 2 {
		t.Fatalf("unknown fields not preserved: %#v", migration.UnknownFields)
	}
	if string(migration.UnknownFields["futureTrait"]) != `{"weight":3}` || string(migration.UnknownFields["dailyLimit"]) != `5` {
		t.Fatalf("unexpected unknown field payloads: %#v", migration.UnknownFields)
	}
}
