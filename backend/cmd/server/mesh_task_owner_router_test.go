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
	for _, coordinated := range []bool{false, true} {
		testTaskOwnerRPCModeOverActualTLS(t, rt, db, sourceDB, services, signer, device, credential, server, definition, coordinated)
	}
}

func testTaskOwnerRPCModeOverActualTLS(t *testing.T, rt *devicemesh.Runtime, db, sourceDB *sql.DB, services *AppServices, signer *agent.IdentityStore, device runtimeidentity.DeviceID, credential string, server *httptest.Server, definition *task_runtime.TaskDefinition, coordinated bool) {
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
	if policy.Coordinated != coordinated {
		if _, err := rt.Coordination.ChangeMode(t.Context(), "core", device.String(), policy.ModeRevision, coordinated, "one"); err != nil {
			t.Fatal(err)
		}
	}
	id := "core-owner-rpc-task"
	if !coordinated {
		id = "device-owner-rpc-task"
	}
	ctx, authority, finish, err := rt.Coordination.Begin(t.Context(), "core", "caller", device.String(), "core", "one", id)
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
	definitionCopy := *definition
	definitionCopy.Checkpoint = true
	definition = &definitionCopy
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
	run := &task_runtime.TaskRun{TaskRunID: id, TaskDefinitionID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, InvocationID: id + "-invocation", ScopeSnapshotID: id + "-snapshot", DefinitionFingerprint: hex.EncodeToString(fingerprint[:]), InputHash: hex.EncodeToString(inputHash[:]), Input: input, Generation: 1, Revision: 1, Status: task_runtime.RunStatusRunning, ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice, ExecutionAttemptID: "core-owner-attempt", ExecutionTarget: task_runtime.TaskExecutionTarget{SpaceID: "core", DeviceID: device, RuntimeID: connection.RuntimeID, RuntimeSessionID: connection.SessionID, ConnectionGeneration: connection.Generation}, CreatedAt: time.Now().UTC()}
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
	params := func(values map[string]any) json.RawMessage {
		values["task_run_id"] = id
		raw, _ := json.Marshal(values)
		return raw
	}
	request := task_runtime.RemoteTaskOwnerRequest{Scope: authority, AuthorityCallID: callID, TaskGeneration: 1, AttemptID: run.ExecutionAttemptID.String(), LeaseID: entry.LeaseID, SessionID: connection.SessionID.String(), ConnectionGeneration: connection.Generation, RequestID: "source-request", Method: "task.storage.set", Params: params(map[string]any{"key": "private", "value": map[string]string{"location": "core-only"}})}
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
	request.Method, request.RequestID, request.Params = "task.storage.get", "read-request", params(map[string]any{"key": "private"})
	if result := invoke(request, true, http.StatusOK); !bytes.Contains(result, []byte(`"location":"core-only"`)) {
		t.Fatalf("Core owner data not returned: %s", result)
	}
	checkpointRequest := request
	checkpointRequest.Method, checkpointRequest.RequestID, checkpointRequest.Params = "task.checkpoint.save", "checkpoint-request", params(map[string]any{"version": 1, "payload": map[string]any{"cursor": 1, "data": map[string]string{"location": "core-only"}}})
	invoke(checkpointRequest, true, http.StatusOK)
	metadata, err := repository.GetLatestCheckpoint(t.Context(), run.TaskRunID)
	if err != nil || metadata == nil || metadata.Version != 1 || len(metadata.Payload) != 0 {
		t.Fatalf("Core task metadata lost checkpoint or copied its body: %+v %v", metadata, err)
	}
	progressRequest := request
	progressRequest.Method, progressRequest.RequestID, progressRequest.Params = "task.progress.save", "progress-request", params(map[string]any{"sequence": 1, "stage": "working", "message": "core-owned progress"})
	invoke(progressRequest, true, http.StatusOK)
	progressRequest.RequestID, progressRequest.Params = "progress-request-2", params(map[string]any{"sequence": 2, "stage": "working", "message": "core-owned progress next"})
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
	sourceCopies, coreCopies := 0, 1
	if !coordinated {
		sourceCopies, coreCopies = 1, 0
	}
	if err := sourceDB.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE resource_id=?`, "task/storage/"+run.TaskRunID).Scan(&copies); err != nil || copies != sourceCopies {
		t.Fatalf("source device mirrored Core task body: %d %v", copies, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources WHERE resource_id=?`, "task/storage/"+run.TaskRunID).Scan(&copies); err != nil || copies != coreCopies {
		t.Fatalf("Core owner body was not stored in the Core kernel: %d %v", copies, err)
	}
	current, err := repository.GetTaskRun(t.Context(), run.TaskRunID)
	if err != nil {
		t.Fatal(err)
	}
	pausing := task_runtime.CloneTaskRun(current)
	pausing.Status, pausing.LeaseID = task_runtime.RunStatusPausing, entry.LeaseID
	pausing.Revision = task_runtime.NextRevision(current.Revision)
	if ok, err := repository.UpdateTaskRunCAS(t.Context(), pausing, current.Status, current.Generation, current.Revision); err != nil || !ok {
		t.Fatalf("pause state was not persisted: %v", err)
	}
	invoke(request, true, http.StatusOK)
	checkpointRequest.RequestID, checkpointRequest.Params = "pause-checkpoint-request", params(map[string]any{"version": 2, "payload": map[string]any{"cursor": 2, "data": map[string]string{"location": "owner-only-paused"}}})
	invoke(checkpointRequest, true, http.StatusOK)
	if err := service.HandleRemotePaused(t.Context(), run.TaskRunID, request.AttemptID, entry.LeaseID, 1); err == nil {
		t.Fatal("old checkpoint confirmed a paused TLS task")
	}
	if err := service.HandleRemotePaused(t.Context(), run.TaskRunID, request.AttemptID, entry.LeaseID, 2); err != nil {
		t.Fatal(err)
	}
	paused, err := repository.GetTaskRun(t.Context(), run.TaskRunID)
	if err != nil || paused.Status != task_runtime.RunStatusPaused || paused.PausedAt == nil {
		t.Fatalf("owner pause was not confirmed: %+v %v", paused, err)
	}
	invoke(request, true, http.StatusConflict)
	if !pending.CompleteBound(run.TaskRunID, request.AttemptID, entry.LeaseID, request.SessionID, request.ConnectionGeneration, false, "paused") {
		t.Fatal("pause stop receipt was not confirmed")
	}
	pending.Cancel(run.TaskRunID, "execution cancelled")
	invoke(request, true, http.StatusConflict)
}
