package mindruntime

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type recordingCleanupExecutor struct {
	calls         []OutboxCleanupItem
	failByStorage map[string]error
}

func (e *recordingCleanupExecutor) CleanupOutboxItem(item OutboxCleanupItem) error {
	e.calls = append(e.calls, item)
	if e.failByStorage != nil {
		if err, ok := e.failByStorage[item.Storage]; ok {
			return err
		}
	}
	return nil
}

func findOutboxItemByStorage(items []OutboxCleanupItem, storage string) (OutboxCleanupItem, bool) {
	for _, item := range items {
		if item.Storage == storage {
			return item, true
		}
	}
	return OutboxCleanupItem{}, false
}

func TestNewDataLifecycleCoordinator(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)
	if c == nil {
		t.Fatal("coordinator should not be nil")
	}
	stats := c.Stats()
	if stats["tombstones"].(int) != 0 {
		t.Fatal("new coordinator should have 0 tombstones")
	}
	if stats["outboxItems"].(int) != 0 {
		t.Fatal("new coordinator should have 0 outbox items")
	}
	if stats["recalcTasks"].(int) != 0 {
		t.Fatal("new coordinator should have 0 recalc tasks")
	}
}

func TestRequestDeletion(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	req := DeletionRequest{
		TargetID:   "test-target-001",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "privacy request",
	}

	tombstone, _ := c.RequestDeletion(req)
	if tombstone.ID == "" {
		t.Fatal("tombstone ID should not be empty")
	}
	if tombstone.TargetID != req.TargetID {
		t.Fatalf("expected target ID %s, got %s", req.TargetID, tombstone.TargetID)
	}
	if tombstone.Status != DeletionStatusBlocked {
		t.Fatalf("expected status blocked, got %s", tombstone.Status)
	}
	if !tombstone.RetrievalBlocked {
		t.Fatal("retrieval should be blocked after deletion request")
	}
}

func TestIsRetrievalBlocked(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	req := DeletionRequest{
		TargetID:   "blocked-target",
		TargetType: "conversation",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	c.RequestDeletion(req)

	if !c.IsRetrievalBlocked("blocked-target") {
		t.Fatal("blocked target should report retrieval blocked")
	}

	if c.IsRetrievalBlocked("non-existent") {
		t.Fatal("non-existent target should not be blocked")
	}
}

func TestIsRetrievalBlockedCaseInsensitive(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	req := DeletionRequest{
		TargetID:   "Case-Sensitive-Target",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	c.RequestDeletion(req)

	if !c.IsRetrievalBlocked("case-sensitive-target") {
		t.Fatal("retrieval blocking should be case insensitive")
	}
}

func TestGetTombstone(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	req := DeletionRequest{
		TargetID:   "get-tombstone-test",
		TargetType: "memory",
		Scope:      DeletionScopeBelief,
		Reason:     "test",
	}
	created, _ := c.RequestDeletion(req)

	found, ok := c.GetTombstone("get-tombstone-test")
	if !ok {
		t.Fatal("should find tombstone for created target")
	}
	if found.TargetID != created.TargetID {
		t.Fatalf("found tombstone target %s != created %s", found.TargetID, created.TargetID)
	}
	if found.Scope != DeletionScopeBelief {
		t.Fatalf("expected scope belief, got %s", found.Scope)
	}
}

func TestGetTombstoneNotFound(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	_, ok := c.GetTombstone("non-existent")
	if ok {
		t.Fatal("should not find tombstone for non-existent target")
	}
}

func TestExecuteOutboxCleanup(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)
	executor := &recordingCleanupExecutor{}
	c.SetOutboxCleanupExecutor(executor)

	req := DeletionRequest{
		TargetID:   "outbox-test",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	c.RequestDeletion(req)

	results, err := c.ExecuteOutboxCleanup()
	if err != nil {
		t.Fatalf("outbox cleanup should succeed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("outbox cleanup should return items")
	}
	if len(executor.calls) != len(results) {
		t.Fatalf("cleanup executor should be called for every item, got %d calls for %d results", len(executor.calls), len(results))
	}

	expectedStorages := map[string]bool{
		"primary": false, "sqlite": false, "qdrant": false, "surrealdb": false,
		"cache": false, "summaries": false, "reflections": false, "traces": false,
	}
	for _, item := range results {
		if item.Storage != "" {
			expectedStorages[item.Storage] = true
		}
		if item.Status != "completed" {
			t.Fatalf("outbox item %s should be completed, got %s", item.ID, item.Status)
		}
		if item.CleanedAt == nil {
			t.Fatalf("outbox item %s should have cleanedAt set", item.ID)
		}
	}
	for storage, found := range expectedStorages {
		if !found {
			t.Fatalf("outbox cleanup missing storage: %s", storage)
		}
	}

	stats := c.Stats()
	if stats["outboxItems"].(int) != len(results) {
		t.Fatal("stats should reflect outbox items count")
	}
	if stats["completed"].(int) != 1 {
		t.Fatalf("tombstone should be completed after all cleanup items complete, got %d", stats["completed"])
	}
	tombstone, ok := c.GetTombstone(req.TargetID)
	if !ok {
		t.Fatal("tombstone should still be available after cleanup")
	}
	if tombstone.Status != DeletionStatusCompleted {
		t.Fatalf("expected tombstone status completed, got %s", tombstone.Status)
	}
	if tombstone.CompletedAt == nil {
		t.Fatal("completedAt should be set when cleanup finishes all items")
	}
	if tombstone.CleanedCount != len(results) {
		t.Fatalf("expected cleaned count %d, got %d", len(results), tombstone.CleanedCount)
	}
}

func TestExecuteOutboxCleanupRequiresExecutor(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)
	c.RequestDeletion(DeletionRequest{
		TargetID:   "missing-executor-test",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	})

	results, err := c.ExecuteOutboxCleanup()
	if err == nil {
		t.Fatal("outbox cleanup should fail without an executor")
	}
	if len(results) != 8 {
		t.Fatalf("expected 6 outbox items, got %d", len(results))
	}
	for _, item := range results {
		if item.Status != "retry" {
			t.Fatalf("item %s should stay retry without executor, got %s", item.ID, item.Status)
		}
		if item.Attempts != 1 {
			t.Fatalf("item %s attempts should be 1, got %d", item.ID, item.Attempts)
		}
		if item.CleanedAt != nil {
			t.Fatalf("item %s should not have cleanedAt on failure", item.ID)
		}
		if item.LastError == "" {
			t.Fatalf("item %s should keep last error", item.ID)
		}
	}
}

func TestExecuteOutboxCleanupRetriesFailedItem(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)
	executor := &recordingCleanupExecutor{
		failByStorage: map[string]error{
			"qdrant": errors.New("qdrant delete failed"),
		},
	}
	c.SetOutboxCleanupExecutor(executor)
	c.RequestDeletion(DeletionRequest{
		TargetID:   "retry-outbox-test",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	})

	results, err := c.ExecuteOutboxCleanup()
	if err == nil {
		t.Fatal("outbox cleanup should return executor error")
	}
	failed, ok := findOutboxItemByStorage(results, "qdrant")
	if !ok {
		t.Fatal("qdrant outbox item should exist")
	}
	if failed.Status != "retry" {
		t.Fatalf("qdrant item should be retry, got %s", failed.Status)
	}
	if failed.Attempts != 1 {
		t.Fatalf("qdrant attempts should be 1, got %d", failed.Attempts)
	}
	if failed.CleanedAt != nil {
		t.Fatal("failed qdrant cleanup should not set cleanedAt")
	}
	if failed.LastError == "" {
		t.Fatal("failed qdrant cleanup should store last error")
	}
	cacheItem, ok := findOutboxItemByStorage(results, "cache")
	if !ok {
		t.Fatal("cache outbox item should exist")
	}
	if cacheItem.Status != "completed" {
		t.Fatalf("successful items should complete, got %s", cacheItem.Status)
	}

	executor.failByStorage = nil
	results, err = c.ExecuteOutboxCleanup()
	if err != nil {
		t.Fatalf("retry cleanup should succeed: %v", err)
	}
	retried, ok := findOutboxItemByStorage(results, "qdrant")
	if !ok {
		t.Fatal("qdrant outbox item should still exist")
	}
	if retried.Status != "completed" {
		t.Fatalf("retried qdrant item should complete, got %s", retried.Status)
	}
	if retried.Attempts != 1 {
		t.Fatalf("retried qdrant attempts should be 1, got %d", retried.Attempts)
	}
	if retried.CleanedAt == nil {
		t.Fatal("retried qdrant cleanup should set cleanedAt")
	}
	if retried.LastError != "" {
		t.Fatalf("retried qdrant cleanup should clear last error, got %s", retried.LastError)
	}
	if len(executor.calls) != 9 {
		t.Fatalf("executor should be called for 6 initial items and 1 retry, got %d", len(executor.calls))
	}
}

func TestGenerateRecalculationTasksAllScope(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	tombstone := DeletionTombstone{
		ID:         "test-tombstone",
		TargetID:   "recalc-target",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
	}

	tasks := c.GenerateRecalculationTasks(tombstone)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 recalc tasks for all scope, got %d", len(tasks))
	}

	zones := map[string]bool{}
	for _, task := range tasks {
		zones[task.AffectedZone] = true
		if task.TriggerType != "deletion" {
			t.Fatalf("task %s should have trigger type deletion", task.ID)
		}
		if task.Status != "pending" {
			t.Fatalf("task %s should be pending", task.ID)
		}
	}
	if !zones["belief"] || !zones["relationship"] || !zones["memory"] {
		t.Fatal("all zones should be covered for all scope")
	}
}

func TestGenerateRecalculationTasksBeliefScope(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	tombstone := DeletionTombstone{
		ID:         "test-belief",
		TargetID:   "recalc-belief",
		TargetType: "belief",
		Scope:      DeletionScopeBelief,
	}

	tasks := c.GenerateRecalculationTasks(tombstone)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 recalc task for belief scope, got %d", len(tasks))
	}
	if tasks[0].AffectedZone != "belief" {
		t.Fatalf("expected belief zone, got %s", tasks[0].AffectedZone)
	}
}

func TestGenerateRecalculationTasksMemoryScope(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	tombstone := DeletionTombstone{
		ID:         "test-memory",
		TargetID:   "recalc-memory",
		TargetType: "memory",
		Scope:      DeletionScopeMemory,
	}

	tasks := c.GenerateRecalculationTasks(tombstone)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 recalc task for memory scope, got %d", len(tasks))
	}
	if tasks[0].AffectedZone != "memory" {
		t.Fatalf("expected memory zone, got %s", tasks[0].AffectedZone)
	}
}

func TestMarkDeletionComplete(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	req := DeletionRequest{
		TargetID:   "complete-test",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	c.RequestDeletion(req)

	tombstone, ok := c.MarkDeletionComplete("complete-test")
	if !ok {
		t.Fatal("should mark deletion complete")
	}
	if tombstone.Status != DeletionStatusCompleted {
		t.Fatalf("expected status completed, got %s", tombstone.Status)
	}
	if tombstone.CompletedAt == nil {
		t.Fatal("completedAt should be set")
	}
}

func TestMarkDeletionCompleteNotFound(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	_, ok := c.MarkDeletionComplete("non-existent")
	if ok {
		t.Fatal("should not mark non-existent deletion as complete")
	}
}

func TestCoordinatorStats(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)
	c.SetOutboxCleanupExecutor(&recordingCleanupExecutor{})

	req1 := DeletionRequest{
		TargetID:   "stats-test-1",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	c.RequestDeletion(req1)

	req2 := DeletionRequest{
		TargetID:   "stats-test-2",
		TargetType: "conversation",
		Scope:      DeletionScopeBelief,
		Reason:     "test",
	}
	c.RequestDeletion(req2)

	if _, err := c.ExecuteOutboxCleanup(); err != nil {
		t.Fatalf("outbox cleanup should succeed: %v", err)
	}
	c.MarkDeletionComplete("stats-test-1")

	stats := c.Stats()
	if stats["tombstones"].(int) != 2 {
		t.Fatalf("expected 2 tombstones, got %d", stats["tombstones"])
	}
	if stats["completed"].(int) != 2 {
		t.Fatalf("expected 2 completed, got %d", stats["completed"])
	}
}

func TestCoordinatorReset(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)
	c.SetOutboxCleanupExecutor(&recordingCleanupExecutor{})

	req := DeletionRequest{
		TargetID:   "reset-test",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	c.RequestDeletion(req)
	if _, err := c.ExecuteOutboxCleanup(); err != nil {
		t.Fatalf("outbox cleanup should succeed: %v", err)
	}

	c.Reset()

	stats := c.Stats()
	if stats["tombstones"].(int) != 0 {
		t.Fatal("after reset tombstones should be 0")
	}
	if stats["outboxItems"].(int) != 0 {
		t.Fatal("after reset outbox items should be 0")
	}
	if stats["recalcTasks"].(int) != 0 {
		t.Fatal("after reset recalc tasks should be 0")
	}
}

func TestGetOutboxItems(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	req := DeletionRequest{
		TargetID:   "get-outbox-test",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	c.RequestDeletion(req)

	items := c.GetOutboxItems()
	if len(items) == 0 {
		t.Fatal("should have outbox items after deletion request")
	}

	for _, item := range items {
		if item.Storage == "" {
			t.Fatal("outbox item should have storage set")
		}
	}
}

func TestGetRecalculationTasks(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	tombstone := DeletionTombstone{
		ID:         "recalc-get-test",
		TargetID:   "get-recalc",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
	}
	c.GenerateRecalculationTasks(tombstone)

	tasks := c.GetRecalculationTasks()
	if len(tasks) != 3 {
		t.Fatalf("expected 3 recalc tasks, got %d", len(tasks))
	}
}

func TestMultipleConcurrentDeletionRequests(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			req := DeletionRequest{
				TargetID:   fmt.Sprintf("concurrent-%d", idx),
				TargetType: "memory",
				Scope:      DeletionScopeAll,
				Reason:     "test",
			}
			c.RequestDeletion(req)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := c.Stats()
	if stats["tombstones"].(int) != 10 {
		t.Fatalf("expected 10 tombstones, got %d", stats["tombstones"])
	}
}

func TestDeletionScopes(t *testing.T) {
	scopes := []DeletionScope{
		DeletionScopeMemory,
		DeletionScopeBelief,
		DeletionScopeRelation,
		DeletionScopeTrace,
		DeletionScopeAll,
	}
	if len(scopes) != 5 {
		t.Fatalf("expected 5 deletion scopes, got %d", len(scopes))
	}
	scopeStrs := make(map[string]bool)
	for _, s := range scopes {
		scopeStrs[string(s)] = true
	}
	if !scopeStrs["memory"] || !scopeStrs["belief"] || !scopeStrs["relation"] || !scopeStrs["trace"] || !scopeStrs["all"] {
		t.Fatal("missing expected deletion scopes")
	}
}

func TestDeletionStatusTransitions(t *testing.T) {
	c := NewDataLifecycleCoordinator(nil)

	req := DeletionRequest{
		TargetID:   "transition-test",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	tombstone, _ := c.RequestDeletion(req)
	if tombstone.Status != DeletionStatusBlocked {
		t.Fatalf("initial status should be blocked, got %s", tombstone.Status)
	}

	completed, ok := c.MarkDeletionComplete("transition-test")
	if !ok {
		t.Fatal("should mark complete")
	}
	if completed.Status != DeletionStatusCompleted {
		t.Fatalf("final status should be completed, got %s", completed.Status)
	}
}

func TestDataLifecyclePersistenceRestoresDerivedCleanupState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "data_lifecycle.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	c := NewDataLifecycleCoordinator(db)
	c.SetOutboxCleanupExecutor(&recordingCleanupExecutor{})
	if err := c.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	req := DeletionRequest{
		TargetID:   "persist-derived-target",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	tombstone, _ := c.RequestDeletion(req)
	for i := 0; i < 10; i++ {
		results, _ := c.ExecuteOutboxCleanup()
		if len(results) == 0 {
			break
		}
	}
	c.GenerateRecalculationTasks(tombstone)
	completed, ok := c.GetTombstone(req.TargetID)
	if !ok {
		t.Fatal("should find persisted deletion after cleanup")
	}
	if completed.Status != DeletionStatusCompleted {
		t.Fatalf("expected completed tombstone before reload, got %s", completed.Status)
	}
	if completed.CompletedAt == nil {
		t.Fatal("completedAt should be set before reload")
	}

	reloaded := NewDataLifecycleCoordinator(db)
	if err := reloaded.InitSchema(); err != nil {
		t.Fatalf("reload schema: %v", err)
	}

	found, ok := reloaded.GetTombstone(req.TargetID)
	if !ok {
		t.Fatal("reloaded coordinator should find persisted tombstone")
	}
	if found.Status != DeletionStatusCompleted {
		t.Fatalf("expected completed tombstone after reload, got %s", found.Status)
	}
	if !reloaded.IsRetrievalBlocked(req.TargetID) {
		t.Fatal("retrieval block should survive reload")
	}

	items := reloaded.GetOutboxItems()
	if len(items) != 8 {
		t.Fatalf("expected 8 persisted outbox items, got %d", len(items))
	}
	for _, item := range items {
		if item.Status != "completed" {
			t.Fatalf("expected persisted outbox item completed, got %s", item.Status)
		}
		if item.CleanedAt == nil {
			t.Fatal("persisted outbox item should keep cleanedAt")
		}
	}

	tasks := reloaded.GetRecalculationTasks()
	if len(tasks) != 3 {
		t.Fatalf("expected 3 persisted recalculation tasks, got %d", len(tasks))
	}
	zones := map[string]bool{}
	for _, task := range tasks {
		zones[task.AffectedZone] = true
		if task.Status != "pending" {
			t.Fatalf("expected persisted recalc task pending, got %s", task.Status)
		}
	}
	if !zones["belief"] || !zones["relationship"] || !zones["memory"] {
		t.Fatal("persisted recalculation tasks should cover all derived zones")
	}
}

func TestDataLifecyclePersistedCleanupQueueExecutesAfterReload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "data_lifecycle_reload.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	c := NewDataLifecycleCoordinator(db)
	if err := c.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	req := DeletionRequest{
		TargetID:   "reload-cleanup-target",
		TargetType: "memory",
		Scope:      DeletionScopeAll,
		Reason:     "test",
	}
	c.RequestDeletion(req)

	reloaded := NewDataLifecycleCoordinator(db)
	executor := &recordingCleanupExecutor{}
	reloaded.SetOutboxCleanupExecutor(executor)
	if err := reloaded.InitSchema(); err != nil {
		t.Fatalf("reload schema: %v", err)
	}
	totalResults := 0
	for i := 0; i < 10; i++ {
		batchResults, err := reloaded.ExecuteOutboxCleanup()
		if err != nil {
			t.Fatalf("reloaded cleanup should succeed: %v", err)
		}
		totalResults += len(batchResults)
		if len(batchResults) == 0 {
			break
		}
	}
	if totalResults != 8 {
		t.Fatalf("expected 8 cleanup results, got %d", totalResults)
	}
	if len(executor.calls) != 8 {
		t.Fatalf("expected 8 cleanup calls after reload, got %d", len(executor.calls))
	}
	tombstone, ok := reloaded.GetTombstone(req.TargetID)
	if !ok {
		t.Fatal("reloaded coordinator should keep tombstone")
	}
	if tombstone.Status != DeletionStatusCompleted {
		t.Fatalf("expected completed tombstone, got %s", tombstone.Status)
	}
	if tombstone.ItemsCount != 8 || tombstone.CleanedCount != 8 || tombstone.FailedCount != 0 {
		t.Fatalf("unexpected tombstone progress: %#v", tombstone)
	}
	if tombstone.CompletedAt == nil {
		t.Fatal("completedAt should be set after reloaded cleanup finishes all items")
	}
}
