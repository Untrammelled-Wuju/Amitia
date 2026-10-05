package agent

import (
	"context"
	"encoding/json"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBootstrapExchangeUsesPublicRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/public/device-mesh/v1/bootstrap/exchange" {
			t.Errorf("unexpected exchange endpoint: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "AmitiaBootstrap test-ticket" {
			t.Error("bootstrap authentication missing")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["deviceId"] != "device-a" || body["runtimeId"] != "runtime-a" {
			t.Error("exchange identity mismatch")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ExchangeResponse{CredentialID: "credential-a", DeviceID: "device-a", RuntimeID: "runtime-a", Protocol: meshprotocol.ProtocolName, EnvelopeVersion: meshprotocol.EnvelopeVersion, SchemaVersion: meshprotocol.SchemaVersion, WebSocketPath: meshprotocol.WebSocketPath})
	}))
	defer server.Close()
	result, err := NewBootstrapClient().Exchange(context.Background(), server.URL+"/", "test-ticket", "device-a", "runtime-a", "windows", "test")
	if err != nil {
		t.Fatal(err)
	}
	if result.CredentialID != "credential-a" {
		t.Fatal("exchange did not return credential")
	}
}
