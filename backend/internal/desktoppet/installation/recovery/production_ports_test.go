package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	runtimev1 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v1"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRecoveryRuntimeRepo(t *testing.T) (*ProductionRuntimeRepo, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:recovery-ports-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&operation.InstallationOperation{},
		&desired.RuntimeDesiredState{},
		&desired.DesiredStateOutboxEvent{},
		&runtimev1.RuntimeCommand{},
		&runtimev1.DeviceCommandSequence{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	facade := runtimev1.NewRuntimeFacade(db, nil)
	return NewProductionRuntimeRepo(db, facade), db
}

func seedRecoveryOperation(t *testing.T, db *gorm.DB, opID string) {
	t.Helper()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	op := operation.InstallationOperation{
		ID: opID, OperationType: operation.TypeUpgrade, SpaceID: "space-1", DeviceID: "device-1",
		RuntimeID: "runtime-1", InstallationID: "ins-1", PetID: "pet-1",
		Status: operation.OpStatusWaitingRuntimeACK, Stage: operation.OpStageWaitingRuntimeACK,
		DesiredRevision: 12, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&op).Error; err != nil {
		t.Fatalf("create operation: %v", err)
	}
}

func seedCurrentDesiredState(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	state := desired.RuntimeDesiredState{
		ID: "rds-1", SpaceID: "space-1", DeviceID: "device-1", RuntimeID: "runtime-1",
		InstallationID: "ins-1", PetID: "pet-1", ReleaseID: "release-2",
		DesiredEnabled: true, DesiredVisible: true, DesiredActionKey: "wave",
		SettingsRevision: 9, DesiredRevision: 27, DesiredHash: "hash-current-27",
		OperationID: "op-newer", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&state).Error; err != nil {
		t.Fatalf("create desired state: %v", err)
	}
}

func seedOutboxSnapshot(t *testing.T, db *gorm.DB, opID string, snapshot coordinator.DesiredStateSnapshot) {
	t.Helper()
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	event := desired.DesiredStateOutboxEvent{
		EventID: "evt-" + opID, EventType: "desired_state_changed",
		SpaceID: snapshot.SpaceID, DeviceID: snapshot.DeviceID, RuntimeID: snapshot.RuntimeID,
		InstallationID: snapshot.InstallationID, DesiredRevision: snapshot.DesiredRevision,
		DesiredHash: snapshot.DesiredHash, OperationID: opID, PayloadJSON: string(payload),
		Status: "pending", CreatedAt: time.Now().UTC().Format("2006-01-02 15:04:05"),
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatalf("create outbox event: %v", err)
	}
}

func recoverySnapshot(revision int64) coordinator.DesiredStateSnapshot {
	return coordinator.DesiredStateSnapshot{
		DesiredRevision: revision, DesiredHash: fmt.Sprintf("hash-rev-%d", revision),
		InstallationID: "ins-1", PetID: "pet-1", ReleaseID: "release-1",
		SpaceID: "space-1", DeviceID: "device-1", RuntimeID: "runtime-1",
		EnsureAbsent: false, DefaultActionKey: "idle", SettingsRevision: 5,
		SettingsSnapshotJSON: `{"alwaysOnTop":true}`,
	}
}

func publisherPayload(t *testing.T, snapshot coordinator.DesiredStateSnapshot) runtimev1.SyncDesiredStatePayload {
	t.Helper()
	var settings json.RawMessage
	if snapshot.SettingsSnapshotJSON != "" {
		if !json.Valid([]byte(snapshot.SettingsSnapshotJSON)) {
			t.Fatalf("invalid settings snapshot fixture")
		}
		settings = json.RawMessage(snapshot.SettingsSnapshotJSON)
	}
	return runtimev1.SyncDesiredStatePayload{
		DesiredRevision: snapshot.DesiredRevision, DesiredHash: snapshot.DesiredHash,
		EnsureAbsent: snapshot.EnsureAbsent, InstallationID: snapshot.InstallationID,
		PetID: snapshot.PetID, ReleaseID: snapshot.ReleaseID,
		RuntimeContractVersion: runtimev1.CurrentSchemaVersion,
		DefaultActionKey:       snapshot.DefaultActionKey, SettingsRevision: snapshot.SettingsRevision,
		SettingsSnapshot: settings,
	}
}

func TestSendDesiredCommandReplaysImmutableOutboxSnapshot(t *testing.T) {
	repo, db := newRecoveryRuntimeRepo(t)
	seedRecoveryOperation(t, db, "op-1")
	seedCurrentDesiredState(t, db)
	seedOutboxSnapshot(t, db, "op-1", recoverySnapshot(12))

	ctx := context.Background()
	if err := repo.SendDesiredCommand(ctx, "op-1", "space-1", "device-1", "runtime-1", "ins-1", 12); err != nil {
		t.Fatalf("send desired command: %v", err)
	}
	cmd, err := repo.facade.Commands().GetCommandByIdempotencyKey("desired:device-1:12")
	if err != nil {
		t.Fatalf("load command: %v", err)
	}
	if cmd.DesiredRevision != 12 {
		t.Fatalf("revision=%d want 12", cmd.DesiredRevision)
	}
	expectedHash := runtimev1.ComputePayloadHash(mustMarshal(t, publisherPayload(t, recoverySnapshot(12))))
	if cmd.PayloadHash != expectedHash {
		t.Fatalf("payload hash=%s want publisher hash %s", cmd.PayloadHash, expectedHash)
	}
	if cmd.ReleaseID != "release-1" || cmd.SettingsRevision != 5 || cmd.PetID != "pet-1" || cmd.InstallationID != "ins-1" {
		t.Fatalf("command payload not replayed from snapshot: %+v", cmd)
	}
}

func TestSendDesiredCommandIdempotentWhenSnapshotCommandExists(t *testing.T) {
	repo, db := newRecoveryRuntimeRepo(t)
	seedRecoveryOperation(t, db, "op-1")
	seedCurrentDesiredState(t, db)
	seedOutboxSnapshot(t, db, "op-1", recoverySnapshot(12))

	seq, err := repo.facade.Commands().AllocateDeviceSequence(nil, "space-1", "device-1", time.Now())
	if err != nil {
		t.Fatalf("allocate sequence: %v", err)
	}
	if _, err := repo.facade.Commands().CreateDurableCommand(
		"space-1", "device-1", string(runtimev1.CommandTypeSyncDesiredState),
		"desired:device-1:12", "desired:device-1", seq, publisherPayload(t, recoverySnapshot(12)),
	); err != nil {
		t.Fatalf("seed command: %v", err)
	}
	if err := repo.SendDesiredCommand(context.Background(), "op-1", "space-1", "device-1", "runtime-1", "ins-1", 12); err != nil {
		t.Fatalf("replay must stay idempotent, got %v", err)
	}
	var count int64
	if err := db.Model(&runtimev1.RuntimeCommand{}).Where("idempotency_key = ?", "desired:device-1:12").Count(&count).Error; err != nil {
		t.Fatalf("count commands: %v", err)
	}
	if count != 1 {
		t.Fatalf("command count=%d want 1", count)
	}
}

func TestSendDesiredCommandMissingSnapshotPreservesOriginalError(t *testing.T) {
	repo, db := newRecoveryRuntimeRepo(t)
	seedRecoveryOperation(t, db, "op-1")
	seedCurrentDesiredState(t, db)

	err := repo.SendDesiredCommand(context.Background(), "op-1", "space-1", "device-1", "runtime-1", "ins-1", 12)
	if err == nil {
		t.Fatal("expected missing snapshot error")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("original error not preserved: %v", err)
	}
	var count int64
	if err := db.Model(&runtimev1.RuntimeCommand{}).Count(&count).Error; err != nil {
		t.Fatalf("count commands: %v", err)
	}
	if count != 0 {
		t.Fatalf("command created without snapshot: %d", count)
	}
}

func TestSendDesiredCommandRevisionConflictKeepsBothRevisions(t *testing.T) {
	repo, db := newRecoveryRuntimeRepo(t)
	seedRecoveryOperation(t, db, "op-1")
	seedCurrentDesiredState(t, db)
	seedOutboxSnapshot(t, db, "op-1", recoverySnapshot(12))

	err := repo.SendDesiredCommand(context.Background(), "op-1", "space-1", "device-1", "runtime-1", "ins-1", 13)
	if err == nil {
		t.Fatal("expected revision conflict")
	}
	if !containsAll(err.Error(), "expected=13", "actual=12") {
		t.Fatalf("conflict error lost revisions: %v", err)
	}
	var count int64
	if err := db.Model(&runtimev1.RuntimeCommand{}).Count(&count).Error; err != nil {
		t.Fatalf("count commands: %v", err)
	}
	if count != 0 {
		t.Fatalf("command created despite conflict: %d", count)
	}
}

func TestResolveDesiredRevisionUsesOutboxOnly(t *testing.T) {
	repo, db := newRecoveryRuntimeRepo(t)
	seedCurrentDesiredState(t, db)

	_, err := repo.ResolveDesiredRevision(context.Background(), "op-newer", "space-1", "device-1")
	if err == nil {
		t.Fatal("expected missing outbox error")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("original error not preserved: %v", err)
	}

	seedOutboxSnapshot(t, db, "op-1", recoverySnapshot(12))
	revision, err := repo.ResolveDesiredRevision(context.Background(), "op-1", "space-1", "device-1")
	if err != nil {
		t.Fatalf("resolve revision: %v", err)
	}
	if revision != 12 {
		t.Fatalf("revision=%d want 12", revision)
	}
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func containsAll(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
