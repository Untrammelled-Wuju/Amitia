package rpc

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestF02_SequentialQuotaLeak(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")
	serviceID := domain.ServiceID("test-service")

	for i := 0; i < 10000; i++ {
		req := &PendingRequest{
			Key: RequestKey{
				RuntimeID: runtimeID,
				ServiceID: serviceID,
				RequestID: fmt.Sprintf("req-%d", i),
			},
			Fingerprint: RequestFingerprint(fmt.Sprintf("fp-%d", i)),
		}

		registered, err := registry.Register(req)
		if err != nil {
			t.Fatalf("iteration %d: register failed: %v", i, err)
		}
		if !registered {
			t.Fatalf("iteration %d: should be registered", i)
		}

		ok, _ := registry.Complete(req.Key, protocol.Envelope{})
		if !ok {
			t.Fatalf("iteration %d: complete failed", i)
		}

		registry.Remove(req.Key)
	}

	if registry.Count() != 0 {
		t.Errorf("registry should be empty, got %d", registry.Count())
	}

	finalReq := &PendingRequest{
		Key: RequestKey{
			RuntimeID: runtimeID,
			ServiceID: serviceID,
			RequestID: "final-req",
		},
		Fingerprint: RequestFingerprint("final-fp"),
	}
	registered, err := registry.Register(finalReq)
	if err != nil {
		t.Fatalf("final register failed: %v", err)
	}
	if !registered {
		t.Fatal("final request should be registered")
	}
	registry.Remove(finalReq.Key)
}

func TestF02_ConcurrentTerminalRace(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")
	serviceID := domain.ServiceID("test-service")

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := &PendingRequest{
				Key: RequestKey{
					RuntimeID: runtimeID,
					ServiceID: serviceID,
					RequestID: fmt.Sprintf("req-%d", idx),
				},
				Fingerprint: RequestFingerprint(fmt.Sprintf("fp-%d", idx)),
			}

			registered, err := registry.Register(req)
			if err != nil {
				return
			}
			if !registered {
				return
			}

			switch idx % 4 {
			case 0:
				registry.Complete(req.Key, protocol.Envelope{})
			case 1:
				registry.Fail(req.Key, fmt.Errorf("test failure"))
			case 2:
				registry.Timeout(req.Key)
			case 3:
				registry.Cancel(req.Key)
			}

			registry.Remove(req.Key)
		}(i)
	}

	wg.Wait()

	if registry.Count() != 0 {
		t.Errorf("registry should be empty after concurrent operations, got %d", registry.Count())
	}
}

func TestF02_StaleGenerationResponse(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")
	serviceID := domain.ServiceID("test-service")

	req := &PendingRequest{
		Key: RequestKey{
			RuntimeID: runtimeID,
			ServiceID: serviceID,
			RequestID: "req-1",
		},
		Fingerprint: RequestFingerprint("fp-1"),
		Generation:  RequestGeneration(5),
	}

	registered, err := registry.Register(req)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if !registered {
		t.Fatal("should be registered")
	}

	staleEnvelope := protocol.Envelope{
		Generation: 3,
	}
	ok, _ := registry.Complete(req.Key, staleEnvelope)
	if ok {
		t.Error("stale generation response should be rejected")
	}

	if req.State == RequestStateCompleted {
		t.Error("request should not be completed with stale generation")
	}

	freshEnvelope := protocol.Envelope{
		Generation: 5,
	}
	ok, _ = registry.Complete(req.Key, freshEnvelope)
	if !ok {
		t.Error("matching generation should be accepted")
	}

	if req.State != RequestStateCompleted {
		t.Errorf("request should be completed, got state: %s", req.State)
	}

	registry.Remove(req.Key)
}

func TestF02_CancelByPeer_ExplicitError(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")
	serviceID := domain.ServiceID("test-service")

	for i := 0; i < 5; i++ {
		req := &PendingRequest{
			Key: RequestKey{
				RuntimeID: runtimeID,
				ServiceID: serviceID,
				RequestID: fmt.Sprintf("req-%d", i),
			},
			Fingerprint: RequestFingerprint(fmt.Sprintf("fp-%d", i)),
		}
		registered, err := registry.Register(req)
		if err != nil || !registered {
			t.Fatalf("register failed for req-%d", i)
		}
	}

	count := registry.CancelByPeer(runtimeID, serviceID)
	if count != 5 {
		t.Errorf("expected 5 cancellations, got %d", count)
	}

	registry.Remove(RequestKey{RuntimeID: runtimeID, ServiceID: serviceID, RequestID: "req-0"})
}

func TestF02_CancelByRuntime_ExplicitError(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")

	for i := 0; i < 3; i++ {
		req := &PendingRequest{
			Key: RequestKey{
				RuntimeID: runtimeID,
				ServiceID: domain.ServiceID(fmt.Sprintf("svc-%d", i)),
				RequestID: fmt.Sprintf("req-%d", i),
			},
			Fingerprint: RequestFingerprint(fmt.Sprintf("fp-%d", i)),
		}
		registered, err := registry.Register(req)
		if err != nil || !registered {
			t.Fatalf("register failed for req-%d", i)
		}
	}

	count := registry.CancelByRuntime(runtimeID)
	if count != 3 {
		t.Errorf("expected 3 cancellations, got %d", count)
	}
}

func TestF02_TimeoutSetsExplicitError(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")
	serviceID := domain.ServiceID("test-service")

	req := &PendingRequest{
		Key: RequestKey{
			RuntimeID: runtimeID,
			ServiceID: serviceID,
			RequestID: "req-1",
		},
		Fingerprint: RequestFingerprint("fp-1"),
	}

	registered, err := registry.Register(req)
	if err != nil || !registered {
		t.Fatal("register failed")
	}

	ok, _ := registry.Timeout(req.Key)
	if !ok {
		t.Error("timeout should succeed")
	}

	if req.err == nil {
		t.Error("timeout should set explicit error")
	}

	if req.State != RequestStateTimedOut {
		t.Errorf("state should be timed_out, got %s", req.State)
	}

	registry.Remove(req.Key)
}

func TestF02_CancelSetsExplicitError(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")
	serviceID := domain.ServiceID("test-service")

	req := &PendingRequest{
		Key: RequestKey{
			RuntimeID: runtimeID,
			ServiceID: serviceID,
			RequestID: "req-1",
		},
		Fingerprint: RequestFingerprint("fp-1"),
	}

	registered, err := registry.Register(req)
	if err != nil || !registered {
		t.Fatal("register failed")
	}

	ok, _ := registry.Cancel(req.Key)
	if !ok {
		t.Error("cancel should succeed")
	}

	if req.err == nil {
		t.Error("cancel should set explicit error")
	}

	if req.State != RequestStateCancelled {
		t.Errorf("state should be cancelled, got %s", req.State)
	}

	registry.Remove(req.Key)
}

func TestF02_RegisterIdempotent(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")
	serviceID := domain.ServiceID("test-service")

	req1 := &PendingRequest{
		Key: RequestKey{
			RuntimeID: runtimeID,
			ServiceID: serviceID,
			RequestID: "req-1",
		},
		Fingerprint: RequestFingerprint("fp-1"),
	}

	registered, err := registry.Register(req1)
	if err != nil || !registered {
		t.Fatal("first register should succeed")
	}

	req2 := &PendingRequest{
		Key: RequestKey{
			RuntimeID: runtimeID,
			ServiceID: serviceID,
			RequestID: "req-1",
		},
		Fingerprint: RequestFingerprint("fp-1"),
	}

	registered, err = registry.Register(req2)
	if err != nil {
		t.Fatal("idempotent register should not error")
	}
	if registered {
		t.Error("duplicate request should return registered=false")
	}

	registry.Remove(req1.Key)
}

func TestF02_RegisterDuplicateConflict(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	runtimeID := domain.RuntimeInstanceID("test-runtime")
	serviceID := domain.ServiceID("test-service")

	req1 := &PendingRequest{
		Key: RequestKey{
			RuntimeID: runtimeID,
			ServiceID: serviceID,
			RequestID: "req-1",
		},
		Fingerprint: RequestFingerprint("fp-1"),
	}

	registered, err := registry.Register(req1)
	if err != nil || !registered {
		t.Fatal("first register should succeed")
	}

	req2 := &PendingRequest{
		Key: RequestKey{
			RuntimeID: runtimeID,
			ServiceID: serviceID,
			RequestID: "req-1",
		},
		Fingerprint: RequestFingerprint("fp-different"),
	}

	registered, err = registry.Register(req2)
	if err == nil {
		t.Error("duplicate request with different fingerprint should error")
	}
	if registered {
		t.Error("conflicting request should not be registered")
	}

	registry.Remove(req1.Key)
}

func TestF02_SendRequestTerminalPaths(t *testing.T) {
	correlator := NewRPCResponseCorrelator(NewPendingRequestRegistry(DefaultPendingRegistryConfig()))

	peer := ipc.Peer{
		RuntimeID: "test-runtime",
		ServiceID: "test-service",
	}

	handle, registered := correlator.RegisterPending(peer, "req-1", 1)
	if !registered {
		t.Fatal("register should succeed")
	}

	if handle.DoneCh() == nil {
		t.Fatal("DoneCh should not be nil")
	}

	key := ipc.TerminalKey{
		RuntimeID: "test-runtime",
		ServiceID: "test-service",
		RequestID: "req-1",
	}

	correlator.Terminalize(key, ipc.TerminalTimedOut, nil)

	select {
	case <-handle.DoneCh():
	case <-time.After(time.Second):
		t.Fatal("DoneCh should be closed after terminal state")
	}
}

func TestF02_HandleResponseCorrelator(t *testing.T) {
	registry := NewPendingRequestRegistry(DefaultPendingRegistryConfig())
	correlator := NewRPCResponseCorrelator(registry)

	peer := ipc.Peer{
		RuntimeID: "test-runtime",
		ServiceID: "test-service",
	}

	handle, registered := correlator.RegisterPending(peer, "req-1", 1)
	if !registered {
		t.Fatal("register should succeed")
	}

	respEnvelope := &protocol.Envelope{
		Type:      protocol.MessageTypeResponse,
		RequestID: "req-1",
		Generation: 1,
	}

	correlator.HandleResponse(peer, respEnvelope)

	select {
	case <-handle.DoneCh():
	case <-time.After(time.Second):
		t.Fatal("DoneCh should be closed after response")
	}

	env, err := handle.Result()
	if err != nil {
		t.Errorf("result should not have error: %v", err)
	}
	if env.RequestID != "req-1" {
		t.Errorf("unexpected request ID: %s", env.RequestID)
	}
}
