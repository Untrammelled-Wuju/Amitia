package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func testTaskOwnerRPCOverActualTLS(t *testing.T, rt *devicemesh.Runtime, db, sourceDB *sql.DB, services *AppServices, signer *agent.IdentityStore, device runtimeidentity.DeviceID, credential string, server *httptest.Server, definition *task_runtime.TaskDefinition) {
	t.Helper()
	corePort := setupMeshLocalDataPort(t)
	corePort.ownerID = "core"
	corePort.store = coordination.NewOwnershipStore(db, "core")
	corePort.services.KernelContainer.DeviceRegistry = host_registry.NewRegistry(db)
	rt.CoreDataPort = corePort
	policy, err := rt.Coordination.Get(t.Context(), "core", device.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Coordination.ChangeMode(t.Context(), "core", device.String(), policy.ModeRevision, true, "one"); err != nil {
		t.Fatal(err)
	}
	ctx, authority, finish, err := rt.Coordination.Begin(t.Context(), "core", "caller", device.String(), "core", "one", "core-task-owner")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	authority.RoleRevision, authority.TurnID, authority.ExecutionID = 3, "core-task-turn", "core-task-execution"
	ctx = coordination.WithScope(ctx, authority)
	binding := &task_runtime.OwnedRuntimeBinding{}
	if err := bindCoreOwnedTaskRuntime(binding, rt, "core"); err != nil {
		t.Fatal(err)
	}
	config := task_runtime.DefaultTaskRuntimeConfig()
	binding.Apply(&config)
	snapshots := scope.NewMemoryScopeStore()
	config.AuthoritySnapshots = snapshots
	repository := kernelsqlite.NewTaskRepository(db)
	service := task_runtime.NewTaskRuntimeService(repository, config)
	services.KernelContainer.TaskRuntimeService = service
	if err := service.PutTaskDefinition(ctx, definition); err != nil {
		t.Fatal(err)
	}
	connection, ok := rt.Hub.GetByDevice("core", device)
	if !ok {
		t.Fatal("task source is not connected")
	}
	encoded, _ := json.Marshal(definition)
	fingerprint := sha256.Sum256(encoded)
	input := json.RawMessage(`{"private":"core-owned-input"}`)
	inputHash := sha256.Sum256(input)
	run := &task_runtime.TaskRun{TaskRunID: "core-owner-rpc-task", TaskDefinitionID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InvocationID: "core-owner-invocation", ScopeSnapshotID: "core-owner-snapshot", DefinitionFingerprint: hex.EncodeToString(fingerprint[:]), InputHash: hex.EncodeToString(inputHash[:]), Input: input, Generation: 1, Revision: 1, Status: task_runtime.RunStatusRunning, ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice, ExecutionAttemptID: "core-owner-attempt", ExecutionTarget: task_runtime.TaskExecutionTarget{SpaceID: "core", DeviceID: device, RuntimeID: connection.RuntimeID, RuntimeSessionID: connection.SessionID, ConnectionGeneration: connection.Generation}, CreatedAt: time.Now().UTC()}
	encodedScope, _ := json.Marshal(authority)
	if err := snapshots.SaveSnapshot(ctx, scope.ScopeSnapshot{SnapshotID: run.ScopeSnapshotID, SpaceID: "core", InvocationID: run.InvocationID, ExtensionID: run.ExtensionID, ModuleID: run.ModuleID, CharacterID: "one", OwnedExecutionScope: encodedScope}); err != nil {
		t.Fatal(err)
	}
	if err := config.OwnedInputs.SaveInput(ctx, run); err != nil {
		t.Fatal(err)
	}
	run.Input = nil
	if err := repository.PutTaskRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	pending := task_runtime.NewPendingTaskManager()
	rt.SetPendingTasks(pending)
	entry, err := pending.Register(task_runtime.TaskExecutionRequest{Run: run, Target: run.ExecutionTarget, AttemptID: run.ExecutionAttemptID}, connection.SessionID.String(), connection.Generation, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer pending.Cancel(run.TaskRunID, "test finished")
	callID, err := rt.Coordination.TrackRemoteAuthority(ctx, authority, connection.SessionID.String(), connection.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if err := pending.BindAuthority(run.TaskRunID, run.ExecutionAttemptID.String(), connection.SessionID.String(), connection.Generation, func(context.Context) error { return nil }, callID); err != nil {
		t.Fatal(err)
	}
	request := task_runtime.RemoteTaskOwnerRequest{Scope: authority, AuthorityCallID: callID, TaskGeneration: 1, AttemptID: run.ExecutionAttemptID.String(), LeaseID: entry.LeaseID, SessionID: connection.SessionID.String(), ConnectionGeneration: connection.Generation, RequestID: "source-request", Method: "task.storage.set", Params: json.RawMessage(`{"task_run_id":"core-owner-rpc-task","key":"private","value":{"location":"core-only"}}`)}
	invoke := func(value task_runtime.RemoteTaskOwnerRequest, sign bool, expected int) []byte {
		t.Helper()
		body, _ := json.Marshal(value)
		httpRequest, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/device-mesh/v1/business/tasks/"+run.TaskRunID+"/owner-rpc", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("Authorization", "AmitiaDevice "+credential)
		if sign {
			if err := signer.SignRequest(httpRequest, "core"); err != nil {
				t.Fatal(err)
			}
		}
		response, err := server.Client().Do(httpRequest)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		output, err := io.ReadAll(io.LimitReader(response.Body, 512<<10))
		if err != nil || response.StatusCode != expected {
			t.Fatalf("owner RPC status=%d expected=%d body=%s err=%v", response.StatusCode, expected, output, err)
		}
		return output
	}
	invoke(request, false, http.StatusUnauthorized)
	invoke(request, true, http.StatusConflict)
	if !pending.ClaimBound(run.TaskRunID, request.AttemptID, entry.LeaseID, request.SessionID, request.ConnectionGeneration, "source", time.Minute) {
		t.Fatal("task lease was not claimed")
	}
	invoke(request, true, http.StatusOK)
	request.Method, request.RequestID, request.Params = "task.storage.get", "read-request", json.RawMessage(`{"task_run_id":"core-owner-rpc-task","key":"private"}`)
	if result := invoke(request, true, http.StatusOK); !bytes.Contains(result, []byte(`"location":"core-only"`)) {
		t.Fatalf("Core owner data not returned: %s", result)
	}
	checkpointRequest := request
	checkpointRequest.Method, checkpointRequest.RequestID, checkpointRequest.Params = "task.checkpoint.save", "checkpoint-request", json.RawMessage(`{"task_run_id":"core-owner-rpc-task","version":1,"payload":{"cursor":1,"data":{"location":"core-only"}}}`)
	invoke(checkpointRequest, true, http.StatusOK)
	metadata, err := repository.GetLatestCheckpoint(t.Context(), run.TaskRunID)
	if err != nil || metadata == nil || metadata.Version != 1 || len(metadata.Payload) != 0 {
		t.Fatalf("Core task metadata lost checkpoint or copied its body: %+v %v", metadata, err)
	}
	progressRequest := request
	progressRequest.Method, progressRequest.RequestID, progressRequest.Params = "task.progress.save", "progress-request", json.RawMessage(`{"task_run_id":"core-owner-rpc-task","sequence":1,"stage":"working","message":"core-owned progress"}`)
	invoke(progressRequest, true, http.StatusOK)
	progressRequest.RequestID, progressRequest.Params = "progress-request-2", json.RawMessage(`{"task_run_id":"core-owner-rpc-task","sequence":2,"stage":"working","message":"core-owned progress next"}`)
	invoke(progressRequest, true, http.StatusOK)
	progress, err := repository.GetProgress(t.Context(), run.TaskRunID)
	if err != nil || progress == nil || progress.Sequence != 2 || progress.Message != "" || progress.Stage != "" {
		t.Fatalf("confirmed progress was throttled or copied into metadata: %+v %v", progress, err)
	}
	for _, mutate := range []func(*task_runtime.RemoteTaskOwnerRequest){func(r *task_runtime.RemoteTaskOwnerRequest) { r.AuthorityCallID = "foreign-call" }, func(r *task_runtime.RemoteTaskOwnerRequest) { r.ConnectionGeneration++ }, func(r *task_runtime.RemoteTaskOwnerRequest) { r.SessionID = "stale" }, func(r *task_runtime.RemoteTaskOwnerRequest) { r.Scope.RoleRevision++ }, func(r *task_runtime.RemoteTaskOwnerRequest) { r.LeaseID = "foreign-lease" }} {
		invalid := request
		mutate(&invalid)
		invoke(invalid, true, http.StatusConflict)
	}
	var copies int
	if err := sourceDB.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE resource_id=?`, "task/storage/"+run.TaskRunID).Scan(&copies); err != nil || copies != 0 {
		t.Fatalf("source device mirrored Core task body: %d %v", copies, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE owner_id='core' AND resource_id=?`, "task/storage/"+run.TaskRunID).Scan(&copies); err != nil || copies != 1 {
		t.Fatalf("Core owner body was not stored in the Core kernel: %d %v", copies, err)
	}
	pending.Cancel(run.TaskRunID, "execution cancelled")
	invoke(request, true, http.StatusConflict)
}
