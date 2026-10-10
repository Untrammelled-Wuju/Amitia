package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestStaleSuccessorCannotRestorePreviousBinding(t *testing.T) {
	handler := &LocalHandler{bindingVersion: 2}
	previous := uint64(1)
	_, err := handler.BindProvider(t.Context(), BindingRequest{CoreID: "old-successor", expectedBindingVersion: &previous})
	var failure *BindingError
	if !errors.As(err, &failure) || failure.Status != 409 || failure.Code != "binding_changed" || handler.bindingVersion != 2 {
		t.Fatalf("stale switch accepted: %v", err)
	}
}

func successorTLSServer(t *testing.T, core string, handler http.Handler) (*httptest.Server, lan.Endpoint) {
	t.Helper()
	addresses, err := lan.PrivateAddresses()
	if err != nil || len(addresses) == 0 {
		t.Skip("LAN integration requires a private interface")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(addresses[0].String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate, pin, err := lan.Certificate(key, core, []net.IP{addresses[0]}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener.Close()
	server.Listener = listener
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, lan.Endpoint{URL: server.URL, Fingerprint: pin, CoreID: core}
}

func TestSuccessorUsesIndependentIdentityAndResumesCandidateWithoutCredentialReplay(t *testing.T) {
	a := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	b := NewLocalHandler(t.TempDir(), runtimeidentity.PlatformWindows)
	a.localCoreID = "core-a"
	b.localCoreID = "core-b"
	t.Cleanup(a.Stop)
	t.Cleanup(b.Stop)
	aIdentity, err := a.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	bIdentity, err := b.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	var approved, channelReady, businessReady atomic.Bool
	var exchanges, offers, bindings atomic.Int32
	openCore := func(core string) (*sql.DB, *coordination.Service) {
		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), core+".db"))
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = db.Close() })
		for _, schema := range []string{`CREATE TABLE kernel_devices(device_id TEXT PRIMARY KEY,space_id TEXT NOT NULL,trust_state TEXT NOT NULL,created_at TEXT NOT NULL,last_seen_at TEXT NOT NULL)`, coordination.PolicySchema, coordination.ProviderSchema, coordination.ResourceSchema, coordination.InboxSchema, coordination.RemoteAuthoritySchema, meshaudit.Schema} {
			if _, err := db.ExecContext(t.Context(), schema); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES(?,?,'trusted','now','now')`, aIdentity.DeviceID.String(), core); err != nil {
			t.Fatal(err)
		}
		return db, coordination.NewService(db)
	}
	bDB, bCoordination := openCore("core-b")
	cDB, _ := openCore("core-c")
	if _, _, err := bCoordination.BindProvider(t.Context(), "core-b"); err != nil {
		t.Fatal(err)
	}
	policy, err := bCoordination.ChangeMode(t.Context(), "core-b", aIdentity.DeviceID.String(), 1, true, "role-b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bCoordination.GrantAdministrator(t.Context(), "core-b", aIdentity.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	oldExecution, oldScope, finish, err := bCoordination.Begin(t.Context(), "core-b", aIdentity.DeviceID.String(), "", "core-b", "role-b", "old-reply")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	oldScope.RequestID = "old-memory"
	if _, err := coordination.NewOwnershipStore(bDB, "core-b").Apply(t.Context(), coordination.Commit{Scope: oldScope, Mutations: []coordination.Mutation{{Kind: "memory", ID: "b-memory", RoleID: "role-b", Body: json.RawMessage(`{"content":"只保存在 B 的旧记忆"}`)}}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := bCoordination.BindProvider(t.Context(), "core-c"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(oldExecution), coordination.ErrScopeExpired) {
		t.Fatal("B 连接 C 后旧 Core 执行未失效")
	}
	policy, err = bCoordination.Get(t.Context(), "core-b", aIdentity.DeviceID.String())
	if err != nil || policy.Administrator || !policy.Coordinated {
		t.Fatalf("旧管理员权限未失效或模式被改变: %+v %v", policy, err)
	}
	streamStarted := make(chan struct{})
	streamCancelled := make(chan struct{})
	cMux := http.NewServeMux()
	cMux.HandleFunc("/api/device-mesh/v1/business/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "AmitiaDevice a-on-c" || r.Header.Get("X-Amitia-Device-Proof") == "" {
			t.Error("新回复未使用 A 在 C 的独立身份")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"coreId\":\"core-c\"}\n\n"))
	})
	cMux.HandleFunc("/api/device-mesh/v1/coordination/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "AmitiaDevice a-on-c" || r.Header.Get("X-Amitia-Device-Proof") == "" {
			t.Error("readiness lacks independent proof")
		}
		json.NewEncoder(w).Encode(map[string]any{"coreId": "core-c", "aiProvider": "core", "coordinationAvailable": businessReady.Load(), "policy": map[string]any{"coordinated": true, "providerEpoch": 1, "modeRevision": 1, "permissionRevision": 1}})
	})
	cMux.HandleFunc("/api/device-mesh/v1/business/roles", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"roleOwnerId": "core-c", "providerEpoch": 1, "modeRevision": 1, "roles": []any{}})
	})
	cMux.HandleFunc("/api/public/device-mesh/v1/pairing/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"spaceId": "core-c", "providerPath": []string{"core-c"}})
	})
	cMux.HandleFunc("/api/device-mesh/v1/provider/successor", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "AmitiaDevice a-on-c" || r.Header.Get("X-Amitia-Device-Proof") == "" {
			t.Error("candidate successor lookup lacks A independent proof")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	cMux.HandleFunc("/api/device-mesh/v1/pairing/successor-offers", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "AmitiaDevice b-on-c" || r.Header.Get("X-Amitia-Device-Proof") == "" {
			t.Error("B admission request lacks its own proof")
		}
		var request struct {
			DeviceID    string `json:"deviceId"`
			Coordinated bool   `json:"coordinated"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.DeviceID != aIdentity.DeviceID.String() || !request.Coordinated {
			t.Error("successor target or mode changed")
		}
		offers.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"offerToken": "successor-offer", "deviceId": request.DeviceID, "approvalRequired": true, "expiresAt": time.Now().Add(time.Minute)})
	})
	cMux.HandleFunc("/api/public/device-mesh/v1/pairing/claim", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("B credential disclosed to A claim")
		}
		var claim struct {
			proof.ClaimBody
			Proof proof.Proof `json:"proof"`
		}
		if json.NewDecoder(r.Body).Decode(&claim) != nil || claim.DeviceID != aIdentity.DeviceID.String() || claim.Proof.PublicKey != aIdentity.PublicKey || proof.Verify(claim.Proof, "core-c", claim.ClaimBody, time.Now()) != nil {
			t.Error("A did not prove independent identity")
		}
		if !approved.Load() {
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]bool{"pending": true})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"ticket": "a-ticket", "spaceId": "core-c", "deviceId": aIdentity.DeviceID.String()})
	})
	cMux.HandleFunc("/api/public/device-mesh/v1/bootstrap/exchange", func(w http.ResponseWriter, r *http.Request) {
		if !approved.Load() || r.Header.Get("Authorization") != "AmitiaBootstrap a-ticket" || r.Header.Get("X-Amitia-Device-Proof") == "" {
			t.Error("bootstrap bypassed independent admission")
		}
		exchanges.Add(1)
		json.NewEncoder(w).Encode(ExchangeResponse{CredentialID: "a-credential", Credential: "a-on-c", SpaceID: "core-c", DeviceID: aIdentity.DeviceID.String(), RuntimeID: aIdentity.RuntimeID.String(), ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339Nano), Protocol: meshprotocol.ProtocolName, EnvelopeVersion: meshprotocol.EnvelopeVersion, SchemaVersion: meshprotocol.SchemaVersion, WebSocketPath: meshprotocol.WebSocketPath})
	})
	cMux.HandleFunc(meshprotocol.WebSocketPath, func(w http.ResponseWriter, r *http.Request) {
		if !channelReady.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("Authorization") != "AmitiaDevice a-on-c" {
			t.Error("A channel uses another device credential")
		}
		connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		var hello protocol.Envelope
		if connection.ReadJSON(&hello) != nil {
			return
		}
		payload, _ := json.Marshal(protocol.HelloAckPayload{Accepted: true, SessionID: "a-on-c-session", ResumeMode: protocol.ResumeModeFresh, ServerTime: time.Now()})
		if connection.WriteJSON(protocol.Envelope{Protocol: meshprotocol.ProtocolName, EnvelopeVersion: 1, MessageType: protocol.MessageTypeHelloAck, SpaceID: hello.SpaceID, DeviceID: hello.DeviceID, RuntimeID: hello.RuntimeID, RuntimeSessionID: "a-on-c-session", ConnectionGeneration: 1, Sequence: 1, PayloadSchemaVersion: 1, Payload: payload, PayloadHash: protocol.ComputePayloadHash(payload)}) != nil {
			return
		}
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	})
	_, cEndpoint := successorTLSServer(t, "core-c", cMux)
	if err := b.credStore.SaveCredential(&StoredCredential{CloudBaseUrl: cEndpoint.URL, Fingerprint: cEndpoint.Fingerprint, SpaceID: "core-c", Credential: "b-on-c", DeviceID: bIdentity.DeviceID, RuntimeID: bIdentity.RuntimeID, ExpiresAt: time.Now().Add(time.Hour), ProviderPath: []string{"core-c"}}); err != nil {
		t.Fatal(err)
	}
	_, bEndpoint := successorTLSServer(t, "core-b", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/device-mesh/v1/business/messages" {
			if r.Header.Get("Authorization") != "AmitiaDevice a-on-b" || r.Header.Get("X-Amitia-Device-Proof") == "" {
				t.Error("旧回复身份无效")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"coreId\":\"core-b\"}\n\n"))
			w.(http.Flusher).Flush()
			close(streamStarted)
			select {
			case <-r.Context().Done():
				close(streamCancelled)
			case <-t.Context().Done():
			}
			return
		}
		if r.URL.Path != "/api/device-mesh/v1/provider/successor" || r.Header.Get("Authorization") != "AmitiaDevice a-on-b" || r.Header.Get("X-Amitia-Device-Proof") == "" {
			t.Error("A predecessor request identity changed")
		}
		result, err := b.PrepareSuccessor(r.Context(), aIdentity.DeviceID.String(), true)
		if err != nil {
			http.Error(w, err.Error(), 503)
			return
		}
		json.NewEncoder(w).Encode(result)
	}))
	if err := a.credStore.SaveCredential(&StoredCredential{CloudBaseUrl: bEndpoint.URL, Fingerprint: bEndpoint.Fingerprint, SpaceID: "core-b", Credential: "a-on-b", DeviceID: aIdentity.DeviceID, RuntimeID: aIdentity.RuntimeID, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	a.SetCredentialObserver(func(credential *StoredCredential) error {
		if credential.SpaceID != "core-c" {
			t.Error("wrong canonical provider")
		}
		bindings.Add(1)
		return nil
	})
	proxyRouter := gin.New()
	proxyRouter.POST("/internal/device-mesh/provider/*path", a.handleProviderProxy)
	oldResponse := httptest.NewRecorder()
	oldRequest := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/provider/api/device-mesh/v1/business/messages", nil)
	oldRequest.RemoteAddr = "127.0.0.1:40000"
	proxyFinished := make(chan struct{})
	go func() {
		defer close(proxyFinished)
		proxyRouter.ServeHTTP(oldResponse, oldRequest)
	}()
	select {
	case <-streamStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("原 Core 回复未建立")
	}
	if err := a.FollowSuccessor(t.Context()); err == nil || exchanges.Load() != 0 || bindings.Load() != 0 {
		t.Fatal("pending approval activated provider")
	}
	select {
	case <-streamCancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("等待新 Core 批准时仍继续旧回复")
	}
	select {
	case <-proxyFinished:
	case <-time.After(3 * time.Second):
		t.Fatal("切换后本机 SSE 未中断")
	}
	old, _ := a.credStore.LoadCredential()
	if old.SpaceID != "core-b" || a.pendingCore != "core-c" || !a.providerPaused {
		t.Fatal("pending handoff did not pause original provider")
	}
	restarted := NewLocalHandler(a.dataDir, runtimeidentity.PlatformWindows)
	defer restarted.Stop()
	if restarted.pendingCore != "core-c" || !restarted.providerPaused {
		t.Fatal("restart lost pending Core or resumed old provider")
	}
	approved.Store(true)
	call, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	err = a.FollowSuccessor(call)
	cancel()
	if err == nil {
		t.Fatal("unready channel activated provider")
	}
	candidate, err := a.credStore.LoadCandidate()
	if err != nil || candidate == nil || candidate.SpaceID != "core-c" {
		t.Fatal("failed cutover lost exchanged candidate")
	}
	old, _ = a.credStore.LoadCredential()
	if old.SpaceID != "core-b" || bindings.Load() != 0 {
		t.Fatal("failed channel changed authoritative provider")
	}
	channelReady.Store(true)
	call, cancel = context.WithTimeout(t.Context(), 3*time.Second)
	err = a.FollowSuccessor(call)
	cancel()
	var failure *BindingError
	if !errors.As(err, &failure) || failure.Code != "provider_business_not_ready" {
		t.Fatalf("unready business became canonical: %v", err)
	}
	old, _ = a.credStore.LoadCredential()
	if old.SpaceID != "core-b" || bindings.Load() != 0 || exchanges.Load() != 1 || !a.providerPaused {
		t.Fatal("business readiness failure changed provider or repeated credential exchange")
	}
	businessReady.Store(true)
	call, cancel = context.WithTimeout(t.Context(), 3*time.Second)
	err = a.FollowSuccessor(call)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	current, _ := a.credStore.LoadCredential()
	if current.SpaceID != "core-c" || current.Credential != "a-on-c" || bindings.Load() != 1 || exchanges.Load() != 1 || offers.Load() != 1 || a.providerPaused {
		t.Fatal("candidate resume repeated admission or did not activate direct C connection")
	}
	if current.PreviousCoreID != "core-b" || current.ProviderChangeID == "" {
		t.Fatal("切换未持久保存原提供者提示")
	}
	statusResponse := httptest.NewRecorder()
	statusContext, _ := gin.CreateTestContext(statusResponse)
	a.handleStatus(statusContext)
	var status map[string]any
	if json.Unmarshal(statusResponse.Body.Bytes(), &status) != nil || status["previousCoreId"] != "core-b" || status["coreId"] != "core-c" || status["providerChangeId"] != current.ProviderChangeID {
		t.Fatal("设备状态未返回可恢复的切换提示")
	}
	if restored, err := NewCredentialStore(a.dataDir).LoadCredential(); err != nil || restored.PreviousCoreID != "core-b" || restored.ProviderChangeID != current.ProviderChangeID {
		t.Fatal("重新读取设备绑定丢失了切换提示")
	}
	if candidate, err := a.credStore.LoadCandidate(); err != nil || candidate != nil {
		t.Fatal("successful cutover retained candidate")
	}
	if pending, err := a.credStore.LoadTransition(); err != nil || pending != nil {
		t.Fatal("completed switch retained approval record", err)
	}
	newResponse := httptest.NewRecorder()
	newRequest := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/provider/api/device-mesh/v1/business/messages", nil)
	newRequest.RemoteAddr = "127.0.0.1:40000"
	proxyRouter.ServeHTTP(newResponse, newRequest)
	if newResponse.Code != http.StatusOK || newResponse.Body.String() != "data: {\"coreId\":\"core-c\"}\n\n" {
		t.Fatalf("新回复未直接由 C 提供: %d %s", newResponse.Code, newResponse.Body.String())
	}
	oldMemory, err := coordination.NewOwnershipStore(bDB, "core-b").List(t.Context(), "memory", "role-b", false)
	if err != nil || len(oldMemory) != 1 || oldMemory[0].ID != "b-memory" {
		t.Fatalf("B 的云端专属数据被删除或改变: %+v %v", oldMemory, err)
	}
	var migrated int
	if err := cDB.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources`).Scan(&migrated); err != nil || migrated != 0 {
		t.Fatalf("B 的云端专属数据被迁移到 C: %d %v", migrated, err)
	}
}
