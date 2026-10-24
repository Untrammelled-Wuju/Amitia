package main

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	runtimev1 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v1"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type desktopPetCanaryCoordinatorStub struct {
	coordinator.InstallationCoordinator
	db      *gorm.DB
	request coordinator.DefaultActionRequest
}

func (s *desktopPetCanaryCoordinatorStub) ChangeDefaultAction(ctx context.Context, request coordinator.DefaultActionRequest) (*coordinator.EnableDisableResult, error) {
	s.request = request
	return &coordinator.EnableDisableResult{
		OperationID:     "operation-canary",
		DesiredRevision: 7,
		Status:          operation.OpStatusWaitingRuntimeACK,
		Stage:           operation.OpStageWaitingRuntimeACK,
	}, nil
}

func (s *desktopPetCanaryCoordinatorStub) GetOperationStatus(ctx context.Context, spaceID, deviceID, operationID string) (*operation.InstallationOperation, error) {
	if err := s.db.Create(&desired.RuntimeDesiredState{
		ID:               "desired-canary",
		SpaceID:          spaceID,
		DeviceID:         deviceID,
		RuntimeID:        s.request.DeviceCtx.RuntimeID,
		InstallationID:   s.request.InstallationID,
		PetID:            "pet-one",
		ReleaseID:        "release-one",
		DesiredEnabled:   true,
		DesiredVisible:   true,
		DesiredActionKey: s.request.DesiredActionKey,
		DesiredRevision:  7,
	}).Error; err != nil {
		return nil, err
	}
	if err := s.db.Create(&projection.InstallationRuntimeProjection{
		ID:                     "projection-canary",
		SpaceID:                spaceID,
		DeviceID:               deviceID,
		RuntimeID:              s.request.DeviceCtx.RuntimeID,
		InstallationID:         s.request.InstallationID,
		PetID:                  "pet-one",
		AppliedDesiredRevision: 7,
		ActualReleaseID:        "release-one",
		ActualVisible:          1,
		ActualActionKey:        s.request.DesiredActionKey,
		ActualHealth:           "healthy",
		RuntimeSyncState:       "applied",
	}).Error; err != nil {
		return nil, err
	}
	if err := s.db.Create(&runtimev1.RuntimeCommand{
		ID:              "command-canary",
		SpaceID:         spaceID,
		DeviceID:        deviceID,
		RuntimeID:       s.request.DeviceCtx.RuntimeID,
		CommandType:     "sync_desired_state",
		Durability:      "durable",
		Status:          string(runtimev1.CommandStatusCompleted),
		DesiredRevision: 7,
		InstallationID:  s.request.InstallationID,
		PetID:           "pet-one",
		ReleaseID:       "release-one",
	}).Error; err != nil {
		return nil, err
	}
	return &operation.InstallationOperation{
		ID:              operationID,
		Status:          operation.OpStatusCompleted,
		Stage:           operation.OpStageCompleted,
		DesiredRevision: 7,
	}, nil
}

func TestExecuteDesktopPetWriteCanarySkipsOnlyWhenNoInstallationsExist(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&installation.Installation{}); err != nil {
		t.Fatalf("migrate installations: %v", err)
	}
	repo := installation.NewRepository(db, nil)
	result, err := executeDesktopPetWriteCanary(
		context.Background(),
		&desktopPetCanaryCoordinatorStub{},
		repo,
		runtimev1.NewRuntimeFacade(db, nil),
	)
	if err != nil {
		t.Fatalf("no-installation canary: %v", err)
	}
	if result == nil || !result.Success || !result.Skipped || result.SkipReason == "" {
		t.Fatalf("no-installation canary was not explicitly skipped: %+v", result)
	}
	port := &appWriteLockoutPort{services: &AppServices{InstallationRepo: repo}}
	if err := port.VerifyCanaryOperation(context.Background(), result); err != nil {
		t.Fatalf("verify skipped canary: %v", err)
	}
}

func TestExecuteDesktopPetWriteCanaryDoesNotSkipWhenRuntimeIsUnavailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&installation.Installation{}); err != nil {
		t.Fatalf("migrate installations: %v", err)
	}
	if err := db.Exec(`CREATE TABLE kernel_device_runtime_sessions (
		space_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		runtime_id TEXT NOT NULL,
		status TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create runtime sessions table: %v", err)
	}
	if err := db.Create(&installation.Installation{
		ID:               "installation-one",
		SpaceID:          "space-one",
		DeviceID:         "device-one",
		Status:           installation.StatusEnabled,
		DefaultActionKey: "idle_normal",
	}).Error; err != nil {
		t.Fatalf("create installation: %v", err)
	}
	_, err = executeDesktopPetWriteCanary(
		context.Background(),
		&desktopPetCanaryCoordinatorStub{},
		installation.NewRepository(db, nil),
		runtimev1.NewRuntimeFacade(db, nil),
	)
	if err == nil {
		t.Fatal("canary reported success without a ready runtime")
	}
}

func TestExecuteDesktopPetWriteCanaryRequiresCoordinatorCommandAndAppliedProjection(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&installation.Installation{},
		&desired.RuntimeDesiredState{},
		&projection.InstallationRuntimeProjection{},
		&runtimev1.RuntimeCommand{},
	); err != nil {
		t.Fatalf("migrate canary schema: %v", err)
	}
	if err := db.Exec(`CREATE TABLE kernel_device_runtime_sessions (
		space_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		runtime_id TEXT NOT NULL,
		status TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create runtime sessions table: %v", err)
	}
	if err := db.Exec(`INSERT INTO kernel_device_runtime_sessions(space_id,device_id,runtime_id,status,updated_at)
		VALUES('space-one','device-one','runtime-one','ready','2026-10-23T12:00:00Z')`).Error; err != nil {
		t.Fatalf("create runtime session: %v", err)
	}
	if err := db.Create(&installation.Installation{
		ID:               "installation-one",
		SpaceID:          "space-one",
		DeviceID:         "device-one",
		PetID:            "pet-one",
		CurrentReleaseID: "release-one",
		Status:           installation.StatusEnabled,
		DefaultActionKey: "idle_normal",
	}).Error; err != nil {
		t.Fatalf("create installation: %v", err)
	}
	coord := &desktopPetCanaryCoordinatorStub{db: db}
	repo := installation.NewRepository(db, nil)
	runtimeFacade := runtimev1.NewRuntimeFacade(db, nil)
	result, err := executeDesktopPetWriteCanary(
		context.Background(),
		coord,
		repo,
		runtimeFacade,
	)
	if err != nil {
		t.Fatalf("execute coordinator canary: %v", err)
	}
	if result == nil || result.Skipped || !result.Success || result.OperationID != "operation-canary" || result.CommandID != "command-canary" || result.ProjectionRevision != result.DesiredRevision {
		t.Fatalf("unexpected canary result: %+v", result)
	}
	if coord.request.InstallationID != "installation-one" || coord.request.DesiredActionKey != "idle_normal" || coord.request.DeviceCtx.RuntimeID != "runtime-one" || coord.request.IdempotencyKey == "" {
		t.Fatalf("canary bypassed coordinator contract: %+v", coord.request)
	}
	port := &appWriteLockoutPort{services: &AppServices{InstallationRepo: repo, DesktopPetRuntimeV1: runtimeFacade}}
	if err := port.VerifyCanaryOperation(context.Background(), result); err != nil {
		t.Fatalf("verify coordinator canary: %v", err)
	}
}

func TestVerifyDesktopPetProductionReader_UsesCanonicalRepositoryV2Data(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&installation.Installation{}, &desired.RuntimeDesiredState{}, &binding.DeviceActiveInstallationBinding{}); err != nil {
		t.Fatalf("migrate reader canary schema: %v", err)
	}
	inst := &installation.Installation{ID: "inst-reader", SpaceID: "u", DeviceID: "d", PetID: "pet", CurrentReleaseID: "rel", Status: installation.StatusEnabled}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("create installation: %v", err)
	}
	state := &desired.RuntimeDesiredState{ID: "ds-reader", SpaceID: "u", DeviceID: "d", RuntimeID: "r", InstallationID: inst.ID, PetID: "pet", ReleaseID: "rel", DesiredRevision: 1}
	if err := db.Create(state).Error; err != nil {
		t.Fatalf("create desired state: %v", err)
	}
	active := &binding.DeviceActiveInstallationBinding{SpaceID: "u", DeviceID: "d", InstallationID: inst.ID, PetID: "pet", ReleaseID: "rel", BindingRevision: 1}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
	repo := installation.NewRepository(db, nil)
	if err := verifyDesktopPetProductionReader(context.Background(), repo); err != nil {
		t.Fatalf("verify canonical production reader: %v", err)
	}
}
