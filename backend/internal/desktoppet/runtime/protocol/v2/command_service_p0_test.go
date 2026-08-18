package v2

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCommandServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&RuntimeCommand{}); err != nil {
		t.Fatalf("migrate runtime command: %v", err)
	}
	return db
}

func TestCommandProgressNeverRegressesWhenAckBeatsTransportMark(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-race", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", CommandType: string(CommandTypePlayAction), Durability: "ephemeral",
		Status: string(CommandStatusQueued), PayloadJSON: `{}`, CreatedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatalf("create command: %v", err)
	}
	if err := svc.MarkDispatching(cmd.ID, "runtime-1", now); err != nil {
		t.Fatalf("dispatching: %v", err)
	}
	if err := svc.MarkRuntimeReceived(cmd.ID, "runtime-1", "session-1", now.Add(time.Millisecond)); err != nil {
		t.Fatalf("runtime received: %v", err)
	}
	// This is the dispatcher write that can race behind a very fast ACK.
	if err := svc.MarkTransportDispatched(cmd.ID, "runtime-1", now.Add(2*time.Millisecond)); err != nil {
		t.Fatalf("late transport mark: %v", err)
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got.Status != string(CommandStatusRuntimeReceived) {
		t.Fatalf("status regressed: got %s", got.Status)
	}
}

func TestCompletedCommandCannotBeOverwrittenByLateFailure(t *testing.T) {
	db := newCommandServiceTestDB(t)
	svc := NewCommandService(db)
	now := time.Now().UTC()
	cmd := &RuntimeCommand{
		ID: "cmd-terminal", UserID: "u", DeviceID: "d", RuntimeID: "runtime-1", CommandType: string(CommandTypeRecenterOnce), Durability: "ephemeral",
		Status: string(CommandStatusQueued), PayloadJSON: `{}`, CreatedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339),
	}
	if err := db.Create(cmd).Error; err != nil {
		t.Fatalf("create command: %v", err)
	}
	if err := svc.MarkCompleted(cmd.ID, "", now); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := svc.MarkFailed(cmd.ID, "LATE", "late failure", now.Add(time.Millisecond)); err == nil {
		t.Fatal("late failure must not overwrite completed terminal state")
	}
	got, err := svc.GetCommand(cmd.ID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if got.Status != string(CommandStatusCompleted) {
		t.Fatalf("terminal status overwritten: got %s", got.Status)
	}
}
