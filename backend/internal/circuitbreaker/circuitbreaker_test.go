package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestNewCircuitBreakerStartsClosed(t *testing.T) {
	cb := New("test-cb", 3, 1*time.Second, 2)
	if cb.State() != StateClosed {
		t.Errorf("expected %s, got %s", StateClosed, cb.State())
	}
}

func TestCircuitBreakerOpensAfterMaxFailures(t *testing.T) {
	cb := New("test-cb", 2, 100*time.Millisecond, 2)
	if !cb.Allow() {
		t.Error("expected Allow true in closed state")
	}
	cb.RecordFailure()
	if !cb.Allow() {
		t.Error("expected Allow true after 1 failure")
	}
	cb.RecordFailure()
	if cb.Allow() {
		t.Error("expected Allow false after max failures")
	}
	if cb.State() != StateOpen {
		t.Errorf("expected %s, got %s", StateOpen, cb.State())
	}
}

func TestCircuitBreakerHalfOpenAfterTimeout(t *testing.T) {
	cb := New("test-cb", 1, 10*time.Millisecond, 2)
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected %s, got %s", StateOpen, cb.State())
	}
	time.Sleep(15 * time.Millisecond)
	if cb.State() != StateHalfOpen {
		t.Errorf("expected %s after timeout, got %s", StateHalfOpen, cb.State())
	}
	if !cb.Allow() {
		t.Error("expected Allow true in half_open")
	}
}

func TestCircuitBreakerClosesAfterHalfOpenSuccesses(t *testing.T) {
	cb := New("test-cb", 1, 10*time.Millisecond, 2)
	cb.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("expected %s after successes, got %s", StateClosed, cb.State())
	}
}

func TestCircuitBreakerOpensFromHalfOpenOnFailure(t *testing.T) {
	cb := New("test-cb", 1, 10*time.Millisecond, 2)
	cb.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected %s, got %s", StateHalfOpen, cb.State())
	}
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("expected %s after half_open failure, got %s", StateOpen, cb.State())
	}
}

func TestCircuitBreakerSuccessResetsFailureCount(t *testing.T) {
	cb := New("test-cb", 3, 100*time.Millisecond, 2)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	if !cb.Allow() {
		t.Error("expected Allow true after success resets failure count")
	}
}

func TestDegradationMatrixRegisterAndFallback(t *testing.T) {
	dm := NewDegradationMatrix()
	called := false
	dm.Register("svc1", func() error {
		called = true
		return nil
	})
	fn := dm.Fallback("svc1")
	if fn == nil {
		t.Fatal("expected fallback function")
	}
	fn()
	if !called {
		t.Error("expected fallback to be called")
	}
}

func TestDegradationMatrixUnknownService(t *testing.T) {
	dm := NewDegradationMatrix()
	fn := dm.Fallback("nonexistent")
	if fn != nil {
		t.Error("expected nil for unknown service")
	}
}

func TestDegradationMatrixFallbackReturnsError(t *testing.T) {
	dm := NewDegradationMatrix()
	dm.Register("svc1", func() error {
		return errors.New("degraded")
	})
	fn := dm.Fallback("svc1")
	err := fn()
	if err == nil {
		t.Error("expected error from fallback")
	}
}
