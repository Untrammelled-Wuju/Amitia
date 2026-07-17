package interaction

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func TestRuntimeInputLoadersLoadRoleLifeNeedAndUnresolvedThreads(t *testing.T) {
	db := openRuntimeLoaderTestDB(t)
	createRuntimeLoaderTestSchema(t, db)
	insertRuntimeLoaderTestData(t, db)

	reg := NewContextLoaderRegistry()
	reg.Register(NewRoleRuntimeProfileContextLoader(character.NewRepository(app.NewAppContext(db, nil))))
	reg.Register(NewLifeContextLoader(db))
	reg.Register(NewNeedContextLoader(db))
	reg.Register(NewUnresolvedThreadContextLoader(db))

	snapshot := reg.LoadAll(context.Background(), InteractionScope{CharacterID: "char-runtime", UserID: "user-1"}, "v-test")

	if snapshot.RuntimeProfile.Status != LoadStatusReady {
		t.Fatalf("expected runtime profile ready, got %s", snapshot.RuntimeProfile.Status)
	}
	if snapshot.RuntimeProfile.Value.CharacterID != "char-runtime" || snapshot.RuntimeProfile.Value.Name != "Amitia" {
		t.Fatalf("unexpected runtime profile: %#v", snapshot.RuntimeProfile.Value)
	}
	if snapshot.RuntimeProfile.Value.PersonalityConfig["version"] != "runtime-profile-v1" {
		t.Fatalf("expected personality config version, got %#v", snapshot.RuntimeProfile.Value.PersonalityConfig)
	}
	if snapshot.Life.Status != LoadStatusReady {
		t.Fatalf("expected life ready, got %s", snapshot.Life.Status)
	}
	if snapshot.Life.Value.Mood != "calm" || snapshot.Life.Value.Energy != 0.72 {
		t.Fatalf("unexpected life state: %#v", snapshot.Life.Value)
	}
	if len(snapshot.Life.Value.Needs) != 2 {
		t.Fatalf("expected life needs loaded, got %#v", snapshot.Life.Value.Needs)
	}
	if snapshot.Needs.Status != LoadStatusReady || snapshot.Needs.Value.Count != 2 {
		t.Fatalf("expected two needs, got %#v", snapshot.Needs)
	}
	if snapshot.Needs.Value.Needs[0].Kind != "connection" || snapshot.Needs.Value.Needs[0].Level != 0.81 {
		t.Fatalf("unexpected first need: %#v", snapshot.Needs.Value.Needs)
	}
	if snapshot.UnresolvedThreads.Status != LoadStatusReady || snapshot.UnresolvedThreads.Value.Count != 1 {
		t.Fatalf("expected one unresolved thread, got %#v", snapshot.UnresolvedThreads)
	}
	if snapshot.UnresolvedThreads.Value.Threads[0].Topic != "boundary repair" {
		t.Fatalf("unexpected unresolved thread: %#v", snapshot.UnresolvedThreads.Value.Threads)
	}
}

func TestRuntimeBudgetIncludesNewRuntimeInputs(t *testing.T) {
	snapshot := ContextSnapshot{
		RuntimeProfile:    FieldReady(RuntimeProfile{CharacterID: "char-runtime"}, "runtimeProfile", "v1"),
		Life:              FieldReady(LifeState{Mood: "calm"}, "life", "v1"),
		Needs:             FieldReady(NeedState{Count: 1}, "needs", "v1"),
		UnresolvedThreads: FieldReady(UnresolvedThreadSet{Count: 1}, "unresolvedThreads", "v1"),
	}

	modules := runtimeBudgetModules(snapshot, PathTypeStandard)
	names := map[string]bool{}
	for _, mod := range modules {
		names[mod.Name] = true
	}
	for _, name := range []string{"runtime_profile", "life", "needs", "unresolved_threads"} {
		if !names[name] {
			t.Fatalf("expected budget module %s in %#v", name, modules)
		}
	}
}

func TestPsycheContextLoaderReadsEmotionMoodJSON(t *testing.T) {
	db := openRuntimeLoaderTestDB(t)
	if err := db.Exec(`CREATE TABLE psyche_states (
		character_id TEXT,
		emotion TEXT DEFAULT '{}',
		mood TEXT DEFAULT '{}',
		stress REAL,
		energy REAL,
		updated_at TEXT
	)`).Error; err != nil {
		t.Fatalf("create psyche_states: %v", err)
	}
	if err := db.Exec(`INSERT INTO psyche_states (character_id, emotion, mood, stress, energy, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"char-psyche", `{"valence":0.7,"arousal":0.6,"dominance":0.4}`,
		`{"moodValence":0.5,"moodArousal":0.3}`, 0.3, 0.85, "2026-07-08").Error; err != nil {
		t.Fatalf("insert psyche: %v", err)
	}

	loader := NewPsycheContextLoader(db)
	state, err := loader.Load(context.Background(), InteractionScope{CharacterID: "char-psyche"}, "v-test")
	if err != nil {
		t.Fatalf("load psyche: %v", err)
	}
	ready, ok := state.Value.(PsycheState)
	if !ok {
		t.Fatalf("expected PsycheState, got %T", state.Value)
	}
	if ready.Valence != 0.7 {
		t.Errorf("expected Valence=0.7, got %f", ready.Valence)
	}
	if ready.Dominance != 0.4 {
		t.Errorf("expected Dominance=0.4, got %f", ready.Dominance)
	}
	if ready.MoodValence != 0.5 {
		t.Errorf("expected MoodValence=0.5, got %f", ready.MoodValence)
	}
	if ready.MoodArousal != 0.3 {
		t.Errorf("expected MoodArousal=0.3, got %f", ready.MoodArousal)
	}
	if ready.Stress != 0.3 {
		t.Errorf("expected Stress=0.3, got %f", ready.Stress)
	}
	if ready.Fatigue >= 0.18 {
		t.Errorf("expected Fatigue<0.18 from energy=0.85, got %f", ready.Fatigue)
	}
}

func openRuntimeLoaderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "runtime_loader.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sqlite handle: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

func createRuntimeLoaderTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE characters (
			id TEXT PRIMARY KEY,
			name TEXT,
			identity TEXT,
			personality TEXT,
			speaking_style TEXT,
			relationship_style TEXT,
			system_prompt TEXT,
			boundary_rules TEXT,
			personality_sliders TEXT,
			base_prompt TEXT,
			generated_prompt TEXT,
			personality_config TEXT,
			chat_style_config TEXT,
			scene_rules TEXT,
			gender TEXT,
			pronoun TEXT,
			self_reference TEXT,
			gender_expression INTEGER,
			life_identity TEXT,
			status TEXT,
			is_default INTEGER,
			sort_order INTEGER,
			created_at TEXT
		)`,
		`CREATE TABLE moods (
			character_id TEXT,
			mood TEXT,
			mood_value TEXT,
			created_at TEXT
		)`,
		`CREATE TABLE psyche_states (
			character_id TEXT,
			emotion TEXT DEFAULT '{}',
			mood TEXT DEFAULT '{}',
			stress REAL,
			energy REAL,
			updated_at TEXT
		)`,
		`CREATE TABLE need_states (
			character_id TEXT,
			need_key TEXT,
			current_value REAL,
			baseline REAL,
			updated_at TEXT
		)`,
		`CREATE TABLE unresolved_threads (
			id TEXT PRIMARY KEY,
			character_id TEXT,
			user_id TEXT,
			topic TEXT,
			reason TEXT,
			severity REAL,
			escalation_level INTEGER,
			created_at TEXT,
			resolved_at TEXT
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
}

func insertRuntimeLoaderTestData(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`INSERT INTO characters (
		id, name, identity, personality, speaking_style, relationship_style, system_prompt,
		boundary_rules, personality_sliders, base_prompt, generated_prompt, personality_config,
		chat_style_config, scene_rules, gender, pronoun, self_reference, gender_expression,
		life_identity, status, is_default, sort_order, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"char-runtime", "Amitia", "assistant", "warm", "direct", "steady", "system",
		"boundaries", "{}", "base", "generated", `{"openness":0.7}`,
		`{"version":"chat-v1"}`, `{"version":"scene-v1"}`, "FEMALE", "她", "我", 60,
		"CUSTOM", "enabled", 1, 1, "2026-07-01 10:00:00").Error; err != nil {
		t.Fatalf("insert character: %v", err)
	}
	if err := db.Exec(`INSERT INTO moods (character_id, mood, mood_value, created_at) VALUES (?, ?, ?, ?)`,
		"char-runtime", "neutral", "calm", "2026-07-01 11:00:00").Error; err != nil {
		t.Fatalf("insert mood: %v", err)
	}
	if err := db.Exec(`INSERT INTO psyche_states (character_id, emotion, mood, stress, energy, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"char-runtime", `{"valence":0.5,"arousal":0.5,"dominance":0.5}`, `{"moodValence":0.5,"moodArousal":0.5}`, 0.2, 0.72, "2026-07-01 11:00:00").Error; err != nil {
		t.Fatalf("insert psyche: %v", err)
	}
	for _, row := range []struct {
		key      string
		current  float64
		baseline float64
	}{
		{"connection", 0.81, 0.5},
		{"rest", 0.33, 0.6},
	} {
		if err := db.Exec(`INSERT INTO need_states (character_id, need_key, current_value, baseline, updated_at) VALUES (?, ?, ?, ?, ?)`,
			"char-runtime", row.key, row.current, row.baseline, "2026-07-01 12:00:00").Error; err != nil {
			t.Fatalf("insert need: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO unresolved_threads (id, character_id, user_id, topic, reason, severity, escalation_level, created_at, resolved_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		"thr-1", "char-runtime", "user-1", "boundary repair", "pending apology", 0.67, 2, "2026-07-01 09:00:00").Error; err != nil {
		t.Fatalf("insert unresolved thread: %v", err)
	}
}
