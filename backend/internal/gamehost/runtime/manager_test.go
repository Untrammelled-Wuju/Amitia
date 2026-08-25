package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type deterministicIDGenerator struct {
	counter int
}

func (g *deterministicIDGenerator) NewRuntimeID() domain.RuntimeInstanceID {
	g.counter++
	return domain.RuntimeInstanceID("rt_test_" + string(rune('0'+g.counter)))
}

func fixedClock() time.Time {
	return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
}

func newTestManager() *Manager {
	return NewManager(ManagerOptions{
		IDGenerator: &deterministicIDGenerator{},
		Clock:       func() time.Time { return fixedClock() },
	})
}

func TestManager_Create_GeneratesIndependentRuntimeID(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	rt1, err := m.Create(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	rt2, err := m.Create(ctx, "plugin-b")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if rt1.ID == rt2.ID {
		t.Error("two Create calls should generate distinct RuntimeIDs")
	}
}

func TestManager_Create_StateIsCreated(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	rt, err := m.Create(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if rt.State != domain.RuntimeStateCreated {
		t.Errorf("initial state = %q, want %q", rt.State, domain.RuntimeStateCreated)
	}
}

func TestManager_Get_ReturnsCopy(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	rt, err := m.Create(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	got, err := m.Get(ctx, rt.ID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}

	got.State = domain.RuntimeStateRunning

	original, _ := m.Get(ctx, rt.ID)
	if original.State == domain.RuntimeStateRunning {
		t.Error("Get should return copy, mutation leaked to internal state")
	}
}

func TestManager_List_ReturnsCopies(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	_, err := m.Create(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	_, err = m.Create(ctx, "plugin-b")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	list, err := m.List(ctx)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List returned %d items, want 2", len(list))
	}
}

func TestManager_UpdateRuntimeState_UsesDomainTransition(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	rt, err := m.Create(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	err = m.UpdateRuntimeState(rt.ID, domain.RuntimeStateStarting, "test", fixedClock())
	if err != nil {
		t.Fatalf("UpdateRuntimeState error: %v", err)
	}

	ref, err := m.GetRuntime(rt.ID)
	if err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	if ref.State != domain.RuntimeStateStarting {
		t.Errorf("state after transition = %q, want %q", ref.State, domain.RuntimeStateStarting)
	}
}

func TestManager_UpdateRuntimeState_InvalidTransitionRejected(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	rt, err := m.Create(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	err = m.UpdateRuntimeState(rt.ID, domain.RuntimeStateRunning, "", fixedClock())
	if err == nil {
		t.Error("expected error for invalid transition created -> running, got nil")
	}
}

func TestManager_UpdateRuntimeHealth_DoesNotReplaceState(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	rt, err := m.Create(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	err = m.UpdateRuntimeHealth(rt.ID, domain.HealthHealthy, "ok", fixedClock())
	if err != nil {
		t.Fatalf("UpdateRuntimeHealth error: %v", err)
	}

	ref, err := m.GetRuntime(rt.ID)
	if err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	if ref.State != domain.RuntimeStateCreated {
		t.Errorf("state after health update = %q, want unchanged %q", ref.State, domain.RuntimeStateCreated)
	}
}

func TestManager_EnsurePrimaryRuntime_Idempotent(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	rt1, created1, err := m.EnsurePrimaryRuntime(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("EnsurePrimaryRuntime error: %v", err)
	}
	if !created1 {
		t.Error("first EnsurePrimaryRuntime should report created=true")
	}

	rt2, created2, err := m.EnsurePrimaryRuntime(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("EnsurePrimaryRuntime second error: %v", err)
	}
	if created2 {
		t.Error("second EnsurePrimaryRuntime should report created=false")
	}
	if rt1.ID != rt2.ID {
		t.Errorf("EnsurePrimaryRuntime not idempotent: %q != %q", rt1.ID, rt2.ID)
	}
}

func TestManager_EnsurePrimaryRuntime_Isolation(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	rtA, _, err := m.EnsurePrimaryRuntime(ctx, "plugin-a")
	if err != nil {
		t.Fatalf("EnsurePrimaryRuntime error: %v", err)
	}
	rtB, _, err := m.EnsurePrimaryRuntime(ctx, "plugin-b")
	if err != nil {
		t.Fatalf("EnsurePrimaryRuntime error: %v", err)
	}

	if rtA.ID == rtB.ID {
		t.Error("different plugins should get different RuntimeIDs")
	}

	if rtA.PluginID != "plugin-a" || rtB.PluginID != "plugin-b" {
		t.Errorf("PluginID mismatch: %q, %q", rtA.PluginID, rtB.PluginID)
	}
}

func TestManager_ListRuntimes_Reflective(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	_, _ = m.Create(ctx, "plugin-a")
	_, _ = m.Create(ctx, "plugin-b")

	refs := m.ListRuntimes()
	if len(refs) != 2 {
		t.Errorf("ListRuntimes returned %d items, want 2", len(refs))
	}
}

func TestManager_Concurrent_Safe(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = m.Create(ctx, domain.PluginID("plugin-"+string(rune('a'+idx))))
		}(i)
	}
	wg.Wait()

	list, _ := m.List(ctx)
	if len(list) != 10 {
		t.Errorf("concurrent Create: got %d runtimes, want 10", len(list))
	}
}

func TestManager_DurableEmergencyResolverAlsoExposesLifecycleIntent(t *testing.T) {
	m := newTestManager()
	ctx := context.Background()
	rt, err := m.Create(ctx, "plugin-a")
	if err != nil {
		t.Fatal(err)
	}
	m.SetEmergencyLatchResolver(func(runtimeID domain.RuntimeInstanceID) bool {
		return runtimeID == rt.ID
	})
	if !m.IsEmergencyLatched(rt.ID) {
		t.Fatal("expected resolver-backed emergency latch")
	}
	intent, err := m.GetLifecycleIntent(rt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if intent != "emergency" {
		t.Fatalf("resolver-backed lifecycle intent=%q, want emergency", intent)
	}
}
