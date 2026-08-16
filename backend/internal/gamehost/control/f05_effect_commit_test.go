package control

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol/contracts"
)

type CommitCaptureSink struct {
	mu      sync.Mutex
	commits []SinkEffectCommitInput
	errors  []error
	result  contracts.SinkEffectCommitResult
}

type SinkEffectCommitInput struct {
	OutputID   string
	Generation uint64
	Epoch      uint64
	Payload    []byte
}

func NewCommitCaptureSink() *CommitCaptureSink {
	return &CommitCaptureSink{
		result: contracts.SinkEffectCommitResult{
			Accepted:   true,
			Committed:  true,
			EffectID:   "",
			Generation: 1,
		},
	}
}

func (s *CommitCaptureSink) ExecuteAuthorized(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commits = append(s.commits, SinkEffectCommitInput{
		OutputID:   permit.OutputID,
		Generation: permit.Generation,
		Epoch:      permit.OutputEpoch,
		Payload:    payload,
	})
	if !s.result.Accepted {
		return fmt.Errorf("effect not accepted by plugin")
	}
	if !s.result.Committed {
		if s.result.ErrorCode != "" {
			return fmt.Errorf("effect commit failed: %s - %s", s.result.ErrorCode, s.result.Message)
		}
		return fmt.Errorf("effect not committed by plugin")
	}
	if s.result.EffectID == "" {
		return fmt.Errorf("effect commit failed: effectId is required and must not be empty")
	}
	if s.result.EffectID != permit.OutputID {
		return fmt.Errorf("effect commit mismatch: expected %s, got %s", permit.OutputID, s.result.EffectID)
	}
	if s.result.Generation == 0 {
		return fmt.Errorf("effect commit failed: generation is required and must not be zero")
	}
	if s.result.Generation != permit.Generation {
		return fmt.Errorf("effect commit generation mismatch: expected %d, got %d", permit.Generation, s.result.Generation)
	}
	return nil
}

func (s *CommitCaptureSink) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.commits)
}

func (s *CommitCaptureSink) SetResult(result contracts.SinkEffectCommitResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result = result
}

func newF05TestGate(t *testing.T) (*PluginOutputGate, *FakeTopology, *FakeAuthorityReader) {
	topo := NewFakeTopology()
	topo.RegisterRuntime("rt-1", "plugin-1")

	rt := NewFakeRuntimeReader()
	rt.SetActive("rt-1", true)
	rt.SetReady("rt-1", true)

	gen := NewFakeGenerationReader()
	gen.SetGeneration("rt-1", 1)

	auth := NewFakeAuthorityReader()
	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 10)

	gate, err := NewPluginOutputGate(PluginOutputGateOptions{
		Clock:            func() time.Time { return time.Now().UTC() },
		Topology:         topo,
		RuntimeReader:    rt,
		GenerationReader: gen,
		PermChecker:      NewFakeEffPermChecker(),
		PolicyChecker:    NoopHostPolicyChecker{},
		Authority:        auth,
		Audit:            NewInMemoryAuthorityAuditSink(),
		Metrics:          NewFakeMetrics(),
		CommitBarrier:    NoopCommitBarrier{},
	})
	if err != nil {
		t.Fatalf("failed to create gate: %v", err)
	}
	return gate, topo, auth
}

func TestF05_OutputIdRequired_RejectsEmpty(t *testing.T) {
	gate, topology, _ := newF05TestGate(t)
	topology.RegisterService("rt-1", "svc-1")
	registry := NewControlSinkRegistry()
	sink := NewRecordingControlEffectSink()
	if err := registry.RegisterEffect(ControlSinkDescriptor{
		SinkID: "sink-1", RuntimeID: "rt-1", PluginID: "plugin-1", ServiceID: "svc-1", Kind: KindCustomRPC, Generation: 1,
	}, sink); err != nil {
		t.Fatalf("register effect sink: %v", err)
	}
	handler := NewControlHandler(gate, registry)

	payload, _ := json.Marshal(ControlOutputInput{SinkID: "sink-1", ServiceID: "svc-1", Epoch: 10, Payload: json.RawMessage(`{}`)})
	response, err := handler.handleControlOutput(context.Background(), rpc.RPCRequest{
		ID: "request-1", PluginID: "plugin-1", RuntimeID: "rt-1", ServiceID: "svc-1", Generation: 1, Payload: payload,
	})
	if err != nil {
		t.Fatalf("handle control output: %v", err)
	}
	if response.Error == nil {
		t.Fatal("expected error for empty outputId, got nil")
	}
	if response.Error.Code != string(domain.ErrInvalidArgument) {
		t.Fatalf("expected invalid_argument error, got: %s", response.Error.Code)
	}
}

func TestF05_OutputIdRequired_AcceptsValid(t *testing.T) {
	gate, topology, _ := newF05TestGate(t)
	topology.RegisterService("rt-1", "svc-1")
	registry := NewControlSinkRegistry()
	sink := NewRecordingControlEffectSink()
	if err := registry.RegisterEffect(ControlSinkDescriptor{
		SinkID: "sink-1", RuntimeID: "rt-1", PluginID: "plugin-1", ServiceID: "svc-1", Kind: KindCustomRPC, Generation: 1,
	}, sink); err != nil {
		t.Fatalf("register effect sink: %v", err)
	}
	handler := NewControlHandler(gate, registry)

	payload, _ := json.Marshal(ControlOutputInput{OutputID: "output-123", SinkID: "sink-1", ServiceID: "svc-1", Epoch: 10, Payload: json.RawMessage(`{}`)})
	response, err := handler.handleControlOutput(context.Background(), rpc.RPCRequest{
		ID: "request-1", PluginID: "plugin-1", RuntimeID: "rt-1", ServiceID: "svc-1", Generation: 1, Payload: payload,
	})
	if err != nil {
		t.Fatalf("handle control output: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("unexpected error: %v", response.Error)
	}
	var result ControlOutputResult
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.OutputID != "output-123" {
		t.Fatalf("outputId mismatch: expected output-123, got %s", result.OutputID)
	}
}

func TestF05_EffectIDAndGenerationRequired_ContractParity(t *testing.T) {
	goResult := contracts.SinkEffectCommitResult{
		Accepted:   true,
		Committed:  true,
		EffectID:   "effect-1",
		Generation: 5,
	}
	data, err := json.Marshal(goResult)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := parsed["effectId"]; !ok {
		t.Fatal("effectId must be present in JSON (no omitempty)")
	}
	if _, ok := parsed["generation"]; !ok {
		t.Fatal("generation must be present in JSON (no omitempty)")
	}
	if parsed["effectId"] != "effect-1" {
		t.Fatalf("effectId mismatch: %v", parsed["effectId"])
	}
	if uint64(parsed["generation"].(float64)) != 5 {
		t.Fatalf("generation mismatch: %v", parsed["generation"])
	}
}

func TestF05_StrictCommitValidation_EmptyEffectID(t *testing.T) {
	sink := NewCommitCaptureSink()
	sink.SetResult(contracts.SinkEffectCommitResult{
		Accepted:   true,
		Committed:  true,
		EffectID:   "",
		Generation: 1,
	})

	gate, _, _ := newF05TestGate(t)
	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "output-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{}`),
		},
		Peer: TrustedPluginIdentity{
			PluginID:   "plugin-1",
			RuntimeID:  "rt-1",
			Generation: 1,
		},
	}

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err == nil {
		t.Fatal("expected error for empty effectId in commit")
	}
}

func TestF05_StrictCommitValidation_ZeroGeneration(t *testing.T) {
	sink := NewCommitCaptureSink()
	sink.SetResult(contracts.SinkEffectCommitResult{
		Accepted:   true,
		Committed:  true,
		EffectID:   "output-1",
		Generation: 0,
	})

	gate, _, _ := newF05TestGate(t)
	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "output-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{}`),
		},
		Peer: TrustedPluginIdentity{
			PluginID:   "plugin-1",
			RuntimeID:  "rt-1",
			Generation: 1,
		},
	}

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err == nil {
		t.Fatal("expected error for zero generation in commit")
	}
}

func TestF05_StrictCommitValidation_MismatchedEffectID(t *testing.T) {
	sink := NewCommitCaptureSink()
	sink.SetResult(contracts.SinkEffectCommitResult{
		Accepted:   true,
		Committed:  true,
		EffectID:   "wrong-effect-id",
		Generation: 1,
	})

	gate, _, _ := newF05TestGate(t)
	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "output-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{}`),
		},
		Peer: TrustedPluginIdentity{
			PluginID:   "plugin-1",
			RuntimeID:  "rt-1",
			Generation: 1,
		},
	}

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err == nil {
		t.Fatal("expected error for mismatched effectId in commit")
	}
}

func TestF05_StrictCommitValidation_MismatchedGeneration(t *testing.T) {
	sink := NewCommitCaptureSink()
	sink.SetResult(contracts.SinkEffectCommitResult{
		Accepted:   true,
		Committed:  true,
		EffectID:   "output-1",
		Generation: 99,
	})

	gate, _, _ := newF05TestGate(t)
	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "output-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{}`),
		},
		Peer: TrustedPluginIdentity{
			PluginID:   "plugin-1",
			RuntimeID:  "rt-1",
			Generation: 1,
		},
	}

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err == nil {
		t.Fatal("expected error for mismatched generation in commit")
	}
}

func TestF05_FinalRevalidation_StaleGeneration(t *testing.T) {
	sink := NewCommitCaptureSink()
	gate, _, auth := newF05TestGate(t)

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "output-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{}`),
		},
		Peer: TrustedPluginIdentity{
			PluginID:   "plugin-1",
			RuntimeID:  "rt-1",
			Generation: 1,
		},
	}

	auth.SetSnapshot("rt-1", domain.ControlModePluginControl, 99)

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err == nil {
		t.Fatal("expected error for stale epoch during final revalidation")
	}
}

func TestF05_FinalRevalidation_GateClosed(t *testing.T) {
	sink := NewCommitCaptureSink()
	gate, _, _ := newF05TestGate(t)

	req := OutputCheckRequest{
		Intent: ControlOutputIntent{
			OutputID:       "output-1",
			RuntimeID:      "rt-1",
			AuthorityEpoch: 10,
			Kind:           KindCustomRPC,
			Payload:        []byte(`{}`),
		},
		Peer: TrustedPluginIdentity{
			PluginID:   "plugin-1",
			RuntimeID:  "rt-1",
			Generation: 1,
		},
	}

	gate.CloseRuntimeOutputs("rt-1")

	_, err := gate.AuthorizeAndDispatch(context.Background(), req, sink)
	if err == nil {
		t.Fatal("expected error for closed gate during final revalidation")
	}
}

func TestF05_Metadata_PreservedThroughRegistration(t *testing.T) {
	registry := NewControlSinkRegistry()
	metadata := json.RawMessage(`{"game":"rpg","version":"1.0"}`)
	if err := registry.Register(ControlSinkDescriptor{
		SinkID:     "sink-1",
		RuntimeID:  "rt-1",
		PluginID:   "plugin-1",
		ServiceID:  "svc-1",
		Kind:       KindCustomRPC,
		Generation: 1,
		Metadata:   metadata,
	}); err != nil {
		t.Fatalf("register sink: %v", err)
	}

	desc, ok := registry.Resolve("rt-1", "svc-1", "sink-1")
	if !ok {
		t.Fatal("sink not found after registration")
	}
	if string(desc.Metadata) != string(metadata) {
		t.Fatalf("metadata mismatch: expected %s, got %s", metadata, desc.Metadata)
	}
}

func TestF05_SinkEffectDispatchPayload_FullChain(t *testing.T) {
	dispatch := contracts.SinkEffectDispatchPayload{
		SinkID:     "sink-1",
		ServiceID:  "svc-1",
		OutputID:   "output-1",
		Epoch:      10,
		Generation: 5,
		Payload:    json.RawMessage(`{"action":"move"}`),
	}

	data, err := json.Marshal(dispatch)
	if err != nil {
		t.Fatalf("marshal dispatch: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["sinkId"] != "sink-1" {
		t.Fatalf("sinkId mismatch: %v", parsed["sinkId"])
	}
	if parsed["outputId"] != "output-1" {
		t.Fatalf("outputId mismatch: %v", parsed["outputId"])
	}
	if uint64(parsed["generation"].(float64)) != 5 {
		t.Fatalf("generation mismatch: %v", parsed["generation"])
	}
	if uint64(parsed["epoch"].(float64)) != 10 {
		t.Fatalf("epoch mismatch: %v", parsed["epoch"])
	}
}
