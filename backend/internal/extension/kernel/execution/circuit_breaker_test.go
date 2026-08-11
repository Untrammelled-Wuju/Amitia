package execution

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type fakeCircuitClock struct {
	now time.Time
}

func (c *fakeCircuitClock) Now() time.Time { return c.now }

func (c *fakeCircuitClock) Add(d time.Duration) { c.now = c.now.Add(d) }

func newCircuitTool(id, runtimeID string) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:            id,
		HasSideEffects: false,
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeMCP,
			RuntimeID:   runtimeID,
		},
	}
}

func TestCircuitCoordinator_AcquireAllowsClosed(t *testing.T) {
	coord := NewCircuitBreakerCoordinatorWithClock(DefaultCircuitBreakerConfig(), &fakeCircuitClock{now: time.Now()})
	tool := newCircuitTool("t1", "mcp-1")

	permit := coord.Acquire(context.Background(), tool)
	if !permit.Allowed {
		t.Fatal("expected first acquire to be allowed")
	}
	if permit.Probe {
		t.Fatal("closed state should not be a probe")
	}
}

func TestCircuitCoordinator_ThresholdOpens(t *testing.T) {
	clock := &fakeCircuitClock{now: time.Unix(1_000_000, 0)}
	config := CircuitBreakerConfig{
		FailureThreshold:        3,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}
	coord := NewCircuitBreakerCoordinatorWithClock(config, clock)
	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	for i := 0; i < 3; i++ {
		permit := coord.Acquire(context.Background(), tool)
		if !permit.Allowed {
			t.Fatalf("iteration %d: expected allowed", i)
		}
		outcome := classifier.Classify(failResult, true)
		coord.Complete(permit, outcome)
		clock.Add(1 * time.Second)
	}

	permit := coord.Acquire(context.Background(), tool)
	if permit.Allowed {
		t.Fatal("expected OPEN to reject acquire")
	}
}

func TestCircuitCoordinator_HalfOpenTransition(t *testing.T) {
	clock := &fakeCircuitClock{now: time.Unix(1_000_000, 0)}
	config := CircuitBreakerConfig{
		FailureThreshold:        2,
		OpenTimeout:             5 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}
	coord := NewCircuitBreakerCoordinatorWithClock(config, clock)
	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	for i := 0; i < 2; i++ {
		permit := coord.Acquire(context.Background(), tool)
		coord.Complete(permit, classifier.Classify(failResult, true))
		clock.Add(1 * time.Second)
	}

	snap := coord.Snapshot(tool)
	if snap.State != CircuitStateOpen {
		t.Fatalf("expected OPEN, got %s", snap.State)
	}

	// Advance past OpenTimeout
	clock.Add(6 * time.Second)
	permit := coord.Acquire(context.Background(), tool)
	if !permit.Allowed {
		t.Fatal("expected HALF_OPEN to allow single probe")
	}
	if !permit.Probe {
		t.Fatal("expected permit.Probe=true in half-open")
	}

	// Concurrent second call should be rejected
	permit2 := coord.Acquire(context.Background(), tool)
	if permit2.Allowed {
		t.Fatal("expected HALF_OPEN with maxInflight=1 to reject 2nd concurrent call")
	}
}

func TestCircuitCoordinator_HalfOpenSuccessRecovers(t *testing.T) {
	clock := &fakeCircuitClock{now: time.Unix(1_000_000, 0)}
	config := CircuitBreakerConfig{
		FailureThreshold:        1,
		OpenTimeout:             1 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}
	coord := NewCircuitBreakerCoordinatorWithClock(config, clock)
	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}
	successResult := capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess}

	permit := coord.Acquire(context.Background(), tool)
	coord.Complete(permit, classifier.Classify(failResult, true))

	clock.Add(2 * time.Second)

	probePermit := coord.Acquire(context.Background(), tool)
	if !probePermit.Allowed || !probePermit.Probe {
		t.Fatal("expected half-open probe permit")
	}
	coord.Complete(probePermit, classifier.Classify(successResult, true))

	snap := coord.Snapshot(tool)
	if snap.State != CircuitStateClosed {
		t.Fatalf("expected recovery to CLOSED, got %s", snap.State)
	}
}

func TestCircuitCoordinator_HalfOpenFailureReopens(t *testing.T) {
	clock := &fakeCircuitClock{now: time.Unix(1_000_000, 0)}
	config := CircuitBreakerConfig{
		FailureThreshold:        1,
		OpenTimeout:             1 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}
	coord := NewCircuitBreakerCoordinatorWithClock(config, clock)
	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	permit := coord.Acquire(context.Background(), tool)
	coord.Complete(permit, classifier.Classify(failResult, true))
	clock.Add(2 * time.Second)

	probePermit := coord.Acquire(context.Background(), tool)
	coord.Complete(probePermit, classifier.Classify(failResult, true))

	snap := coord.Snapshot(tool)
	if snap.State != CircuitStateOpen {
		t.Fatalf("expected probe failure to re-OPEN, got %s", snap.State)
	}
}

func TestCircuitCoordinator_HalfOpenNeutralReleasesSlot(t *testing.T) {
	clock := &fakeCircuitClock{now: time.Unix(1_000_000, 0)}
	config := CircuitBreakerConfig{
		FailureThreshold:        1,
		OpenTimeout:             1 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}
	coord := NewCircuitBreakerCoordinatorWithClock(config, clock)
	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	permit := coord.Acquire(context.Background(), tool)
	coord.Complete(permit, classifier.Classify(failResult, true))
	clock.Add(2 * time.Second)

	probePermit := coord.Acquire(context.Background(), tool)
	if !probePermit.Allowed {
		t.Fatal("expected probe allowed")
	}

	// Caller exits before Runtime due to permission revocation → Neutral
	coord.Complete(probePermit, CircuitOutcomeNeutral)

	// State should remain half_open (still waiting) and slot should be released
	snap := coord.Snapshot(tool)
	if snap.HalfOpenInFlight != 0 {
		t.Fatalf("expected half-open slot released, got inflight=%d", snap.HalfOpenInFlight)
	}
	if snap.State != CircuitStateHalfOpen {
		t.Fatalf("expected half_open after neutral, got %s", snap.State)
	}

	// Next call can probe again
	permit2 := coord.Acquire(context.Background(), tool)
	if !permit2.Allowed {
		t.Fatal("expected half-open slot to be reusable after neutral")
	}
	coord.Complete(permit2, classifier.Classify(capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess}, true))
}

func TestCircuitCoordinator_SuccessResetsFailures(t *testing.T) {
	clock := &fakeCircuitClock{now: time.Unix(1_000_000, 0)}
	config := CircuitBreakerConfig{
		FailureThreshold:        3,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}
	coord := NewCircuitBreakerCoordinatorWithClock(config, clock)
	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}
	successResult := capability.UnifiedToolResult{Status: capability.ToolResultStatusSuccess}

	for i := 0; i < 2; i++ {
		permit := coord.Acquire(context.Background(), tool)
		coord.Complete(permit, classifier.Classify(failResult, true))
	}

	permit := coord.Acquire(context.Background(), tool)
	coord.Complete(permit, classifier.Classify(successResult, true))

	permit = coord.Acquire(context.Background(), tool)
	coord.Complete(permit, classifier.Classify(failResult, true))
	coord.Complete(permit, classifier.Classify(failResult, true))

	snap := coord.Snapshot(tool)
	if snap.State != CircuitStateClosed {
		t.Fatalf("expected consecutive failures reset, got state=%s", snap.State)
	}
	if snap.ConsecutiveFailures != 2 {
		t.Fatalf("expected 2 consecutive failures after reset, got %d", snap.ConsecutiveFailures)
	}
}

func TestCircuitCoordinator_SameRuntimeSharedCircuit(t *testing.T) {
	coord := NewCircuitBreakerCoordinatorWithClock(CircuitBreakerConfig{
		FailureThreshold:        2,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}, &fakeCircuitClock{now: time.Unix(1_000_000, 0)})

	toolA := newCircuitTool("tool-a", "mcp-shared")
	toolB := newCircuitTool("tool-b", "mcp-shared")

	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	p1 := coord.Acquire(context.Background(), toolA)
	coord.Complete(p1, classifier.Classify(failResult, true))
	p2 := coord.Acquire(context.Background(), toolA)
	coord.Complete(p2, classifier.Classify(failResult, true))

	p3 := coord.Acquire(context.Background(), toolB)
	if p3.Allowed {
		t.Fatal("expected toolB with same mcp-shared runtime to be blocked because circuit is open")
	}
}

func TestCircuitCoordinator_DifferentRuntimeIsolated(t *testing.T) {
	coord := NewCircuitBreakerCoordinatorWithClock(CircuitBreakerConfig{
		FailureThreshold:        1,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}, &fakeCircuitClock{now: time.Unix(1_000_000, 0)})

	toolA := newCircuitTool("tool-a", "mcp-1")
	toolB := newCircuitTool("tool-b", "mcp-2")

	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	p1 := coord.Acquire(context.Background(), toolA)
	coord.Complete(p1, classifier.Classify(failResult, true))

	p2 := coord.Acquire(context.Background(), toolB)
	if !p2.Allowed {
		t.Fatal("expected different runtime to be isolated and allowed")
	}
}

func TestCircuitCoordinator_EmptyRuntimeIDFallback(t *testing.T) {
	coord := NewCircuitBreakerCoordinatorWithClock(CircuitBreakerConfig{
		FailureThreshold:        1,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}, &fakeCircuitClock{now: time.Unix(1_000_000, 0)})

	toolA := newCircuitTool("tool-a", "")
	toolB := newCircuitTool("tool-b", "")

	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	p1 := coord.Acquire(context.Background(), toolA)
	coord.Complete(p1, classifier.Classify(failResult, true))

	p2 := coord.Acquire(context.Background(), toolB)
	if !p2.Allowed {
		t.Fatal("empty runtimeID fallback to tool:<id> should keep them isolated")
	}
}

func TestCircuitCoordinator_Reset(t *testing.T) {
	coord := NewCircuitBreakerCoordinatorWithClock(CircuitBreakerConfig{
		FailureThreshold:        1,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}, &fakeCircuitClock{now: time.Unix(1_000_000, 0)})

	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	p1 := coord.Acquire(context.Background(), tool)
	coord.Complete(p1, classifier.Classify(failResult, true))

	snap := coord.Snapshot(tool)
	if snap.State != CircuitStateOpen {
		t.Fatal("expected OPEN")
	}

	coord.ResetTool(tool)

	snap = coord.Snapshot(tool)
	if snap.State != CircuitStateClosed {
		t.Fatalf("expected CLOSED after reset, got %s", snap.State)
	}

	permit := coord.Acquire(context.Background(), tool)
	if !permit.Allowed {
		t.Fatal("expected allowed after reset")
	}
}

func TestCircuitCoordinator_RefusedPermitDoesNotRefresh(t *testing.T) {
	clock := &fakeCircuitClock{now: time.Unix(1_000_000, 0)}
	config := CircuitBreakerConfig{
		FailureThreshold:        1,
		OpenTimeout:             30 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}
	coord := NewCircuitBreakerCoordinatorWithClock(config, clock)
	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	p1 := coord.Acquire(context.Background(), tool)
	coord.Complete(p1, classifier.Classify(failResult, true))

	// Advance to near timeout but not over
	clock.Add(29 * time.Second)

	// Many refused calls
	for i := 0; i < 100; i++ {
		p := coord.Acquire(context.Background(), tool)
		if p.Allowed {
			t.Fatal("OPEN should refuse")
		}
	}

	clock.Add(2 * time.Second)

	// Now a single probe should succeed
	probePermit := coord.Acquire(context.Background(), tool)
	if !probePermit.Allowed {
		t.Fatal("expected half-open probe after timeout")
	}
}

func TestCircuitCoordinator_EnumValues(t *testing.T) {
	if string(CircuitStateClosed) != "closed" ||
		string(CircuitStateOpen) != "open" ||
		string(CircuitStateHalfOpen) != "half_open" {
		t.Fatal("circuit state enum values changed")
	}
	if string(CircuitOutcomeSuccess) != "success" ||
		string(CircuitOutcomeFailure) != "failure" ||
		string(CircuitOutcomeNeutral) != "neutral" {
		t.Fatal("circuit outcome enum values changed")
	}
}

type circuitEventCapture struct {
	events []string
}

func (c *circuitEventCapture) Record(snap CircuitSnapshot, from, to CircuitState, reason string) {
	if from != to {
		c.events = append(c.events, string(from)+"->"+string(to)+":"+reason)
	}
}

func TestCircuitCoordinator_EventsEmitted(t *testing.T) {
	capture := &circuitEventCapture{}
	coord := NewCircuitBreakerCoordinatorWithClock(CircuitBreakerConfig{
		FailureThreshold:        1,
		OpenTimeout:             1 * time.Second,
		HalfOpenMaxInflight:     1,
		HalfOpenSuccessThreshold: 1,
	}, &fakeCircuitClock{now: time.Unix(1_000_000, 0)})
	coord.SetEventHook(capture.Record)

	tool := newCircuitTool("t1", "mcp-1")
	classifier := NewCircuitResultClassifier()
	failResult := capability.UnifiedToolResult{
		Status: capability.ToolResultStatusFailed,
		Error:  &capability.ToolError{Code: capability.ErrorCodeConnectionLost, Retryable: true},
	}

	p := coord.Acquire(context.Background(), tool)
	coord.Complete(p, classifier.Classify(failResult, true))

	if len(capture.events) == 0 {
		t.Fatal("expected at least one state transition event")
	}
	if capture.events[0] != "closed->open:threshold_reached" {
		t.Fatalf("unexpected event: %s", capture.events[0])
	}
}
