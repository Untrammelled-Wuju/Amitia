package handshake_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/handshake"
)

func TestReadyGate_PreReady(t *testing.T) {
	gate := handshake.NewReadyGate([]string{handshake.HelloMethod, "control.request.cancel"})

	if !gate.IsAllowedPreReady(handshake.HelloMethod) {
		t.Error("hello method should be allowed pre-ready")
	}

	if !gate.IsAllowedPreReady("control.request.cancel") {
		t.Error("cancel method should be allowed pre-ready")
	}

	if gate.IsAllowedPreReady("example.game.move") {
		t.Error("custom rpc should not be allowed pre-ready")
	}
}

func TestReadyGate_AfterReady(t *testing.T) {
	gate := handshake.NewReadyGate([]string{handshake.HelloMethod})

	gate.MarkReady("conn-1")

	if !gate.IsReady("conn-1") {
		t.Error("conn-1 should be ready")
	}

	if !gate.CanProcess("conn-1", "example.game.move") {
		t.Error("ready connection can process any method")
	}
}

func TestReadyGate_CanProcess(t *testing.T) {
	gate := handshake.NewReadyGate([]string{handshake.HelloMethod})

	if gate.CanProcess("conn-1", "example.game.move") {
		t.Error("pre-ready connection cannot process custom rpc")
	}

	if !gate.CanProcess("conn-1", handshake.HelloMethod) {
		t.Error("pre-ready connection can send hello")
	}
}

func TestReadyGate_Remove(t *testing.T) {
	gate := handshake.NewReadyGate(nil)
	gate.MarkReady("conn-1")
	gate.MarkReady("conn-2")

	gate.Remove("conn-1")

	if gate.IsReady("conn-1") {
		t.Error("conn-1 should not be ready after remove")
	}
	if !gate.IsReady("conn-2") {
		t.Error("conn-2 should still be ready")
	}
}

func TestReadyGate_MarkNotReady(t *testing.T) {
	gate := handshake.NewReadyGate(nil)
	gate.MarkReady("conn-1")

	gate.MarkNotReady("conn-1")

	if gate.IsReady("conn-1") {
		t.Error("conn-1 should not be ready")
	}
}

func TestReadyGate_ReadyCount(t *testing.T) {
	gate := handshake.NewReadyGate(nil)

	if gate.ReadyCount() != 0 {
		t.Errorf("initial ready count should be 0, got %d", gate.ReadyCount())
	}

	gate.MarkReady("conn-1")
	gate.MarkReady("conn-2")

	if gate.ReadyCount() != 2 {
		t.Errorf("ready count should be 2, got %d", gate.ReadyCount())
	}
}

func TestReadyGate_AllowPreReady(t *testing.T) {
	gate := handshake.NewReadyGate(nil)

	gate.AllowPreReady("control.request.cancel")

	if !gate.IsAllowedPreReady("control.request.cancel") {
		t.Error("dynamically allowed method should work")
	}
}

func TestReadyGateWaitReadyUnblocksAfterMarkReady(t *testing.T) {
	gate := handshake.NewReadyGate(nil)
	gate.Register("conn-wait")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- gate.WaitReady(ctx, "conn-wait") }()
	time.Sleep(10 * time.Millisecond)
	gate.MarkReady("conn-wait")
	if err := <-done; err != nil {
		t.Fatalf("WaitReady() error = %v", err)
	}
}

func TestReadyGateWaitReadyFailsWhenConnectionRemoved(t *testing.T) {
	gate := handshake.NewReadyGate(nil)
	gate.Register("conn-removed")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- gate.WaitReady(ctx, "conn-removed") }()
	time.Sleep(10 * time.Millisecond)
	gate.Remove("conn-removed")
	if err := <-done; !errors.Is(err, handshake.ErrReadyGateRemoved) {
		t.Fatalf("WaitReady() error = %v, want ErrReadyGateRemoved", err)
	}
}

func TestReadyGateWaitReadyHonorsContext(t *testing.T) {
	gate := handshake.NewReadyGate(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := gate.WaitReady(ctx, "conn-timeout"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitReady() error = %v, want context deadline", err)
	}
}
