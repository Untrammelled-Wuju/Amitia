package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestTaskOwnerClientUsesBoundCoreAndRejectsRedirectsOrLateResponses(t *testing.T) {
	for _, scenario := range []string{"valid", "late_connection", "late_authority", "redirect", "denied", "large", "invalid", "off", "unsigned", "insecure", "foreign_scope", "unsupported"} {
		t.Run(scenario, func(t *testing.T) {
			var requests atomic.Int32
			var expired atomic.Bool
			var client *MeshClient
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/api/device-mesh/v1/business/tasks/task/owner-rpc" || r.Header.Get("Authorization") != "AmitiaDevice fixture" || r.Header.Get("X-Test-Signed") != "core" || r.TLS.Version < tls.VersionTLS13 {
					t.Error("owner request escaped its bound transport")
				}
				var request protocol.TaskOwnerRPCRequest
				if json.NewDecoder(r.Body).Decode(&request) != nil || request.AuthorityCallID != "call" || request.TaskGeneration != 2 || request.AttemptID != "attempt" || request.LeaseID != "lease" || request.SessionID != "session" || request.ConnectionGeneration != 4 {
					t.Error("owner request lost execution binding")
				}
				switch scenario {
				case "late_connection":
					client.sessionMu.Lock()
					client.connectionGen++
					client.sessionMu.Unlock()
				case "late_authority":
					expired.Store(true)
				case "redirect":
					w.Header().Set("Location", "/other-core")
					w.WriteHeader(http.StatusTemporaryRedirect)
					return
				case "denied":
					w.WriteHeader(http.StatusConflict)
					return
				case "large":
					_, _ = w.Write([]byte(strings.Repeat("x", (512<<10)+4097)))
					return
				case "invalid":
					_, _ = w.Write([]byte(`{"code":200}`))
					return
				}
				_, _ = w.Write([]byte(`{"code":200,"data":{"value":"core-only"}}`))
			}))
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			config := MeshClientConfig{CloudBaseURL: server.URL, SpaceID: "core", Credential: "fixture", Identity: &LocalIdentity{DeviceID: "device", RuntimeID: "runtime"}, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots}, SignRequest: func(r *http.Request, core string) error { r.Header.Set("X-Test-Signed", core); return nil }}
			if scenario == "unsigned" {
				config.SignRequest = nil
			}
			if scenario == "insecure" {
				config.CloudBaseURL = strings.Replace(server.URL, "https:", "http:", 1)
			}
			client = NewMeshClient(config)
			defer client.Stop()
			client.setState(StateReady)
			client.sessionID, client.connectionGen = "session", 4
			authority := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", TargetDeviceID: "device", InitiatorDeviceID: "caller", ResourceOwnerID: "core", RoleOwnerID: "core", Coordinated: true, RoleID: "role"}
			if scenario == "off" {
				authority.Coordinated = false
			}
			raw, _ := json.Marshal(authority)
			dispatch := protocol.TaskDispatchPayload{TaskRunID: "task", TaskGeneration: 2, AttemptID: "attempt", LeaseID: "lease", AuthorityCallID: "call", OwnedExecutionScope: raw, DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 4}
			if scenario == "foreign_scope" {
				authority.RoleID = "different"
			}
			ctx := coordination.WithAdditionalGuard(coordination.WithScope(t.Context(), authority), func(context.Context) error {
				if expired.Load() {
					return coordination.ErrScopeExpired
				}
				return nil
			})
			method := "task.storage.get"
			if scenario == "unsupported" {
				method = "core.config.set"
			}
			result, err := client.callTaskOwner(ctx, dispatch, "request", method, json.RawMessage(`{"task_run_id":"task","key":"cursor"}`))
			if scenario == "valid" {
				if err != nil || string(result) != `{"value":"core-only"}` || requests.Load() != 1 {
					t.Fatalf("owner response failed: %s %v", result, err)
				}
			} else if err == nil || len(result) != 0 {
				t.Fatalf("invalid owner response accepted: %s %v", result, err)
			}
			if scenario == "off" || scenario == "unsigned" || scenario == "insecure" || scenario == "foreign_scope" || scenario == "unsupported" {
				if requests.Load() != 0 {
					t.Fatal("unauthorized owner operation reached network")
				}
			} else if requests.Load() != 1 {
				t.Fatal("owner client retried or followed a redirect")
			}
		})
	}
}
