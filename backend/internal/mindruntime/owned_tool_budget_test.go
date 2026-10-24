package mindruntime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func testOwnedToolBudget() OwnedToolBudget {
	return OwnedToolBudget{SpaceID: "core", RoleID: "role-a", OwnerID: "owner-a", ParentInteractionID: "parent-a", CoordinationID: "coord-a", AssignmentID: "assignment-a", Limit: 3}
}

func TestOwnedToolBudgetAtomicAdmissionAndRestart(t *testing.T) {
	store, db := usageTestStore(t)
	budget := testOwnedToolBudget()
	if err := store.ReserveOwnedToolCall(context.Background(), budget, "call-1", "shell_run"); err != nil {
		t.Fatal(err)
	}
	if err := store.ReserveOwnedToolCall(context.Background(), budget, "call-1", "shell_run"); !errors.Is(err, ErrOwnedToolEffectUncertain) {
		t.Fatalf("duplicate effect must not repeat: %v", err)
	}
	next := budget
	next.AssignmentID = "assignment-b"
	if err := store.ReserveOwnedToolCall(context.Background(), next, "call-1", "read_file"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewModelUsageStore(db, "core")
	if err != nil {
		t.Fatal(err)
	}
	if err := reloaded.ReserveOwnedToolCall(context.Background(), next, "call-2", "read_file"); err != nil {
		t.Fatal(err)
	}
	if err := reloaded.ReserveOwnedToolCall(context.Background(), budget, "call-2", "shell_run"); !errors.Is(err, ErrOwnedToolBudgetExceeded) {
		t.Fatalf("overspend allowed after restart: %v", err)
	}
	var count int64
	if err := db.Model(&OwnedToolCallRecord{}).Where("space_id=? AND coordination_id=?", budget.SpaceID, budget.CoordinationID).Count(&count).Error; err != nil || count != 3 {
		t.Fatalf("persisted entries: %d %v", count, err)
	}
}

func TestOwnedToolBudgetConcurrentSiblingsCannotOverspend(t *testing.T) {
	store, db := usageTestStore(t)
	budget := testOwnedToolBudget()
	budget.Limit = 4
	var succeeded atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 32; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			scope := budget
			scope.AssignmentID = fmt.Sprintf("child-%d", n%4)
			err := store.ReserveOwnedToolCall(context.Background(), scope, fmt.Sprintf("call-%d", n), "tool")
			if err == nil {
				succeeded.Add(1)
			}
		}(n)
	}
	wg.Wait()
	var count int64
	if err := db.Model(&OwnedToolCallRecord{}).Where("space_id=? AND coordination_id=?", budget.SpaceID, budget.CoordinationID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count > int64(budget.Limit) || succeeded.Load() != int32(count) {
		t.Fatalf("concurrent budget breached, admitted %d persisted %d", succeeded.Load(), count)
	}
	if count == 0 {
		t.Fatal("all budget admissions failed")
	}
}

func TestOwnedToolBudgetFailClosed(t *testing.T) {
	store, _ := usageTestStore(t)
	budget := testOwnedToolBudget()
	for _, mutation := range []func(*OwnedToolBudget){
		func(b *OwnedToolBudget) { b.Limit = 0 },
		func(b *OwnedToolBudget) { b.Limit = 257 },
		func(b *OwnedToolBudget) { b.OwnerID = "" },
		func(b *OwnedToolBudget) { b.CoordinationID = "" },
	} {
		candidate := budget
		mutation(&candidate)
		if err := store.ReserveOwnedToolCall(context.Background(), candidate, "id", "tool"); err == nil {
			t.Fatalf("accepted invalid budget %+v", candidate)
		}
	}
	if err := store.ReserveOwnedToolCall(context.Background(), budget, "", "tool"); err == nil {
		t.Fatal("accepted empty call id")
	}
}
