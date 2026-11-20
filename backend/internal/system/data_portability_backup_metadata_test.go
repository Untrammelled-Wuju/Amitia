package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/system/dataportability"
)

func TestEncryptedBackupMetadataIsListedAndRemovedWithBackup(t *testing.T) {
	dataDir := t.TempDir()
	coordinator := dataportability.NewCoordinator(dataDir, "1.2.3", "macos", "baseline-v1", dataportability.NewStagingManager(dataDir))
	if err := coordinator.RegisterContributors(backupListTestContributor{}); err != nil {
		t.Fatal(err)
	}
	svc := &service{dataDir: dataDir, coordinator: coordinator}
	created, err := svc.CreateDataPortabilityBackup(context.Background(), "qa-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	name, _ := created["name"].(string)
	if !strings.HasSuffix(name, ".enc") {
		t.Fatalf("expected encrypted backup, got %q", name)
	}
	metadataPath := filepath.Join(dataDir, "backups", name+".meta.json")
	if _, err := os.Stat(metadataPath); err != nil {
		t.Fatalf("metadata sidecar missing: %v", err)
	}
	listed, err := svc.ListDataPortabilityBackups()
	if err != nil {
		t.Fatal(err)
	}
	backups, _ := listed["backups"].([]map[string]interface{})
	if len(backups) != 1 {
		t.Fatalf("sidecar must not be listed as a backup: %v", backups)
	}
	if backups[0]["backupId"] != created["backupId"] || backups[0]["backupId"] == "" {
		t.Fatalf("backupId=%v want %v", backups[0]["backupId"], created["backupId"])
	}
	if backups[0]["appVersion"] != "1.2.3" {
		t.Fatalf("appVersion=%v", backups[0]["appVersion"])
	}
	if backups[0]["createdAt"] != created["createdAt"] {
		t.Fatalf("createdAt=%v want %v", backups[0]["createdAt"], created["createdAt"])
	}
	if err := svc.DeleteDataPortabilityBackup(name); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
		t.Fatalf("metadata sidecar not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "backups", name)); !os.IsNotExist(err) {
		t.Fatalf("backup not removed: %v", err)
	}
}
