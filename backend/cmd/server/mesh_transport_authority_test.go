package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/spaceidentity"
)

func TestOwnedAuthorityAcrossRealTLSMeshReconnectAndPermissionChange(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	dir := t.TempDir()
	identityStore := agent.NewIdentityStore(dir)
	identity, err := identityStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	space, err := spaceidentity.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Model(&character.Character{}).Where("space_id=?", p.legacySpaceID).Update("space_id", space.SpaceID()).Error; err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "core.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	registry := host_registry.NewRegistry(db)
	for _, device := range []runtimeidentity.DeviceID{"caller", identity.DeviceID} {
		if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: device, Platform: runtimeidentity.PlatformWindows, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
			t.Fatal(err)
		}
	}
	rt, err := devicemesh.NewCloudRuntime(db, registry)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := credential.GenerateRawCredential()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stored := &agent.StoredCredential{SpaceID: "core", DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, CredentialID: "transport-test", Credential: raw, ExpiresAt: now.Add(time.Hour)}
	if err := agent.NewCredentialStore(dir).SaveCredential(stored); err != nil {
		t.Fatal(err)
	}
	if err := credential.NewRepository(db).Create(t.Context(), &credential.DeviceRuntimeCredential{ID: stored.CredentialID, SpaceID: stored.SpaceID, DeviceID: stored.DeviceID, RuntimeID: stored.RuntimeID, CredentialHash: credential.HashRawCredential(raw), Status: credential.CredentialActive, CreatedAt: now, LastUsedAt: now, ExpiresAt: stored.ExpiresAt, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO kernel_device_identity_keys(device_id,public_key,created_at) VALUES(?,?,?)`, identity.DeviceID, identity.PublicKey, now.Unix()); err != nil {
		t.Fatal(err)
	}
	pending := capability.NewPendingInvocationManager()
	rt.SetPendingInvocations(pending)
	rt.Handler.SetOnInvocationResult(func(result protocol.RuntimeResultPayload) {
		unified := capability.NewToolSuccessResult(result.InvocationID, "")
		unified.RuntimeSessionID, unified.Generation = result.RuntimeSessionID.String(), result.ConnectionGeneration
		unified.DeviceID, unified.RuntimeID, unified.Structured = result.DeviceID.String(), result.RuntimeID.String(), result.Result
		pending.Complete(result.InvocationID, unified)
	})
	rt.Handler.SetOnInvocationError(func(result protocol.RuntimeErrorPayload) {
		unified := capability.NewToolFailureResult(result.InvocationID, "", &capability.ToolError{Code: result.ErrorCode, Message: result.Message, Retryable: result.Retryable})
		unified.RuntimeSessionID, unified.Generation = result.RuntimeSessionID.String(), result.ConnectionGeneration
		unified.DeviceID, unified.RuntimeID = result.DeviceID.String(), result.RuntimeID.String()
		pending.Fail(result.InvocationID, unified)
	})
	router := gin.New()
	router.GET(meshprotocol.WebSocketPath, credential.DeviceAuthMiddleware(rt.CredentialSvc, registry), rt.Handler.HandleWS)
	router.GET("/authority-probe", credential.DeviceAuthMiddleware(rt.CredentialSvc, registry), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	ownerServices := &AppServices{DeviceMesh: rt, KernelContainer: &kernel.Container{}}
	registerMeshTaskOwnerRouter(router.Group("/api/device-mesh/v1/business", security.AuthenticationMiddleware(security.AuthConfig{Mode: "network", SpaceID: "core", DeviceCredentials: rt.CredentialSvc, DeviceRegistry: registry, Coordination: rt.Coordination})), ownerServices)
	server := httptest.NewTLSServer(router)
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	probe := func(sign bool, expected int) *http.Request {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/authority-probe", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "AmitiaDevice "+raw)
		if sign {
			if err := identityStore.SignRequest(request, "core"); err != nil {
				t.Fatal(err)
			}
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != expected {
			t.Fatalf("proof authorization status=%d want=%d", response.StatusCode, expected)
		}
		return request
	}
	probe(false, http.StatusUnauthorized)
	firstProof := probe(true, http.StatusNoContent)
	replayed, err := server.Client().Do(firstProof.Clone(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	replayed.Body.Close()
	if replayed.StatusCode != http.StatusUnauthorized {
		t.Fatal("signed request replay accepted")
	}
	handler, err := newDeviceOwnedDataHandler(p.services, dir)
	if err != nil {
		t.Fatal(err)
	}
	sourcePort, err := newMeshLocalDataPort(p.services, dir, identity.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	nativeStarted, nativeStopped := make(chan struct{}), make(chan struct{})
	newClient := func() *agent.MeshClient {
		dispatcher := agent.NewRuntimeDispatcher()
		dispatcher.RegisterCancellable("coordination.data", handler)
		dispatcher.RegisterCancellable("test.native", func(ctx context.Context, _ protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
			close(nativeStarted)
			<-ctx.Done()
			close(nativeStopped)
			return nil, ctx.Err()
		})
		sourceDB := p.services.KernelContainer.DeviceRegistry.Database()
		client := agent.NewMeshClient(agent.MeshClientConfig{CloudBaseURL: server.URL, Credential: raw, SpaceID: "core", Identity: identity, SignRequest: identityStore.SignRequest, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots}, RuntimeDispatcher: dispatcher, ExecutionJournal: executionjournal.NewStore(sourceDB), ExecutionGuard: agent.NewOwnedToolGuard(sourceDB, dir, sourcePort)})
		client.SetCredentialStore(agent.NewCredentialStore(dir))
		client.Start()
		t.Cleanup(client.Stop)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		if err := client.WaitReady(ctx); err != nil {
			t.Fatal(err)
		}
		return client
	}
	client := newClient()
	ctx, scope, finish, err := rt.Coordination.Begin(t.Context(), "core", "caller", identity.DeviceID.String(), "core", "one", "transport-input")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	scope.RoleRevision = 3
	scope.TurnID, scope.ExecutionID = "transport-turn", "transport-execution"
	ctx = coordination.WithScope(ctx, scope)
	binding := &task_runtime.OwnedRuntimeBinding{}
	if err := bindCoreOwnedTaskRuntime(binding, rt, "core"); err != nil {
		t.Fatal(err)
	}
	taskConfig := task_runtime.DefaultTaskRuntimeConfig()
	binding.Apply(&taskConfig)
	otherCore := scope
	otherCore.CoreID, otherCore.SpaceID, otherCore.AuthorizationRealm = "other-core", "other-core", "other-core"
	if _, _, err := taskConfig.OwnedExecutionGuard(t.Context(), otherCore, nil); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("本机 Core 恢复了其他 Core 的任务授权: %v", err)
	}
	taskContext, taskFinish, err := taskConfig.OwnedExecutionGuard(t.Context(), scope, nil)
	if err != nil {
		t.Fatalf("实际 TLS 通道不能恢复设备任务授权: %v", err)
	}
	defer taskFinish()
	if err := coordination.ValidateCurrent(taskContext); err != nil {
		t.Fatalf("实际 TLS 通道角色复核失败: %v", err)
	}
	taskInput := json.RawMessage(`{"private":"task-input-only-at-source"}`)
	taskHash := sha256.Sum256(taskInput)
	sourceTaskRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceTaskRoot, "task.cjs"), []byte("module.exports=async()=>({success:true});"), 0600); err != nil {
		t.Fatal(err)
	}
	sourceDefinition := &task_runtime.TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module", Entry: "task.cjs", InstalledGeneration: 2}
	if err := task_runtime.PinTaskEntry(t.Context(), sourceTaskRoot, sourceDefinition); err != nil {
		t.Fatal(err)
	}
	sourceTaskConfig := task_runtime.DefaultTaskRuntimeConfig()
	sourceGeneration := int64(2)
	sourceTaskConfig.InstalledDefinitionValidator = func(_ context.Context, definition *task_runtime.TaskDefinition) error {
		if definition.InstalledGeneration != sourceGeneration {
			return coordination.ErrScopeExpired
		}
		return nil
	}
	sourceTaskConfig.EntryResolver = func(ctx context.Context, definition *task_runtime.TaskDefinition) (string, error) {
		return task_runtime.ResolveTaskEntry(ctx, sourceTaskRoot, definition)
	}
	sourceTaskService := task_runtime.NewTaskRuntimeService(kernelsqlite.NewTaskRepository(p.services.KernelContainer.DeviceRegistry.Database()), sourceTaskConfig)
	p.services.KernelContainer.TaskRuntimeService = sourceTaskService
	if err := sourceTaskService.PutTaskDefinition(t.Context(), sourceDefinition); err != nil {
		t.Fatal(err)
	}
	coreDefinition := *sourceDefinition
	coreDefinition.InstalledGeneration, coreDefinition.DefinitionHash = 7, "core-local-task-declaration"
	coreEncoded, err := json.Marshal(coreDefinition)
	if err != nil {
		t.Fatal(err)
	}
	coreFingerprint := sha256.Sum256(coreEncoded)
	taskRun := &task_runtime.TaskRun{TaskRunID: "transport-owned-task", TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module", ScopeSnapshotID: "transport-task-snapshot", DefinitionFingerprint: hex.EncodeToString(coreFingerprint[:]), InputHash: hex.EncodeToString(taskHash[:]), Input: taskInput}
	if err := taskConfig.OwnedInputs.SaveInput(taskContext, taskRun); err != nil {
		t.Fatalf("实际 TLS 设备任务输入未获所有者确认: %v", err)
	}
	taskRun.Input = nil
	if input, err := taskConfig.OwnedInputs.Input(taskContext, taskRun); err != nil || string(input) != string(taskInput) {
		t.Fatalf("实际 TLS 设备任务输入读取失败: %v", err)
	}
	targetPin, err := taskConfig.OwnedTargetDefinitions.Prepare(taskContext, taskRun, &coreDefinition)
	if err != nil || targetPin.DeviceID != identity.DeviceID.String() || targetPin.InstalledGeneration != 2 {
		t.Fatalf("实际 TLS 目标插件版本未确认: %+v %v", targetPin, err)
	}
	if _, err := taskConfig.OwnedTargetDefinitions.Prepare(taskContext, taskRun, &coreDefinition); err != nil {
		t.Fatalf("确认版本不能重读: %v", err)
	}
	sourceGeneration++
	sourceDefinition.InstalledGeneration = sourceGeneration
	if err := sourceTaskService.PutTaskDefinition(t.Context(), sourceDefinition); err != nil {
		t.Fatal(err)
	}
	if _, err := taskConfig.OwnedTargetDefinitions.Prepare(taskContext, taskRun, &coreDefinition); !task_runtime.IsTaskErrorCode(err, task_runtime.ErrTaskDefinitionInvalid) {
		t.Fatalf("实际 TLS 目标升级仍允许旧任务重派: %v", err)
	}
	commit := coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "message", ID: "transport-message", RoleID: "one", Body: json.RawMessage(`{"id":"transport-message","conversationId":"transport-conversation","role":"user","content":"source only"}`)}}}
	if _, err := rt.Commit(ctx, commit); err != nil {
		t.Fatal(err)
	}
	var mirrored int
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources`).Scan(&mirrored); err != nil || mirrored != 0 {
		t.Fatalf("Core mirrored source data: %d %v", mirrored, err)
	}
	conflicting := commit
	conflicting.Mutations = []coordination.Mutation{{Kind: "message", ID: "transport-message", RoleID: "one", Body: json.RawMessage(`{"id":"transport-message","conversationId":"transport-conversation","role":"user","content":"changed same request"}`)}}
	if _, err := rt.Commit(ctx, conflicting); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("source permanent rejection lost its error identity: %v", err)
	}
	if sources, err := rt.Coordination.PendingRemoteSources(t.Context()); err != nil || len(sources) != 0 {
		t.Fatalf("acknowledged rejection kept unknown authority: %+v %v", sources, err)
	}
	connection, ok := rt.Hub.GetByDevice("core", identity.DeviceID)
	if !ok {
		t.Fatal("missing live source")
	}
	lostCall, err := rt.Coordination.TrackRemoteAuthority(t.Context(), scope, connection.SessionID.String(), connection.Generation)
	if err != nil {
		t.Fatal(err)
	}
	client.Stop()
	wait, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	if err := client.WaitStopped(wait); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	client = newClient()
	if err := rt.ReconcileRemoteDevice(t.Context(), "core", identity.DeviceID.String()); err != nil {
		t.Fatal(err)
	}
	if err := coordination.ValidateSourceCall(t.Context(), p.services.KernelContainer.DeviceRegistry.Database(), "core", lostCall); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("unknown old connection action stayed usable: %v", err)
	}
	nativePort := capability.NewMeshDeviceRuntimeInvocationPort(&capability.MeshRuntimePorts{Hub: rt.Hub, PendingInvocations: pending})
	nativeResult := make(chan capability.UnifiedToolResult, 1)
	route := capability.RuntimeExecutionRoute{Placement: capability.ProviderPlacementDevice, SpaceID: "core", DeviceID: identity.DeviceID, RuntimeID: identity.RuntimeID, RemoteDevice: true, Binding: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeInternal, HandlerName: "test.native"}}
	go func() {
		nativeResult <- nativePort.Execute(ctx, capability.DeviceRuntimeInvocationRequest{Route: route, Binding: route.Binding, Invocation: capability.ToolInvocationContext{InvocationID: "native-action", IdempotencyKey: "native-action", SpaceID: "core", DeadlineDuration: 10 * time.Second}, Input: json.RawMessage(`{}`)})
	}()
	nativeWait, endNativeWait := context.WithTimeout(t.Context(), 5*time.Second)
	defer endNativeWait()
	select {
	case <-nativeStarted:
	case <-nativeWait.Done():
		t.Fatal("native scoped action did not start")
	}
	policy, err := rt.Coordination.ChangeMode(nativeWait, "core", "caller", 1, true, "one")
	if err != nil || policy.PermissionRevision <= scope.PermissionRevision {
		t.Fatalf("live source did not acknowledge authority barrier: %+v %v", policy, err)
	}
	if !errors.Is(context.Cause(ctx), coordination.ErrScopeExpired) {
		t.Fatal("old caller execution was not interrupted")
	}
	if !errors.Is(context.Cause(taskContext), coordination.ErrScopeExpired) {
		t.Fatal("权限切换未中断恢复后的设备任务授权")
	}
	if _, err := taskConfig.OwnedInputs.Input(taskContext, taskRun); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("权限切换后旧任务仍能读取设备正文: %v", err)
	}
	select {
	case <-nativeStopped:
	case <-nativeWait.Done():
		t.Fatal("owner fence completed before active native work stopped")
	}
	select {
	case result := <-nativeResult:
		if result.Status == capability.ToolResultStatusSuccess {
			t.Fatal("cancelled native action reported success")
		}
	case <-nativeWait.Done():
		t.Fatal("native caller did not receive cancellation")
	}
	if err := coordination.ValidateSourceAuthority(t.Context(), p.services.KernelContainer.DeviceRegistry.Database(), scope); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("delayed old permission passed source guard: %v", err)
	}
	freshCtx, fresh, endFresh, err := rt.Coordination.Begin(t.Context(), "core", "caller", identity.DeviceID.String(), "core", "one", "transport-new")
	if err != nil {
		t.Fatal(err)
	}
	defer endFresh()
	fresh.RoleRevision = 3
	if _, err := rt.Snapshot(freshCtx, fresh, coordination.DataQuery{ConversationID: "transport-conversation", ResourceKind: "message"}); err != nil {
		t.Fatalf("new authorized source request blocked: %v", err)
	}
	permanent := conflicting
	permanent.Scope = fresh
	permanent.Scope.RequestID = "transport-permanent"
	if _, err := rt.Commit(freshCtx, permanent); !errors.Is(err, coordination.ErrResourceVersion) {
		t.Fatalf("source CAS rejection was not preserved: %v", err)
	}
	if err := rt.ResumeDeviceData(t.Context(), identity.DeviceID.String()); err != nil {
		t.Fatal(err)
	}
	failures, err := rt.Coordination.DeliveryFailures(t.Context(), fresh, "transport-conversation")
	if err != nil || len(failures) != 2 || failures[0].RequestID != "transport-permanent" {
		t.Fatalf("permanent save failure was not exposed: %+v %v", failures, err)
	}
	if pending, err := rt.Coordination.PendingRequest(t.Context(), identity.DeviceID.String(), "transport-permanent"); err != nil || pending != nil {
		t.Fatalf("rejected payload remained at Core: %+v %v", pending, err)
	}
	if _, err := rt.Commit(freshCtx, permanent); !errors.Is(err, coordination.ErrDeliveryRejected) {
		t.Fatalf("permanently rejected request was queued again: %v", err)
	}
	testTaskOwnerRPCOverActualTLS(t, rt, db, p.services.KernelContainer.DeviceRegistry.Database(), ownerServices, identityStore, identity.DeviceID, raw, server, &coreDefinition)
	if err := rt.Coordination.RevokeDevice(t.Context(), "core", identity.DeviceID.String(), func(tx *sql.Tx) error {
		if err := registry.RevokeDeviceTx(t.Context(), tx, identity.DeviceID); err != nil {
			return err
		}
		return rt.CredentialSvc.RevokeAllForDeviceTx(t.Context(), tx, "core", identity.DeviceID)
	}); err != nil {
		t.Fatal(err)
	}
	rt.Hub.CloseDevice("core", identity.DeviceID)
	revokedCtx, endRevoked := context.WithTimeout(t.Context(), 5*time.Second)
	defer endRevoked()
	if err := client.WaitStopped(revokedCtx); err != nil || client.State() != agent.StateRevoked {
		t.Fatalf("live revocation kept reconnecting: state=%s err=%v", client.State(), err)
	}
	probe(true, http.StatusUnauthorized)
}
