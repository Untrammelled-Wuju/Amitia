package sandbox_webui

import (
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter()

	for i := 0; i < MaxBridgeCallsPerSec; i++ {
		if !rl.Allow("sess-1", MethodReady) {
			t.Errorf("call %d should be allowed", i)
		}
	}

	if rl.Allow("sess-1", MethodReady) {
		t.Error("call exceeding limit should be denied")
	}
}

func TestRateLimiterDifferentSessions(t *testing.T) {
	rl := NewRateLimiter()

	for i := 0; i < MaxBridgeCallsPerSec; i++ {
		if !rl.Allow("sess-1", MethodReady) {
			t.Errorf("call %d for sess-1 should be allowed", i)
		}
	}

	if !rl.Allow("sess-2", MethodReady) {
		t.Error("sess-2 should be allowed independently")
	}
}

func TestRateLimiterReset(t *testing.T) {
	rl := NewRateLimiter()

	for i := 0; i < MaxBridgeCallsPerSec; i++ {
		rl.Allow("sess-1", MethodReady)
	}

	rl.Reset("sess-1")

	if !rl.Allow("sess-1", MethodReady) {
		t.Error("after reset, call should be allowed")
	}
}

func TestMethodLimit(t *testing.T) {
	limit, window := methodLimit(MethodReady)
	if limit != MaxBridgeCallsPerSec {
		t.Errorf("MethodReady limit should be %d, got %d", MaxBridgeCallsPerSec, limit)
	}
	if window != time.Second {
		t.Errorf("MethodReady window should be 1s, got %v", window)
	}

	limit, window = methodLimit(MethodActionInvoke)
	if limit != MaxActionsPerMin {
		t.Errorf("MethodActionInvoke limit should be %d, got %d", MaxActionsPerMin, limit)
	}
	if window != time.Minute {
		t.Errorf("MethodActionInvoke window should be 1m, got %v", window)
	}
}
