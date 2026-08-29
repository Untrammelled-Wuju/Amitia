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

func TestProjectionBridge_RecenterACKCompletesOriginalOperation(t *testing.T) {
	db := newProjectionBridgeP0TestDB(t)
	ctx := context.Background()
	if err := db.Create(&runtimeSessionIdentity{ID: "sess-1", UserID: "u", DeviceID: "d", RuntimeID: "r"}).Error; err != nil {
		t.Fatalf("create session identity: %v", err)
	}
	op := &operation.InstallationOperation{
		ID: "op-recenter-1", OperationType: operation.TypeRecenter,
		UserID: "u", DeviceID: "d", RuntimeID: "r", InstallationID: "inst",
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
	if err := db.Create(&runtimeSessionIdentity{ID: "sess-2", UserID: "u", DeviceID: "d", RuntimeID: "r"}).Error; err != nil {
		t.Fatalf("create session identity: %v", err)
	}
	op := &operation.InstallationOperation{
		ID: "op-recenter-fast", OperationType: operation.TypeRecenter,
		UserID: "u", DeviceID: "d", RuntimeID: "r", InstallationID: "inst",
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
