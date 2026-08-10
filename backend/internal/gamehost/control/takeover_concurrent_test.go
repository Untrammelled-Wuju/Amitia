package control

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestConcurrent_100Takeovers(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-concurrent-takeover-1")
	pluginID := domain.PluginID("plugin-concurrent-takeover-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Setup transition failed: %v", err)
	}

	var successCount atomic.Int32
	var noopCount atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Takeover(ctx, TakeoverRequest{
				RuntimeID: runtimeID,
				PluginID:  pluginID,
				Actor:     ActorUser,
			})
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	finalSnap, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if finalSnap.Mode != domain.ControlModeUserControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModeUserControl, finalSnap.Mode)
	}
	if finalSnap.Epoch != 3 {
		t.Errorf("expected epoch 3, got %d", finalSnap.Epoch)
	}
	_ = noopCount
}

func TestConcurrent_100Releases(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-concurrent-release-1")
	pluginID := domain.PluginID("plugin-concurrent-release-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	var successCount atomic.Int32
	var staleCount atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Release(ctx, ReleaseRequest{
				RuntimeID:  runtimeID,
				PluginID:   pluginID,
				TargetMode: domain.ControlModeObserveOnly,
				Actor:      ActorUser,
			})
			if err == nil {
				successCount.Add(1)
			} else {
				staleCount.Add(1)
			}
		}()
	}

	wg.Wait()

	finalSnap, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if successCount.Load() != 1 {
		t.Errorf("expected exactly 1 successful release, got %d", successCount.Load())
	}
	_ = staleCount

	if finalSnap.Mode != domain.ControlModeObserveOnly {
		t.Errorf("expected mode %q after release, got %q", domain.ControlModeObserveOnly, finalSnap.Mode)
	}
}

func TestConcurrent_TakeoverVsPluginTransition(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-concurrent-race-1")
	pluginID := domain.PluginID("plugin-concurrent-race-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Setup transition failed: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			modes := []domain.ControlMode{
				domain.ControlModePluginControl,
				domain.ControlModeSharedControl,
			}
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = mgr.Transition(ctx, runtimeID, TransitionRequest{
						Target: modes[idx%2],
						Actor:  ActorPlugin,
						Reason: ReasonPluginRequest,
					})
				}
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = svc.Takeover(ctx, TakeoverRequest{
			RuntimeID: runtimeID,
			PluginID:  pluginID,
			Actor:     ActorUser,
		})
	}()

	close(stop)
	wg.Wait()

	finalSnap, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if finalSnap.Epoch < 2 {
		t.Errorf("expected epoch >= 2 after takeover race, got %d", finalSnap.Epoch)
	}
}

func TestTakeoverContext_Recorded(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-ctx-1")
	pluginID := domain.PluginID("plugin-ctx-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Setup transition failed: %v", err)
	}

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	tc, ok := svc.TakeoverContext(runtimeID)
	if !ok {
		t.Fatal("expected TakeoverContext to be recorded")
	}
	if tc.TakenFromMode != domain.ControlModePluginControl {
		t.Errorf("expected taken from mode %q, got %q", domain.ControlModePluginControl, tc.TakenFromMode)
	}
	if tc.RuntimeID != runtimeID {
		t.Errorf("expected runtime id %q, got %q", runtimeID, tc.RuntimeID)
	}
}

func TestTakeoverContext_ClearedAfterRelease(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-ctx-2")
	pluginID := domain.PluginID("plugin-ctx-2")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	_, ok := svc.TakeoverContext(runtimeID)
	if !ok {
		t.Fatal("expected TakeoverContext after takeover")
	}

	_, err = svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModeObserveOnly,
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	_, ok = svc.TakeoverContext(runtimeID)
	if ok {
		t.Error("expected TakeoverContext to be cleared after release")
	}
}

func TestOldEpochInvalidation_AfterTakeover(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-old-epoch-1")
	pluginID := domain.PluginID("plugin-old-epoch-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	pluginSnap, err := mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition to plugin failed: %v", err)
	}

	oldToken := AuthorityToken{
		RuntimeID: runtimeID,
		Epoch:     pluginSnap.Epoch,
	}

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	currentSnap, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !currentSnap.IsStale(oldToken) {
		t.Error("expected old plugin epoch token to be stale after takeover")
	}
	if currentSnap.Epoch == oldToken.Epoch {
		t.Errorf("expected new epoch (%d) != old epoch (%d)", currentSnap.Epoch, oldToken.Epoch)
	}
}

func TestABA_TakeoverReleaseTakeover(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-aba-1")
	pluginID := domain.PluginID("plugin-aba-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)
	rtReader.SetReady(runtimeID, true)

	_, err = mgr.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition to plugin failed: %v", err)
	}

	pluginSnap1, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	originalEpoch := pluginSnap1.Epoch

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("First takeover failed: %v", err)
	}

	result, err := svc.Release(ctx, ReleaseRequest{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		TargetMode: domain.ControlModePluginControl,
		Actor:      ActorUser,
	})
	if err != nil {
		t.Fatalf("Release to plugin failed: %v", err)
	}

	if result.NewMode != domain.ControlModePluginControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModePluginControl, result.NewMode)
	}

	finalSnap, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if finalSnap.Epoch <= originalEpoch {
		t.Errorf("expected epoch > %d after ABA, got %d", originalEpoch, finalSnap.Epoch)
	}

	if finalSnap.Mode != domain.ControlModePluginControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModePluginControl, finalSnap.Mode)
	}
}

func TestTakeover_DoesNotStopRuntime(t *testing.T) {
	svc, mgr, rtReader, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-nostop-1")
	pluginID := domain.PluginID("plugin-nostop-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rtReader.SetActive(runtimeID, true)

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     ActorUser,
	})
	if err != nil {
		t.Fatalf("Takeover failed: %v", err)
	}

	snap, err := mgr.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.Mode != domain.ControlModeUserControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModeUserControl, snap.Mode)
	}
}

func TestTakeover_EmptyActor(t *testing.T) {
	svc, mgr, _, _ := setupTakeoverService(t)
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-empty-actor-1")
	pluginID := domain.PluginID("plugin-empty-actor-1")

	_, err := mgr.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = svc.Takeover(ctx, TakeoverRequest{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Actor:     "",
	})
	if err == nil {
		t.Fatal("expected error for empty actor, got nil")
	}
}
