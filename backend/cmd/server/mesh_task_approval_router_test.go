package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestLocalTaskApprovalControlRejectsRemoteAndUntrustedDecisions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local-token")
	credentials, err := security.NewLocalCredentialStore(path)
	if err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	registerLocalSourceTaskApprovalRouter(router.Group("/internal/device-mesh", security.LocalRuntimeControlMiddleware(credentials, "127.0.0.1:18899")), nil)
	for _, scenario := range []struct {
		name, remote, token, body string
		expected                  int
	}{
		{"remote-core", "192.168.1.5:1234", string(token), `{"expectedRevision":1,"approved":true}`, 403},
		{"missing-local-token", "127.0.0.1:1234", "", `{"expectedRevision":1,"approved":true}`, 401},
		{"forged-authority", "127.0.0.1:1234", string(token), `{"expectedRevision":1,"approved":true,"executionScope":{"coreId":"foreign"}}`, 400},
		{"missing-decision", "127.0.0.1:1234", string(token), `{"expectedRevision":1}`, 400},
		{"stale-revision", "127.0.0.1:1234", string(token), `{"expectedRevision":0,"approved":true}`, 400},
		{"unavailable", "127.0.0.1:1234", string(token), `{"expectedRevision":1,"approved":true}`, 503},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/task-approvals/test/decision", strings.NewReader(scenario.body))
			request.RemoteAddr = scenario.remote
			request.Header.Set("X-Amitia-Local-Token", strings.TrimSpace(scenario.token))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != scenario.expected {
				t.Fatalf("status=%d expected=%d", recorder.Code, scenario.expected)
			}
		})
	}
}

func TestLocalTaskApprovalUsesCurrentPairAndSourceRoleThroughActualAPI(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b := newThreeCoreFixture(t, "approval-a", schemas), newThreeCoreFixture(t, "approval-b", schemas)
	pairThreeCoreFixtures(t, a, b)
	registry := permission.NewPermissionDefinitionRegistry()
	registry.Register(permission.PermissionDefinition{ID: "test.source.read", Category: permission.CategoryFilesystem, AllowedScopes: []permission.ScopeType{permission.ScopeExtension}, BackgroundAllowed: true, RequiresPerUse: true, RemoteExecution: permission.RemoteExecutionRequireApproval})
	broker := permission.NewDefaultPermissionBroker(registry, permission.NewMemoryPermissionStorage())
	t.Cleanup(func() { _ = broker.Close() })
	installThreeCoreCatalogTasks(t, a, "1.0.0", func(config *task_runtime.TaskRuntimeConfig) {
		guard := task_runtime.NewSourceTaskPermissionGuard(broker)
		config.SourcePermissionGuard = func(ctx context.Context, run *task_runtime.TaskRun, definition *task_runtime.TaskDefinition, input json.RawMessage) error {
			err := guard(ctx, run, definition, input)
			if err != nil {
				t.Logf("Source permission guard: %v", err)
			}
			return err
		}
		config.SourceApprovalRecorder = broker
	})
	repository := kernelsqlite.NewTaskRepository(a.services.KernelContainer.DeviceRegistry.Database())
	definition, err := repository.GetTaskDefinition(t.Context(), "same-task")
	if err != nil {
		t.Fatal(err)
	}
	definition.PermissionRequirementStrings = []string{"test.source.read"}
	if err := repository.PutTaskDefinition(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	authority := readThreeCoreCatalogAuthority(t, b, a, a)
	authority.RequestID, authority.TurnID, authority.ExecutionID = "approve-request", "approve-turn", "approve-execution"
	ctx := coordination.WithScope(t.Context(), authority)
	runtime := a.services.KernelContainer.TaskRuntimeService
	pin, err := runtime.DescribeInstalledTask(ctx, definition.TaskID, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	input := json.RawMessage(`{"private":"source execution only"}`)
	hash := sha256.Sum256(input)
	run := task_runtime.TaskRun{TaskRunID: "approve-run", InvocationID: "approve-run", ScopeSnapshotID: "approve-run", TaskDefinitionID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InputHash: hex.EncodeToString(hash[:]), ExecutionTarget: task_runtime.TaskExecutionTarget{SpaceID: runtimeidentity.SpaceID(b.core), DeviceID: a.device.DeviceID, RuntimeID: a.device.RuntimeID, RuntimeSessionID: "source-session", ConnectionGeneration: 1}}
	store := a.services.DeviceMesh.LocalHandler.CredentialStore()
	if err := store.SaveCursor(&agent.SessionCursor{RuntimeSessionID: run.ExecutionTarget.RuntimeSessionID, ConnectionGeneration: run.ExecutionTarget.ConnectionGeneration}); err != nil {
		t.Fatal(err)
	}
	request := task_runtime.SourceTaskPermissionRequest{Scope: authority, Run: run, Target: pin, Input: input}
	ack, err := runtime.CheckInstalledTaskPermissions(ctx, request)
	if err != nil || ack.ApprovalID == "" || ack.Allowed {
		t.Fatalf("pending approval=%+v err=%v", ack, err)
	}
	tokenPath := filepath.Join(t.TempDir(), "local-token")
	credentials, err := security.NewLocalCredentialStore(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	registerLocalSourceTaskApprovalRouter(router.Group("/internal/device-mesh", security.LocalRuntimeControlMiddleware(credentials, "127.0.0.1:18899")), a.services)
	call := func(method, path, body string, expected int) []byte {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.RemoteAddr = "127.0.0.1:1234"
		request.Header.Set("X-Amitia-Local-Token", strings.TrimSpace(string(token)))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != expected {
			t.Fatalf("local status=%d expected=%d body=%s", recorder.Code, expected, recorder.Body.String())
		}
		return recorder.Body.Bytes()
	}
	var page struct {
		Code int                                      `json:"code"`
		Data []task_runtime.SourceTaskApprovalDetails `json:"data"`
	}
	if err := json.Unmarshal(call(http.MethodGet, "/internal/device-mesh/task-approvals", "", 200), &page); err != nil || page.Code != 200 || len(page.Data) != 1 || len(page.Data[0].Permissions) != 1 {
		t.Fatalf("local approvals=%+v err=%v", page, err)
	}
	path := "/internal/device-mesh/task-approvals/" + ack.ApprovalID + "/decision"
	if err := store.SaveCursor(&agent.SessionCursor{RuntimeSessionID: "replacement-session", ConnectionGeneration: 2}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(call(http.MethodGet, "/internal/device-mesh/task-approvals", "", 200), &page); err != nil || len(page.Data) != 0 {
		t.Fatal("重连后的旧审批继续可见")
	}
	call(http.MethodPost, path, `{"expectedRevision":1,"approved":true}`, 409)
	if err := store.SaveCursor(&agent.SessionCursor{RuntimeSessionID: run.ExecutionTarget.RuntimeSessionID, ConnectionGeneration: run.ExecutionTarget.ConnectionGeneration}); err != nil {
		t.Fatal(err)
	}
	call(http.MethodPost, path, `{"expectedRevision":2,"approved":true}`, 409)
	call(http.MethodPost, path, `{"expectedRevision":1,"approved":true}`, 200)
	ack, err = runtime.CheckInstalledTaskPermissions(ctx, request)
	if err != nil || !ack.Allowed {
		t.Fatalf("approved permission=%+v err=%v", ack, err)
	}
	revokePath := "/internal/device-mesh/task-approvals/" + ack.ApprovalID + "/revoke"
	call(http.MethodPost, revokePath, `{"expectedRevision":1}`, 409)
	call(http.MethodPost, revokePath, `{"expectedRevision":2}`, 200)
	ack, err = runtime.CheckInstalledTaskPermissions(ctx, request)
	if err != nil || ack.Allowed || ack.ApprovalStatus != "revoked" {
		t.Fatalf("已撤销的资源审批继续可用: %+v %v", ack, err)
	}
	if err := coordination.FenceSourceAuthority(context.Background(), a.services.KernelContainer.DeviceRegistry.Database(), b.core, a.device.DeviceID.String(), authority.TargetPermissionRevision); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(call(http.MethodGet, "/internal/device-mesh/task-approvals", "", 200), &page); err != nil || len(page.Data) != 0 {
		t.Fatal("已撤销设备的旧审批继续可见")
	}
}
