package recovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCompletedUninstallRecovery(t *testing.T) {
	for _, tc := range []struct {
		name           string
		missingStorage bool
		mutate         func(*installation.Installation, *projection.InstallationRuntimeProjection, *desired.RuntimeDesiredState, *binding.DeviceActiveInstallationBinding)
	}{
		{name: "confirmed absence"},
		{name: "missing storage then retry", missingStorage: true},
		{name: "different space", mutate: func(_ *installation.Installation, p *projection.InstallationRuntimeProjection, _ *desired.RuntimeDesiredState, _ *binding.DeviceActiveInstallationBinding) {
			p.SpaceID = "other"
		}},
		{name: "different device", mutate: func(_ *installation.Installation, p *projection.InstallationRuntimeProjection, _ *desired.RuntimeDesiredState, _ *binding.DeviceActiveInstallationBinding) {
			p.DeviceID = "other"
		}},
		{name: "older revision", mutate: func(_ *installation.Installation, p *projection.InstallationRuntimeProjection, _ *desired.RuntimeDesiredState, _ *binding.DeviceActiveInstallationBinding) {
			p.AppliedDesiredRevision = 1
		}},
		{name: "runtime still using installation", mutate: func(i *installation.Installation, p *projection.InstallationRuntimeProjection, _ *desired.RuntimeDesiredState, _ *binding.DeviceActiveInstallationBinding) {
			p.InstallationID, p.ActualReleaseID = i.ID, i.CurrentReleaseID
		}},
		{name: "installation active", mutate: func(i *installation.Installation, _ *projection.InstallationRuntimeProjection, _ *desired.RuntimeDesiredState, _ *binding.DeviceActiveInstallationBinding) {
			i.IsActive = 1
		}},
		{name: "desired enabled again", mutate: func(_ *installation.Installation, _ *projection.InstallationRuntimeProjection, d *desired.RuntimeDesiredState, _ *binding.DeviceActiveInstallationBinding) {
			d.DesiredEnabled = true
		}},
		{name: "active binding", mutate: func(i *installation.Installation, _ *projection.InstallationRuntimeProjection, _ *desired.RuntimeDesiredState, b *binding.DeviceActiveInstallationBinding) {
			b.InstallationID = i.ID
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			db, err := gorm.Open(sqlite.Open(filepath.Join(root, "recovery.db")+"?_busy_timeout=5000"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := sqlDB.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := db.AutoMigrate(&installation.Installation{}, &operation.InstallationOperation{}, &projection.InstallationRuntimeProjection{}, &desired.RuntimeDesiredState{}, &binding.DeviceActiveInstallationBinding{}, &installation.TrashEntry{}); err != nil {
				t.Fatal(err)
			}
			registry := security.NewPathRootRegistry()
			installRoot := filepath.Join(root, "installations")
			if err := registry.CreateAndRegister(security.RootInstallations, installRoot); err != nil {
				t.Fatal(err)
			}
			if err := registry.CreateAndRegister(security.RootStorageTrash, filepath.Join(root, "trash")); err != nil {
				t.Fatal(err)
			}
			inst := installation.Installation{ID: "ins", SpaceID: "space", DeviceID: "device", PetID: "pet", CurrentReleaseID: "release", InstallStorageKey: ".staging/ins", Status: installation.StatusUninstalling, LifecycleState: installation.LifecycleUninstalling, DesiredState: installation.DesiredDisabled, RuntimeSyncState: installation.SyncConfirmed}
			op := operation.InstallationOperation{ID: "op", InstallationID: inst.ID, SpaceID: inst.SpaceID, DeviceID: inst.DeviceID, OperationType: operation.TypeUninstall, Status: operation.OpStatusCompleted, Stage: operation.OpStageCompleted, DesiredRevision: 2}
			proj := projection.InstallationRuntimeProjection{ID: "projection", SpaceID: inst.SpaceID, DeviceID: inst.DeviceID, InstallationID: "different", RuntimeSyncState: projection.SyncStateApplied, AppliedDesiredRevision: 3}
			state := desired.RuntimeDesiredState{ID: "desired", SpaceID: inst.SpaceID, DeviceID: inst.DeviceID, InstallationID: inst.ID, DesiredRevision: 2}
			active := binding.DeviceActiveInstallationBinding{SpaceID: inst.SpaceID, DeviceID: inst.DeviceID, InstallationID: "different"}
			if tc.mutate != nil {
				tc.mutate(&inst, &proj, &state, &active)
			}
			for _, row := range []any{&inst, &op, &proj, &state, &active} {
				if err := db.Create(row).Error; err != nil {
					t.Fatal(err)
				}
			}
			source := filepath.Join(installRoot, inst.InstallStorageKey)
			finalizer := NewProductionRuntimeFinalizer(db, installation.NewRepository(db, nil).(installation.RepositoryV2), registry)
			if tc.missingStorage {
				resolved, err := registry.Resolve(security.RootInstallations, inst.InstallStorageKey)
				if err != nil {
					t.Fatal(err)
				}
				worker := NewRecoveryWorker(NewInstallRepoAdapter(db), WithDesiredStateFinalizer(finalizer))
				err = worker.tick(context.Background())
				var pathError *os.PathError
				if !errors.Is(err, os.ErrNotExist) || !errors.As(err, &pathError) || pathError.Path != resolved {
					t.Fatalf("missing storage error lost its cause: %v", err)
				}
				var unchanged installation.Installation
				if err := db.First(&unchanged, "id = ?", inst.ID).Error; err != nil {
					t.Fatal(err)
				}
				if unchanged.Status != inst.Status || unchanged.LifecycleState != inst.LifecycleState {
					t.Fatalf("failed recovery changed installation: %+v", unchanged)
				}
			}
			if err := os.MkdirAll(source, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "pet.txt"), []byte("asset"), 0o600); err != nil {
				t.Fatal(err)
			}
			var scans sync.WaitGroup
			for range 2 {
				scans.Go(func() {
					worker := NewRecoveryWorker(NewInstallRepoAdapter(db), WithDesiredStateFinalizer(finalizer))
					if err := worker.tick(context.Background()); err != nil {
						t.Error(err)
					}
				})
			}
			scans.Wait()
			var actual installation.Installation
			var original operation.InstallationOperation
			var trash []installation.TrashEntry
			if err := db.First(&actual, "id = ?", inst.ID).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&original, "id = ?", op.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, op) {
				t.Fatalf("terminal operation changed: %+v", original)
			}
			if err := db.Find(&trash).Error; err != nil {
				t.Fatal(err)
			}
			_, sourceErr := os.Stat(source)
			if tc.mutate != nil {
				if actual.Status != inst.Status || actual.LifecycleState != inst.LifecycleState || len(trash) != 0 || sourceErr != nil {
					t.Fatalf("unsafe recovery: %+v, trash=%+v, source=%v", actual, trash, sourceErr)
				}
				return
			}
			if actual.Status != installation.StatusUninstalled || actual.LifecycleState != installation.LifecycleUninstalled || len(trash) != 1 || trash[0].OperationID != op.ID || trash[0].Status != "retained" || !errors.Is(sourceErr, os.ErrNotExist) {
				t.Fatalf("incomplete recovery: %+v, trash=%+v, source=%v", actual, trash, sourceErr)
			}
			dest, err := registry.Resolve(security.RootStorageTrash, trash[0].StorageKey)
			if err != nil {
				t.Fatal(err)
			}
			asset, err := os.ReadFile(filepath.Join(dest, "pet.txt"))
			if err != nil || string(asset) != "asset" {
				t.Fatalf("recovered asset=%q, error=%v", asset, err)
			}
		})
	}
}
