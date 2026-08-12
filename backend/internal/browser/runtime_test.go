package browser

import (
	"context"
	"testing"
)

func TestRuntimeStateInitialState(t *testing.T) {
	state := newRuntimeState()
	current, gen := state.current()
	if current != BrowserRuntimeStopped {
		t.Fatalf("expected initial state stopped, got: %s", current)
	}
	if gen != 0 {
		t.Fatalf("expected initial generation 0, got: %d", gen)
	}
}

func TestRuntimeStateValidTransitions(t *testing.T) {
	tests := []struct {
		name   string
		from   BrowserRuntimeState
		to     BrowserRuntimeState
		expect bool
	}{
		{"stopped->starting", BrowserRuntimeStopped, BrowserRuntimeStarting, true},
		{"stopped->ready", BrowserRuntimeStopped, BrowserRuntimeReady, false},
		{"stopped->stopped", BrowserRuntimeStopped, BrowserRuntimeStopped, false},
		{"starting->ready", BrowserRuntimeStarting, BrowserRuntimeReady, true},
		{"starting->failed", BrowserRuntimeStarting, BrowserRuntimeFailed, true},
		{"ready->stopping", BrowserRuntimeReady, BrowserRuntimeStopping, true},
		{"ready->failed", BrowserRuntimeReady, BrowserRuntimeFailed, true},
		{"ready->ready", BrowserRuntimeReady, BrowserRuntimeReady, false},
		{"stopping->stopped", BrowserRuntimeStopping, BrowserRuntimeStopped, true},
		{"stopping->failed", BrowserRuntimeStopping, BrowserRuntimeFailed, true},
		{"failed->starting", BrowserRuntimeFailed, BrowserRuntimeStarting, true},
		{"failed->ready", BrowserRuntimeFailed, BrowserRuntimeReady, false},
		{"failed->stopped", BrowserRuntimeFailed, BrowserRuntimeStopped, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := newRuntimeState()
			state.state = tc.from
			result := state.setState(tc.to)
			if result != tc.expect {
				t.Fatalf("transition %s->%s expected %v, got %v", tc.from, tc.to, tc.expect, result)
			}
		})
	}
}

func TestRuntimeStateIncrementGeneration(t *testing.T) {
	state := newRuntimeState()
	g1 := state.incrementGeneration()
	if g1 != 1 {
		t.Fatalf("expected generation 1, got: %d", g1)
	}
	g2 := state.incrementGeneration()
	if g2 != 2 {
		t.Fatalf("expected generation 2, got: %d", g2)
	}
	_, gen := state.current()
	if gen != 2 {
		t.Fatalf("expected current generation 2, got: %d", gen)
	}
}

func TestRuntimeStateReadyCheck(t *testing.T) {
	state := newRuntimeState()
	if state.isReady() {
		t.Fatal("state should not be ready initially")
	}
	state.setStarting()
	if state.isReady() {
		t.Fatal("state should not be ready while starting")
	}
	state.setReady()
	if !state.isReady() {
		t.Fatal("state should be ready")
	}
	state.setFailed()
	if state.isReady() {
		t.Fatal("state should not be ready after failure")
	}
}

func TestAtomicCounter(t *testing.T) {
	var counter atomicCounter
	if counter.get() != 0 {
		t.Fatal("initial counter value should be 0")
	}
	if counter.next() != 1 {
		t.Fatal("first increment should be 1")
	}
	if counter.next() != 2 {
		t.Fatal("second increment should be 2")
	}
	if counter.get() != 2 {
		t.Fatal("get should return 2")
	}
}

func TestDisabledRuntime(t *testing.T) {
	rt := &disabledRuntime{}
	ctx := context.Background()

	info, err := rt.Start(ctx)
	if err == nil {
		t.Fatal("disabled runtime Start should fail")
	}
	if info != nil {
		t.Fatal("disabled runtime Start should return nil info")
	}
	if !IsBrowserDisabled(err) {
		t.Fatalf("expected browser_disabled error, got: %v", err)
	}

	if stopErr := rt.Stop(ctx); stopErr == nil || !IsBrowserDisabled(stopErr) {
		t.Fatalf("disabled runtime Stop should return disabled error, got: %v", stopErr)
	}

	status := rt.Status(ctx)
	if status.State != BrowserRuntimeStopped {
		t.Fatalf("expected stopped state, got: %s", status.State)
	}

	health := rt.Health(ctx)
	if health != BrowserHealthUnavailable {
		t.Fatalf("expected unavailable health, got: %s", health)
	}
}

func TestUnsupportedManagers(t *testing.T) {
	ctx := context.Background()

	sm := &unsupportedSessionManager{}
	if _, err := sm.CreateSession(ctx); err == nil || !IsUnsupportedOperation(err) {
		t.Fatal("unsupportedSessionManager.CreateSession should return unsupported error")
	}

	tm := &unsupportedTabManager{}
	if _, err := tm.CreateTab(ctx, "s1"); err == nil || !IsUnsupportedOperation(err) {
		t.Fatal("unsupportedTabManager.CreateTab should return unsupported error")
	}

	nav := &unsupportedNavigator{}
	if _, err := nav.Navigate(ctx, "s1", "t1", NavigateRequest{}); err == nil || !IsUnsupportedOperation(err) {
		t.Fatal("unsupportedNavigator.Navigate should return unsupported error")
	}

	obs := &unsupportedObserver{}
	if _, err := obs.GetDOMSnapshot(ctx, "s1", "t1", 3); err == nil || !IsUnsupportedOperation(err) {
		t.Fatal("unsupportedObserver.GetDOMSnapshot should return unsupported error")
	}

	inter := &unsupportedInteractor{}
	if _, err := inter.Click(ctx, "s1", "t1", BrowserElementRef{}); err == nil || !IsUnsupportedOperation(err) {
		t.Fatal("unsupportedInteractor.Click should return unsupported error")
	}

	rt := &unsupportedResourceTransfer{}
	if _, err := rt.Download(ctx, BrowserDownloadRequest{}); err == nil || !IsUnsupportedOperation(err) {
		t.Fatal("unsupportedResourceTransfer.Download should return unsupported error")
	}
}

func TestProductionProviderDisabled(t *testing.T) {
	provider := NewDisabledProvider()
	rt := provider.Runtime()
	ctx := context.Background()

	info, err := rt.Start(ctx)
	if err == nil {
		t.Fatal("disabled provider runtime Start should fail")
	}
	if info != nil {
		t.Fatal("disabled provider runtime Start should return nil info")
	}

	caps := provider.BrowserCapabilities()
	if caps.SupportsNavigation || caps.SupportsDOM || caps.SupportsInteraction {
		t.Fatal("disabled provider capabilities should all be false")
	}
}

func TestRuntimeControllerStop(t *testing.T) {
	engine := &fakeEngine{state: BrowserRuntimeReady}
	controller := NewRuntimeController(engine)
	ctx := context.Background()

	if err := controller.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if !engine.stopped {
		t.Fatal("engine should be stopped")
	}
}

type fakeEngine struct {
	state   BrowserRuntimeState
	stopped bool
}

func (e *fakeEngine) Start(_ context.Context) (*BrowserRuntimeInfo, error) {
	e.state = BrowserRuntimeReady
	return &BrowserRuntimeInfo{State: BrowserRuntimeReady}, nil
}

func (e *fakeEngine) Stop(_ context.Context) error {
	e.stopped = true
	e.state = BrowserRuntimeStopped
	return nil
}

func (e *fakeEngine) Status(_ context.Context) BrowserRuntimeInfo {
	return BrowserRuntimeInfo{State: e.state}
}

func (e *fakeEngine) Health(_ context.Context) BrowserRuntimeHealth {
	if e.state == BrowserRuntimeReady {
		return BrowserHealthHealthy
	}
	return BrowserHealthUnavailable
}

func (e *fakeEngine) Contexts() BrowserContextController {
	return &fakeContextController{}
}

func (e *fakeEngine) Targets() BrowserTargetController {
	return &fakeTargetController{}
}

func (e *fakeEngine) Pages() BrowserPageController {
	return &fakePageController{}
}

type fakeContextController struct{}

func (c *fakeContextController) CreateBrowserContext(_ context.Context) (BrowserContextID, error) {
	return BrowserContextID("fake-context-1"), nil
}

func (c *fakeContextController) DisposeBrowserContext(_ context.Context, _ BrowserContextID) error {
	return nil
}

type fakeTargetController struct{}

func (c *fakeTargetController) CreateTarget(_ context.Context, _ BrowserContextID, _ string) (TargetID, error) {
	return TargetID("fake-target-1"), nil
}

func (c *fakeTargetController) CloseTarget(_ context.Context, _ TargetID) error {
	return nil
}

func (c *fakeTargetController) ActivateTarget(_ context.Context, _ TargetID) error {
	return nil
}

func (c *fakeTargetController) TargetInfo(_ context.Context, targetID TargetID) (TargetInfo, error) {
	return TargetInfo{TargetID: targetID, Type: "page"}, nil
}
