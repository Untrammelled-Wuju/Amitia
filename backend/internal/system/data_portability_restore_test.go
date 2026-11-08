package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/system/dataportability"
)

func TestDataPortabilityRestoreOperationsShareCoordinatorGuard(t *testing.T) {
	dir := t.TempDir()
	coord := dataportability.NewCoordinator(dir, "test", "test", "test", dataportability.NewStagingManager(dir))
	first := &service{dataDir: dir, coordinator: coord}
	second := &service{dataDir: dir, coordinator: coord}
	ctx := context.Background()
	release, err := first.coordinator.BeginRestoreOperation()
	if err != nil {
		t.Fatal(err)
	}
	_, previewErr := second.PreviewDataPortabilityRestore(ctx, "missing.amitia-backup", "")
	_, executeErr := second.ExecuteDataPortabilityRestore(ctx, "restore-test")
	cancelErr := second.CancelDataPortabilityRestore()
	release()
	for operation, err := range map[string]error{
		"preview": previewErr,
		"execute": executeErr,
		"cancel":  cancelErr,
	} {
		if !errors.Is(err, dataportability.ErrRestoreBusy) {
			t.Errorf("%s: expected restore busy, got %v", operation, err)
		}
	}
	if _, err := second.PreviewDataPortabilityRestore(ctx, "missing.amitia-backup", ""); !errors.Is(err, dataportability.ErrBackupNotFound) {
		t.Fatalf("preview after release: %v", err)
	}
}

func TestInspectDataPortabilityImportCleansUploadWhenCoordinatorUnavailable(t *testing.T) {
	dir := t.TempDir()
	upload := filepath.Join(dir, "upload.amitia-backup")
	if err := os.WriteFile(upload, []byte("upload"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := &service{dataDir: dir}
	if _, err := svc.InspectDataPortabilityImport(context.Background(), upload, ""); !errors.Is(err, dataportability.ErrBackupUnavailable) {
		t.Fatalf("unexpected inspect result: %v", err)
	}
	if _, err := os.Stat(upload); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unavailable coordinator retained the upload: %v", err)
	}
}
