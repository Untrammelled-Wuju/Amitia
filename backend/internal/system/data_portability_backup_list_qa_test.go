package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
)

type backupListTestContributor struct{}

func (backupListTestContributor) ID() string             { return "qa-list" }
func (backupListTestContributor) Name() string           { return "QA list" }
func (backupListTestContributor) Dependencies() []string { return nil }
func (backupListTestContributor) ComponentIDs() []string { return []string{"qa.list"} }
func (backupListTestContributor) Plan(context.Context, dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	return []dataportability.BackupComponentPlan{{ID: "qa.list", LogicalName: "qa.list", Kind: dataportability.KindDataset, Required: true, ItemCount: 1}}, nil
}
func (backupListTestContributor) Export(_ context.Context, _ dataportability.BackupRequest, out dataportability.BackupWriter) error {
	writer, err := out.CreateComponent("qa.list", "qa.list", dataportability.KindDataset)
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(`{"id":"one"}` + "\n")); err != nil {
		return err
	}
	return writer.Close()
}
func (backupListTestContributor) PreviewImport(context.Context, dataportability.ImportPreviewRequest, dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	return nil, nil
}
func (backupListTestContributor) Import(context.Context, dataportability.ImportRequest, dataportability.BackupReader) error {
	return nil
}

func TestDataPortabilityBackupListHidesInvalidAndTemporaryFiles(t *testing.T) {
	dataDir := t.TempDir()
	coordinator := dataportability.NewCoordinator(dataDir, "test", "test", "baseline-v1", dataportability.NewStagingManager(dataDir))
	if err := coordinator.RegisterContributors(backupListTestContributor{}); err != nil {
		t.Fatal(err)
	}
	result, err := coordinator.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:   dataportability.ScopeAll,
		Profile: dataportability.ProfilePortable,
		Purpose: dataportability.PurposeMigration,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(dataDir, "backups")
	corruptName := "backup-" + uuid.NewString() + ".amitia-backup"
	if err := os.WriteFile(filepath.Join(backupDir, corruptName), []byte(`{"broken":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "."+uuid.NewString()+".amitia-backup.partial"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := &service{dataDir: dataDir, coordinator: coordinator}
	listed, err := svc.ListDataPortabilityBackups()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(listed["backups"])
	if err != nil {
		t.Fatal(err)
	}
	var backups []map[string]any
	if err := json.Unmarshal(raw, &backups); err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 || backups[0]["name"] != filepath.Base(result.Path) {
		t.Fatalf("unexpected backups %#v", backups)
	}
}
