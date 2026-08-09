package checkpoint

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type fakeResolver struct {
	descriptors map[domain.PluginID]domain.PluginDescriptor
}

func (r *fakeResolver) Resolve(pluginID domain.PluginID) (domain.PluginDescriptor, bool) {
	d, ok := r.descriptors[pluginID]
	return d, ok
}

func TestCheckpointManager_CreateAndLoad(t *testing.T) {
	store, _ := newTestStore(t)
	resolver := &fakeResolver{descriptors: make(map[domain.PluginID]domain.PluginDescriptor)}
	mgr, err := NewCheckpointManager(store, resolver)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	services := []domain.ServiceID{"bridge", "agent"}

	md, err := mgr.CreateMetadata(ctx, "rt-1", "com.test.plugin", "ext-1", "1.0.0", "rev-abc", now)
	if err != nil {
		t.Fatalf("failed to create metadata: %v", err)
	}

	cp, err := mgr.SaveCreatedCheckpoint(ctx, "rt-1", "com.test.plugin", services, "rev-abc", now)
	if err != nil {
		t.Fatalf("failed to save created checkpoint: %v", err)
	}

	if cp.RuntimeState != domain.RuntimeStateCreated {
		t.Fatalf("expected created state, got: %s", cp.RuntimeState)
	}
	if len(cp.Services) != 2 {
		t.Fatalf("expected 2 services, got: %d", len(cp.Services))
	}
	if cp.CleanShutdown != false {
		t.Fatalf("expected CleanShutdown=false")
	}

	loadedMD, err := mgr.LoadMetadata(ctx, "rt-1")
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	if loadedMD.RuntimeID != md.RuntimeID {
		t.Fatal("metadata runtime id mismatch")
	}

	loadedCP, err := mgr.LoadCheckpoint(ctx, "rt-1")
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}
	if loadedCP.RuntimeState != domain.RuntimeStateCreated {
		t.Fatal("checkpoint state mismatch")
	}
}

func TestCheckpointManager_SaveRunningCheckpoint(t *testing.T) {
	store, _ := newTestStore(t)
	resolver := &fakeResolver{}
	mgr, err := NewCheckpointManager(store, resolver)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	_, _ = mgr.CreateMetadata(ctx, "rt-running", "com.test", "ext-1", "1.0.0", "rev-x", now)
	_, _ = mgr.SaveCreatedCheckpoint(ctx, "rt-running", "com.test", []domain.ServiceID{"svc-a"}, "rev-x", now)

	services := []ServiceCheckpoint{
		{
			ServiceID: "svc-a",
			State:     runtime.ServiceStateRunning,
			Required:  true,
			UpdatedAt: now,
		},
	}

	cp, err := mgr.SaveRunningCheckpoint(ctx, "rt-running", "com.test", services, "rev-x", true, now.Add(time.Second))
	if err != nil {
		t.Fatalf("failed to save running checkpoint: %v", err)
	}

	if cp.RuntimeState != domain.RuntimeStateRunning {
		t.Fatalf("expected running, got: %s", cp.RuntimeState)
	}
	if cp.CleanShutdown != false {
		t.Fatalf("expected CleanShutdown=false while running")
	}
	if cp.LastKnownGoodAt == nil {
		t.Fatal("expected LastKnownGoodAt to be set")
	}

	loaded, err := mgr.LoadCheckpoint(ctx, "rt-running")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LastKnownGoodAt == nil {
		t.Fatal("loaded LastKnownGoodAt missing")
	}
}

func TestCheckpointManager_SaveStoppedCheckpoint(t *testing.T) {
	store, _ := newTestStore(t)
	resolver := &fakeResolver{}
	mgr, err := NewCheckpointManager(store, resolver)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	_, _ = mgr.CreateMetadata(ctx, "rt-stop", "com.test", "ext-1", "1.0.0", "rev-x", now)

	services := []ServiceCheckpoint{
		{ServiceID: "svc-a", State: runtime.ServiceStateRunning, Required: true, UpdatedAt: now},
	}
	_, _ = mgr.SaveRunningCheckpoint(ctx, "rt-stop", "com.test", services, "rev-x", true, now)

	cp, err := mgr.SaveStoppedCheckpoint(ctx, "rt-stop", "com.test", true, "user_stop", now.Add(time.Second))
	if err != nil {
		t.Fatalf("failed to save stopped: %v", err)
	}

	if cp.RuntimeState != domain.RuntimeStateStopped {
		t.Fatalf("expected stopped, got: %s", cp.RuntimeState)
	}
	if !cp.CleanShutdown {
		t.Fatal("expected CleanShutdown=true")
	}
	if cp.Reason != "user_stop" {
		t.Fatalf("expected reason 'user_stop', got: %s", cp.Reason)
	}
}

func TestCheckpointManager_SaveFailedCheckpoint(t *testing.T) {
	store, _ := newTestStore(t)
	resolver := &fakeResolver{}
	mgr, err := NewCheckpointManager(store, resolver)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	_, _ = mgr.CreateMetadata(ctx, "rt-fail", "com.test", "ext-1", "1.0.0", "rev-x", now)

	cp, err := mgr.SaveFailedCheckpoint(ctx, "rt-fail", "com.test", "service_failed", now)
	if err != nil {
		t.Fatalf("failed to save failed: %v", err)
	}

	if cp.RuntimeState != domain.RuntimeStateFailed {
		t.Fatalf("expected failed, got: %s", cp.RuntimeState)
	}
	if cp.CleanShutdown != false {
		t.Fatal("expected CleanShutdown=false for failure")
	}
	if cp.Reason != "service_failed" {
		t.Fatalf("expected reason 'service_failed', got: %s", cp.Reason)
	}
}

func TestCheckpointManager_ValidateCleanShutdown(t *testing.T) {
	store, _ := newTestStore(t)
	resolver := &fakeResolver{}
	mgr, err := NewCheckpointManager(store, resolver)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	_, _ = mgr.CreateMetadata(ctx, "rt-validate", "com.test", "ext-1", "1.0.0", "rev-x", now)
	_, _ = mgr.SaveCreatedCheckpoint(ctx, "rt-validate", "com.test", []domain.ServiceID{"svc-a"}, "rev-x", now)
	_, _ = mgr.SaveStoppedCheckpoint(ctx, "rt-validate", "com.test", true, "shutdown", now.Add(time.Second))

	if err := mgr.ValidateStoredCheckpoint(ctx, "rt-validate"); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestCheckpointManager_DescriptorRevisionStale(t *testing.T) {
	store, _ := newTestStore(t)

	descriptor := domain.PluginDescriptor{
		ID:          "com.stale",
		ExtensionID: "ext-1",
		Name:        "Stale",
		Version:     "1.0.0",
		Services: []domain.ServiceDescriptor{
			{ID: "svc-a", Name: "Service A", Kind: domain.ServiceKindProcess, Required: true},
		},
	}

	resolver := &fakeResolver{descriptors: map[domain.PluginID]domain.PluginDescriptor{
		"com.stale": descriptor,
	}}

	mgr, err := NewCheckpointManager(store, resolver)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	_, _ = mgr.CreateMetadata(ctx, "rt-stale", "com.stale", "ext-1", "1.0.0", "old-revision", now)
	_, _ = mgr.SaveCreatedCheckpoint(ctx, "rt-stale", "com.stale", []domain.ServiceID{"svc-a"}, "old-revision", now)

	err = mgr.ValidateStoredCheckpoint(ctx, "rt-stale")
	if err == nil {
		t.Fatal("expected stale revision error")
	}
	if !isErrKind(err, ErrStaleRevision) {
		t.Fatalf("expected stale_revision, got: %v", err)
	}
}

func TestCheckpointManager_NilStore(t *testing.T) {
	_, err := NewCheckpointManager(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestCheckpointManager_ReasonTruncation(t *testing.T) {
	store, _ := newTestStore(t)
	resolver := &fakeResolver{}
	mgr, err := NewCheckpointManager(store, resolver)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	longReason := ""
	for i := 0; i < 500; i++ {
		longReason += "x"
	}

	_, _ = mgr.CreateMetadata(ctx, "rt-reason", "com.test", "ext-1", "1.0.0", "rev", now)
	cp, err := mgr.SaveStoppedCheckpoint(ctx, "rt-reason", "com.test", true, longReason, now)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	if len(cp.Reason) > MaxReasonLength {
		t.Fatalf("reason length %d exceeds max %d", len(cp.Reason), MaxReasonLength)
	}
}
