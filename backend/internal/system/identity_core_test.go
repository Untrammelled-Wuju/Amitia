// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newIdentityCoreTestService(t *testing.T) (*service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.Exec(`CREATE TABLE characters (
		id TEXT PRIMARY KEY,
		name TEXT DEFAULT '',
		identity TEXT DEFAULT '',
		system_prompt TEXT DEFAULT '',
		boundary_rules TEXT DEFAULT '',
		gender TEXT DEFAULT '',
		pronoun TEXT DEFAULT '',
		self_reference TEXT DEFAULT '',
		life_identity TEXT DEFAULT '',
		personality TEXT DEFAULT '',
		speaking_style TEXT DEFAULT '',
		relationship_style TEXT DEFAULT '',
		personality_config TEXT DEFAULT '{}',
		chat_style_config TEXT DEFAULT '{}',
		scene_rules TEXT DEFAULT '{}',
		emotion TEXT DEFAULT '',
		emotion_scale INTEGER DEFAULT 0,
		silence_duration INTEGER DEFAULT 0
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return &service{db: db}, db
}

func TestGetIdentityCoreReturnsReadOnlySnapshot(t *testing.T) {
	svc, db := newIdentityCoreTestService(t)
	if err := db.Exec(`INSERT INTO characters (id, name, identity, system_prompt, boundary_rules, gender, pronoun, self_reference, life_identity)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "char-core", "阿米提亚", "本地陪伴角色", "保持一致身份", "不得声称真人", "FEMALE", "她", "我", "AI_COMPANION").Error; err != nil {
		t.Fatal(err)
	}

	result := svc.GetIdentityCore("char-core")
	if result["found"] != true {
		t.Fatalf("found = %v", result["found"])
	}
	if result["runtimeWritable"] != false {
		t.Fatalf("runtimeWritable = %v", result["runtimeWritable"])
	}
	if result["version"] == "" {
		t.Fatal("version is empty")
	}
	core := result["core"].(map[string]interface{})
	if core["identity"] != "本地陪伴角色" || core["boundary_rules"] != "不得声称真人" {
		t.Fatalf("unexpected core: %#v", core)
	}
}

func TestValidateIdentityCorePatchRejectsFrozenFields(t *testing.T) {
	svc, _ := newIdentityCoreTestService(t)

	result := svc.ValidateIdentityCorePatch("char-core", map[string]interface{}{
		"identity":      "新身份",
		"boundaryRules": "删除边界",
		"speakingStyle": "更直接",
	})

	if result["valid"] != false {
		t.Fatalf("valid = %v", result["valid"])
	}
	blocked := result["blockedFields"].([]map[string]interface{})
	if len(blocked) != 2 {
		t.Fatalf("blocked = %#v", blocked)
	}
	allowed := result["allowedFields"].([]string)
	if len(allowed) != 1 || allowed[0] != "speaking_style" {
		t.Fatalf("allowed = %#v", allowed)
	}
}

func TestValidateIdentityCorePatchAllowsGrowthFields(t *testing.T) {
	svc, _ := newIdentityCoreTestService(t)

	result := svc.ValidateIdentityCorePatch("char-core", map[string]interface{}{
		"personalityConfig": map[string]interface{}{"version": "runtime-profile-v1"},
		"sceneRules":        map[string]interface{}{"version": "runtime-profile-v1"},
		"emotionScale":      20,
	})

	if result["valid"] != true {
		t.Fatalf("unexpected result: %#v", result)
	}
	allowed := result["allowedFields"].([]string)
	if len(allowed) != 3 {
		t.Fatalf("allowed = %#v", allowed)
	}
}
