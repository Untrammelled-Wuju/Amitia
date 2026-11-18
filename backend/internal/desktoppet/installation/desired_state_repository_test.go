package installation

import (
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRuntimeDesiredStateIsScopedToInstallation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:desired-per-installation?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&desired.RuntimeDesiredState{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db, nil)
	for index, id := range []string{"pet-a", "pet-b"} {
		state := &desired.RuntimeDesiredState{SpaceID: "space", DeviceID: "device", InstallationID: id, DesiredRevision: int64(index + 1), DesiredEnabled: true}
		if _, err := repo.UpsertRuntimeDesiredStateCAS(db, "space", "device", state, -1); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"pet-a", "pet-b"} {
		state, err := repo.GetRuntimeDesiredStateTx(db, "space", "device", id)
		if err != nil || state == nil || state.InstallationID != id || !state.DesiredEnabled {
			t.Fatalf("installation %s state = %+v, error = %v", id, state, err)
		}
	}
}

func TestActiveBindingsAreScopedToInstallation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:bindings-per-installation?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&binding.DeviceActiveInstallationBinding{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db, nil)
	for _, id := range []string{"pet-a", "pet-b"} {
		if err := repo.UpsertActiveBindingTx(db, &binding.DeviceActiveInstallationBinding{SpaceID: "space", DeviceID: "device", InstallationID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.DeleteActiveBindingTx(db, "space", "device", "pet-a"); err != nil {
		t.Fatal(err)
	}
	other, err := repo.GetActiveBindingForSpaceDeviceTx(db, "space", "device", "pet-b")
	if err != nil || other == nil || other.InstallationID != "pet-b" {
		t.Fatalf("other active binding = %+v, error = %v", other, err)
	}
}
