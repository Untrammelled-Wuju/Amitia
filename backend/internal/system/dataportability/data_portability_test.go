package dataportability_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	coresystem "github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

func newPortabilityFixture(t *testing.T) (*gorm.DB, *dataportability.Coordinator, string) {
	t.Helper()
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&character.Character{}, &chat.Conversation{}, &chat.Message{}, &coresystem.AppSetting{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS kernel_device_owned_resources (
		owner_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		role_id TEXT NOT NULL,
		source_id TEXT NOT NULL DEFAULT '',
		revision INTEGER NOT NULL,
		deleted INTEGER NOT NULL DEFAULT 0,
		body TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY(owner_id, kind, resource_id)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	coord := dataportability.NewCoordinator(dataDir, "test", "test", "migrations-v1-1", dataportability.NewStagingManager(dataDir))
	if err := coord.RegisterContributors(dataportability.NewCharacterContributor(db), chat.NewChatBackupContributor(db), coresystem.NewSettingsBackupContributor(db), business.NewRuntimeStateBackupContributor(sqlDB)); err != nil {
		t.Fatal(err)
	}
	return db, coord, dataDir
}

func seedPortabilityFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&character.Character{ID: "role-a", SpaceID: "space", Name: "A"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&character.Character{ID: "role-b", SpaceID: "space", Name: "B"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&chat.Conversation{ID: "conv-a", SpaceID: "space", Title: "A chat", Channel: "web"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&chat.Conversation{ID: "conv-b", SpaceID: "space", Title: "B chat", Channel: "web"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&chat.Message{ID: "msg-a", ConversationID: "conv-a", CharacterID: "role-a", Sequence: 1, Role: "user", Content: "A only"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&chat.Message{ID: "msg-b", ConversationID: "conv-b", CharacterID: "role-b", Sequence: 1, Role: "user", Content: "B only"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&coresystem.AppSetting{Key: "portable-test", Value: "true"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,deleted,body,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		"owner", "psyche", "psyche-a", "role-a", 1, 0, `{"mood":"calm"}`, "2026-10-23T12:00:00Z").Error; err != nil {
		t.Fatal(err)
	}
}

func TestCharacterScopeExportsOnlyRelatedChats(t *testing.T) {
	db, coord, _ := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	result, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:       dataportability.ScopeCharacter,
		CharacterID: "role-a",
		Profile:     dataportability.ProfilePortable,
		Purpose:     dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest.Scope != string(dataportability.ScopeCharacter) {
		t.Fatalf("scope=%s", result.Manifest.Scope)
	}
	reader, err := dataportability.NewArchiveReader(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	component, err := reader.OpenComponent("datasets/chat.messages.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer component.Close()
	scanner := bufio.NewScanner(component)
	var messages []map[string]interface{}
	for scanner.Scan() {
		var record map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0]["id"] != "msg-a" || messages[0]["characterId"] != "role-a" {
		t.Fatalf("unexpected character export messages: %#v", messages)
	}
}

func TestEncryptedBackupRoundTripAndWrongPassphrase(t *testing.T) {
	db, coord, dataDir := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	result, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:      dataportability.ScopeAll,
		Profile:    dataportability.ProfilePortable,
		Purpose:    dataportability.PurposeMigration,
		Passphrase: "correct horse battery staple",
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Manifest.Encrypted || !dataportability.IsEncryptedFile(result.Path) {
		t.Fatal("backup was not encrypted")
	}
	decrypted := filepath.Join(dataDir, "nested", "staging", "decrypted.amitia-backup")
	if err := dataportability.DecryptFile(result.Path, decrypted, "wrong password"); err == nil {
		t.Fatal("wrong passphrase was accepted")
	}
	if _, err := os.Stat(decrypted); !os.IsNotExist(err) {
		t.Fatalf("failed decryption left a partial file: %v", err)
	}
	if err := dataportability.DecryptFile(result.Path, decrypted, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	verification, err := dataportability.VerifyArchive(decrypted)
	if err != nil || verification == nil || !verification.PackageIntegrity {
		t.Fatalf("decrypted archive failed verification: %#v %v", verification, err)
	}
}

func TestStagedImportReturnsPreviewAndConsumesTicket(t *testing.T) {
	db, coord, dataDir := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	result, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:       dataportability.ScopeCharacter,
		CharacterID: "role-a",
		Profile:     dataportability.ProfilePortable,
		Purpose:     dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := coord.StageImport(context.Background(), result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Preview == nil || ticket.Preview.Manifest.CharacterID != "role-a" {
		t.Fatalf("unexpected preview: %#v", ticket.Preview)
	}
	committed, err := coord.CommitStagedImport(context.Background(), ticket.ID, dataportability.CollisionDuplicate, false)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Operation == nil || committed.Operation.Status != "completed" || committed.PreImportBackup == "" {
		t.Fatalf("unexpected commit result: %#v", committed)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "imports", ".tickets", ticket.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("import ticket was not consumed: %v", err)
	}
}

func TestSettingsImportIgnoresContributorsOutsideArchive(t *testing.T) {
	db, coord, _ := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	result, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:   dataportability.ScopeSettings,
		Profile: dataportability.ProfilePortable,
		Purpose: dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := coord.StageImport(context.Background(), result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Preview == nil {
		t.Fatal("missing preview")
	}
	for _, component := range ticket.Preview.Components {
		if component.ComponentID == "character.records.v1" || component.ComponentID == "chat.conversations" || component.ComponentID == "chat.messages" {
			t.Fatalf("settings preview included out-of-scope component %s", component.ComponentID)
		}
	}
	if err := coord.CancelStagedImport(ticket.ID); err != nil {
		t.Fatal(err)
	}
}
