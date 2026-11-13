package projection

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProjectionBridgeP0TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&InstallationRuntimeProjection{},
		&runtimeSessionIdentity{},
		&runtimeCommandProjection{},
		&operation.InstallationOperation{},
	); err != nil {
		t.Fatalf("migrate bridge test schema: %v", err)
	}
	return db
}

func TestProjectionBridge_UninstallACKLeavesStorageFinalizationPending(t *testing.T) {
	db := newProjectionBridgeP0TestDB(t)
	if err := db.Exec(`CREATE TABLE desktop_pet_runtime_desired_states (space_id TEXT, device_id TEXT, desired_revision INTEGER, operation_id TEXT);
		INSERT INTO desktop_pet_runtime_desired_states VALUES ('u','d',9,'op-uninstall');`).Error; err != nil {
		t.Fatal(err)
	}
	op := operation.InstallationOperation{
		ID: "op-uninstall", OperationType: operation.TypeUninstall, SpaceID: "u", DeviceID: "d", DesiredRevision: 9,
		Status: operation.OpStatusWaitingRuntimeACK, Stage: operation.OpStageWaitingRuntimeACK,
	}
	if err := db.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	bridge := NewProjectionBridge(db, NewService(db))
	if err := bridge.completeDesiredStateOperationFromACK(t.Context(), "u", "d", 9, "completed", "", ""); err != nil {
		t.Fatal(err)
	}
	var saved operation.InstallationOperation
	if err := db.Where("id = ?", op.ID).Take(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != operation.OpStatusWaitingRuntimeACK || saved.Stage != operation.OpStageWaitingRuntimeACK {
		t.Fatalf("runtime ACK skipped uninstall finalization: status=%s stage=%s", saved.Status, saved.Stage)
	}
}

func TestProjectionBridge_RecenterACKCompletesOriginalOperation(t *testing.T) {
	db := newProjectionBridgeP0TestDB(t)
	ctx := context.Background()
	if err := db.Create(&runtimeSessionIdentity{ID: "sess-1", SpaceID: "u", DeviceID: "d", RuntimeID: "r"}).Error; err != nil {
		t.Fatalf("create session identity: %v", err)
	}
	op := &operation.InstallationOperation{
		ID: "op-recenter-1", OperationType: operation.TypeRecenter,
		SpaceID: "u", DeviceID: "d", RuntimeID: "r", InstallationID: "inst",
		Status: operation.OpStatusWaitingRuntimeACK, Stage: operation.OpStageWaitingRuntimeACK,
	}
	if err := db.Create(op).Error; err != nil {
		t.Fatalf("create operation: %v", err)
	}
	if err := db.Create(&runtimeCommandProjection{ID: "cmd-1", IdempotencyKey: "recenter:" + op.ID + ":session-1"}).Error; err != nil {
		t.Fatalf("create command projection: %v", err)
	}
	payload, err := json.Marshal(CommandAckPayload{CommandID: "cmd-1", Status: "completed"})
	if err != nil {
		t.Fatalf("marshal ack: %v", err)
	}
	bridge := NewProjectionBridge(db, NewService(db))
	if err := bridge.processEvent(ctx, RuntimeEventRecord{ID: "evt-1", EventType: "runtime.command.acknowledged", Payload: payload, RuntimeSessionID: "sess-1", CommandID: "cmd-1"}); err != nil {
		t.Fatalf("process ack: %v", err)
	}
	var got operation.InstallationOperation
	if err := db.Where("id = ?", op.ID).Take(&got).Error; err != nil {
		t.Fatalf("load operation: %v", err)
	}
	if got.Status != operation.OpStatusCompleted || got.Stage != operation.OpStageCompleted {
		t.Fatalf("expected original recenter operation completed, got status=%s stage=%s", got.Status, got.Stage)
	}
}

func TestProjectionBridge_RecenterACKBeforeCoordinatorStagePersistsIsRetryable(t *testing.T) {
	db := newProjectionBridgeP0TestDB(t)
	ctx := context.Background()
	if err := db.Create(&runtimeSessionIdentity{ID: "sess-2", SpaceID: "u", DeviceID: "d", RuntimeID: "r"}).Error; err != nil {
		t.Fatalf("create session identity: %v", err)
	}
	op := &operation.InstallationOperation{
		ID: "op-recenter-fast", OperationType: operation.TypeRecenter,
		SpaceID: "u", DeviceID: "d", RuntimeID: "r", InstallationID: "inst",
		Status: operation.OpStatusCreated, Stage: operation.OpStageRequestValidated,
	}
	if err := db.Create(op).Error; err != nil {
		t.Fatalf("create operation: %v", err)
	}
	if err := db.Create(&runtimeCommandProjection{ID: "cmd-fast", IdempotencyKey: "recenter:" + op.ID}).Error; err != nil {
		t.Fatalf("create command projection: %v", err)
	}
	payload, _ := json.Marshal(CommandAckPayload{CommandID: "cmd-fast", Status: "completed"})
	bridge := NewProjectionBridge(db, NewService(db))
	event := RuntimeEventRecord{ID: "evt-fast", EventType: "runtime.command.acknowledged", Payload: payload, RuntimeSessionID: "sess-2", CommandID: "cmd-fast"}
	if err := bridge.processEvent(ctx, event); err == nil {
		t.Fatal("expected early terminal ACK to remain retryable until coordinator persists waiting_runtime_ack")
	}
	if err := db.Model(&operation.InstallationOperation{}).Where("id = ?", op.ID).Updates(map[string]any{
		"status": operation.OpStatusWaitingRuntimeACK,
		"stage":  operation.OpStageWaitingRuntimeACK,
	}).Error; err != nil {
		t.Fatalf("advance operation: %v", err)
	}
	if err := bridge.processEvent(ctx, event); err != nil {
		t.Fatalf("retry terminal ACK: %v", err)
	}
	var got operation.InstallationOperation
	if err := db.Where("id = ?", op.ID).Take(&got).Error; err != nil {
		t.Fatalf("load operation: %v", err)
	}
	if got.Status != operation.OpStatusCompleted {
		t.Fatalf("expected retry to complete original operation, got %s", got.Status)
	}
}

func TestProjectionBridge_DesiredRejectedFailsWaitingOperation(t *testing.T) {
	db := newProjectionBridgeP0TestDB(t)
	ctx := context.Background()
	if err := db.Create(&runtimeSessionIdentity{ID: "sess-reject", SpaceID: "u", DeviceID: "d", RuntimeID: "r"}).Error; err != nil {
		t.Fatal(err)
	}
	op := &operation.InstallationOperation{
		ID:              "op-reject",
		OperationType:   operation.TypeEnable,
		SpaceID:         "u",
		DeviceID:        "d",
		RuntimeID:       "r",
		InstallationID:  "inst-reject",
		Status:          operation.OpStatusWaitingRuntimeACK,
		Stage:           operation.OpStageWaitingRuntimeACK,
		DesiredRevision: 7,
	}
	if err := db.Create(op).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&runtimeCommandProjection{ID: "cmd-reject", DesiredRevision: 7}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE desktop_pet_runtime_desired_states (
		space_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		operation_id TEXT NOT NULL,
		desired_revision INTEGER NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_runtime_desired_states
		(space_id, device_id, operation_id, desired_revision) VALUES (?, ?, ?, ?)`,
		"u", "d", "op-newer-settings", 9).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE desktop_pet_installations (
		id TEXT PRIMARY KEY,
		space_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		runtime_sync_state TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_installations
		(id, space_id, device_id, runtime_sync_state) VALUES (?, ?, ?, ?)`,
		op.InstallationID, "u", "d", "pending").Error; err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(desiredStateResultPayload{
		CommandID:       "cmd-reject",
		DesiredRevision: 7,
		ErrorCode:       "RUNTIME_INIT_FAILED",
		ErrorMessage:    "action config invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	bridge := NewProjectionBridge(db, NewService(db))
	if err := bridge.processEvent(ctx, RuntimeEventRecord{
		ID:               "evt-reject",
		EventType:        "runtime.state.desired_rejected",
		Payload:          payload,
		RuntimeSessionID: "sess-reject",
		CommandID:        "cmd-reject",
	}); err != nil {
		t.Fatal(err)
	}
	var got operation.InstallationOperation
	if err := db.Where("id = ?", op.ID).Take(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != operation.OpStatusFailedTerminal ||
		got.Stage != operation.OpStatusFailedTerminal ||
		got.ErrorCode != "RUNTIME_INIT_FAILED" ||
		got.ErrorMessage != "action config invalid" {
		t.Fatalf("unexpected operation failure projection: %+v", got)
	}
	var syncState string
	if err := db.Table("desktop_pet_installations").
		Where("id = ?", op.InstallationID).
		Pluck("runtime_sync_state", &syncState).Error; err != nil {
		t.Fatal(err)
	}
	if syncState != "failed" {
		t.Fatalf("installation sync state=%q want failed", syncState)
	}
}
