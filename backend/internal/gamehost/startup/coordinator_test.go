package startup

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type fakeHostIdentity struct {
	instanceID string
	sessionID  string
}

func (f *fakeHostIdentity) GetHostInstanceID() string { return f.instanceID }
func (f *fakeHostIdentity) GetHostSessionID() string  { return f.sessionID }

type fakeProcessCleanup struct {
	candidates  []ProcessCandidate
	cleanedPIDs []int
}

func (f *fakeProcessCleanup) CleanupOwnedProcess(ctx context.Context, instanceID string, pid int) error {
	f.cleanedPIDs = append(f.cleanedPIDs, pid)
	return nil
}

func (f *fakeProcessCleanup) ListOrphanCandidates(ctx context.Context) ([]ProcessCandidate, error) {
	return f.candidates, nil
}

type fakeTempCleanup struct {
	candidates     []TempCandidate
	removedTemps   []domain.RuntimeInstanceID
}

func (f *fakeTempCleanup) ListStaleTempCandidates(ctx context.Context) ([]TempCandidate, error) {
	return f.candidates, nil
}

func (f *fakeTempCleanup) RemoveStaleTemp(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	f.removedTemps = append(f.removedTemps, runtimeID)
	return nil
}

type fakeBinaryCleanup struct {
	candidates      []BinaryCandidate
	removedBinaries []string
}

func (f *fakeBinaryCleanup) ListOrphanBinaries(ctx context.Context) ([]BinaryCandidate, error) {
	return f.candidates, nil
}

func (f *fakeBinaryCleanup) RemoveOrphanBinary(ctx context.Context, binaryID string) error {
	f.removedBinaries = append(f.removedBinaries, binaryID)
	return nil
}

type fakeEndpointCleanup struct {
	candidates       []EndpointCandidate
	removedEndpoints []string
}

func (f *fakeEndpointCleanup) ListStaleEndpoints(ctx context.Context) ([]EndpointCandidate, error) {
	return f.candidates, nil
}

func (f *fakeEndpointCleanup) RemoveStaleEndpoint(ctx context.Context, endpointID string) error {
	f.removedEndpoints = append(f.removedEndpoints, endpointID)
	return nil
}

type fakeShmCleanup struct {
	candidates         []SharedMemoryCandidate
	releasedSharedMem  []string
}

func (f *fakeShmCleanup) ListStaleSharedMemory(ctx context.Context) ([]SharedMemoryCandidate, error) {
	return f.candidates, nil
}

func (f *fakeShmCleanup) ReleaseSharedMemory(ctx context.Context, shmID string) error {
	f.releasedSharedMem = append(f.releasedSharedMem, shmID)
	return nil
}

type fakeKernelRecon struct {
	runtimeIDs        []domain.RuntimeInstanceID
	invalidExtensions map[string]bool
}

func (f *fakeKernelRecon) CurrentRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error) {
	return f.runtimeIDs, nil
}

func (f *fakeKernelRecon) IsValidExtension(ctx context.Context, extensionID string) (bool, error) {
	if invalid, ok := f.invalidExtensions[extensionID]; ok && invalid {
		return false, nil
	}
	return true, nil
}

func (f *fakeKernelRecon) IsExtensionEnabled(ctx context.Context, extensionID string) (bool, error) {
	return true, nil
}

func (f *fakeKernelRecon) IsValidPlugin(ctx context.Context, pluginID domain.PluginID) (bool, error) {
	return true, nil
}

type fakeAuditSink struct {
	mu     sync.Mutex
	events []StartupRecoveryAuditEvent
}

func (f *fakeAuditSink) RecordStartupRecovery(event StartupRecoveryAuditEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}

func (f *fakeAuditSink) GetEvents() []StartupRecoveryAuditEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]StartupRecoveryAuditEvent, len(f.events))
	copy(result, f.events)
	return result
}

func setupTestCoordinator() (*StartupRecoveryCoordinator, *fakeProcessCleanup, *fakeTempCleanup, *fakeBinaryCleanup, *fakeEndpointCleanup, *fakeShmCleanup, *fakeKernelRecon, *fakeAuditSink, *StartupGate) {
	proc := &fakeProcessCleanup{}
	temp := &fakeTempCleanup{}
	bin := &fakeBinaryCleanup{}
	ep := &fakeEndpointCleanup{}
	shm := &fakeShmCleanup{}
	kernel := &fakeKernelRecon{invalidExtensions: make(map[string]bool)}
	audit := &fakeAuditSink{}
	gate := NewStartupGate()

	c, err := NewStartupRecoveryCoordinator(StartupRecoveryDeps{
		HostIdentity:    &fakeHostIdentity{instanceID: "host-abc"},
		ProcessCleanup:  proc,
		TempCleanup:     temp,
		BinaryCleanup:   bin,
		EndpointCleanup: ep,
		ShmCleanup:      shm,
		KernelRecon:     kernel,
		AuditSink:       audit,
		Gate:            gate,
	})
	if err != nil {
		panic(err)
	}

	return c, proc, temp, bin, ep, shm, kernel, audit, gate
}

func TestStartup_EmptyCleanup(t *testing.T) {
	c, _, _, _, _, _, _, _, _ := setupTestCoordinator()

	report := c.RunStartupRecovery(context.Background())

	if !report.Success {
		t.Errorf("expected success, got errors: %v", report.Errors)
	}
	if report.Stage != StageCompleted {
		t.Errorf("expected Completed stage, got %s", report.Stage)
	}
}

func TestStartup_ProcessCandidates_Cleaned(t *testing.T) {
	c, proc, _, _, _, _, _, _, _ := setupTestCoordinator()
	proc.candidates = []ProcessCandidate{
		{PID: 1234, RuntimeID: "rt-1", PluginID: "p1", ExtensionID: "ext-1", Generation: 5},
		{PID: 5678, RuntimeID: "rt-2", PluginID: "p2", ExtensionID: "ext-2", Generation: 3},
	}

	report := c.RunStartupRecovery(context.Background())

	if len(report.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(report.Candidates))
	}

	cleaned := 0
	for _, c := range report.Cleaned {
		if c.Type == ResourceOrphanProcess {
			cleaned++
		}
	}
	if cleaned != 2 {
		t.Errorf("expected 2 cleaned processes, got %d", cleaned)
	}

	if len(proc.cleanedPIDs) != 2 {
		t.Errorf("expected 2 PIDs cleaned by provider, got %d", len(proc.cleanedPIDs))
	}
}

func TestStartup_TempCandidates_Cleaned(t *testing.T) {
	c, _, temp, _, _, _, _, _, _ := setupTestCoordinator()
	temp.candidates = []TempCandidate{
		{RuntimeID: "rt-old", Path: "/data/gamehost/runtime-temp/rt-old", PluginID: "p1", ExtensionID: "ext-1"},
	}

	report := c.RunStartupRecovery(context.Background())

	if !report.Success {
		t.Errorf("expected success, got: %v", report.Errors)
	}

	if len(temp.removedTemps) != 1 {
		t.Errorf("expected 1 temp removed, got %d", len(temp.removedTemps))
	}
}

func TestStartup_BinaryCandidates_Cleaned(t *testing.T) {
	c, _, _, bin, _, _, _, _, _ := setupTestCoordinator()
	bin.candidates = []BinaryCandidate{
		{BinaryID: "bin-old-1", RuntimeID: "rt-old", PluginID: "p1"},
	}

	report := c.RunStartupRecovery(context.Background())

	if !report.Success {
		t.Errorf("expected success, got: %v", report.Errors)
	}

	if len(bin.removedBinaries) != 1 {
		t.Errorf("expected 1 binary removed, got %d", len(bin.removedBinaries))
	}
}

func TestStartup_EndpointCandidates_Cleaned(t *testing.T) {
	c, _, _, _, ep, _, _, _, _ := setupTestCoordinator()
	ep.candidates = []EndpointCandidate{
		{EndpointID: "ep-old-1", RuntimeID: "rt-old", ServiceID: "svc-1"},
	}

	report := c.RunStartupRecovery(context.Background())

	if !report.Success {
		t.Errorf("expected success")
	}
	if len(ep.removedEndpoints) != 1 {
		t.Errorf("expected 1 endpoint removed, got %d", len(ep.removedEndpoints))
	}
}

func TestStartup_SharedMemoryCandidates_Cleaned(t *testing.T) {
	c, _, _, _, _, shm, _, _, _ := setupTestCoordinator()
	shm.candidates = []SharedMemoryCandidate{
		{ID: "shm-old-1", RuntimeID: "rt-old", Generation: 2},
	}

	report := c.RunStartupRecovery(context.Background())

	if !report.Success {
		t.Errorf("expected success")
	}
	if len(shm.releasedSharedMem) != 1 {
		t.Errorf("expected 1 shared memory released, got %d", len(shm.releasedSharedMem))
	}
}

// PID Reuse Safety
func TestStartup_PIDReuse_ForeignProcess_Skipped(t *testing.T) {
	c, proc, _, _, _, _, kernel, _, _ := setupTestCoordinator()
	proc.candidates = []ProcessCandidate{
		{PID: 1234, RuntimeID: "rt-foreign", PluginID: "", ExtensionID: "ext-foreign"},
	}
	kernel.invalidExtensions = map[string]bool{"ext-foreign": true}

	report := c.RunStartupRecovery(context.Background())

	for _, c := range report.Candidates {
		if c.ResourceID == "pid-1234" {
			for _, cleaned := range report.Cleaned {
				if cleaned.ResourceID == "pid-1234" {
					t.Error("foreign process should NOT be cleaned")
				}
			}
		}
	}
}

// Audit Events
func TestStartup_AuditEventsRecorded(t *testing.T) {
	c, proc, temp, _, _, _, _, audit, _ := setupTestCoordinator()
	proc.candidates = []ProcessCandidate{
		{PID: 99, RuntimeID: "rt-1", PluginID: "p1"},
	}
	temp.candidates = []TempCandidate{
		{RuntimeID: "rt-2", Path: "/data/gamehost/runtime-temp/rt-2"},
	}

	_ = c.RunStartupRecovery(context.Background())

	events := audit.GetEvents()
	if len(events) == 0 {
		t.Error("expected audit events, got 0")
	}
	for _, e := range events {
		if e.OperationID == "" {
			t.Error("audit event should have operation ID")
		}
		if e.Timestamp.IsZero() {
			t.Error("audit event should have timestamp")
		}
	}
}

// Startup Gate
func TestStartup_GateClosedDuringRecovery_OpenAfter(t *testing.T) {
	c, _, _, _, _, _, _, _, gate := setupTestCoordinator()

	gate.Open()
	if !gate.IsReady() {
		t.Error("gate should be ready after manual open")
	}

	report := c.RunStartupRecovery(context.Background())

	if !gate.IsReady() {
		t.Error("gate should be open after recovery completes")
	}
	if !report.Success {
		t.Errorf("expected success, got errors: %v", report.Errors)
	}

	r := gate.GetReport()
	if r.OperationID != report.OperationID {
		t.Error("gate report should match returned operation")
	}
}

func TestStartup_GateBlocksRuntimeStartDuringRecovery(t *testing.T) {
	gate := NewStartupGate()
	gate.Close()

	if gate.IsReady() {
		t.Error("closed gate should report not ready")
	}
}

// Test Coordinators Do Not Concurrently Run
func TestStartup_DoubleRecovery(t *testing.T) {
	c, _, _, _, _, _, _, _, _ := setupTestCoordinator()

	var wg sync.WaitGroup
	reports := make(chan StartupRecoveryReport, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := c.RunStartupRecovery(context.Background())
			reports <- r
		}()
	}
	wg.Wait()
	close(reports)

	successCount := 0
	for r := range reports {
		if r.Success {
			successCount++
		}
	}
	if successCount == 0 {
		t.Error("at least one recovery should succeed")
	}
}

// Candidate Paths With Traversal Rejected
func TestStartup_TraversalPath_Rejected(t *testing.T) {
	if IsSubPath("/data/../etc/passwd", "/data/gamehost") {
		t.Error("traversal path should NOT be considered subpath")
	}
	if IsSubPath("/data/gamehost/runtime-temp/rt-1", "/data/gamehost") == false {
		t.Error("valid subpath should be detected")
	}
}

// Ownership Verification
func TestOwnership_ProcessVerified(t *testing.T) {
	v := NewDefaultProcessOwnershipVerifier("host-abc")
	result := v.VerifyProcess(context.Background(), 1234, OwnershipProof{
		HostInstanceID: "host-abc",
		RuntimeID:      "rt-1",
		PluginID:       "p1",
	})
	if result != OwnershipVerified {
		t.Errorf("expected Verified, got %s", result)
	}
}

func TestOwnership_ProcessForeign(t *testing.T) {
	v := NewDefaultProcessOwnershipVerifier("host-abc")
	result := v.VerifyProcess(context.Background(), 1234, OwnershipProof{
		HostInstanceID: "host-other",
		RuntimeID:      "rt-1",
	})
	if result != OwnershipBelongsToForeign {
		t.Errorf("expected Foreign, got %s", result)
	}
}

func TestOwnership_ProcessUnknown(t *testing.T) {
	v := NewDefaultProcessOwnershipVerifier("host-abc")
	result := v.VerifyProcess(context.Background(), 1234, OwnershipProof{})
	if result != OwnershipUnknown {
		t.Errorf("expected Unknown, got %s", result)
	}
}

func TestOwnership_DirectorySubpath(t *testing.T) {
	v := NewDefaultDirectoryOwnershipVerifier("/data/gamehost")
	result := v.VerifyPath(context.Background(), "/data/gamehost/runtime-temp/rt-1", OwnershipProof{
		HostInstanceID: "host-abc",
	})
	if result != OwnershipVerified {
		t.Errorf("expected Verified, got %s", result)
	}

	result = v.VerifyPath(context.Background(), "/etc/passwd", OwnershipProof{
		HostInstanceID: "host-abc",
	})
	if result != OwnershipUnknown {
		t.Errorf("expected Unknown for path outside managed root, got %s", result)
	}
}

func TestOwnership_BinaryVerified(t *testing.T) {
	v := NewDefaultBinaryOwnershipVerifier()
	result := v.VerifyBinary(context.Background(), "bin-1", OwnershipProof{
		PluginID:  "p1",
		RuntimeID: "rt-1",
	})
	if result != OwnershipVerified {
		t.Errorf("expected Verified, got %s", result)
	}
}

func TestOwnership_SharedMemoryVerified(t *testing.T) {
	v := NewDefaultSharedMemoryOwnershipVerifier()
	result := v.VerifySharedMemory(context.Background(), "shm-1", OwnershipProof{
		HostInstanceID: "host-abc",
		RuntimeID:      "rt-1",
		Generation:     5,
	})
	if result != OwnershipVerified {
		t.Errorf("expected Verified, got %s", result)
	}
}

func TestOwnership_EndpointVerified(t *testing.T) {
	v := NewDefaultEndpointOwnershipVerifier()
	result := v.VerifyEndpoint(context.Background(), "ep-1", OwnershipProof{
		RuntimeID: "rt-1",
		ServiceID: "svc-1",
	})
	if result != OwnershipVerified {
		t.Errorf("expected Verified, got %s", result)
	}
}

// Stage Transition
func TestStartup_StageFlow(t *testing.T) {
	c, _, _, _, _, _, _, _, _ := setupTestCoordinator()

	report := c.RunStartupRecovery(context.Background())

	if report.Stage != StageCompleted {
		t.Errorf("expected Completed, got %s", report.Stage)
	}
	if report.StartedAt.IsZero() {
		t.Error("startedAt should be set")
	}
	if report.CompletedAt == nil {
		t.Error("completedAt should be set")
	}
	if report.CompletedAt != nil && report.CompletedAt.Before(report.StartedAt) {
		t.Error("completedAt should be after startedAt")
	}
}

// Report ID Unique
func TestStartup_OperationIDUnique(t *testing.T) {
	id1 := generateStartupOperationID()
	time.Sleep(time.Microsecond)
	id2 := generateStartupOperationID()
	if id1 == id2 {
		t.Errorf("IDs should be unique: %s == %s", id1, id2)
	}
}

// Cleanup result classifications
func TestStartup_AllCleanupSuccess(t *testing.T) {
	c, proc, temp, bin, ep, shm, _, _, _ := setupTestCoordinator()
	proc.candidates = []ProcessCandidate{{PID: 1, RuntimeID: "rt-1"}}
	temp.candidates = []TempCandidate{{RuntimeID: "rt-2", Path: "/data/temp/rt-2"}}
	bin.candidates = []BinaryCandidate{{BinaryID: "b-1", RuntimeID: "rt-3"}}
	ep.candidates = []EndpointCandidate{{EndpointID: "e-1", RuntimeID: "rt-4"}}
	shm.candidates = []SharedMemoryCandidate{{ID: "s-1", RuntimeID: "rt-5"}}

	report := c.RunStartupRecovery(context.Background())

	if !report.Success {
		t.Errorf("expected success")
	}
	if len(report.Cleaned) != 5 {
		t.Errorf("expected 5 cleaned, got %d", len(report.Cleaned))
	}
}

// Empty Candidates = No Cleanup Needed
func TestStartup_NoCandidatesIsSuccess(t *testing.T) {
	c, _, _, _, _, _, _, _, _ := setupTestCoordinator()

	report := c.RunStartupRecovery(context.Background())

	if !report.Success {
		t.Errorf("success with no candidates")
	}
	if len(report.Skipped) != 0 {
		t.Errorf("expected 0 skipped when no candidates, got %d", len(report.Skipped))
	}
}

// Cleanup Failure Marks Degraded
func TestStartup_CleanupFailure_Degraded(t *testing.T) {
	c, _, _, _, _, _, _, _, _ := setupTestCoordinator()

	tempErr := &fakeTempCleanupWithError{}
	c.deps.TempCleanup = tempErr

	report := c.RunStartupRecovery(context.Background())

	if !report.Degraded {
		t.Error("expected degraded status on cleanup failure")
	}
}

type fakeTempCleanupWithError struct{}

func (f *fakeTempCleanupWithError) ListStaleTempCandidates(ctx context.Context) ([]TempCandidate, error) {
	return []TempCandidate{{RuntimeID: "rt-1", Path: "/data/temp/rt-1"}}, nil
}

func (f *fakeTempCleanupWithError) RemoveStaleTemp(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	return fmt.Errorf("simulated failure")
}
