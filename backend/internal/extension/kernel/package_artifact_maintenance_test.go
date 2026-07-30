package kernel

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type packageArtifactMaintenanceFake struct {
	mu             sync.Mutex
	events         []string
	expireFailures int
	expireCalls    int
	gcCalls        int
}

func (f *packageArtifactMaintenanceFake) ExpirePackagePreviews(context.Context, time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, "expire")
	f.expireCalls++
	if f.expireFailures > 0 {
		f.expireFailures--
		return 0, errors.New("expire failed")
	}
	return 1, nil
}

func (f *packageArtifactMaintenanceFake) ReleaseExpiredArtifactReferences(context.Context, time.Time) (int64, error) {
	f.mu.Lock()
	f.events = append(f.events, "release")
	f.mu.Unlock()
	return 1, nil
}

func (f *packageArtifactMaintenanceFake) VerifyDueArtifacts(context.Context, time.Time, int) (PackageArtifactVerificationResult, error) {
	f.mu.Lock()
	f.events = append(f.events, "verify")
	f.mu.Unlock()
	return PackageArtifactVerificationResult{}, nil
}

func (f *packageArtifactMaintenanceFake) CollectGarbage(context.Context, time.Time, time.Duration, int) (PackageArtifactGCResult, error) {
	f.mu.Lock()
	f.events = append(f.events, "gc")
	f.gcCalls++
	f.mu.Unlock()
	return PackageArtifactGCResult{}, nil
}

func (f *packageArtifactMaintenanceFake) snapshot() ([]string, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.events...), f.expireCalls, f.gcCalls
}

func TestPackageArtifactMaintenanceStartsAfterRecoveryAndStops(t *testing.T) {
	fake := &packageArtifactMaintenanceFake{events: []string{"recovery"}}
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	maintenance, err := NewPackageArtifactMaintenance(fake, PackageArtifactMaintenanceConfig{
		Interval: time.Hour, VerificationAge: time.Hour, InitialVerificationAge: time.Hour,
		Retention: time.Hour, BatchSize: 10, Now: func() time.Time { return now }, OnError: func(error) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := maintenance.Start(ctx); err != nil {
		t.Fatal(err)
	}
	events, _, gcCalls := fake.snapshot()
	expected := []string{"recovery", "expire", "release", "verify"}
	if len(events) != len(expected) {
		t.Fatalf("unexpected startup sequence: %#v", events)
	}
	for index := range expected {
		if events[index] != expected[index] {
			t.Fatalf("unexpected startup sequence: %#v", events)
		}
	}
	if gcCalls != 0 {
		t.Fatal("initial maintenance deleted recovery evidence")
	}
	cancel()
	maintenance.Stop()
	if maintenance.Status().Running {
		t.Fatal("maintenance remained running after cancellation")
	}
	maintenance.Stop()
}

func TestPackageArtifactMaintenanceTickOrder(t *testing.T) {
	fake := &packageArtifactMaintenanceFake{}
	maintenance, err := NewPackageArtifactMaintenance(fake, PackageArtifactMaintenanceConfig{
		Interval: time.Hour, VerificationAge: time.Hour, InitialVerificationAge: time.Hour,
		Retention: time.Hour, BatchSize: 10, Now: time.Now, OnError: func(error) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := maintenance.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	events, _, _ := fake.snapshot()
	expected := []string{"expire", "release", "verify", "gc"}
	for index := range expected {
		if len(events) <= index || events[index] != expected[index] {
			t.Fatalf("unexpected tick sequence: %#v", events)
		}
	}
	status := maintenance.Status()
	if status.RunCount != 1 || status.FailureCount != 0 || status.LastEndedAt.IsZero() {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestPackageArtifactMaintenanceErrorDoesNotStopLoop(t *testing.T) {
	fake := &packageArtifactMaintenanceFake{expireFailures: 2}
	errorsSeen := make(chan error, 10)
	maintenance, err := NewPackageArtifactMaintenance(fake, PackageArtifactMaintenanceConfig{
		Interval: 5 * time.Millisecond, VerificationAge: time.Hour, InitialVerificationAge: time.Hour,
		Retention: time.Hour, BatchSize: 10, Now: time.Now, OnError: func(err error) { errorsSeen <- err },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := maintenance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		_, expireCalls, gcCalls := fake.snapshot()
		if expireCalls >= 3 && gcCalls >= 1 {
			break
		}
		if time.Now().After(deadline) {
			maintenance.Stop()
			t.Fatalf("maintenance did not continue after error: expires=%d gc=%d", expireCalls, gcCalls)
		}
		time.Sleep(time.Millisecond)
	}
	maintenance.Stop()
	if len(errorsSeen) < 2 {
		t.Fatalf("maintenance errors were not observable: %d", len(errorsSeen))
	}
	status := maintenance.Status()
	if status.RunCount < 2 || status.FailureCount < 1 {
		t.Fatalf("failure status not retained: %#v", status)
	}
	_, expireCalls, gcCalls := fake.snapshot()
	if gcCalls >= expireCalls {
		t.Fatalf("gc was not conservative after reference errors: expires=%d gc=%d", expireCalls, gcCalls)
	}
}
