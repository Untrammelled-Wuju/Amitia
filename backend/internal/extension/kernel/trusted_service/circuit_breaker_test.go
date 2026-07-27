package trusted_service

import (
	"testing"
	"time"
)

func TestCircuitBreaker_Initial(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitConfig())
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected initial state closed, got %s", cb.Status())
	}
	if !cb.AllowStart() {
		t.Fatal("expected AllowStart=true in closed state")
	}
}

func TestCircuitBreaker_OpenAfterThreshold(t *testing.T) {
	cfg := CircuitConfig{
		FailureThreshold:  3,
		RecoveryAfter:     100 * time.Millisecond,
		HalfOpenAttempts:  1,
		ResetAfterHealthy: 1 * time.Second,
	}
	cb := NewCircuitBreaker(cfg)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after %d failures, got %s", 3, cb.Status())
	}
	if cb.AllowStart() {
		t.Fatal("expected AllowStart=false in open state")
	}
}

func TestCircuitBreaker_HalfOpenAfterRecovery(t *testing.T) {
	cfg := CircuitConfig{
		FailureThreshold:  2,
		RecoveryAfter:     50 * time.Millisecond,
		HalfOpenAttempts:  1,
		ResetAfterHealthy: 1 * time.Second,
	}
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open, got %s", cb.Status())
	}

	time.Sleep(60 * time.Millisecond)

	if cb.Status() != CircuitHalfOpen {
		t.Fatalf("expected half_open after recovery, got %s", cb.Status())
	}
	if !cb.AllowStart() {
		t.Fatal("expected AllowStart=true in half_open state")
	}
}

func TestCircuitBreaker_CloseAfterHalfOpenSuccess(t *testing.T) {
	cfg := CircuitConfig{
		FailureThreshold:  2,
		RecoveryAfter:     50 * time.Millisecond,
		HalfOpenAttempts:  1,
		ResetAfterHealthy: 1 * time.Second,
	}
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(60 * time.Millisecond)

	if cb.Status() != CircuitHalfOpen {
		t.Fatalf("expected half_open, got %s", cb.Status())
	}

	cb.RecordSuccess()

	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after half_open success, got %s", cb.Status())
	}
}

func TestCircuitBreaker_ReopenOnHalfOpenFailure(t *testing.T) {
	cfg := CircuitConfig{
		FailureThreshold:  2,
		RecoveryAfter:     50 * time.Millisecond,
		HalfOpenAttempts:  1,
		ResetAfterHealthy: 1 * time.Second,
	}
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(60 * time.Millisecond)

	if cb.Status() != CircuitHalfOpen {
		t.Fatalf("expected half_open, got %s", cb.Status())
	}

	cb.RecordFailure()

	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after half_open failure, got %s", cb.Status())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitConfig())
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	cb.Reset()

	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after reset, got %s", cb.Status())
	}
	if cb.FailureCount() != 0 {
		t.Fatalf("expected 0 failures after reset, got %d", cb.FailureCount())
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cfg := CircuitConfig{
		FailureThreshold:  5,
		RecoveryAfter:     60 * time.Second,
		HalfOpenAttempts:  1,
		ResetAfterHealthy: 100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(150 * time.Millisecond)

	cb.RecordSuccess()

	if cb.FailureCount() != 0 {
		t.Fatalf("expected 0 failures after reset, got %d", cb.FailureCount())
	}
}
