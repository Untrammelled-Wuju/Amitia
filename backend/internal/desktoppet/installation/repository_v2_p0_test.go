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
	if err := db.AutoMigrate(&Installation{}); err != nil {
		t.Fatalf("migrate installation: %v", err)
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
