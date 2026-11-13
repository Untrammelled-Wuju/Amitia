package kernel

import (
	"context"
	"errors"
	"strings"
	"testing"

	petboundary "github.com/u-ai/backend/internal/desktoppet/plugin_boundary"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestPackageUninstallRequiresOwnedDesktopPetCleanup(t *testing.T) {
	ctx := t.Context()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	cleanupErr := errors.New("QA desktop pet installation cleanup interrupted")
	calls := 0
	interrupted := true
	boundary, err := petboundary.NewBoundaryWith(nil, petboundary.DefaultAdapters(), petboundary.WithUninstallCleanup(func(_ context.Context, extensionID domain.ExtensionID, spaceID string) error {
		calls++
		if string(extensionID) != installed.ExtensionID || spaceID != "user-1" {
			t.Errorf("cleanup extension = %s, space = %s", extensionID, spaceID)
		}
		if interrupted {
			return cleanupErr
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	container.DesktopPetPluginBoundary = boundary
	operation, err := NewHostPluginPackageManager(runtime, container.RuntimeProfile).Uninstall(ctx, UninstallHostPluginPackageRequest{
		SpaceID: "user-1", ExtensionID: installed.ExtensionID, IdempotencyKey: "qa-owned-pet-uninstall",
	})
	if calls != 1 || !errors.Is(err, cleanupErr) {
		t.Fatalf("cleanup calls = %d, uninstall error = %v", calls, err)
	}
	if operation.OperationID == "" {
		operations, listErr := container.PackageRepository.ListIncompleteOperations(ctx)
		if listErr != nil || len(operations) != 1 {
			t.Fatalf("recoverable operations = %v, error = %v", operations, listErr)
		}
		operation = operations[0]
	}
	saved, _, err := container.PackageRepository.GetOperation(ctx, "user-1", operation.OperationID)
	if err != nil || saved.Status != string(PackageOperationRequiresRecovery) || !strings.Contains(saved.ErrorDetail, cleanupErr.Error()) {
		t.Fatalf("operation = %+v, error = %v", saved, err)
	}
	interrupted = false
	if err := runtime.RecoverPackageOperations(ctx); err != nil {
		t.Fatal(err)
	}
	saved, _, err = container.PackageRepository.GetOperation(ctx, "user-1", operation.OperationID)
	if err != nil || calls != 2 || saved.Status != string(PackageOperationCompleted) {
		t.Fatalf("cleanup calls = %d, operation = %+v, error = %v", calls, saved, err)
	}
}
