package dataportability_test

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	coresystem "github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

func rewriteArchiveManifest(t *testing.T, source, destination string, mutate func(*dataportability.BackupManifest)) {
	t.Helper()
	reader, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	output, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)
	for _, entry := range reader.File {
		content, readErr := entry.Open()
		if readErr != nil {
			t.Fatal(readErr)
		}
		data, readErr := io.ReadAll(content)
		if closeErr := content.Close(); readErr == nil {
			readErr = closeErr
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		if entry.Name == "manifest.json" {
			var manifest dataportability.BackupManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			mutate(&manifest)
			data, err = json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
		}
		target, err := writer.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: entry.Method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := target.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

type failingBackupContributor struct{}

func (failingBackupContributor) ID() string             { return "qa-failing" }
func (failingBackupContributor) Name() string           { return "QA failing" }
func (failingBackupContributor) Dependencies() []string { return nil }
func (failingBackupContributor) ComponentIDs() []string { return []string{"qa.failing"} }
func (failingBackupContributor) Plan(context.Context, dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	return []dataportability.BackupComponentPlan{{ID: "qa.failing", LogicalName: "qa.failing", Kind: dataportability.KindDataset, Required: true}}, nil
}
func (failingBackupContributor) Export(context.Context, dataportability.BackupRequest, dataportability.BackupWriter) error {
	return errors.New("QA intentional backup failure")
}
func (failingBackupContributor) PreviewImport(context.Context, dataportability.ImportPreviewRequest, dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	return nil, nil
}
func (failingBackupContributor) Import(context.Context, dataportability.ImportRequest, dataportability.BackupReader) error {
	return nil
}

type cancelingRestoreContributor struct {
	cancel context.CancelFunc
	cause  error
}

func (*cancelingRestoreContributor) ID() string             { return "qa-cancel" }
func (*cancelingRestoreContributor) Name() string           { return "QA cancellation" }
func (*cancelingRestoreContributor) Dependencies() []string { return []string{"character"} }
func (*cancelingRestoreContributor) ComponentIDs() []string { return []string{"qa.cancel"} }
func (*cancelingRestoreContributor) Plan(context.Context, dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	return []dataportability.BackupComponentPlan{{ID: "qa.cancel", LogicalName: "qa.cancel", Kind: dataportability.KindDataset, ItemCount: 1}}, nil
}
func (*cancelingRestoreContributor) Export(_ context.Context, _ dataportability.BackupRequest, out dataportability.BackupWriter) error {
	w, err := out.CreateComponent("qa.cancel", "qa.cancel", dataportability.KindDataset)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "{}\n")
	return errors.Join(err, w.Close())
}
func (*cancelingRestoreContributor) PreviewImport(context.Context, dataportability.ImportPreviewRequest, dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	return nil, nil
}
func (c *cancelingRestoreContributor) Import(ctx context.Context, _ dataportability.ImportRequest, _ dataportability.BackupReader) error {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
		return errors.Join(c.cause, ctx.Err())
	}
	return ctx.Err()
}

func TestRestoreRollsBackAfterClientCancellation(t *testing.T) {
	db, coord, _ := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cause := errors.New("QA connection dropped after the role write")
	if err := coord.RegisterContributors(&cancelingRestoreContributor{cancel: cancel, cause: cause}); err != nil {
		t.Fatal(err)
	}
	backup, err := coord.CreateBackup(ctx, dataportability.BackupRequest{
		Scope: dataportability.ScopeAll, Profile: dataportability.ProfilePortable, Purpose: dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := coord.CreateRestoreTicket(ctx, backup.Path, backup.BackupID, backup.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&character.Character{}).Where("id = ?", "role-a").Update("name", "before restore").Error; err != nil {
		t.Fatal(err)
	}
	result, err := coord.ExecuteRestore(ctx, ticket, "", nil)
	if result == nil || result.Status != "rolled_back" || !errors.Is(err, cause) || !errors.Is(err, context.Canceled) {
		t.Fatalf("restore cancellation: %#v %v", result, err)
	}
	var role character.Character
	if err := db.Where("id = ?", "role-a").Take(&role).Error; err != nil {
		t.Fatal(err)
	}
	if role.Name != "before restore" {
		t.Fatalf("rollback left restored role name %q", role.Name)
	}
}

func newPortabilityFixture(t *testing.T) (*gorm.DB, *dataportability.Coordinator, string) {
	t.Helper()
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&character.Character{}, &chat.Conversation{}, &chat.Message{}, &chat.ModelConfig{}, &coresystem.AppSetting{}); err != nil {
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
	if err := coord.RegisterContributors(dataportability.NewCharacterContributor(db, nil), chat.NewChatBackupContributor(db), coresystem.NewSettingsBackupContributor(db), chat.NewModelConfigBackupContributor(db), business.NewRuntimeStateBackupContributor(sqlDB)); err != nil {
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

func TestFailedBackupDoesNotPublishPartialArchive(t *testing.T) {
	dataDir := t.TempDir()
	coord := dataportability.NewCoordinator(dataDir, "test", "test", "migrations-v1-1", dataportability.NewStagingManager(dataDir))
	if err := coord.RegisterContributors(failingBackupContributor{}); err != nil {
		t.Fatal(err)
	}
	_, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:   dataportability.ScopeAll,
		Profile: dataportability.ProfilePortable,
		Purpose: dataportability.PurposeMigration,
	}, "", nil)
	if err == nil || !strings.Contains(err.Error(), "QA intentional backup failure") {
		t.Fatalf("unexpected error: %v", err)
	}
	entries, readErr := os.ReadDir(filepath.Join(dataDir, "backups"))
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("failed backup published %d files", len(entries))
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

func TestEmptyBackupAndReplacementPreviewLeaveNoStagedArchive(t *testing.T) {
	_, coord, dataDir := newPortabilityFixture(t)
	req := dataportability.BackupRequest{
		Scope: dataportability.ScopeAll, Profile: dataportability.ProfilePortable, Purpose: dataportability.PurposeMigration,
	}
	var staged []string
	for i := 0; i < 2; i++ {
		backup, err := coord.CreateBackup(t.Context(), req, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		manifest, err := coord.RestorePreview(t.Context(), backup.Path)
		if err != nil {
			t.Fatal(err)
		}
		count := int64(-1)
		for _, component := range manifest.Components {
			if component.ID == "character.records.v1" {
				count = component.ItemCount
			}
		}
		if count != 0 {
			t.Fatalf("expected empty character component, got %d", count)
		}
		ticket, err := coord.CreateRestoreTicket(t.Context(), backup.Path, backup.BackupID, manifest)
		if err != nil {
			t.Fatal(err)
		}
		staged = append(staged, ticket.ArchivePath)
	}
	if _, err := os.Stat(staged[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("previous preview left staged data: %v", err)
	}
	if err := coord.CancelRestore(); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(dataDir, "imports", ".restore", "*.amitia-backup"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("cancel left staged archives: %v, %v", matches, err)
	}
}

func TestPreviewImportCleansStagingAfterRejectedManifest(t *testing.T) {
	_, coord, dataDir := newPortabilityFixture(t)
	archivePath := filepath.Join(dataDir, "rejected.amitia-backup")
	writer := dataportability.NewArchiveWriter(archivePath)
	manifest, err := json.Marshal(dataportability.BackupManifest{
		Format: "unsupported",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if err := writer.Finalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := coord.PreviewImport(t.Context(), archivePath); !errors.Is(err, dataportability.ErrImportFormatUnsupported) {
		t.Fatalf("unexpected preview result: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "imports", ".staging"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected preview left staging entries: %v", entries)
	}
}

func TestVerifyArchiveRejectsMissingComponentChecksum(t *testing.T) {
	db, coord, dataDir := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	backup, err := coord.CreateBackup(t.Context(), dataportability.BackupRequest{
		Scope: dataportability.ScopeAll, Profile: dataportability.ProfilePortable, Purpose: dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(dataDir, "tampered.amitia-backup")
	rewriteArchiveManifest(t, backup.Path, tampered, func(manifest *dataportability.BackupManifest) {
		manifest.Components[0].SHA256 = ""
	})
	result, err := dataportability.VerifyArchive(tampered)
	if result == nil || result.PackageIntegrity || result.ComponentsFailed == 0 {
		t.Fatalf("archive without a component checksum was accepted: %#v %v", result, err)
	}
	if _, err := coord.RestorePreview(t.Context(), tampered); !errors.Is(err, dataportability.ErrRestoreIntegrityFailed) {
		t.Fatalf("restore preview accepted the tampered archive: %v", err)
	}
}

func TestExpiredImportRemovesStagedData(t *testing.T) {
	db, coord, dataDir := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	backup, err := coord.CreateBackup(t.Context(), dataportability.BackupRequest{
		Scope: dataportability.ScopeAll, Profile: dataportability.ProfilePortable, Purpose: dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := coord.StageImport(t.Context(), backup.Path)
	if err != nil {
		t.Fatal(err)
	}
	ticketPath := filepath.Join(dataDir, "imports", ".tickets", ticket.ID+".json")
	data, err := os.ReadFile(ticketPath)
	if err != nil {
		t.Fatal(err)
	}
	var stored dataportability.ImportTicket
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	stored.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	data, err = json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ticketPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := coord.CommitStagedImport(t.Context(), ticket.ID, dataportability.CollisionSkip, false); !errors.Is(err, dataportability.ErrImportTicketExpired) {
		t.Fatalf("unexpected expired import result: %v", err)
	}
	if _, err := os.Stat(ticketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired ticket was retained: %v", err)
	}
	if _, err := os.Stat(ticket.ArchivePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired staged archive was retained: %v", err)
	}
}

func TestStartupCleanupRemovesAbandonedImportTransients(t *testing.T) {
	db, coord, dataDir := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	backup, err := coord.CreateBackup(t.Context(), dataportability.BackupRequest{
		Scope: dataportability.ScopeAll, Profile: dataportability.ProfilePortable, Purpose: dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := coord.StageImport(t.Context(), backup.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		filepath.Join("imports", ".staging", "abandoned"),
		filepath.Join("imports", ".uploads"),
	} {
		dir := filepath.Join(dataDir, relative)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "partial"), []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := coord.CleanupTransientImports(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(dataDir, "imports", ".tickets", ticket.ID+".json"),
		ticket.ArchivePath,
	} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("abandoned import data was retained: %s: %v", path, err)
		}
	}
	for _, relative := range []string{
		filepath.Join("imports", ".staging"),
		filepath.Join("imports", ".uploads"),
	} {
		entries, err := os.ReadDir(filepath.Join(dataDir, relative))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("abandoned transient entries remain in %s: %v", relative, entries)
		}
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

func TestSensitiveSettingsRequireEncryptionAndRoundTrip(t *testing.T) {
	db, coord, dataDir := newPortabilityFixture(t)
	if err := db.Create(&coresystem.AppSetting{Key: "api_key", Value: "settings-secret"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&chat.ModelConfig{
		Name:               "private-model",
		APIType:            "openai",
		BaseURL:            "https://example.invalid",
		APIKey:             "model-secret",
		ModelName:          "private",
		IsActive:           1,
		ProviderConfigJSON: `{"endpoint":"private","nested":{"accessToken":"provider-secret"},"mode":"safe"}`,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:          dataportability.ScopeSettings,
		Profile:        dataportability.ProfilePortable,
		Purpose:        dataportability.PurposeMigration,
		IncludeSecrets: true,
	}, "", nil); !errors.Is(err, dataportability.ErrBackupEncryptionRequired) {
		t.Fatalf("unencrypted secret export error = %v", err)
	}

	portable, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:   dataportability.ScopeSettings,
		Profile: dataportability.ProfilePortable,
		Purpose: dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if portable.Manifest.IncludesSecrets {
		t.Fatal("portable export marked secrets as included")
	}
	for _, component := range portable.Manifest.Components {
		if component.ID == coresystem.ComponentIDSettings || component.ID == chat.ComponentIDModelConfigs {
			if !component.Sensitive {
				t.Fatalf("%s lost its sensitive marker", component.ID)
			}
		}
	}
	assertArchiveOmitsSecrets(t, portable.Path, "settings-secret", "model-secret", "provider-secret")
	ticket, err := coord.StageImport(context.Background(), portable.Path)
	if err != nil {
		t.Fatal(err)
	}
	warned := false
	for _, component := range ticket.Preview.Components {
		if component.ComponentID == chat.ComponentIDModelConfigs && len(component.Warnings) > 0 {
			warned = true
		}
	}
	if !warned {
		t.Fatal("portable preview did not warn about omitted model credentials")
	}
	if err := coord.CancelStagedImport(ticket.ID); err != nil {
		t.Fatal(err)
	}

	secretBackup, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:          dataportability.ScopeSettings,
		Profile:        dataportability.ProfilePortable,
		Purpose:        dataportability.PurposeMigration,
		IncludeSecrets: true,
		Passphrase:     "secret export passphrase",
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !secretBackup.Manifest.IncludesSecrets || !dataportability.IsEncryptedFile(secretBackup.Path) {
		t.Fatal("secret export was not encrypted or recorded")
	}
	decrypted := filepath.Join(dataDir, "secret-export.amitia-backup")
	if err := dataportability.DecryptFile(secretBackup.Path, decrypted, "secret export passphrase"); err != nil {
		t.Fatal(err)
	}
	assertArchiveContainsSecrets(t, decrypted, "settings-secret", "model-secret", "provider-secret")
}

func TestSensitiveModelImportReauthorizesAndActivatesUsableConfig(t *testing.T) {
	db, coord, dataDir := newPortabilityFixture(t)
	seedPortabilityFixture(t, db)
	if err := db.Create(&chat.ModelConfig{
		Name:               "private-model",
		APIType:            "openai",
		BaseURL:            "https://example.invalid",
		APIKey:             "model-secret",
		ModelName:          "private",
		IsActive:           1,
		ProviderConfigJSON: `{"endpoint":"private","nested":{"accessToken":"provider-secret"},"mode":"safe"}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	backup, err := coord.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:          dataportability.ScopeSettings,
		Profile:        dataportability.ProfilePortable,
		Purpose:        dataportability.PurposeMigration,
		IncludeSecrets: true,
		Passphrase:     "secret import passphrase",
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	decrypted := filepath.Join(dataDir, "secret-import.amitia-backup")
	if err := dataportability.DecryptFile(backup.Path, decrypted, "secret import passphrase"); err != nil {
		t.Fatal(err)
	}
	ticket, err := coord.StageImport(context.Background(), decrypted)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Preview == nil || !ticket.Preview.Manifest.IncludesSecrets {
		t.Fatal("secret import preview lost manifest metadata")
	}
	if _, err := coord.CommitStagedImport(context.Background(), ticket.ID, dataportability.CollisionDuplicate, true); err != nil {
		t.Fatal(err)
	}
	var active []chat.ModelConfig
	if err := db.Where("is_active = 1").Find(&active).Error; err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID == 0 || active[0].APIKey != "model-secret" {
		t.Fatalf("active imported config = %#v", active)
	}
}

func assertArchiveOmitsSecrets(t *testing.T, path string, secrets ...string) {
	t.Helper()
	reader, err := dataportability.NewArchiveReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, id := range []string{coresystem.ComponentIDSettings, chat.ComponentIDModelConfigs} {
		rc, err := reader.OpenComponent("datasets/" + id + ".ndjson")
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, secret := range secrets {
			if strings.Contains(string(data), secret) {
				t.Fatalf("%s leaked %s", id, secret)
			}
		}
	}
}

func assertArchiveContainsSecrets(t *testing.T, path string, secrets ...string) {
	t.Helper()
	reader, err := dataportability.NewArchiveReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var data []byte
	for _, id := range []string{coresystem.ComponentIDSettings, chat.ComponentIDModelConfigs} {
		rc, err := reader.OpenComponent("datasets/" + id + ".ndjson")
		if err != nil {
			t.Fatal(err)
		}
		part, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		data = append(data, part...)
	}
	for _, secret := range secrets {
		if !strings.Contains(string(data), secret) {
			t.Fatalf("archive omitted %s", secret)
		}
	}
}
