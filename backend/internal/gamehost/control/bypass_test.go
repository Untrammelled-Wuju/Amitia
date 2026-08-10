package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RecordingControlEffectSink struct {
	mu      sync.Mutex
	allowed []OutputPermit
	errors  []error
}

func NewRecordingControlEffectSink() *RecordingControlEffectSink {
	return &RecordingControlEffectSink{}
}

func (s *RecordingControlEffectSink) ExecuteAuthorized(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowed = append(s.allowed, permit)
	return nil
}

func (s *RecordingControlEffectSink) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.allowed)
}

func newCustomTestGate() (*PluginOutputGate, *FakeTopology, *FakeAuthorityReader) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 10)

	gate := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:         func() time.Time { return time.Now().UTC() },
		Topology:      topo,
		RuntimeReader: rt,
		PermChecker:   NewFakeEffPermChecker(),
		Authority:     auth,
	})

	return gate, topo, auth
}

func TestPluginOutputGate_CustomRPC_GoesThroughGate(t *testing.T) {
	gate, _, _ := newCustomTestGate()
	sink := NewRecordingControlEffectSink()

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "custom-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{"method":"foo.bar","params":{}}`),
		},
		Peer: newTestPeer("rt-1", "", "plugin-1"),
	}

	_, _ = gate.AuthorizeAndDispatch(context.Background(), req, sink)

	if sink.Calls() != 1 {
		t.Fatalf("expected exactly 1 sink call for gated custom RPC, got %d", sink.Calls())
	}
}

func TestPluginOutputGate_ChannelOutput_GoesThroughGate(t *testing.T) {
	gate, _, _ := newCustomTestGate()
	sink := NewRecordingControlEffectSink()

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "channel-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindChannel,
			Payload:        []byte(`{"channel":"game","data":"move"}`),
		},
		Peer: newTestPeer("rt-1", "", "plugin-1"),
	}

	_, _ = gate.AuthorizeAndDispatch(context.Background(), req, sink)

	if sink.Calls() != 1 {
		t.Fatalf("expected exactly 1 sink call for gated channel, got %d", sink.Calls())
	}
}

func TestPluginOutputGate_GateDeny_SinkMustNotExecute(t *testing.T) {
	gate, _, _ := newCustomTestGate()
	sink := NewRecordingControlEffectSink()

	newTestIntentWithModeOutput := func() ControlOutputIntent {
		return ControlOutputIntent{
			OutputID:       "block-me",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 99,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{}`),
		}
	}

	req := OutputCheckRequest{
		Intent: newTestIntentWithModeOutput(),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err == nil {
		t.Fatal("expected error for stale epoch (gate deny)")
	}
	if sink.Calls() != 0 {
		t.Fatalf("sink must NOT execute when gate DENIES, got %d calls", sink.Calls())
	}
}

func TestPluginOutputGate_PayloadOpaque_NotParsed(t *testing.T) {
	gate, _, auth := newCustomTestGate()

	sink := ControlEffectSinkFunc(func(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error {
		return nil
	})

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "opaque-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{"method":"game.move","position":{"x":1,"y":2,"z":3}}`),
		},
		Peer: newTestPeer("rt-1", "", "plugin-1"),
	}

	decision, _ := gate.Check(context.Background(), req)
	if decision.Deny() {
		t.Fatalf("gate must not parse payload and reject based on game payload: %s", decision.Reason)
	}

	auth.SetSnapshot("rt-1", domain.ControlModeUserControl, 10)
	decision2, _ := gate.Check(context.Background(), req)
	if !decision2.Deny() {
		t.Fatal("gate must deny plugin output when user mode, regardless of payload")
	}

	_ = sink.ExecuteAuthorized(context.Background(), "rt-1", "", "plugin-1", OutputPermit{}, req.Payload)
}

func TestPluginOutputGate_UserModeSinkMustNotExecute(t *testing.T) {
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

	sink := NewRecordingControlEffectSink()

	_, _ = mgr.Transition(context.Background(), "rt-1", TransitionRequest{
		Target: domain.ControlModeUserControl,
		Actor:  ActorUser,
		Reason: ReasonUserRequest,
	})

	snap, _ := mgr.Get(context.Background(), "rt-1")

	req := OutputCheckRequest{
		Intent: newTestOutputIntent("rt-1", "", snap.Epoch),
		Peer:   newTestPeer("rt-1", "", "plugin-1"),
	}

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err == nil {
		t.Fatal("expected error for user mode output")
	}
	if sink.Calls() != 0 {
		t.Fatalf("sink must NOT execute in user mode, got %d calls", sink.Calls())
	}
}
