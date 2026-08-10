package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestPluginOutputGate_RaceOutputVsTakeover(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)

	mgr := NewControlAuthorityManager(ControlAuthorityManagerOptions{})
	_, _ = mgr.Create(context.Background(), "rt-1", "plugin-1")
	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModePluginControl,
		Actor:  ActorSystem,
		Reason: ReasonRuntimeLifecycle,
	})

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     mgr,
	})

	var wg sync.WaitGroup
	const N = 200
	for i := 0; i < N; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			snap, _ := mgr.Get(context.Background(), "rt-1")
			req := OutputCheckRequest{
				Intent: newTestOutputIntent("rt-1", "", snap.Epoch),
				Peer:   newTestPeer("rt-1", "", "plugin-1"),
			}
			_, _ = gate.Check(context.Background(), req)
		}()
		go func() {
			defer wg.Done()
			_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
				Target: domain.ControlModeUserControl,
				Actor:  ActorUser,
				Reason: ReasonUserRequest,
			})
			_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
				Target: domain.ControlModePluginControl,
				Actor:  ActorSystem,
				Reason: ReasonSystemRecovery,
			})
		}()
	}
	wg.Wait()
}

func TestPluginOutputGate_RaceOutputVsPermissionRevoke(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	perm := NewFakeEffPermChecker()
	perm.SetAllowed("rt-1//plugin-1", true)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   perm,
		Authority:     auth,
	})

	var wg sync.WaitGroup
	const N = 100
	for i := 0; i < N; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			req := OutputCheckRequest{
				Intent: newTestOutputIntent("rt-1", "", 5),
				Peer:   newTestPeer("rt-1", "", "plugin-1"),
			}
			_, _ = gate.Check(context.Background(), req)
		}()
		go func() {
			defer wg.Done()
			perm.SetAllowed("rt-1//plugin-1", false)
			perm.SetAllowed("rt-1//plugin-1", true)
		}()
	}
	wg.Wait()
}

func TestPluginOutputGate_RaceOutputVsRuntimeStop(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)
	rt.SetStopping("rt-1", false)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	var wg sync.WaitGroup
	const N = 100
	for i := 0; i < N; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			req := OutputCheckRequest{
				Intent: newTestOutputIntent("rt-1", "", 5),
				Peer:   newTestPeer("rt-1", "", "plugin-1"),
			}
			_, _ = gate.Check(context.Background(), req)
		}()
		go func() {
			defer wg.Done()
			rt.SetStopping("rt-1", true)
			rt.SetStopping("rt-1", false)
		}()
	}
	wg.Wait()
}

func TestPluginOutputGate_RaceGateCloseVsOutput(t *testing.T) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 5)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	var wg sync.WaitGroup
	const N = 100
	for i := 0; i < N; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			req := OutputCheckRequest{
				Intent: newTestOutputIntent("rt-1", "", 5),
				Peer:   newTestPeer("rt-1", "", "plugin-1"),
			}
			_, _ = gate.Check(context.Background(), req)
		}()
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				gate.CloseRuntimeOutputs("rt-1")
			} else {
				gate.ReopenRuntimeOutputs("rt-1")
			}
		}(i)
	}
	wg.Wait()
}
