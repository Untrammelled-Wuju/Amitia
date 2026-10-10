package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestFreshCandidateSuccessorCannotReplaceChangedCandidateOrActiveBinding(t *testing.T) {
	h := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	t.Cleanup(h.Stop)
	identity, err := h.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	original := &StoredCredential{CredentialID: "original", Credential: "original-secret", SpaceID: "core-b", DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, ExpiresAt: time.Now().Add(time.Hour)}
	replaced := *original
	replaced.CredentialID, replaced.Credential = "replacement", "replacement-secret"
	if err := h.credStore.SaveCandidate(&replaced); err != nil {
		t.Fatal(err)
	}
	version := h.bindingVersion
	if resumed, err := h.resumeProviderBinding(t.Context(), &version, original); !resumed || err == nil {
		t.Fatal("stale follow intent resumed replacement candidate", err)
	}
	_, err = h.BindProvider(t.Context(), BindingRequest{expectedBindingVersion: &version, expectedCandidate: original, CoreID: "core-c"})
	var failure *BindingError
	if !errors.As(err, &failure) || failure.Code != "binding_changed" || h.bindingVersion != version {
		t.Fatal("stale candidate intent changed binding", err)
	}
	if err := h.credStore.SaveCandidate(original); err != nil {
		t.Fatal(err)
	}
	if err := h.credStore.SaveCredential(&replaced); err != nil {
		t.Fatal(err)
	}
	_, err = h.BindProvider(t.Context(), BindingRequest{expectedBindingVersion: &version, expectedCandidate: original, CoreID: "core-c"})
	if !errors.As(err, &failure) || failure.Code != "binding_changed" || h.bindingVersion != version {
		t.Fatal("fresh candidate replaced canonical binding", err)
	}
	active, err := h.credStore.LoadCredential()
	if err != nil || active == nil || active.CredentialID != replaced.CredentialID {
		t.Fatal("rejected handoff cleared existing canonical binding", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := h.FollowSuccessor(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled candidate handoff continued", err)
	}
	_, err = h.BindProvider(ctx, BindingRequest{expectedCandidate: original})
	if !errors.Is(err, context.Canceled) || h.bindingVersion != version {
		t.Fatal("cancelled candidate exchange changed binding", err)
	}
}

func TestFreshCandidateWithoutSuccessorRemainsPaused(t *testing.T) {
	h := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	t.Cleanup(h.Stop)
	h.localCoreID = "core-a"
	identity, err := h.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/device-mesh/v1/coordination/me", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc("/api/device-mesh/v1/provider/successor", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(meshprotocol.WebSocketPath, func(w http.ResponseWriter, r *http.Request) {
		connection, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		var hello protocol.Envelope
		if err := connection.ReadJSON(&hello); err != nil {
			return
		}
		payload, _ := json.Marshal(protocol.HelloAckPayload{Accepted: true, SessionID: "pending-b", ResumeMode: protocol.ResumeModeFresh, ServerTime: time.Now()})
		if err := connection.WriteJSON(protocol.Envelope{Protocol: meshprotocol.ProtocolName, EnvelopeVersion: 1, MessageType: protocol.MessageTypeHelloAck, SpaceID: hello.SpaceID, DeviceID: hello.DeviceID, RuntimeID: hello.RuntimeID, RuntimeSessionID: "pending-b", ConnectionGeneration: 1, Sequence: 1, PayloadSchemaVersion: 1, Payload: payload, PayloadHash: protocol.ComputePayloadHash(payload)}); err != nil {
			return
		}
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	})
	_, endpoint := successorTLSServer(t, "core-b", mux)
	candidate := &StoredCredential{CloudBaseUrl: endpoint.URL, Fingerprint: endpoint.Fingerprint, CredentialID: "pending-b", Credential: "candidate-secret", SpaceID: "core-b", DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := h.credStore.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	if err := h.FollowSuccessor(t.Context()); err == nil {
		t.Fatal("unready candidate with no successor reported success")
	}
	if active, err := h.credStore.LoadCredential(); err != nil || active != nil {
		t.Fatal("candidate with no successor became canonical", err)
	}
	if transition, err := h.credStore.LoadTransition(); err != nil || transition != nil {
		t.Fatal("missing successor created transition", err)
	}
	h.providerMu.Lock()
	paused := h.providerPaused
	h.providerMu.Unlock()
	if !paused {
		t.Fatal("missing successor resumed inference")
	}
	if retained, err := h.credStore.LoadCandidate(); err != nil || !sameCandidateBinding(retained, candidate) {
		t.Fatal("missing successor discarded original candidate", err)
	}
}

func TestFreshCandidateSuccessorRejectsExpiredAndForeignDeviceIdentity(t *testing.T) {
	h := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	t.Cleanup(h.Stop)
	identity, err := h.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []bool{false, true} {
		candidate := &StoredCredential{CredentialID: "unactivated", Credential: "candidate-secret", SpaceID: "core-b", DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, ExpiresAt: time.Now().Add(-time.Minute)}
		if foreign {
			candidate.ExpiresAt = time.Now().Add(time.Hour)
			candidate.DeviceID = "another-device"
		}
		if err := h.credStore.SaveCandidate(candidate); err != nil {
			t.Fatal(err)
		}
		if err := h.FollowSuccessor(t.Context()); err == nil {
			t.Fatal("invalid fresh candidate entered successor admission")
		}
		if active, err := h.credStore.LoadCredential(); err != nil || active != nil {
			t.Fatal("invalid candidate became canonical", err)
		}
		if pending, err := h.credStore.LoadTransition(); err != nil || pending != nil {
			t.Fatal("invalid candidate created transition authority", err)
		}
		retained, err := h.credStore.LoadCandidate()
		if err != nil || !sameCandidateBinding(retained, candidate) {
			t.Fatal("invalid candidate was silently removed or replaced", err)
		}
	}
}
