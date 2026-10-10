package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestCandidateCancelledExchangeDoesNotPersistOrReplaceCanonical(t *testing.T) {
	h := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	t.Cleanup(h.Stop)
	h.localCoreID = "core-a"
	identity, err := h.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	original := &StoredCredential{CredentialID: "original", Credential: "original-secret", SpaceID: "original-core", DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := h.credStore.SaveCredential(original); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/public/device-mesh/v1/pairing/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"spaceId": "core-b", "providerPath": []string{"core-b"}})
	})
	mux.HandleFunc("/api/public/device-mesh/v1/bootstrap/exchange", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_ = json.NewEncoder(w).Encode(ExchangeResponse{CredentialID: "candidate", Credential: "candidate-secret", SpaceID: "core-b", DeviceID: identity.DeviceID.String(), RuntimeID: identity.RuntimeID.String(), ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339Nano), Protocol: meshprotocol.ProtocolName, EnvelopeVersion: meshprotocol.EnvelopeVersion, SchemaVersion: meshprotocol.SchemaVersion, WebSocketPath: meshprotocol.WebSocketPath})
	})
	_, endpoint := successorTLSServer(t, "core-b", mux)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := h.BindProvider(ctx, BindingRequest{CloudBaseURL: endpoint.URL, CoreID: endpoint.CoreID, Fingerprint: endpoint.Fingerprint, BootstrapTicket: "independent-ticket"})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("exchange did not start")
	}
	cancel()
	close(release)
	if err := <-done; err == nil {
		t.Fatal("cancelled exchange reported binding success")
	}
	if candidate, err := h.credStore.LoadCandidate(); err != nil || candidate != nil {
		t.Fatal("cancelled exchange persisted candidate", err)
	}
	if active, err := h.credStore.LoadCredential(); err != nil || !sameCandidateBinding(active, original) {
		t.Fatal("cancelled exchange replaced original canonical", err)
	}
}

func TestCandidateCancelledBusinessGateDoesNotPromoteCanonical(t *testing.T) {
	h := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	t.Cleanup(h.Stop)
	identity, err := h.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	original := &StoredCredential{CredentialID: "original", Credential: "original-secret", SpaceID: "original-core", DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := h.credStore.SaveCredential(original); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var policyReads, observed atomic.Int32
	h.SetCredentialObserver(func(*StoredCredential) error { observed.Add(1); return nil })
	mux := http.NewServeMux()
	mux.HandleFunc("/api/device-mesh/v1/coordination/me", func(w http.ResponseWriter, r *http.Request) {
		if policyReads.Add(1) == 2 {
			close(started)
			<-release
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"coreId": "core-b", "coordinationAvailable": true, "aiProvider": "core", "policy": map[string]any{"providerEpoch": 1, "modeRevision": 1, "permissionRevision": 1}})
	})
	mux.HandleFunc("/api/device-mesh/v1/business/roles", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"roleOwnerId": identity.DeviceID.String(), "providerEpoch": 1, "modeRevision": 1, "roles": []any{}})
	})
	mux.HandleFunc(meshprotocol.WebSocketPath, func(w http.ResponseWriter, r *http.Request) {
		connection, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		var hello protocol.Envelope
		if connection.ReadJSON(&hello) != nil {
			return
		}
		payload, _ := json.Marshal(protocol.HelloAckPayload{Accepted: true, SessionID: "candidate", ResumeMode: protocol.ResumeModeFresh, ServerTime: time.Now()})
		if connection.WriteJSON(protocol.Envelope{Protocol: meshprotocol.ProtocolName, EnvelopeVersion: 1, MessageType: protocol.MessageTypeHelloAck, SpaceID: hello.SpaceID, DeviceID: hello.DeviceID, RuntimeID: hello.RuntimeID, RuntimeSessionID: "candidate", ConnectionGeneration: 1, Sequence: 1, PayloadSchemaVersion: 1, Payload: payload, PayloadHash: protocol.ComputePayloadHash(payload)}) != nil {
			return
		}
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	})
	_, endpoint := successorTLSServer(t, "core-b", mux)
	candidate := &StoredCredential{CloudBaseUrl: endpoint.URL, Fingerprint: endpoint.Fingerprint, CredentialID: "candidate", Credential: "candidate-secret", SpaceID: "core-b", DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := h.credStore.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	configuration, err := lan.PinnedTLS(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		_, err := h.activateProvider(ctx, candidate, identity, configuration)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("final business policy validation did not start")
	}
	cancel()
	close(release)
	if err := <-done; err == nil || observed.Load() != 0 {
		t.Fatal("cancelled business gate promoted provider", err)
	}
	if active, err := h.credStore.LoadCredential(); err != nil || !sameCandidateBinding(active, original) {
		t.Fatal("cancelled business gate replaced canonical", err)
	}
	if retained, err := h.credStore.LoadCandidate(); err != nil || !sameCandidateBinding(retained, candidate) {
		t.Fatal("cancelled business gate changed original candidate", err)
	}
}
