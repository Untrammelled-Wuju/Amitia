package installation

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRepositoryV2TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Installation{}, &RuntimeSettings{}); err != nil {
		t.Fatalf("migrate installation/runtime settings: %v", err)
	}
	return db
}

func TestRepositoryV2TransactionReadsItsOwnWritesAndRollsBack(t *testing.T) {
	db := newRepositoryV2TestDB(t)
	repo := &repository{db: db}
	rollbackErr := errors.New("force rollback")

	err := repo.Transaction(context.Background(), func(tx RepositoryV2) error {
		inst := &Installation{
			ID: "inst-tx", UserID: "user-1", DeviceID: "device-1", PetID: "pet-1",
			PackageID: "pkg-1", PackageVersion: "1.0.0", Status: StatusInstalled,
			LifecycleState: LifecycleInstalled,
		}
		if err := tx.CreateInstallationTx(tx.DB(), inst); err != nil {
			return err
		}
		got, err := tx.GetInstallationForUserDevice("user-1", "device-1", "inst-tx")
		if err != nil {
			return err
		}
		if got == nil || got.ID != inst.ID {
			t.Fatalf("transactional read did not see own write: %#v", got)
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected rollback error, got %v", err)
	}

	var count int64
	if err := db.Model(&Installation{}).Where("id = ?", "inst-tx").Count(&count).Error; err != nil {
		t.Fatalf("count installation: %v", err)
	}
	if count != 0 {
		t.Fatalf("transaction escaped/committed unexpectedly: count=%d", count)
	}
}

func TestTxRepositoryAdapterDoesNotStartNestedTransaction(t *testing.T) {
	db := newRepositoryV2TestDB(t)
	repo := &repository{db: db}
	if err := repo.Transaction(context.Background(), func(tx RepositoryV2) error {
		return tx.Transaction(context.Background(), func(nested RepositoryV2) error {
			if nested.DB() != tx.DB() {
				t.Fatal("nested transaction escaped current gorm transaction")
			}
			return nil
		})
	}); err != nil {
		t.Fatalf("transaction: %v", err)
	}
}

func TestRepositoryV2RuntimeSettingsEnforcesUserDeviceOwnership(t *testing.T) {
	db := newRepositoryV2TestDB(t)
	repo := &repository{db: db}
	inst := &Installation{
		ID: "inst-owned", UserID: "user-1", DeviceID: "device-1", PetID: "pet-1",
		PackageID: "pkg-1", PackageVersion: "1.0.0", Status: StatusInstalled,
		LifecycleState: LifecycleInstalled,
	}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("seed installation: %v", err)
	}
	settings := &RuntimeSettings{
		ID: "settings-owned", InstallationID: inst.ID, Scale: 1, SettingsRevision: 1,
	}
	if err := db.Create(settings).Error; err != nil {
		t.Fatalf("seed runtime settings: %v", err)
	}

	if _, err := repo.GetRuntimeSettingsForUserDevice("user-2", "device-1", inst.ID); !errors.Is(err, ErrInstallationNotFound) {
		t.Fatalf("wrong user must not read settings, got %v", err)
	}
	if _, err := repo.GetRuntimeSettingsForUserDevice("user-1", "device-2", inst.ID); !errors.Is(err, ErrInstallationNotFound) {
		t.Fatalf("wrong device must not read settings, got %v", err)
	}
	got, err := repo.GetRuntimeSettingsForUserDevice("user-1", "device-1", inst.ID)
	if err != nil || got == nil || got.ID != settings.ID {
		t.Fatalf("owner read failed: got=%#v err=%v", got, err)
	}
}

func TestRepositoryV2RuntimeSettingsCASEnforcesOwnership(t *testing.T) {
	db := newRepositoryV2TestDB(t)
	repo := &repository{db: db}
	inst := &Installation{
		ID: "inst-cas-owned", UserID: "user-1", DeviceID: "device-1", PetID: "pet-1",
		PackageID: "pkg-1", PackageVersion: "1.0.0", Status: StatusInstalled,
		LifecycleState: LifecycleInstalled,
	}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("seed installation: %v", err)
	}
	if err := db.Create(&RuntimeSettings{
		ID: "settings-cas-owned", InstallationID: inst.ID, Scale: 1, SettingsRevision: 1,
	}).Error; err != nil {
		t.Fatalf("seed runtime settings: %v", err)
	}

	if _, err := repo.UpdateRuntimeSettingsCAS(db, inst.ID, "user-2", "device-1", 1, map[string]interface{}{"scale": 2.0}); !errors.Is(err, ErrInstallationNotFound) {
		t.Fatalf("wrong user must not update settings, got %v", err)
	}
	if _, err := repo.UpdateRuntimeSettingsCAS(db, inst.ID, "user-1", "device-2", 1, map[string]interface{}{"scale": 2.0}); !errors.Is(err, ErrInstallationNotFound) {
		t.Fatalf("wrong device must not update settings, got %v", err)
	}
	updated, err := repo.UpdateRuntimeSettingsCAS(db, inst.ID, "user-1", "device-1", 1, map[string]interface{}{"scale": 2.0})
	if err != nil {
		t.Fatalf("owner CAS update failed: %v", err)
	}
	if updated.Scale != 2 || updated.SettingsRevision != 2 {
		t.Fatalf("unexpected updated settings: scale=%v revision=%d", updated.Scale, updated.SettingsRevision)
	}
}
