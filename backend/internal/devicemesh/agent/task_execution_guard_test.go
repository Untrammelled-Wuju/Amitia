package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type lockedTaskRoleGuard struct{ mu sync.Mutex }

func (g *lockedTaskRoleGuard) WithSourceRole(_ context.Context, _ coordination.ExecutionScope, fn func() error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return fn()
}

func TestLongOwnedTaskDoesNotBlockOwnerRPCOrUnpairAndCannotReturnAfterUnpair(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, schema := range []string{coordination.SourceAuthoritySchema, coordination.CancelledAuthoritySchema} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	credentials := NewCredentialStore(root)
	if err := credentials.SaveCredential(&StoredCredential{CredentialID: "credential", Credential: "test", SpaceID: "core", DeviceID: "source", RuntimeID: "runtime", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	authority := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", InitiatorDeviceID: "caller", TargetDeviceID: "source", RoleID: "role", RoleRevision: 1, ResourceOwnerID: "source", RoleOwnerID: "source", RequestID: "request", TurnID: "turn", ExecutionID: "execution", ProviderEpoch: 1, TargetProviderEpoch: 1, PermissionRevision: 1, TargetPermissionRevision: 1, ModeRevision: 1}
	encoded, _ := json.Marshal(authority)
	invocation := protocol.RuntimeInvokePayload{InvocationID: "task-attempt", AuthorityCallID: "task-call", SpaceID: "core", DeviceID: "source", RuntimeID: "runtime", RuntimeType: "task", OwnedExecutionScope: encoded}
	guard := NewOwnedToolGuard(db, root, &lockedTaskRoleGuard{})
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	finishTask := func() { releaseOnce.Do(func() { close(release) }) }
	defer finishTask()
	result := make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() {
		_, err := guard(ctx, invocation, func(current context.Context) (*protocol.RuntimeResultPayload, error) {
			close(started)
			select {
			case <-release:
			case <-current.Done():
				return nil, current.Err()
			}
			return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"private":"late"}`)}, nil
		})
		result <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("task did not start")
	}
	probe := invocation
	probe.RuntimeType, probe.AuthorityCallID, probe.InvocationID = "native", "owner-rpc", "owner-rpc"
	probeDone := make(chan error, 1)
	go func() {
		_, err := guard(ctx, probe, func(context.Context) (*protocol.RuntimeResultPayload, error) { return nil, nil })
		probeDone <- err
	}()
	select {
	case err := <-probeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("long task blocked its owner RPC")
	}
	unpairDone := make(chan error, 1)
	go func() { unpairDone <- credentials.DeleteCredential() }()
	select {
	case err := <-unpairDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("long task blocked unpair")
	}
	finishTask()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("unpaired task returned private result")
		}
	case <-ctx.Done():
		t.Fatal("task guard did not reject late completion")
	}
}
