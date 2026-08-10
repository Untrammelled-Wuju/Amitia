package control

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestConcurrent_TransitionSameEpochOnlyOneSucceeds(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-concurrent-1")
	pluginID := domain.PluginID("plugin-concurrent-1")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	targetModes := []domain.ControlMode{
		domain.ControlModeAssist,
		domain.ControlModeSharedControl,
		domain.ControlModePluginControl,
		domain.ControlModeUserControl,
		domain.ControlModeSuspended,
	}

	var successCount atomic.Int32
	var wg sync.WaitGroup

	for _, mode := range targetModes {
		wg.Add(1)
		go func(target domain.ControlMode) {
			defer wg.Done()
			_, err := m.Transition(ctx, runtimeID, TransitionRequest{
				Target:        target,
				Actor:         ActorUser,
				Reason:        ReasonUserRequest,
				ExpectedEpoch: 1,
				UseExpected:   true,
			})
			if err == nil {
				successCount.Add(1)
			}
		}(mode)
	}

	wg.Wait()

	if successCount.Load() != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount.Load())
	}

	snap, err := m.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.Epoch != 2 {
		t.Errorf("expected epoch 2, got %d", snap.Epoch)
	}
}

func TestConcurrent_GetVsTransition(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-concurrent-2")
	pluginID := domain.PluginID("plugin-concurrent-2")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = m.Get(ctx, runtimeID)
				}
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			modes := []domain.ControlMode{
				domain.ControlModePluginControl,
				domain.ControlModeObserveOnly,
			}
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = m.Transition(ctx, runtimeID, TransitionRequest{
						Target: modes[idx%2],
						Actor:  ActorHost,
						Reason: ReasonHostPolicy,
					})
				}
			}
		}(i)
	}

	close(stop)
	wg.Wait()

	snap, err := m.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Final Get failed: %v", err)
	}
	if snap.Epoch < 1 {
		t.Errorf("expected epoch >= 1, got %d", snap.Epoch)
	}
}

func TestConcurrent_TransitionVsRemove(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		runtimeID := domain.RuntimeInstanceID("rt-concurrent-remove-" + string(rune('a'+i)))
		pluginID := domain.PluginID("plugin-concurrent-" + string(rune('a'+i)))

		_, err := m.Create(ctx, runtimeID, pluginID)
		if err != nil {
			t.Fatalf("Create failed for %q: %v", runtimeID, err)
		}

		wg.Add(2)
		go func(rt domain.RuntimeInstanceID) {
			defer wg.Done()
			_, _ = m.Transition(ctx, rt, TransitionRequest{
				Target: domain.ControlModePluginControl,
				Actor:  ActorPlugin,
				Reason: ReasonPluginRequest,
			})
		}(runtimeID)

		go func(rt domain.RuntimeInstanceID) {
			defer wg.Done()
			_ = m.Remove(ctx, rt)
			m.CleanupRuntimeLock(rt)
		}(runtimeID)
	}

	wg.Wait()
}

func TestConcurrent_MultipleRuntimesParallel(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()

	numRuntimes := 50
	var wg sync.WaitGroup

	for i := 0; i < numRuntimes; i++ {
		runtimeID := domain.RuntimeInstanceID("rt-parallel-" + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		pluginID := domain.PluginID("plugin-parallel-" + string(rune('a'+i%26)))

		_, err := m.Create(ctx, runtimeID, pluginID)
		if err != nil {
			t.Fatalf("Create failed for %q: %v", runtimeID, err)
		}

		wg.Add(1)
		go func(rt domain.RuntimeInstanceID) {
			defer wg.Done()
			_, _ = m.Transition(ctx, rt, TransitionRequest{
				Target: domain.ControlModePluginControl,
				Actor:  ActorPlugin,
				Reason: ReasonPluginRequest,
			})
			_, _ = m.Transition(ctx, rt, TransitionRequest{
				Target: domain.ControlModeUserControl,
				Actor:  ActorUser,
				Reason: ReasonUserRequest,
			})
			_, _ = m.Transition(ctx, rt, TransitionRequest{
				Target: domain.ControlModeSharedControl,
				Actor:  ActorHost,
				Reason: ReasonHostPolicy,
			})
		}(runtimeID)
	}

	wg.Wait()

	list, err := m.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != numRuntimes {
		t.Errorf("expected %d runtimes, got %d", numRuntimes, len(list))
	}
}

func TestConcurrent_100GoroutinesSameBaseEpoch(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-100-goroutines")
	pluginID := domain.PluginID("plugin-100-goroutines")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var successCount atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			modes := []domain.ControlMode{
				domain.ControlModePluginControl,
				domain.ControlModeUserControl,
				domain.ControlModeSharedControl,
			}
			_, err := m.Transition(ctx, runtimeID, TransitionRequest{
				Target:        modes[idx%3],
				Actor:         ActorUser,
				Reason:        ReasonUserRequest,
				ExpectedEpoch: 1,
				UseExpected:   true,
			})
			if err == nil {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	if successCount.Load() != 1 {
		t.Errorf("expected exactly 1 success out of 100 goroutines, got %d", successCount.Load())
	}

	snap, err := m.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if snap.Epoch != 2 {
		t.Errorf("expected epoch 2, got %d", snap.Epoch)
	}
}

func TestABA_EpochIdentifiesStaleToken(t *testing.T) {
	m := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	ctx := context.Background()
	runtimeID := domain.RuntimeInstanceID("rt-aba")
	pluginID := domain.PluginID("plugin-aba")

	_, err := m.Create(ctx, runtimeID, pluginID)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	snap1, err := m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition 1 failed: %v", err)
	}

	oldToken := snap1.Token()

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModeUserControl,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})
	if err != nil {
		t.Fatalf("Transition 2 failed: %v", err)
	}

	_, err = m.Transition(ctx, runtimeID, TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorPlugin,
		Reason: ReasonPluginRequest,
	})
	if err != nil {
		t.Fatalf("Transition 3 failed: %v", err)
	}

	currentSnap, err := m.Get(ctx, runtimeID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !currentSnap.IsStale(oldToken) {
		t.Error("expected old token to be stale, but it was not")
	}

	if currentSnap.Mode != domain.ControlModePluginControl {
		t.Errorf("expected mode %q, got %q", domain.ControlModePluginControl, currentSnap.Mode)
	}

	if oldToken.Epoch >= currentSnap.Epoch {
		t.Errorf("expected old epoch %d < current epoch %d", oldToken.Epoch, currentSnap.Epoch)
	}
}
