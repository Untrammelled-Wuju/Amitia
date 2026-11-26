//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func lifecycleLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s01 "+format, args...)
}

func lifecycleClock() time.Time {
	return time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
}

func newLifecycleMachine(t *testing.T, initial LifecycleState) *Machine {
	t.Helper()
	machine, err := NewMachineWithClock(initial, lifecycleClock)
	if err != nil {
		t.Fatalf("NewMachineWithClock(%s): %v", initial, err)
	}
	return machine
}

func TestLifecycleHappyPath(t *testing.T) {
	ctx := context.Background()
	machine := newLifecycleMachine(t, LifecycleAbsent)
	steps := []struct {
		event LifecycleEvent
		want  LifecycleState
	}{
		{EventInstall, LifecycleInstalled},
		{EventEnable, LifecycleEnabled},
		{EventActivate, LifecycleActivated},
		{EventWarm, LifecycleWarm},
		{EventStart, LifecycleRunning},
		{EventDrain, LifecycleWarm},
		{EventSuspend, LifecycleSuspended},
		{EventResume, LifecycleActivated},
		{EventWarm, LifecycleWarm},
		{EventStart, LifecycleRunning},
		{EventSuspend, LifecycleSuspended},
		{EventUninstall, LifecycleAbsent},
	}
	for index, step := range steps {
		epoch := uint64(index + 1)
		transition, err := machine.Apply(ctx, step.event, epoch)
		if err != nil {
			t.Fatalf("step %d %s: %v", index, step.event, err)
		}
		if !transition.Changed || transition.To != step.want {
			t.Fatalf("step %d %s -> %s want %s", index, step.event, transition.To, step.want)
		}
		if machine.State() != step.want || machine.Epoch() != epoch {
			t.Fatalf("step %d state=%s epoch=%d", index, machine.State(), machine.Epoch())
		}
	}
	if len(machine.History()) != len(steps) {
		t.Fatalf("history=%d want %d", len(machine.History()), len(steps))
	}
	lifecycleLog(t, "happy steps=12 final=absent epoch=%d history=12", machine.Epoch())
}

func TestLifecycleInstalledCannotJumpToRunning(t *testing.T) {
	ctx := context.Background()
	machine := newLifecycleMachine(t, LifecycleInstalled)
	transition, err := machine.Apply(ctx, EventStart, 1)
	if !errors.Is(err, ErrLifecycleTransition) {
		t.Fatalf("err=%v want ErrLifecycleTransition", err)
	}
	if !strings.Contains(err.Error(), "installed + start") {
		t.Fatalf("error does not name the pair: %v", err)
	}
	if transition.Changed || machine.State() != LifecycleInstalled || machine.Epoch() != 0 {
		t.Fatalf("rejected transition mutated state: %+v", transition)
	}
	if len(machine.History()) != 0 {
		t.Fatalf("rejected transition entered history: %d", len(machine.History()))
	}
	lifecycleLog(t, "no_skip installed_start=ErrLifecycleTransition state=installed epoch=0")
}

func TestLifecycleNoBackwardWhileRunning(t *testing.T) {
	ctx := context.Background()
	machine := newLifecycleMachine(t, LifecycleRunning)
	for _, event := range []LifecycleEvent{EventUninstall, EventDisable, EventDeactivate, EventResume} {
		if _, err := machine.Apply(ctx, event, 1); !errors.Is(err, ErrLifecycleTransition) {
			t.Fatalf("running + %s err=%v want ErrLifecycleTransition", event, err)
		}
	}
	if machine.State() != LifecycleRunning {
		t.Fatalf("state=%s want running", machine.State())
	}
	lifecycleLog(t, "no_backward running_events=4 rejected=true state=running")
}

func TestLifecycleIdempotentReplay(t *testing.T) {
	ctx := context.Background()
	machine := newLifecycleMachine(t, LifecycleAbsent)
	if _, err := machine.Apply(ctx, EventInstall, 1); err != nil {
		t.Fatalf("install: %v", err)
	}
	first, err := machine.Apply(ctx, EventEnable, 2)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	replay, err := machine.Apply(ctx, EventEnable, 2)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replay.Changed || replay.From != first.From || replay.To != first.To {
		t.Fatalf("replay=%+v first=%+v", replay, first)
	}
	if machine.Epoch() != 2 || machine.State() != LifecycleEnabled || len(machine.History()) != 2 {
		t.Fatalf("replay advanced state: epoch=%d state=%s history=%d", machine.Epoch(), machine.State(), len(machine.History()))
	}
	if _, err := machine.Apply(ctx, EventDisable, 2); !errors.Is(err, ErrLifecycleStale) {
		t.Fatalf("conflicting event on applied epoch err=%v want ErrLifecycleStale", err)
	}
	lifecycleLog(t, "idempotent replay_changed=false history=2 conflict=ErrLifecycleStale")
}

func TestLifecycleStaleEpochRejected(t *testing.T) {
	ctx := context.Background()
	machine := newLifecycleMachine(t, LifecycleAbsent)
	for index, epoch := range []uint64{1, 2, 4, 5, 6} {
		if _, err := machine.Apply(ctx, []LifecycleEvent{EventInstall, EventEnable, EventActivate, EventWarm, EventStart}[index], epoch); err != nil {
			t.Fatalf("epoch %d: %v", epoch, err)
		}
	}
	transition, err := machine.Apply(ctx, EventSuspend, 3)
	if !errors.Is(err, ErrLifecycleStale) {
		t.Fatalf("stale err=%v want ErrLifecycleStale", err)
	}
	if transition.Changed || machine.State() != LifecycleRunning || machine.Epoch() != 6 {
		t.Fatalf("stale event mutated state: %+v", transition)
	}
	lifecycleLog(t, "stale epoch=3 current=6 rejected=true state=running")
}

func TestLifecycleInvalidInputAndCancel(t *testing.T) {
	if _, err := NewMachine(LifecycleState("bogus")); !errors.Is(err, ErrLifecycleInvalid) {
		t.Fatalf("bogus state err=%v", err)
	}
	machine := newLifecycleMachine(t, LifecycleInstalled)
	if _, err := machine.Apply(context.Background(), LifecycleEvent("bogus"), 1); !errors.Is(err, ErrLifecycleInvalid) {
		t.Fatalf("bogus event err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := machine.Apply(ctx, EventEnable, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v want context.Canceled", err)
	}
	if machine.State() != LifecycleInstalled || machine.Epoch() != 0 {
		t.Fatalf("invalid input mutated state: %s/%d", machine.State(), machine.Epoch())
	}
	lifecycleLog(t, "invalid bogus_state=ErrLifecycleInvalid bogus_event=ErrLifecycleInvalid cancel=context.Canceled")
}

func TestLifecycleMinimumHops(t *testing.T) {
	cases := []struct {
		from LifecycleState
		to   LifecycleState
		want int
	}{
		{LifecycleInstalled, LifecycleRunning, 4},
		{LifecycleAbsent, LifecycleRunning, 5},
		{LifecycleRunning, LifecycleAbsent, 2},
		{LifecycleRunning, LifecycleInstalled, 2},
		{LifecycleSuspended, LifecycleInstalled, 1},
		{LifecycleEnabled, LifecycleWarm, 2},
	}
	for _, item := range cases {
		hops, ok := MinHops(item.from, item.to)
		if !ok || hops != item.want {
			t.Fatalf("%s -> %s hops=%d ok=%v want %d", item.from, item.to, hops, ok, item.want)
		}
	}
	if _, ok := MinHops(LifecycleState("bogus"), LifecycleRunning); ok {
		t.Fatal("invalid state produced a path")
	}
	lifecycleLog(t, "hops installed_running=4 absent_running=5 running_absent=2")
}

func TestLifecycleDeriveFromExistingEnums(t *testing.T) {
	cases := []struct {
		installation InstallationState
		mode         EnablementState
		desired      DesiredRuntimeState
		actual       ActualRuntimeState
		want         LifecycleState
		wantErr      error
	}{
		{InstallationStateNotInstalled, EnablementDisabled, DesiredRuntimeStopped, ActualRuntimeStopped, LifecycleAbsent, nil},
		{InstallationStateInstalled, EnablementDisabled, DesiredRuntimeStopped, ActualRuntimeStopped, LifecycleInstalled, nil},
		{InstallationStateInstalled, EnablementEnabled, DesiredRuntimeStopped, ActualRuntimeStopped, LifecycleEnabled, nil},
		{InstallationStateInstalled, EnablementEnabled, DesiredRuntimeStarted, ActualRuntimeReady, LifecycleWarm, nil},
		{InstallationStateInstalled, EnablementEnabled, DesiredRuntimeStarted, ActualRuntimeRunning, LifecycleRunning, nil},
		{InstallationStateInstalled, EnablementDisabled, DesiredRuntimeStopped, ActualRuntimeRunning, "", ErrLifecycleTransition},
		{InstallationStateNotInstalled, EnablementEnabled, DesiredRuntimeStopped, ActualRuntimeStopped, "", ErrLifecycleTransition},
		{InstallationStateInstalling, EnablementDisabled, DesiredRuntimeStopped, ActualRuntimeStopped, "", ErrLifecycleInvalid},
	}
	for _, item := range cases {
		got, err := DeriveLifecycle(item.installation, item.mode, item.desired, item.actual)
		if item.wantErr != nil {
			if !errors.Is(err, item.wantErr) {
				t.Fatalf("%s/%s/%s/%s err=%v want %v", item.installation, item.mode, item.desired, item.actual, err, item.wantErr)
			}
			continue
		}
		if err != nil || got != item.want {
			t.Fatalf("%s/%s/%s/%s got=%s err=%v want %s", item.installation, item.mode, item.desired, item.actual, got, err, item.want)
		}
	}
	lifecycleLog(t, "derive cases=8 stable=5 inconsistent=2 transitional=1")
}

func TestLifecycleProjectionsStayConsistent(t *testing.T) {
	for _, state := range AllLifecycleStates() {
		installation, err := state.Installation()
		if err != nil {
			t.Fatalf("%s installation: %v", state, err)
		}
		mode, err := state.Enablement()
		if err != nil {
			t.Fatalf("%s enablement: %v", state, err)
		}
		desired, err := state.Desired()
		if err != nil {
			t.Fatalf("%s desired: %v", state, err)
		}
		actual, err := state.Actual()
		if err != nil {
			t.Fatalf("%s actual: %v", state, err)
		}
		derived, err := DeriveLifecycle(installation, mode, desired, actual)
		if err != nil {
			t.Fatalf("%s derive: %v", state, err)
		}
		expected := state
		if state == LifecycleActivated {
			expected = LifecycleEnabled
		}
		if derived != expected {
			t.Fatalf("%s projected to %s want %s (installation=%s mode=%s desired=%s actual=%s)", state, derived, expected, installation, mode, desired, actual)
		}
		for _, event := range LegalEvents(state) {
			target, ok := CanTransition(state, event)
			if !ok || target == state {
				t.Fatalf("%s + %s -> %s ok=%v", state, event, target, ok)
			}
			if !target.Valid() {
				t.Fatalf("%s + %s produced invalid state %s", state, event, target)
			}
		}
	}
	if _, ok := CanTransition(LifecycleInstalled, EventWarm); ok {
		t.Fatal("installed + warm is legal")
	}
	lifecycleLog(t, "projections states=7 suspended_derives=suspended legal_edges=16")
}

func TestLifecycleConcurrentReplay(t *testing.T) {
	ctx := context.Background()
	machine := newLifecycleMachine(t, LifecycleInstalled)
	var changed atomic.Int32
	var applied atomic.Int32
	var wg sync.WaitGroup
	for index := 0; index < 8; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transition, err := machine.Apply(ctx, EventEnable, 1)
			if err != nil {
				t.Errorf("concurrent apply: %v", err)
				return
			}
			if transition.Changed {
				changed.Add(1)
			}
			applied.Add(1)
		}()
	}
	wg.Wait()
	if applied.Load() != 8 {
		t.Fatalf("applied=%d want 8", applied.Load())
	}
	if changed.Load() != 1 {
		t.Fatalf("changed=%d want exactly 1", changed.Load())
	}
	if machine.State() != LifecycleEnabled || machine.Epoch() != 1 || len(machine.History()) != 1 {
		t.Fatalf("state=%s epoch=%d history=%d", machine.State(), machine.Epoch(), len(machine.History()))
	}
	lifecycleLog(t, "concurrent workers=8 changed=1 history=1 state=enabled")
}
