package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/pkg/app"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRuntimeContextLoaderRegistryRegistersCompleteRuntimeInputs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "services_runtime.db")), &gorm.Config{})
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
	createServicesRuntimeCharacterSchema(t, db)
	if err := db.Exec(`INSERT INTO characters (id, name, personality_config, chat_style_config, scene_rules, is_default, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"char-runtime", "Amitia", `{}`, `{}`, `{}`, 1, 1, "2026-07-01 10:00:00").Error; err != nil {
		t.Fatalf("insert character: %v", err)
	}

	appCtx := app.NewAppContext(db, nil)
	registry := newRuntimeContextLoaderRegistry(appCtx, character.NewRepository(appCtx))
	registry.LoadAll(context.Background(), interaction.InteractionScope{CharacterID: "char-runtime", ConversationID: "conv-runtime", Channel: "web"}, "v-test")

	registered := map[string]bool{}
	for _, stat := range registry.Stats() {
		registered[stat.Name] = true
	}
	for _, name := range []string{"runtimeProfile", "channel", "conversation", "psyche", "relationship", "beliefs", "life", "needs", "unresolvedThreads"} {
		if !registered[name] {
			t.Fatalf("expected runtime loader %s to be registered, got %#v", name, registered)
		}
	}
}

func createServicesRuntimeCharacterSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE characters (
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
		is_default INTEGER,
		sort_order INTEGER,
		created_at TEXT
	)`).Error; err != nil {
		t.Fatalf("create characters schema: %v", err)
	}
	statements := []string{
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, message_count INTEGER DEFAULT 0, updated_at TEXT)`,
		`CREATE TABLE psyche_states (character_id TEXT, stress REAL, energy REAL, updated_at TEXT)`,
		`CREATE TABLE relationship_states (character_id TEXT, relation_data TEXT, updated_at TEXT)`,
		`CREATE TABLE memories (character_id TEXT, key TEXT, value TEXT, confidence REAL, importance REAL, updated_at TEXT)`,
		`CREATE TABLE moods (character_id TEXT, mood TEXT, mood_value TEXT, created_at TEXT)`,
		`CREATE TABLE need_states (character_id TEXT, need_key TEXT, current_value REAL, baseline REAL, updated_at TEXT)`,
		`CREATE TABLE unresolved_threads (id TEXT PRIMARY KEY, character_id TEXT, user_id TEXT, topic TEXT, reason TEXT, severity REAL, escalation_level INTEGER, created_at TEXT, resolved_at TEXT)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create runtime schema: %v", err)
		}
	}
}
