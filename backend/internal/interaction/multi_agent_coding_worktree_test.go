package interaction

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type testCodingWorktreeAllocator struct {
	mu        sync.Mutex
	calls     []WorkerRunRequest
	err       error
	workspace CodingWorkerWorkspace
}

func (a *testCodingWorktreeAllocator) EnsureCodingWorkerWorktree(_ context.Context, req WorkerRunRequest) (CodingWorkerWorkspace, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, req)
	return a.workspace, a.err
}

type testCodingUnderlyingWorker struct {
	mu       sync.Mutex
	requests []WorkerRunRequest
	stopped  bool
}

func (w *testCodingUnderlyingWorker) StartWorker(_ context.Context, req WorkerRunRequest) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.requests = append(w.requests, req)
	return "child-" + req.AssignmentID, nil
}

func (w *testCodingUnderlyingWorker) WorkerInteractionID(req WorkerRunRequest) string {
	return "child-" + req.AssignmentID
}

func (w *testCodingUnderlyingWorker) WorkerIsActive(id string) bool {
	return id == "child-task-a"
}

func (w *testCodingUnderlyingWorker) Shutdown(context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopped = true
	return nil
}

func codingWorktreeRequest() WorkerRunRequest {
	return WorkerRunRequest{
		Source: "multi_agent", WorkerRole: "coding_worker", AssignmentID: "task-a",
		ConversationID: "parent-conversation", WorkspaceID: "main",
		WorkspaceRootURI: "amitia://workspace/@main", WorkspaceKind: "local",
		WorkspaceName: "Original", SpaceID: "space-a", WorkspaceDeviceID: "device-a",
	}
}

func TestCodingWorkerAllocatesBeforeStartAndUsesOnlyIsolatedMount(t *testing.T) {
	underlying := &testCodingUnderlyingWorker{}
	allocator := &testCodingWorktreeAllocator{workspace: CodingWorkerWorkspace{
		WorkspaceID: "isolated", WorkspaceRootURI: "amitia://workspace/@isolated",
		WorkspaceKind: "local", WorkspaceName: "Coding Agent task-a",
	}}
	runner := NewCodingWorktreeWorkerRunner(underlying, allocator)
	req := codingWorktreeRequest()
	child, err := runner.StartWorker(context.Background(), req)
	if err != nil || child != "child-task-a" {
		t.Fatalf("child: %q %v", child, err)
	}
	if len(allocator.calls) != 1 || len(underlying.requests) != 1 {
		t.Fatalf("invalid provisioning order: %+v %+v", allocator.calls, underlying.requests)
	}
	got := underlying.requests[0]
	if got.WorkspaceID != "isolated" || got.WorkspaceRootURI != "amitia://workspace/@isolated" ||
		got.WorkspaceKind != "local" || got.SpaceID != req.SpaceID ||
		got.ConversationID != req.ConversationID || got.WorkspaceDeviceID != req.WorkspaceDeviceID {
		t.Fatalf("worker did not receive isolated and correctly scoped mount: %+v", got)
	}
	if runner.(interface{ WorkerInteractionID(WorkerRunRequest) string }).WorkerInteractionID(req) != child ||
		!runner.(interface{ WorkerIsActive(string) bool }).WorkerIsActive(child) {
		t.Fatal("worker reservation and recovery hooks not forwarded")
	}
	if err := runner.(interface{ Shutdown(context.Context) error }).Shutdown(context.Background()); err != nil || !underlying.stopped {
		t.Fatalf("shutdown was not forwarded: %v", err)
	}
}

func TestCodingWorkerFailsClosedBeforeDispatch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*WorkerRunRequest)
	}{
		{"invalid source", func(r *WorkerRunRequest) { r.Source = "interactive" }},
		{"missing assignment", func(r *WorkerRunRequest) { r.AssignmentID = "" }},
		{"missing conversation", func(r *WorkerRunRequest) { r.ConversationID = "" }},
		{"missing workspace", func(r *WorkerRunRequest) { r.WorkspaceID = "" }},
		{"missing root", func(r *WorkerRunRequest) { r.WorkspaceRootURI = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			worker := &testCodingUnderlyingWorker{}
			allocator := &testCodingWorktreeAllocator{}
			req := codingWorktreeRequest()
			tc.mutate(&req)
			_, err := NewCodingWorktreeWorkerRunner(worker, allocator).StartWorker(context.Background(), req)
			if err == nil || len(worker.requests) > 0 || len(allocator.calls) > 0 {
				t.Fatalf("unauthorized dispatch: %v %+v", err, worker.requests)
			}
		})
	}
}

func TestCodingWorkerAllocatorFailurePreservesParentWorkspace(t *testing.T) {
	failure := errors.New("no local repository")
	allocator := &testCodingWorktreeAllocator{err: failure}
	worker := &testCodingUnderlyingWorker{}
	req := codingWorktreeRequest()
	_, err := NewCodingWorktreeWorkerRunner(worker, allocator).StartWorker(context.Background(), req)
	if !errors.Is(err, failure) || len(worker.requests) != 0 || req.WorkspaceID != "main" {
		t.Fatalf("unsafe fallback to shared workspace: %v %+v", err, worker.requests)
	}
}

func TestCodingWorkerRejectsInvalidAllocationResult(t *testing.T) {
	allocator := &testCodingWorktreeAllocator{workspace: CodingWorkerWorkspace{
		WorkspaceID: "unexpected", WorkspaceRootURI: "", WorkspaceKind: "remote",
	}}
	worker := &testCodingUnderlyingWorker{}
	_, err := NewCodingWorktreeWorkerRunner(worker, allocator).StartWorker(context.Background(), codingWorktreeRequest())
	if err == nil || len(worker.requests) != 0 {
		t.Fatalf("invalid allocation reached worker: %v", err)
	}
}

func TestNonCodingWorkerPassesThroughWithoutProvision(t *testing.T) {
	allocator := &testCodingWorktreeAllocator{}
	worker := &testCodingUnderlyingWorker{}
	req := codingWorktreeRequest()
	req.WorkerRole = "researcher"
	_, err := NewCodingWorktreeWorkerRunner(worker, allocator).StartWorker(context.Background(), req)
	if err != nil || len(allocator.calls) != 0 || len(worker.requests) != 1 || worker.requests[0].WorkspaceID != "main" {
		t.Fatalf("non-coding worker was modified: %v", err)
	}
}

func TestCodingWorkerConcurrentStartDoesNotBypassAllocator(t *testing.T) {
	allocator := &testCodingWorktreeAllocator{workspace: CodingWorkerWorkspace{
		WorkspaceID: "isolated", WorkspaceRootURI: "amitia://workspace/@isolated", WorkspaceKind: "local",
	}}
	worker := &testCodingUnderlyingWorker{}
	runner := NewCodingWorktreeWorkerRunner(worker, allocator)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := runner.StartWorker(context.Background(), codingWorktreeRequest()); err != nil {
				t.Errorf("concurrent provision: %v", err)
			}
		}()
	}
	wg.Wait()
	if len(allocator.calls) != 8 || len(worker.requests) != 8 {
		t.Fatalf("some workers bypassed provisioning: %d %d", len(allocator.calls), len(worker.requests))
	}
}

func TestCodingWorkerRejectsAllocatorSharedOrMismatchedMount(t *testing.T) {
	cases := []struct {
		name      string
		workspace CodingWorkerWorkspace
	}{
		{"shared source", CodingWorkerWorkspace{WorkspaceID: "main", WorkspaceRootURI: "amitia://workspace/@main", WorkspaceKind: "local"}},
		{"shared source trailing slash", CodingWorkerWorkspace{WorkspaceID: "main", WorkspaceRootURI: "amitia://workspace/@main/", WorkspaceKind: "local"}},
		{"wrong mount identity", CodingWorkerWorkspace{WorkspaceID: "isolated", WorkspaceRootURI: "amitia://workspace/@other", WorkspaceKind: "local"}},
		{"path traversal", CodingWorkerWorkspace{WorkspaceID: "isolated", WorkspaceRootURI: "amitia://workspace/@isolated/../main", WorkspaceKind: "local"}},
		{"query redirect", CodingWorkerWorkspace{WorkspaceID: "isolated", WorkspaceRootURI: "amitia://workspace/@isolated?device=other", WorkspaceKind: "local"}},
		{"fragment redirect", CodingWorkerWorkspace{WorkspaceID: "isolated", WorkspaceRootURI: "amitia://workspace/@isolated#other", WorkspaceKind: "local"}},
		{"invalid mount id", CodingWorkerWorkspace{WorkspaceID: "isolated/escape", WorkspaceRootURI: "amitia://workspace/@isolated/escape", WorkspaceKind: "local"}},
		{"remote mount", CodingWorkerWorkspace{WorkspaceID: "isolated", WorkspaceRootURI: "amitia://workspace/@isolated", WorkspaceKind: "remote"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allocator := &testCodingWorktreeAllocator{workspace: tc.workspace}
			worker := &testCodingUnderlyingWorker{}
			_, err := NewCodingWorktreeWorkerRunner(worker, allocator).StartWorker(context.Background(), codingWorktreeRequest())
			if err == nil || len(worker.requests) != 0 {
				t.Fatalf("invalid mount reached underlying worker: %v %+v", err, worker.requests)
			}
		})
	}
}

func TestCodingWorkerAllowsValidIsolatedMountWithTrailingSlash(t *testing.T) {
	allocator := &testCodingWorktreeAllocator{workspace: CodingWorkerWorkspace{
		WorkspaceID: "isolated_01", WorkspaceRootURI: "amitia://workspace/@isolated_01/", WorkspaceKind: "local",
	}}
	worker := &testCodingUnderlyingWorker{}
	request := codingWorktreeRequest()
	request.WorkspaceRootURI += "/"
	id, err := NewCodingWorktreeWorkerRunner(worker, allocator).StartWorker(context.Background(), request)
	if err != nil || id == "" || len(worker.requests) != 1 || worker.requests[0].WorkspaceID != "isolated_01" {
		t.Fatalf("valid isolated mount rejected: %v %+v", err, worker.requests)
	}
}

func TestCodingWorkerRejectsMalformedParentMountWithoutProvision(t *testing.T) {
	for _, root := range []string{"amitia://workspace/@main/other", "amitia://workspace/@../main", "amitia://workspace/@main?x=1", "file:///tmp/main"} {
		allocator := &testCodingWorktreeAllocator{workspace: CodingWorkerWorkspace{WorkspaceID: "child", WorkspaceRootURI: "amitia://workspace/@child", WorkspaceKind: "local"}}
		worker := &testCodingUnderlyingWorker{}
		request := codingWorktreeRequest()
		request.WorkspaceRootURI = root
		_, err := NewCodingWorktreeWorkerRunner(worker, allocator).StartWorker(context.Background(), request)
		if err == nil || len(worker.requests) != 0 || len(allocator.calls) != 0 {
			t.Fatalf("malformed parent mount allowed allocation or dispatch: %s: %v", root, err)
		}
	}
}
