package mindruntime

import (
	"testing"
	"time"
)

func TestNewBudgetTracker(t *testing.T) {
	bt := NewBudgetTracker()
	if bt == nil {
		t.Fatal("expected non-nil BudgetTracker")
	}
}

func TestRegisterBudget(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxModelCalls:  10,
		MaxInputTokens: 10000,
		MaxQueueMillis: 5000,
		MaxTotalMillis: 30000,
		Paths:          []string{"generation", "embedding"},
	}

	bt.RegisterBudget("generation", config)

	snap := bt.Snapshot("generation")
	if snap.Path != "generation" {
		t.Fatalf("expected path generation, got %s", snap.Path)
	}
	if snap.MaxCalls != 10 {
		t.Fatalf("expected max calls 10, got %d", snap.MaxCalls)
	}
}

func TestStartEndCall(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxModelCalls:  5,
		MaxInputTokens: 50000,
	}
	bt.RegisterBudget("generation", config)

	status := bt.StartCall("generation", 1000)
	if status != BudgetNormal {
		t.Fatalf("expected normal status, got %s", status)
	}

	snap := bt.Snapshot("generation")
	if snap.ActualCalls != 1 {
		t.Fatalf("expected 1 actual call, got %d", snap.ActualCalls)
	}
	if snap.InputTokens != 1000 {
		t.Fatalf("expected 1000 input tokens, got %d", snap.InputTokens)
	}

	bt.EndCall("generation", false)
}

func TestBudgetExhaustedMaxCalls(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxModelCalls: 2,
	}
	bt.RegisterBudget("generation", config)

	bt.StartCall("generation", 100)
	bt.StartCall("generation", 100)
	status := bt.StartCall("generation", 100)

	if status != BudgetExhausted {
		t.Fatalf("expected exhausted status, got %s", status)
	}
}

func TestBudgetExhaustedInputTokens(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxInputTokens: 500,
	}
	bt.RegisterBudget("generation", config)

	bt.StartCall("generation", 600)

	snap := bt.Snapshot("generation")
	if snap.Status != "" {
		t.Logf("snapshot before exhaustion check: %+v", snap)
	}

	exhausted, _ := bt.IsExhausted("generation")
	if !exhausted {
		t.Fatal("expected budget exhausted due to input tokens")
	}
}

func TestBudgetWarning(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxInputTokens: 1000,
	}
	bt.RegisterBudget("generation", config)

	status := bt.StartCall("generation", 800)
	if status != BudgetNormal {
		t.Fatalf("expected normal, got %s", status)
	}
}

func TestBudgetDeadlineExceeded(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxTotalMillis: 1,
	}
	bt.RegisterBudget("generation", config)

	bt.StartCall("generation", 100)
	time.Sleep(5 * time.Millisecond)

	exhausted, reason := bt.IsExhausted("generation")
	if !exhausted {
		t.Fatal("expected deadline exceeded")
	}
	_ = reason
}

func TestBudgetQueueTimeExceeded(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxQueueMillis: 1,
	}
	bt.RegisterBudget("generation", config)

	bt.StartCall("generation", 100)
	time.Sleep(5 * time.Millisecond)

	exhausted, reason := bt.IsExhausted("generation")
	if !exhausted {
		t.Fatal("expected queue time exceeded")
	}
	_ = reason
}

func TestBudgetCacheHit(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxModelCalls: 5,
	}
	bt.RegisterBudget("generation", config)

	bt.StartCall("generation", 100)
	bt.EndCall("generation", true)

	snap := bt.Snapshot("generation")
	if snap.CacheHits != 1 {
		t.Fatalf("expected 1 cache hit, got %d", snap.CacheHits)
	}
}

func TestBudgetCancellation(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxModelCalls: 5,
	}
	bt.RegisterBudget("generation", config)

	bt.StartCall("generation", 100)
	bt.RecordCancellation("generation", "SUPERSEDED", "generation")

	snapshots := bt.AllSnapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if snapshots[0].CancelReason != "SUPERSEDED" {
		t.Fatalf("expected SUPERSEDED reason, got %s", snapshots[0].CancelReason)
	}
	if snapshots[0].TimeoutStage != "generation" {
		t.Fatalf("expected generation stage, got %s", snapshots[0].TimeoutStage)
	}
}

func TestBudgetReset(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxModelCalls: 5,
	}
	bt.RegisterBudget("generation", config)

	bt.StartCall("generation", 100)
	bt.StartCall("generation", 200)
	bt.Reset("generation")

	snap := bt.Snapshot("generation")
	if snap.ActualCalls != 0 {
		t.Fatalf("expected 0 actual calls after reset, got %d", snap.ActualCalls)
	}
	if snap.InputTokens != 0 {
		t.Fatalf("expected 0 input tokens after reset, got %d", snap.InputTokens)
	}

	snapshots := bt.AllSnapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot recorded, got %d", len(snapshots))
	}
}

func TestBudgetAllowedForReply(t *testing.T) {
	bt := NewBudgetTracker()

	if !bt.AllowedForReply("reply") {
		t.Fatal("expected reply allowed")
	}
	if !bt.AllowedForReply("safety_check") {
		t.Fatal("expected safety_check allowed")
	}
	if !bt.AllowedForReply("sqlite_commit") {
		t.Fatal("expected sqlite_commit allowed")
	}
}

func TestBudgetDegradeReason(t *testing.T) {
	bt := NewBudgetTracker()

	config := BudgetConfig{
		MaxModelCalls: 3,
	}
	bt.RegisterBudget("generation", config)

	bt.StartCall("generation", 100)
	bt.StartCall("generation", 100)
	bt.StartCall("generation", 100)

	reason := bt.DegradeReason("generation")
	if reason == "" {
		t.Fatal("expected non-empty degrade reason")
	}
}

func TestBudgetUnknownPath(t *testing.T) {
	bt := NewBudgetTracker()

	snap := bt.Snapshot("unknown")
	if snap.Path != "unknown" {
		t.Fatalf("expected path unknown, got %s", snap.Path)
	}

	status := bt.StartCall("unknown", 100)
	if status != BudgetNormal {
		t.Fatalf("expected normal for unknown path, got %s", status)
	}

	exhausted, _ := bt.IsExhausted("unknown")
	if exhausted {
		t.Fatal("expected not exhausted for unknown path")
	}
}
