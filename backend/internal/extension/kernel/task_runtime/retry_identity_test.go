package task_runtime

import (
	"strings"
	"sync"
	"testing"
)

func TestRound105RetryIdentityIsStableAndOriginBound(t *testing.T) {
	origin := &TaskRun{TaskRunID: "tr-original", OperationID: "op", InvocationID: "turn", TaskDefinitionID: "definition", DefinitionFingerprint: strings.Repeat("f", 64), ScopeSnapshotID: "scope", PermissionSnapshotID: "permissions", DependencySnapshotID: "dependencies", ExtensionID: "extension", ModuleID: "module", InputHash: "hash", Attempt: 1, MaxAttempts: 3, Generation: 7}
	id := retryTaskRunID(origin.TaskRunID)
	if id == "" || id == origin.TaskRunID || len(id) != 39 || retryTaskRunID(origin.TaskRunID) != id || retryTaskRunID("another") == id || retryTaskRunID("") != "" {
		t.Fatalf("retry identity unstable: %q", id)
	}
	retried := &TaskRun{TaskRunID: id, OperationID: origin.OperationID, InvocationID: origin.InvocationID, TaskDefinitionID: origin.TaskDefinitionID, DefinitionFingerprint: origin.DefinitionFingerprint, ScopeSnapshotID: origin.ScopeSnapshotID, PermissionSnapshotID: origin.PermissionSnapshotID, DependencySnapshotID: origin.DependencySnapshotID, ExtensionID: origin.ExtensionID, ModuleID: origin.ModuleID, InputHash: origin.InputHash, Attempt: 2, MaxAttempts: 3, Generation: 8}
	if !matchingRetryRun(origin, retried) {
		t.Fatal("valid retry identity was rejected")
	}
	retried.Generation = 9
	retried.Attempt = 3
	if !matchingRetryRun(origin, retried) {
		t.Fatal("existing retry must stay identifiable after a later execution generation")
	}
	retried.ScopeSnapshotID = "other-owner"
	if matchingRetryRun(origin, retried) {
		t.Fatal("different owner reused retry identity")
	}
	retried.ScopeSnapshotID = origin.ScopeSnapshotID
	retried.InputHash = "different-input"
	if matchingRetryRun(origin, retried) {
		t.Fatal("different input reused retry identity")
	}
}

func TestRound105ConcurrentRetryIDsAreIdentical(t *testing.T) {
	const workers = 64
	results := make(chan string, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- retryTaskRunID("tr-concurrent") }()
	}
	wg.Wait()
	close(results)
	for id := range results {
		if id != retryTaskRunID("tr-concurrent") {
			t.Fatalf("concurrent retries split into different IDs: %s", id)
		}
	}
}
