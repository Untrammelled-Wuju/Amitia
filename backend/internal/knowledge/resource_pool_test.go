package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestResourceGateHonorsCapacityAndCancellation(t *testing.T) {
	gate := newResourceGate(1)
	if gate == nil || gate.Capacity() != 1 {
		t.Fatalf("unexpected gate capacity: %#v", gate)
	}
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err := gate.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second acquire should time out, got %v", err)
	}
	release()
	secondRelease, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	secondRelease()
}
