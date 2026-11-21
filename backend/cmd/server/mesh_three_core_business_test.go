package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agentplatform"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	meshserver "github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/mcpruntime"
	mcpapi "github.com/u-ai/backend/internal/extension/mcpruntime/api"
	mcpservice "github.com/u-ai/backend/internal/extension/mcpruntime/service"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/spaceidentity"
	"gorm.io/gorm"
)

type threeCoreSchemas struct {
	business, kernel []byte
	roleSpace        string
}

func newThreeCoreSchemas(t *testing.T) *threeCoreSchemas {
	t.Helper()
	port := setupMeshLocalDataPort(t)
	dir := t.TempDir()
	businessPath, kernelPath := filepath.Join(dir, "business-baseline.db"), filepath.Join(dir, "kernel-baseline.db")
	if err := port.services.DB.Exec("VACUUM INTO ?", businessPath).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := port.services.KernelContainer.DeviceRegistry.Database().ExecContext(t.Context(), "VACUUM INTO ?", kernelPath); err != nil {
		t.Fatal(err)
	}
	businessBytes, err := os.ReadFile(businessPath)
	if err != nil {
		t.Fatal(err)
	}
	kernelBytes, err := os.ReadFile(kernelPath)
	if err != nil {
		t.Fatal(err)
	}
	return &threeCoreSchemas{business: businessBytes, kernel: kernelBytes, roleSpace: port.spaceID}
}

func (s *threeCoreSchemas) open(t *testing.T) *AppServices {
	t.Helper()
	dir := t.TempDir()
	businessPath, kernelPath := filepath.Join(dir, "business.db"), filepath.Join(dir, "kernel.db")
	if err := os.WriteFile(businessPath, s.business, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kernelPath, s.kernel, 0600); err != nil {
		t.Fatal(err)
	}
	businessDB, err := gorm.Open(sqlite.Open(businessPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	businessSQL, err := businessDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = businessSQL.Close() })
	kernelDB, err := sql.Open("sqlite", kernelPath)
	if err != nil {
		t.Fatal(err)
	}
	kernelDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = kernelDB.Close() })
	return &AppServices{DB: businessDB, KernelContainer: &kernel.Container{DeviceRegistry: host_registry.NewRegistry(kernelDB)}}
}

type threeCoreModel struct {
	core       string
	started    chan struct{}
	cancelled  chan struct{}
	calls      atomic.Int32
	forwarded  chan *business.ForwardedContext
	roles      chan string
	worldbooks chan []coordination.Resource
}

func (m *threeCoreModel) GenerateOwnedReply(ctx context.Context, input business.Inference) (business.Generation, error) {
	m.calls.Add(1)
	m.roles <- input.Snapshot.Role.Name
	if m.worldbooks != nil {
		select {
		case m.worldbooks <- append([]coordination.Resource(nil), input.Snapshot.Resources...):
		default:
		}
	}
	if input.Message == "等待切换" {
		close(m.started)
		<-ctx.Done()
		close(m.cancelled)
		return business.Generation{}, ctx.Err()
	}
	if input.Context != nil {
		m.forwarded <- input.Context
	}
	return business.Generation{Text: "回复来自 " + m.core, Tokens: 1}, nil
}
func (*threeCoreModel) ExtractOwnedMemory(context.Context, business.Inference, business.Generation) ([]business.DerivedMemory, error) {
	return nil, nil
}

type threeCoreFixture struct {
	core       string
	services   *AppServices
	local      *agent.LocalHandler
	identity   *agent.IdentityStore
	device     *agent.LocalIdentity
	pairing    *pairing.Service
	endpoint   lan.Endpoint
	http       *http.Client
	router     *gin.Engine
	model      *threeCoreModel
	platform   runtimeidentity.Platform
	dataDir    string
	dispatcher interface {
		RegisterCancellable(string, agent.CancellableRuntimeInvokeHandler)
	}
}

// provisionThreeCoreSpaceIdentity writes the fixture Space identity before any
// service opens the data directory. Production derives the core id from the
// Space identity (services.LocalSpaceID = localSpace.SpaceID()), so the mesh
// core, the portable Space and the artifact owner space are one value; the
// fixture must keep the same invariant or attachment ownership cannot resolve.
func provisionThreeCoreSpaceIdentity(t *testing.T, dir, label string) string {
	t.Helper()
	sum := sha256.Sum256([]byte("three-core-space:" + label + "\x00" + dir))
	spaceID := label
	if !strings.HasPrefix(spaceID, "space_") || len(spaceID) < len("space_")+16 {
		spaceID = "space_" + strings.ReplaceAll(label, "-", "_") + "-" + hex.EncodeToString(sum[:8])
	}
	instanceID := "inst_" + hex.EncodeToString(sum[8:16])
	identity := map[string]any{"schemaVersion": 1, "spaceId": spaceID, "instanceId": instanceID, "createdAt": time.Now().UTC()}
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "identity", "space.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return spaceID
}

func newThreeCoreFixture(t *testing.T, core string, schemas *threeCoreSchemas, platforms ...runtimeidentity.Platform) *threeCoreFixture {
	t.Helper()
	platform := runtimeidentity.PlatformWindows
	if len(platforms) == 1 {
		platform = platforms[0]
	}
	sourceServices := schemas.open(t)
	dir := t.TempDir()
	core = provisionThreeCoreSpaceIdentity(t, dir, core)
	identity := agent.NewIdentityStore(dir)
	device, err := identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	ownerSpace, err := spaceidentity.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := spaceidentity.InitializeDefault(dir); err != nil {
		t.Fatal(err)
	}
	if err := sourceServices.DB.Model(&character.Character{}).Where("space_id=?", schemas.roleSpace).Update("space_id", ownerSpace.SpaceID()).Error; err != nil {
		t.Fatal(err)
	}
	for _, model := range []any{&agentplatform.AgentProfile{}, &agentplatform.AgentRevision{}, &agentplatform.AgentRelease{}} {
		if err := sourceServices.DB.Model(model).Where("space_id=?", schemas.roleSpace).Update("space_id", ownerSpace.SpaceID()).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := sourceServices.DB.Model(&character.Character{}).Where("id=?", "one").Update("name", core+" 的角色").Error; err != nil {
		t.Fatal(err)
	}
	registry := sourceServices.KernelContainer.DeviceRegistry
	db := registry.Database()
	if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: runtimeidentity.SpaceID(core), DeviceID: device.DeviceID, Platform: platform, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
		t.Fatal(err)
	}
	rt, err := newMeshCloudRuntime(t, db, registry)
	if err != nil {
		t.Fatal(err)
	}
	local := agent.NewLocalHandler(dir, platform)
	local.SetLocalCoreID(core)
	rt.LocalHandler, rt.LocalDeviceID = local, device.DeviceID.String()
	artifactRuntime, err := BuildArtifactRuntime(sourceServices.DB, filepath.Join(dir, "artifacts", "blobs", "sha256"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := artifactRuntime.Service.SetStorageDeviceID(rt.LocalDeviceID); err != nil {
		t.Fatal(err)
	}
	sourceServices.Artifact = artifactRuntime
	local.SetCredentialObserver(func(credential *agent.StoredCredential) error {
		_, _, err := rt.Coordination.BindProvider(t.Context(), credential.SpaceID.String())
		return err
	})
	corePort, err := newMeshLocalDataPort(sourceServices, dir, core)
	if err != nil {
		t.Fatal(err)
	}
	sourcePort, err := newMeshLocalDataPort(sourceServices, dir, device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	rt.CoreDataPort, rt.LocalDeviceDataPort = corePort, sourcePort
	if _, _, err := rt.Coordination.BindProvider(t.Context(), core); err != nil {
		t.Fatal(err)
	}
	dispatcher := agent.NewRuntimeDispatcher()
	sourceHandler, err := newDeviceOwnedDataHandler(sourceServices, dir)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher.RegisterCancellable("coordination.data", sourceHandler)
	local.SetDispatcher(dispatcher)
	local.SetExecutionGuard(agent.NewOwnedToolGuard(db, dir, sourcePort))
	pending := capability.NewPendingInvocationManager()
	rt.SetPendingInvocations(pending)
	pair, err := pairing.NewService(db, dir, runtimeidentity.SpaceID(core), registry, rt.BootstrapSvc)
	if err != nil {
		t.Fatal(err)
	}
	rt.BusinessCoordinationReady = true
	model := &threeCoreModel{core: core, started: make(chan struct{}), cancelled: make(chan struct{}), forwarded: make(chan *business.ForwardedContext, 4), roles: make(chan string, 8), worldbooks: make(chan []coordination.Resource, 16)}
	services := sourceServices
	services.LocalSpaceID = core
	services.DeviceMesh, services.OwnedBusiness = rt, business.NewEngine(rt.Coordination, rt, model)
	f := &threeCoreFixture{core: core, services: services, local: local, identity: identity, device: device, pairing: pair, router: gin.New(), model: model, platform: platform, dataDir: dir}
	f.dispatcher = dispatcher
	authMW := security.AuthenticationMiddleware(security.AuthConfig{Mode: "network", SpaceID: core, LocalDeviceID: device.DeviceID, DeviceCredentials: rt.CredentialSvc, DeviceRegistry: registry, Coordination: rt.Coordination})
	deps := &meshserver.RouterDeps{DB: db, Sessions: rt.GetSessions(), BootstrapSvc: rt.BootstrapSvc, CredentialSvc: rt.CredentialSvc, Hub: rt.Hub, Handler: rt.Handler, Probe: rt.Probe, DeviceReg: registry, PairingSvc: pair, Coordination: rt.Coordination, BusinessCoordinationReady: true, ProviderPath: local.ProviderPath, LANEndpoints: local.LANEndpoints,
		GetSpaceID: func(c *gin.Context) (runtimeidentity.SpaceID, bool) {
			actor := security.GetActor(c)
			if actor == nil {
				return "", false
			}
			return actor.SpaceID, true
		},
		GetDeviceID: func(c *gin.Context) (runtimeidentity.DeviceID, bool) {
			actor := security.GetActor(c)
			if actor == nil {
				return "", false
			}
			return actor.DeviceID, true
		},
		Successor: func(ctx context.Context, device string) (any, error) {
			result, err := local.PrepareSuccessor(ctx, device)
			if result == nil {
				return nil, err
			}
			return result, err
		},
		InvocationResultHandler: func(reply protocol.RuntimeResultPayload) {
			result := capability.NewToolSuccessResult(reply.InvocationID, "")
			result.RuntimeSessionID, result.Generation = reply.RuntimeSessionID.String(), reply.ConnectionGeneration
			result.DeviceID, result.RuntimeID, result.Structured = reply.DeviceID.String(), reply.RuntimeID.String(), reply.Result
			pending.Complete(reply.InvocationID, result)
		},
		InvocationErrorHandler: func(reply protocol.RuntimeErrorPayload) {
			result := capability.NewToolFailureResult(reply.InvocationID, "", &capability.ToolError{Code: reply.ErrorCode, Message: reply.Message})
			result.RuntimeSessionID, result.Generation = reply.RuntimeSessionID.String(), reply.ConnectionGeneration
			result.DeviceID, result.RuntimeID = reply.DeviceID.String(), reply.RuntimeID.String()
			pending.Fail(reply.InvocationID, result)
		},
		TaskClaimPayloadHandler: func(protocol.TaskClaimPayload) bool { return false }, TaskHeartbeatPayloadHandler: func(protocol.TaskHeartbeatPayload) bool { return false }, TaskProgressPayloadHandler: func(protocol.TaskProgressPayload) {}, TaskCheckpointPayloadHandler: func(protocol.TaskCheckpointPayload) {}, TaskCompletePayloadHandler: func(protocol.TaskCompletePayload) {}, DisconnectHandler: func(string, int64) {},
	}
	if err := meshserver.RegisterCloudRoutes(f.router, authMW, nil, nil, deps); err != nil {
		t.Fatal(err)
	}
	services.MCPRuntime = &mcpservice.Runtime{API: mcpapi.Services{Repository: mcp.NewRepository(services.DB)}}
	registerMeshBusinessRouter(f.router.Group("/api", authMW), services, core)
	local.RegisterRoutes(f.router, func(c *gin.Context) { c.Next() })
	addresses, err := lan.PrivateAddresses()
	if err != nil || len(addresses) == 0 {
		t.Skip("三 Core 联测需要局域网接口")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(addresses[0].String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	tlsKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate, pin, err := lan.Certificate(tlsKey, core, []net.IP{addresses[0]}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(f.router)
	server.Listener.Close()
	server.Listener = listener
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	f.endpoint = lan.Endpoint{URL: server.URL, Fingerprint: pin, CoreID: core}
	local.SetLANEndpoints([]lan.Endpoint{f.endpoint})
	configuration, err := lan.PinnedTLS(f.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = configuration
	transport.Proxy = nil
	f.http = &http.Client{Transport: transport, Timeout: 15 * time.Second}
	t.Cleanup(func() { rt.Stop(); transport.CloseIdleConnections(); server.Close() })
	return f
}

func provisionFixtureSelfCredential(t *testing.T, f *threeCoreFixture) {
	t.Helper()
	identity, err := f.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := credential.GenerateRawCredential()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	stored := &agent.StoredCredential{
		CloudBaseUrl: f.endpoint.URL,
		Fingerprint:  f.endpoint.Fingerprint,
		CredentialID: "fixture-self-" + f.core,
		Credential:   raw,
		SpaceID:      runtimeidentity.SpaceID(f.core),
		DeviceID:     identity.DeviceID,
		RuntimeID:    identity.RuntimeID,
		ExpiresAt:    now.Add(time.Hour),
	}
	db := f.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_identity_keys(device_id,public_key,created_at) VALUES(?,?,?) ON CONFLICT(device_id) DO UPDATE SET public_key=excluded.public_key,revoked=0`, identity.DeviceID, identity.PublicKey, now.Unix()); err != nil {
		t.Fatal(err)
	}
	if err := credential.NewRepository(db).Create(t.Context(), &credential.DeviceRuntimeCredential{
		ID: stored.CredentialID, SpaceID: stored.SpaceID, DeviceID: stored.DeviceID, RuntimeID: stored.RuntimeID,
		CredentialHash: credential.HashRawCredential(raw), Status: credential.CredentialActive,
		CreatedAt: now, LastUsedAt: now, ExpiresAt: stored.ExpiresAt, Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := agent.NewCredentialStore(f.dataDir).SaveCredential(stored); err != nil {
		t.Fatal(err)
	}
}

func (f *threeCoreFixture) request(t *testing.T, caller *threeCoreFixture, method, path string, payload any, expected int, originalManagementHeaders ...map[string]string) []byte {
	t.Helper()
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequestWithContext(t.Context(), method, f.endpoint.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if caller != nil {
		management := method != http.MethodGet && (path == "/api/device-mesh/v1/coordination/me" || strings.HasPrefix(path, "/api/device-mesh/v1/pairing/approvals/") || strings.HasPrefix(path, "/api/device-mesh/v1/devices/") || strings.HasPrefix(path, "/api/device-mesh/v1/business/devices/"))
		if len(originalManagementHeaders) > 0 {
			for key, value := range originalManagementHeaders[0] {
				request.Header.Set(key, value)
			}
		} else if management {
			var state struct {
				CoreID string              `json:"coreId"`
				Policy coordination.Policy `json:"policy"`
			}
			if err := json.Unmarshal(f.request(t, caller, http.MethodGet, "/api/device-mesh/v1/coordination/me", nil, http.StatusOK), &state); err != nil || state.CoreID != f.core || state.Policy.DeviceID != caller.device.DeviceID.String() {
				t.Fatalf("invalid original management authority: %+v %v", state, err)
			}
			request.Header.Set(security.ExpectedCoreHeader, state.CoreID)
			request.Header.Set(security.ExpectedConfigurationPolicyHeader, fmt.Sprintf("%d:%d:%d", state.Policy.ProviderEpoch, state.Policy.ModeRevision, state.Policy.PermissionRevision))
		}
		credential, err := caller.local.LoadCredential()
		if err != nil || credential == nil {
			t.Fatal("缺少当前设备凭证")
		}
		request.Header.Set("Authorization", "AmitiaDevice "+credential.Credential)
		if err := caller.identity.SignRequest(request, f.core); err != nil {
			t.Fatal(err)
		}
	}
	response, err := f.http.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	encoded, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(encoded) > 4<<20 {
		t.Fatal("联测响应无效")
	}
	if response.StatusCode != expected {
		var failure struct {
			Message string `json:"message"`
			Msg     string `json:"msg"`
			Data    any    `json:"data"`
			Code    string `json:"code"`
		}
		_ = json.Unmarshal(encoded, &failure)
		if failure.Message == "" {
			failure.Message = failure.Msg
		}
		skipUnsupportedDeviceIsolationBody(t, failure.Message)
		t.Fatalf("%s %s: status=%d expected=%d code=%s message=%s data=%v", method, path, response.StatusCode, expected, failure.Code, failure.Message, failure.Data)
	}
	return encoded
}

func pairThreeCoreFixtures(t *testing.T, caller, provider *threeCoreFixture) {
	t.Helper()
	_, token, err := provider.pairing.CreateOffer(t.Context(), provider.device.DeviceID, time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	makeClaim := func(expected int) []byte {
		payload, _ := json.Marshal(map[string]any{"endpoint": provider.endpoint, "offerToken": token, "label": caller.core})
		request := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/pairing/claim", bytes.NewReader(payload))
		request.RemoteAddr = "127.0.0.1:40000"
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		caller.router.ServeHTTP(recorder, request)
		if recorder.Code != expected {
			var failure struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(recorder.Body.Bytes(), &failure)
			t.Fatalf("扫码配对状态=%d expected=%d message=%s", recorder.Code, expected, failure.Message)
		}
		return recorder.Body.Bytes()
	}
	makeClaim(http.StatusAccepted)
	approvals, err := provider.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 {
		t.Fatal("初始配对未要求独立审批")
	}
	if err := provider.pairing.DecideApproval(t.Context(), approvals[0].RequestID, approvals[0].Revision, true); err != nil {
		t.Fatal(err)
	}
	var accepted struct {
		Ticket string `json:"ticket"`
	}
	if json.Unmarshal(makeClaim(http.StatusOK), &accepted) != nil || accepted.Ticket == "" {
		t.Fatal("已批准配对未返回有效票据")
	}
	if _, err := caller.local.BindProvider(t.Context(), agent.BindingRequest{CloudBaseURL: provider.endpoint.URL, Fingerprint: provider.endpoint.Fingerprint, CoreID: provider.core, BootstrapTicket: accepted.Ticket}); err != nil {
		t.Fatal(err)
	}
}

func TestThreeCoreSameProviderRepairKeepsCandidateRoleReadiness(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	caller, provider := newThreeCoreFixture(t, "core-a", schemas), newThreeCoreFixture(t, "core-b", schemas)
	pairThreeCoreFixtures(t, caller, provider)
	identity, err := caller.identity.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := credential.GenerateRawCredential()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	stored := &agent.StoredCredential{
		CloudBaseUrl: provider.endpoint.URL,
		Fingerprint:  provider.endpoint.Fingerprint,
		CredentialID: "candidate-" + provider.core,
		Credential:   raw,
		SpaceID:      runtimeidentity.SpaceID(provider.core),
		DeviceID:     identity.DeviceID,
		RuntimeID:    identity.RuntimeID,
		ExpiresAt:    now.Add(time.Hour),
	}
	if err := credential.NewRepository(provider.services.KernelContainer.DeviceRegistry.Database()).Create(t.Context(), &credential.DeviceRuntimeCredential{
		ID: stored.CredentialID, SpaceID: stored.SpaceID, DeviceID: stored.DeviceID, RuntimeID: stored.RuntimeID,
		CredentialHash: credential.HashRawCredential(raw), Status: credential.CredentialActive,
		CreatedAt: now, LastUsedAt: now, ExpiresAt: stored.ExpiresAt, Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := agent.NewCredentialStore(caller.dataDir).SaveCandidate(stored); err != nil {
		t.Fatal(err)
	}
	resumed, err := caller.local.ResumeProviderBinding(t.Context())
	if err != nil || !resumed {
		t.Fatalf("同一中心候选绑定未完成: resumed=%t err=%v", resumed, err)
	}
	var roles struct {
		Roles []coordination.Role `json:"roles"`
	}
	if err := json.Unmarshal(provider.request(t, caller, http.MethodGet, "/api/device-mesh/v1/business/roles", nil, http.StatusOK), &roles); err != nil {
		t.Fatal(err)
	}
	if len(roles.Roles) == 0 {
		t.Fatal("同一中心重新配对后角色目录为空")
	}
}

func TestThreeRealCoreBusinessSwitchPreservesOwnershipAndExpiresAdministrator(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b, c := newThreeCoreFixture(t, "core-a", schemas), newThreeCoreFixture(t, "core-b", schemas), newThreeCoreFixture(t, "core-c", schemas)
	pairThreeCoreFixtures(t, a, b)
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, b, a, policy.ModeRevision)
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	policy, err = b.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), b.core, a.device.DeviceID.String(), policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	var previous struct {
		Data business.Response `json:"data"`
	}
	if json.Unmarshal(b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "before-switch", "characterId": "one", "message": "旧对话"}, http.StatusOK), &previous) != nil || !previous.Data.Saved || previous.Data.Scope.ResourceOwnerID != a.device.DeviceID.String() || !previous.Data.Scope.Coordinated {
		t.Fatal("统筹对话没有保存在设备")
	}
	if name := <-b.model.roles; name != b.core+" 的角色" {
		t.Fatal("统筹模式使用了设备角色而不是 Core 角色")
	}
	oldScope := previous.Data.Scope
	oldScope.ResourceOwnerID = b.core
	oldScope.RoleOwnerID = b.core
	oldScope.RequestID, oldScope.TurnID, oldScope.ExecutionID = "exclusive-b-state", "exclusive-b-turn", "exclusive-b-execution"
	oldContext, endExclusive, err := b.services.DeviceMesh.Coordination.Restore(t.Context(), oldScope, b.services.DeviceMesh)
	if err != nil {
		t.Fatal(err)
	}
	_, err = coordination.NewOwnershipStore(b.services.KernelContainer.DeviceRegistry.Database(), b.core).Apply(oldContext, coordination.Commit{Scope: oldScope, Mutations: []coordination.Mutation{
		{Kind: "memory", ID: "exclusive-memory", RoleID: "one", Body: json.RawMessage(`{"key":"B 的旧记忆","value":"仅保存在 B"}`)},
		{Kind: "summary", ID: "exclusive-summary", RoleID: "one", Body: json.RawMessage(`{"summary":"B 的旧摘要"}`)},
		{Kind: "continuity", ID: "exclusive-continuity", RoleID: "one", Body: json.RawMessage(`{"title":"B 的旧持续事项","status":"paused"}`)},
	}})
	endExclusive()
	if err != nil {
		t.Fatal(err)
	}
	oldDone := make(chan struct{})
	oldRecorder := httptest.NewRecorder()
	oldBody, _ := json.Marshal(map[string]any{"requestId": "interrupt-on-switch", "characterId": "one", "message": "等待切换"})
	go func() {
		defer close(oldDone)
		request := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/provider/api/device-mesh/v1/business/messages", bytes.NewReader(oldBody))
		request.RemoteAddr = "127.0.0.1:40000"
		request.Header.Set("Content-Type", "application/json")
		a.router.ServeHTTP(oldRecorder, request)
	}()
	select {
	case <-b.model.started:
	case <-time.After(10 * time.Second):
		t.Fatal("旧对话未开始")
	}
	pairThreeCoreFixtures(t, b, c)
	select {
	case <-b.model.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("B 连接 C 后旧回复未中断")
	}
	policy, err = b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil || policy.Administrator || !policy.Coordinated {
		t.Fatal("旧管理员未失效或统筹状态丢失")
	}
	if err := a.local.FollowSuccessor(t.Context()); err == nil {
		t.Fatal("未经 C 批准就切换了 A")
	}
	select {
	case <-oldDone:
	case <-time.After(5 * time.Second):
		t.Fatal("A 的旧代理对话仍未结束")
	}
	original, err := a.local.LoadCredential()
	if err != nil || original.SpaceID.String() != b.core {
		t.Fatal("审批前覆盖了原绑定")
	}
	approvals, err := c.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 || approvals[0].DeviceID != a.device.DeviceID.String() {
		t.Fatal("C 未独立受理 A 的配对审批")
	}
	if err := c.pairing.DecideApproval(t.Context(), approvals[0].RequestID, approvals[0].Revision, true); err != nil {
		t.Fatal(err)
	}
	followApprovedThreeCoreSuccessor(t, a, c)
	current, err := a.local.LoadCredential()
	if err != nil || current.SpaceID.String() != c.core || current.DeviceID != a.device.DeviceID || current.PreviousCoreID != b.core || current.ProviderChangeID == "" {
		t.Fatal("新绑定缺少独立身份或持久切换提示")
	}
	bCredential, _ := b.local.LoadCredential()
	if current.Credential == bCredential.Credential || current.CredentialID == bCredential.CredentialID {
		t.Fatal("A 复用了 B 在 C 的凭证")
	}
	nextPolicy, err := c.services.DeviceMesh.Coordination.Get(t.Context(), c.core, a.device.DeviceID.String())
	if err == nil && !nextPolicy.Coordinated {
		ensureCoordinatedForTest(t, c, a, nextPolicy.ModeRevision)
		nextPolicy, err = c.services.DeviceMesh.Coordination.Get(t.Context(), c.core, a.device.DeviceID.String())
	}
	if err != nil || !nextPolicy.Coordinated || nextPolicy.Administrator {
		t.Fatal("C 未保留 A 的统筹模式或错误继承管理员权限")
	}
	oldCount := 0
	deviceDatabase := a.services.KernelContainer.DeviceRegistry.Database()
	if err := deviceDatabase.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND deleted=0`, a.device.DeviceID.String()).Scan(&oldCount); err != nil || oldCount < 2 {
		t.Fatalf("设备专属对话读取失败: count=%d error=%v", oldCount, err)
	}
	migrated := 0
	if err := c.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=?`, b.core).Scan(&migrated); err != nil || migrated != 0 {
		t.Fatal("B 专属数据被迁移到 C")
	}
	retainedExclusive := 0
	if err := b.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND resource_id IN ('exclusive-memory','exclusive-summary','exclusive-continuity') AND deleted=0`, b.core).Scan(&retainedExclusive); err != nil || retainedExclusive != 3 {
		t.Fatal("B 的专属记忆、摘要或持续事项丢失")
	}
	contextMessages := []business.ContextMessage{{ID: "previous-user", Role: "user", Content: "旧对话", OwnerID: a.device.DeviceID.String()}, {ID: "previous-assistant", Role: "assistant", Content: previous.Data.Text, OwnerID: a.device.DeviceID.String()}}
	var resumed struct {
		Data business.Response `json:"data"`
	}
	payload := map[string]any{"requestId": "after-switch", "conversationId": previous.Data.ConversationID, "characterId": "one", "message": "继续对话", "context": business.ForwardedContext{PreviousCoreID: b.core, ConversationID: previous.Data.ConversationID, Summary: "上一轮由 B 提供服务", Messages: contextMessages}}
	if json.Unmarshal(c.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", payload, http.StatusOK), &resumed) != nil || !resumed.Data.Saved || resumed.Data.Scope.CoreID != c.core || resumed.Data.Scope.ResourceOwnerID != a.device.DeviceID.String() || !resumed.Data.Scope.Coordinated || resumed.Data.ConversationID != previous.Data.ConversationID || resumed.Data.Text != "回复来自 "+c.core {
		t.Fatal("新对话未由 C 计算并保存")
	}
	if name := <-c.model.roles; name != c.core+" 的角色" {
		t.Fatal("切换后的统筹对话未使用新 Core 角色")
	}
	select {
	case received := <-c.model.forwarded:
		if received.PreviousCoreID != b.core || len(received.Messages) != 2 || received.Messages[1].Content != previous.Data.Text {
			t.Fatal("当前上下文未传递到 C")
		}
	case <-time.After(time.Second):
		t.Fatal("C 没有收到当前对话上下文")
	}
	retained := 0
	if err := deviceDatabase.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND deleted=0`, a.device.DeviceID.String()).Scan(&retained); err != nil || retained != oldCount+2 {
		t.Fatal("C 继续对话没有写回原设备或修改了其他设备消息")
	}
}

func followApprovedThreeCoreSuccessor(t *testing.T, source, provider *threeCoreFixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var lastErr error
	for {
		lastErr = source.local.FollowSuccessor(ctx)
		credential, err := source.local.LoadCredential()
		if err == nil && credential != nil && credential.SpaceID.String() == provider.core {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("已批准的服务切换未完成: %v", lastErr)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestThreeRealCoreBusinessSwitchKeepsDeviceOwnedConversationWhenCoordinationOff(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b, c := newThreeCoreFixture(t, "device-mode-a", schemas), newThreeCoreFixture(t, "device-mode-b", schemas), newThreeCoreFixture(t, "device-mode-c", schemas)
	pairThreeCoreFixtures(t, a, b)
	var before struct {
		Data business.Response `json:"data"`
	}
	if json.Unmarshal(b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", map[string]any{"requestId": "device-before-switch", "characterId": "one", "message": "设备原有对话"}, http.StatusOK), &before) != nil || !before.Data.Saved || before.Data.Scope.Coordinated || before.Data.Scope.ResourceOwnerID != a.device.DeviceID.String() {
		t.Fatal("非统筹对话未由设备保存")
	}
	if name := <-b.model.roles; name != b.core+" 的角色" {
		t.Fatal("非统筹对话未使用当前 Core 的角色")
	}
	pairThreeCoreFixtures(t, b, c)
	if err := a.local.FollowSuccessor(t.Context()); err == nil {
		t.Fatal("非统筹设备未经新 Core 批准就切换")
	}
	approvals, err := c.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 || approvals[0].DeviceID != a.device.DeviceID.String() {
		t.Fatal("新 Core 没有独立审批非统筹设备")
	}
	if err := c.pairing.DecideApproval(t.Context(), approvals[0].RequestID, approvals[0].Revision, true); err != nil {
		t.Fatal(err)
	}
	followApprovedThreeCoreSuccessor(t, a, c)
	policy, err := c.services.DeviceMesh.Coordination.Get(t.Context(), c.core, a.device.DeviceID.String())
	if err != nil || policy.Coordinated || policy.Administrator {
		t.Fatal("切换错误改变了设备统筹状态或继承了管理员权限")
	}
	var after struct {
		Data business.Response `json:"data"`
	}
	payload := map[string]any{"requestId": "device-after-switch", "conversationId": before.Data.ConversationID, "characterId": "one", "message": "继续原设备对话", "context": business.ForwardedContext{PreviousCoreID: b.core, ConversationID: before.Data.ConversationID, Messages: []business.ContextMessage{{ID: "device-original-user", OwnerID: a.device.DeviceID.String(), Role: "user", Content: "设备原有对话"}, {ID: "device-original-assistant", OwnerID: a.device.DeviceID.String(), Role: "assistant", Content: before.Data.Text}}}}
	if json.Unmarshal(c.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/messages", payload, http.StatusOK), &after) != nil || !after.Data.Saved || after.Data.Scope.CoreID != c.core || after.Data.Scope.Coordinated || after.Data.Scope.ResourceOwnerID != a.device.DeviceID.String() || after.Data.ConversationID != before.Data.ConversationID || after.Data.Text != "回复来自 "+c.core {
		t.Fatal("非统筹切换后没有由 C 计算并回写原设备对话")
	}
	if name := <-c.model.roles; name != c.core+" 的角色" {
		t.Fatal("非统筹切换后没有使用新 Core 的角色")
	}
	for _, provider := range []*threeCoreFixture{b, c} {
		privateCopies := 0
		if err := provider.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND deleted=0`, a.device.DeviceID.String()).Scan(&privateCopies); err != nil || privateCopies != 0 {
			t.Fatal("Core 保存了设备私有对话副本")
		}
	}
	stored := 0
	if err := a.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND deleted=0 AND json_extract(CAST(body AS TEXT),'$.conversationId')=?`, a.device.DeviceID.String(), before.Data.ConversationID).Scan(&stored); err != nil || stored != 4 {
		t.Fatalf("原设备对话未连续保存: count=%d error=%v", stored, err)
	}
}
func TestPeerRoleCatalogFollowsDeviceRoleDomainAndAdministrator(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center, admin, peer := newThreeCoreFixture(t, "role-center", schemas), newThreeCoreFixture(t, "role-admin", schemas), newThreeCoreFixture(t, "role-peer", schemas)
	pairThreeCoreFixtures(t, admin, center)
	pairThreeCoreFixtures(t, peer, center)
	localPath := "/api/device-mesh/v1/business/roles?domain=local&device=" + peer.device.DeviceID.String()
	centerPath := "/api/device-mesh/v1/business/roles?domain=center&device=" + peer.device.DeviceID.String()
	var failure struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(center.request(t, admin, http.MethodGet, localPath, nil, http.StatusForbidden), &failure) != nil || failure.Code != "mesh.administrator_required" {
		t.Fatalf("未授权的跨设备角色目录未被拒绝: code=%s message=%s", failure.Code, failure.Message)
	}
	policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, admin, policy.ModeRevision)
	policy, err = center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := center.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), center.core, admin.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	missingPath := "/api/device-mesh/v1/business/roles?domain=local&device=" + uuid.NewString()
	if json.Unmarshal(center.request(t, admin, http.MethodGet, missingPath, nil, http.StatusNotFound), &failure) != nil || failure.Code != "mesh.device_not_found" {
		t.Fatalf("不存在的设备未返回未接入错误: code=%s message=%s", failure.Code, failure.Message)
	}
	if json.Unmarshal(center.request(t, admin, http.MethodGet, centerPath, nil, http.StatusConflict), &failure) != nil || failure.Code != "mesh.device_not_coordinated" {
		t.Fatalf("本机模式设备被当作统筹角色目录: code=%s message=%s", failure.Code, failure.Message)
	}
	var local struct {
		Roles       []coordination.Role `json:"roles"`
		RoleOwnerID string              `json:"roleOwnerId"`
		Domain      string              `json:"domain"`
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, localPath, nil, http.StatusOK), &local); err != nil || local.RoleOwnerID != peer.device.DeviceID.String() || local.Domain != coordination.RoleDomainLocal || len(local.Roles) == 0 || local.Roles[0].Name != peer.core+" 的角色" {
		t.Fatalf("管理员未读取到目标设备本机角色: roleOwner=%q domain=%q roles=%+v error=%v", local.RoleOwnerID, local.Domain, local.Roles, err)
	}
	policy, err = center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, peer.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, peer, policy.ModeRevision)
	var coordinatedLocal struct {
		Roles       []coordination.Role `json:"roles"`
		RoleOwnerID string              `json:"roleOwnerId"`
		Domain      string              `json:"domain"`
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, localPath, nil, http.StatusOK), &coordinatedLocal); err != nil || coordinatedLocal.RoleOwnerID != peer.device.DeviceID.String() || coordinatedLocal.Domain != coordination.RoleDomainLocal || len(coordinatedLocal.Roles) == 0 || coordinatedLocal.Roles[0].Name != peer.core+" 的角色" {
		t.Fatalf("统筹模式设备的本机角色未对管理员开放: roleOwner=%q domain=%q roles=%+v error=%v", coordinatedLocal.RoleOwnerID, coordinatedLocal.Domain, coordinatedLocal.Roles, err)
	}
	var central struct {
		Roles       []coordination.Role `json:"roles"`
		RoleOwnerID string              `json:"roleOwnerId"`
		Domain      string              `json:"domain"`
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, centerPath, nil, http.StatusOK), &central); err != nil || central.RoleOwnerID != center.core || central.Domain != coordination.RoleDomainCenter || len(central.Roles) == 0 || central.Roles[0].Name != center.core+" 的角色" {
		t.Fatalf("管理员未读取到统筹中心角色: roleOwner=%q domain=%q roles=%+v error=%v", central.RoleOwnerID, central.Domain, central.Roles, err)
	}
}

func TestConsoleMCPRequiresAdministratorAndWritesCenterServers(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center, admin, peer := newThreeCoreFixture(t, "mcp-center", schemas), newThreeCoreFixture(t, "mcp-admin", schemas), newThreeCoreFixture(t, "mcp-peer", schemas)
	pairThreeCoreFixtures(t, admin, center)
	pairThreeCoreFixtures(t, peer, center)
	path := "/api/device-mesh/v1/business/console/mcp/servers"
	var failure struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	created := map[string]any{"name": "center-tools", "displayName": "中心工具", "transport": "streamable_http", "endpoint": "https://mcp.example.com/rpc", "authType": "none", "enabled": false}
	if json.Unmarshal(center.request(t, admin, http.MethodPost, path, created, http.StatusForbidden), &failure) != nil || failure.Code != "mesh.administrator_required" {
		t.Fatalf("未授权的设备写入了统筹中心 MCP: code=%s message=%s", failure.Code, failure.Message)
	}
	policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, admin, policy.ModeRevision)
	policy, err = center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := center.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), center.core, admin.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	var added struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodPost, path, created, http.StatusCreated), &added); err != nil || added.Data.ID == "" || added.Data.Name != "center-tools" {
		t.Fatalf("管理员未能在统筹中心创建 MCP: id=%q name=%q error=%v", added.Data.ID, added.Data.Name, err)
	}
	var listed struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if json.Unmarshal(center.request(t, peer, http.MethodGet, path, nil, http.StatusForbidden), &failure) != nil || failure.Code != "mesh.administrator_required" {
		t.Fatalf("普通设备读取了统筹中心 MCP: code=%s message=%s", failure.Code, failure.Message)
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, path, nil, http.StatusOK), &listed); err != nil || len(listed.Data) != 1 || listed.Data[0].ID != added.Data.ID || listed.Data[0].Name != "center-tools" {
		t.Fatalf("统筹中心 MCP 列表未包含新建服务: %+v error=%v", listed.Data, err)
	}
	isolated := mcp.NewRepository(peer.services.DB)
	if records, err := isolated.ListServers(t.Context()); err != nil || len(records) != 0 {
		t.Fatalf("统筹中心 MCP 泄漏到设备本机库: %+v error=%v", records, err)
	}
}

func TestConsoleSkillsRequireAdministratorAndListCenterSkills(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	center, admin, peer := newThreeCoreFixture(t, "skill-center", schemas), newThreeCoreFixture(t, "skill-admin", schemas), newThreeCoreFixture(t, "skill-peer", schemas)
	pairThreeCoreFixtures(t, admin, center)
	pairThreeCoreFixtures(t, peer, center)
	root := t.TempDir()
	container, err := kernel.NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	kernelRuntime, err := kernel.NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	kernelRuntime.SetContainer(container)
	center.services.Extension = &extension.Runtime{Kernel: kernelRuntime}
	built, err := kernel.BuildLocalAgentSkillPackage(t.Context(), center.core, "center-skill", map[string][]byte{
		"SKILL.md": []byte("---\nname: center-skill\ndescription: Center console skill\nmetadata:\n  version: 1.0.0\nactivation:\n  mode: explicit\n---\n\nCenter console skill.\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := kernelRuntime.PreviewPackage(t.Context(), kernel.PackagePreviewRequest{SpaceID: center.core, ScopeType: "global", FileName: built.FileName, AllowUnsignedLocal: true}, bytes.NewReader(built.Archive))
	if err != nil {
		t.Fatal(err)
	}
	confirmations := map[string]bool{}
	for _, item := range preview.RequiredConfirmations {
		confirmations[item] = true
	}
	confirmation, err := kernelRuntime.ConfirmPackagePreview(t.Context(), kernel.PackagePreviewConfirmationRequest{SessionID: preview.SessionID, SpaceID: center.core, ScopeType: "global", Confirmations: confirmations})
	if err != nil {
		t.Fatal(err)
	}
	installed, err := kernelRuntime.ExecutePackageInstall(t.Context(), kernel.PackageInstallRequest{SessionID: preview.SessionID, SpaceID: center.core, ScopeType: "global", ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "console-agent-skill"})
	if err != nil {
		t.Fatal(err)
	}
	if err := kernelRuntime.Enable(t.Context(), installed.ExtensionID); err != nil {
		t.Fatal(err)
	}
	path := "/api/device-mesh/v1/business/console/skills"
	var failure struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(center.request(t, peer, http.MethodGet, path, nil, http.StatusForbidden), &failure) != nil || failure.Code != "mesh.administrator_required" {
		t.Fatalf("普通设备读取了统筹中心 skill: code=%s message=%s", failure.Code, failure.Message)
	}
	if json.Unmarshal(center.request(t, peer, http.MethodPost, path+"/import/preview", nil, http.StatusForbidden), &failure) != nil || failure.Code != "mesh.administrator_required" {
		t.Fatalf("普通设备向统筹中心导入了 skill: code=%s message=%s", failure.Code, failure.Message)
	}
	policy, err := center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	ensureCoordinatedForTest(t, center, admin, policy.ModeRevision)
	policy, err = center.services.DeviceMesh.Coordination.Get(t.Context(), center.core, admin.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := center.services.DeviceMesh.Coordination.GrantAdministrator(t.Context(), center.core, admin.device.DeviceID.String(), policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	var listed struct {
		Items []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			OwnerSpaceID string `json:"ownerSpaceId"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(center.request(t, admin, http.MethodGet, path, nil, http.StatusOK), &listed); err != nil || listed.Total != 1 || len(listed.Items) != 1 || listed.Items[0].Name != "center-skill" || listed.Items[0].OwnerSpaceID != center.core {
		t.Fatalf("管理员未读取到统筹中心 skill: %+v error=%v", listed, err)
	}
}

