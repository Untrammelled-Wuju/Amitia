package sandbox_webui

import (
	"strings"
	"testing"
	"time"
)

func TestCircuitBreakerClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute, time.Minute)

	if !cb.Allow() {
		t.Error("circuit should be closed and allow traffic")
	}

	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Error("state should remain closed after success")
	}
}

func TestCircuitBreakerOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute, time.Minute)

	cb.RecordFailure("crash1")
	cb.RecordFailure("crash2")

	if cb.State() != CircuitClosed {
		t.Error("should still be closed with 2 failures")
	}

	cb.RecordFailure("crash3")

	if cb.State() != CircuitOpen {
		t.Error("should be open with 3 failures")
	}

	if cb.Allow() {
		t.Error("should deny when circuit is open")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Minute, 50*time.Millisecond)

	cb.RecordFailure("crash")

	if cb.State() != CircuitOpen {
		t.Error("should be open")
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.Allow() {
		t.Error("should allow in half-open state after cooldown")
	}

	if cb.State() != CircuitHalfOpen {
		t.Error("should be half-open")
	}

	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Error("should be closed after success in half-open")
	}
}

func TestPerformanceMonitor(t *testing.T) {
	pm := NewPerformanceMonitor()

	pm.Record("sess-1", PerformanceSnapshot{
		Timestamp:   time.Now().UTC(),
		CPUPercent:  50,
		MemoryBytes: 100 * 1024 * 1024,
		DOMNodes:    500,
		FrameRate:   60,
	})

	violations := pm.CheckBudget("sess-1", DefaultPerformanceBudget())
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %v", violations)
	}

	pm.Record("sess-1", PerformanceSnapshot{
		Timestamp:   time.Now().UTC(),
		CPUPercent:  90,
		MemoryBytes: 500 * 1024 * 1024,
		DOMNodes:    6000,
		FrameRate:   15,
	})

	violations = pm.CheckBudget("sess-1", DefaultPerformanceBudget())
	if len(violations) == 0 {
		t.Error("expected violations for exceeded budget")
	}
}

func TestPreloadBuilder(t *testing.T) {
	session := &WebSession{
		SessionID:      "test-session",
		ContributionID: "contrib-001",
		Origin:         "amitia-extension://ext-1/mod-1",
		Nonce:          "test-nonce-123",
		Token:          "test-token-456",
		Generation:     1,
	}

	pb := NewPreloadBuilder()
	script, err := pb.Build(session)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if script == "" {
		t.Error("script should not be empty")
	}

	if !strings.Contains(script, "test-session") {
		t.Error("script should contain session ID")
	}

	if !strings.Contains(script, "test-nonce-123") {
		t.Error("script should contain nonce")
	}

	if !strings.Contains(script, "test-token-456") {
		t.Error("script should contain token")
	}

	if !strings.Contains(script, "contrib-001") {
		t.Error("script should contain contributionId")
	}

	if !strings.Contains(script, "amitia-extension://ext-1/mod-1") {
		t.Error("script should contain origin")
	}

	if !strings.Contains(script, ProtocolVersion) {
		t.Error("script should contain protocol version")
	}

	if !strings.Contains(script, `protocolVersion:protocolVersion,session:sessionId,nonce:nonce,generation:generation,contributionId:contributionId`) {
		t.Error("Ready message must include all required fields: protocolVersion, session, nonce, generation, contributionId")
	}
}
