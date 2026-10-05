package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type authorityTaskExecutor struct{ calls int }

func (e *authorityTaskExecutor) ExecuteOwnedDispatch(ctx context.Context, dispatch protocol.TaskDispatchPayload) (json.RawMessage, error) {
	if dispatch.TaskRunID != "task" || dispatch.AttemptID == "" || dispatch.LeaseID == "" || dispatch.AuthorityCallID == "" || len(dispatch.OwnedExecutionScope) == 0 {
		return nil, coordination.ErrWrongOwner
	}
	return e.Execute(ctx, dispatch.TaskDefinitionID, nil)
}

func (e *authorityTaskExecutor) Execute(ctx context.Context, _ string, _ map[string]interface{}) (json.RawMessage, error) {
	if scope, ok := coordination.FromContext(ctx); !ok || scope.RoleID != "role" || scope.ResourceOwnerID != "device" {
		return nil, coordination.ErrWrongOwner
	}
	e.calls++
	return json.RawMessage(`{"completed":true}`), nil
}

func TestTaskAuthorityIsCheckedBeforeLeaseAndBeforeCachedResult(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "task.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, schema := range []string{coordination.SourceAuthoritySchema, coordination.CancelledAuthoritySchema, executionjournal.Schema, executionjournal.FenceSchema} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	if err := NewCredentialStore(root).SaveCredential(&StoredCredential{CredentialID: "task-credential", Credential: "task-test", SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	client := NewMeshClient(MeshClientConfig{SpaceID: "core", Identity: &LocalIdentity{DeviceID: "device", RuntimeID: "runtime"}, ExecutionJournal: executionjournal.NewStore(db), ExecutionGuard: NewOwnedToolGuard(db, root, testSourceRoleGuard{})})
	defer client.Stop()
	client.setState(StateReady)
	client.sessionID, client.connectionGen = "task-session", 1
	worker := NewTaskWorker(client)
	executor := &authorityTaskExecutor{}
	worker.SetTaskRuntime(executor)
	dispatch := protocol.TaskDispatchPayload{TaskRunID: "task", AttemptID: "attempt", LeaseID: "lease", TaskDefinitionID: "test-action", Input: json.RawMessage(`{}`), DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "task-session", ConnectionGeneration: 1}
	if err := worker.ExecuteTask(t.Context(), dispatch); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("unscoped task reached lease claim: %v", err)
	}
	scope := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", InitiatorDeviceID: "caller", TargetDeviceID: "device", PermissionRevision: 1, TargetPermissionRevision: 1, ModeRevision: 1, ProviderEpoch: 1, TargetProviderEpoch: 1, RoleRevision: 1, ResourceOwnerID: "device", RoleOwnerID: "device", RoleID: "role", RequestID: "task-request"}
	scope.ExecutionID, scope.TurnID = "task-execution", "task-turn"
	dispatch.AuthorityCallID = "authority-task"
	dispatch.OwnedExecutionScope, _ = json.Marshal(scope)
	dispatch.TargetDefinitionPin, _ = json.Marshal(protocol.TargetTaskDefinitionPin{DeviceID: "device", TaskID: dispatch.TaskDefinitionID, ExtensionID: "extension", ModuleID: "module", InstalledGeneration: 1, DefinitionFingerprint: strings.Repeat("a", 64), PortableFingerprint: strings.Repeat("b", 64), EntryHash: "sha256:" + strings.Repeat("c", 64)})
	for i, input := range []string{`{"taskType":"other-action"}`, `{"taskType":null}`, `{"taskType":7}`, `{"taskType":""}`} {
		invalid := dispatch
		invalid.TaskRunID = fmt.Sprintf("invalid-task-%d", i)
		invalid.AttemptID = fmt.Sprintf("invalid-attempt-%d", i)
		invalid.AuthorityCallID = fmt.Sprintf("invalid-authority-%d", i)
		invalid.Input = json.RawMessage(input)
		if _, err := worker.executeAuthorizedTask(t.Context(), invalid); err == nil || executor.calls != 0 {
			t.Fatalf("input changed authorized handler: calls=%d err=%v", executor.calls, err)
		}
	}
	if _, err := worker.executeAuthorizedTask(t.Context(), dispatch); err != nil || executor.calls != 1 {
		t.Fatalf("authorized task rejected: calls=%d err=%v", executor.calls, err)
	}
	if _, err := worker.executeAuthorizedTask(t.Context(), dispatch); err != nil || executor.calls != 1 {
		t.Fatalf("completed task repeated: calls=%d err=%v", executor.calls, err)
	}
	changedPin := dispatch
	var pin protocol.TargetTaskDefinitionPin
	if err := json.Unmarshal(changedPin.TargetDefinitionPin, &pin); err != nil {
		t.Fatal(err)
	}
	pin.InstalledGeneration++
	changedPin.TargetDefinitionPin, _ = json.Marshal(pin)
	if _, err := worker.executeAuthorizedTask(t.Context(), changedPin); err == nil || executor.calls != 1 {
		t.Fatalf("new target version read old cached result: calls=%d err=%v", executor.calls, err)
	}
	if err := coordination.CancelSourceAuthority(t.Context(), db, "core", []string{dispatch.AuthorityCallID}); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.executeAuthorizedTask(t.Context(), dispatch); !errors.Is(err, coordination.ErrScopeExpired) || executor.calls != 1 {
		t.Fatalf("cancelled task read cached result: calls=%d err=%v", executor.calls, err)
	}
	dispatch.AuthorityCallID = "fresh-task-authority"
	if err := coordination.FenceSourceAuthority(t.Context(), db, "core", "caller", 1); err != nil {
		t.Fatal(err)
	}
	if err := worker.ExecuteTask(t.Context(), dispatch); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("expired task reached lease claim: %v", err)
	}
	client.setState(StateBackoff)
	if _, err := worker.executeAuthorizedTask(t.Context(), dispatch); err == nil || executor.calls != 1 {
		t.Fatalf("disconnected task read result or executed: calls=%d err=%v", executor.calls, err)
	}
}
