package mindruntime

import (
	"testing"
	"time"
)

func TestCircuitBreakerAllowClosed(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultCircuitBreakerConfig())
	if !cb.Allow() {
		t.Fatal("closed circuit should allow")
	}
}

func TestCircuitBreakerAllowDisabled(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.Enabled = false
	cb := NewCircuitBreaker("test", cfg)
	cb.State = CircuitOpen
	if !cb.Allow() {
		t.Fatal("disabled circuit should always allow")
	}
}

func TestCircuitBreakerTripAndReset(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 3
	cb := NewCircuitBreaker("test", cfg)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open after 3 failures, got %s", cb.Status())
	}
	if cb.Allow() {
		t.Fatal("open circuit should not allow before timeout")
	}
	if cb.TotalFail != 3 {
		t.Fatalf("expected 3 total failures, got %d", cb.TotalFail)
	}
}

func TestCircuitBreakerHalfOpenTransition(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.SuccessThreshold = 2
	cfg.OpenTimeout = 10 * time.Millisecond

	cb := NewCircuitBreaker("test", cfg)
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.Status() != CircuitOpen {
		t.Fatalf("expected open, got %s", cb.Status())
	}

	time.Sleep(20 * time.Millisecond)

	if cb.Status() != CircuitHalfOpen {
		t.Fatalf("expected half_open after timeout, got %s", cb.Status())
	}

	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after 2 successes, got %s", cb.Status())
	}
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.OpenTimeout = 10 * time.Millisecond

	cb := NewCircuitBreaker("test", cfg)
	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(20 * time.Millisecond)

	cb.RecordFailure()
	if cb.Status() != CircuitOpen {
		t.Fatalf("half-open failure should reopen circuit, got %s", cb.Status())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultCircuitBreakerConfig())
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.Status() != CircuitOpen {
		t.Fatal("expected open")
	}

	cb.Reset()
	if cb.Status() != CircuitClosed {
		t.Fatalf("expected closed after reset, got %s", cb.Status())
	}
	if cb.Failures != 0 {
		t.Fatalf("expected 0 failures, got %d", cb.Failures)
	}
}

func TestCircuitBreakerRegistryRegisterAndGet(t *testing.T) {
	reg := NewCircuitBreakerRegistry()
	reg.Register("db", DefaultCircuitBreakerConfig())

	cb := reg.Get("db")
	if cb == nil {
		t.Fatal("expected non-nil circuit breaker")
	}
	if cb.Name != "db" {
		t.Fatalf("expected name db, got %s", cb.Name)
	}

	reg.Register("db", DefaultCircuitBreakerConfig())
}

func TestCircuitBreakerRegistryAllowed(t *testing.T) {
	reg := NewCircuitBreakerRegistry()

	if !reg.Allowed("nonexistent") {
		t.Fatal("nonexistent breaker should allow")
	}

	reg.Register("db", DefaultCircuitBreakerConfig())
	if !reg.Allowed("db") {
		t.Fatal("new breaker should allow")
	}
}

func TestCircuitBreakerRegistryRecord(t *testing.T) {
	reg := NewCircuitBreakerRegistry()
	reg.Register("db", DefaultCircuitBreakerConfig())

	reg.RecordFailure("db")
	reg.RecordSuccess("db")

	cb := reg.Get("db")
	if cb.TotalCalls != 2 {
		t.Fatalf("expected 2 calls, got %d", cb.TotalCalls)
	}

	reg.RecordFailure("nonexistent")
	reg.RecordSuccess("nonexistent")
}

func TestCircuitBreakerRegistryHealthReport(t *testing.T) {
	reg := NewCircuitBreakerRegistry()

	report := reg.HealthReport("nonexistent")
	if !report.Healthy {
		t.Fatal("nonexistent breaker should report healthy")
	}

	reg.Register("db", DefaultCircuitBreakerConfig())
	report = reg.HealthReport("db")
	if !report.Healthy {
		t.Fatal("new breaker should report healthy")
	}
}

func TestCircuitBreakerRegistryAllHealthReports(t *testing.T) {
	reg := NewCircuitBreakerRegistry()
	reg.Register("db", DefaultCircuitBreakerConfig())
	reg.Register("redis", DefaultCircuitBreakerConfig())
	reg.Register("llm", DefaultCircuitBreakerConfig())

	reports := reg.AllHealthReports()
	if len(reports) != 3 {
		t.Fatalf("expected 3 reports, got %d", len(reports))
	}
}

func TestCircuitBreakerHalfOpenCount(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cfg.FailureThreshold = 2
	cfg.OpenTimeout = 10 * time.Millisecond
	cfg.HalfOpenMaxRequest = 2

	cb := NewCircuitBreaker("test", cfg)
	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(20 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("first half-open request should be allowed")
	}
	cb.RecordSuccess()

	if !cb.Allow() {
		t.Fatal("second half-open request should be allowed")
	}
	cb.RecordSuccess()
}
