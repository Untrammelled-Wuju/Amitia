package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRevokedCredentialHandshakeStopsWithoutRetry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"mesh.credential_revoked","message":"权限已撤销"}`))
	}))
	defer server.Close()
	client := NewMeshClient(MeshClientConfig{CloudBaseURL: server.URL, Credential: "test-only", SpaceID: "core", Identity: &LocalIdentity{DeviceID: "device", RuntimeID: "runtime"}})
	client.Start()
	defer client.Stop()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := client.WaitStopped(ctx); err != nil || client.State() != StateRevoked || attempts.Load() != 1 {
		t.Fatalf("revoked handshake retries: state=%s attempts=%d err=%v", client.State(), attempts.Load(), err)
	}
}
