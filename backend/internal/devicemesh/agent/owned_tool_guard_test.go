package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	_ "modernc.org/sqlite"
)

type testSourceRoleGuard struct{}

func (testSourceRoleGuard) WithSourceRole(_ context.Context, scope coordination.ExecutionScope, execute func() error) error {
	if scope.RoleID != "role" {
		return coordination.ErrRoleRequired
	}
	return execute()
}

func TestOwnedToolGuardRejectsExpiredAuthorityBeforeCachedResult(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "authority.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, schema := range []string{coordination.SourceAuthoritySchema, coordination.CancelledAuthoritySchema} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	credential := &StoredCredential{CredentialID: "id", Credential: "token", SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", ExpiresAt: time.Now().Add(time.Hour)}
	store := NewCredentialStore(root)
	if err := store.SaveCredential(credential); err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", InitiatorDeviceID: "caller", TargetDeviceID: "device", PermissionRevision: 1, TargetPermissionRevision: 1, ModeRevision: 1, ProviderEpoch: 1, TargetProviderEpoch: 1, RoleRevision: 1, ResourceOwnerID: "device", RoleOwnerID: "device", RoleID: "role", RequestID: "request"}
	encoded, _ := json.Marshal(scope)
	invocation := protocol.RuntimeInvokePayload{InvocationID: "invoke", AuthorityCallID: "authority", SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", OwnedExecutionScope: encoded}
	guard := NewOwnedToolGuard(db, root, testSourceRoleGuard{})
	calls := 0
	cached := func(context.Context) (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{Status: "success"}, nil
	}
	if _, err := guard(t.Context(), invocation, cached); err != nil || calls != 1 {
		t.Fatalf("valid tool: calls=%d err=%v", calls, err)
	}
	missingRevision := scope
	missingRevision.TargetProviderEpoch = 0
	incomplete := invocation
	incomplete.OwnedExecutionScope, _ = json.Marshal(missingRevision)
	if _, err := guard(t.Context(), incomplete, cached); !errors.Is(err, coordination.ErrWrongOwner) || calls != 1 {
		t.Fatalf("incomplete target version accepted: calls=%d err=%v", calls, err)
	}
	if err := coordination.FenceSourceAuthority(t.Context(), db, "core", "caller", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := guard(t.Context(), invocation, cached); !errors.Is(err, coordination.ErrScopeExpired) || calls != 1 {
		t.Fatalf("cached result bypassed fence: calls=%d err=%v", calls, err)
	}
	scope.PermissionRevision = 2
	invocation.OwnedExecutionScope, _ = json.Marshal(scope)
	if err := coordination.CancelSourceAuthority(t.Context(), db, "core", []string{"authority"}); err != nil {
		t.Fatal(err)
	}
	if _, err := guard(t.Context(), invocation, cached); !errors.Is(err, coordination.ErrScopeExpired) || calls != 1 {
		t.Fatalf("cancelled tool executed: calls=%d err=%v", calls, err)
	}
	invocation.AuthorityCallID = "new-authority"
	if _, err := guard(t.Context(), invocation, cached); err != nil || calls != 2 {
		t.Fatalf("new authority rejected: calls=%d err=%v", calls, err)
	}
	deadlineCtx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := guard(deadlineCtx, invocation, func(current context.Context) (*protocol.RuntimeResultPayload, error) {
		<-current.Done()
		return &protocol.RuntimeResultPayload{Status: "success"}, nil
	}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired execution returned a result: %v", err)
	}
	if err := store.DeleteCredential(); err != nil {
		t.Fatal(err)
	}
	if _, err := guard(context.Background(), invocation, cached); err == nil || calls != 2 {
		t.Fatalf("unpaired tool executed: calls=%d err=%v", calls, err)
	}
}

func TestOwnedToolGuardRejectsMissingAndWrongTargetScope(t *testing.T) {
	guard := NewOwnedToolGuard(nil, t.TempDir())
	for _, invocation := range []protocol.RuntimeInvokePayload{{}, {AuthorityCallID: "authority"}, {OwnedExecutionScope: []byte(`{}`)}, {AuthorityCallID: "authority", OwnedExecutionScope: []byte(`not-json`)}} {
		if _, err := guard(t.Context(), invocation, func(context.Context) (*protocol.RuntimeResultPayload, error) {
			t.Fatal("invalid request executed")
			return nil, nil
		}); !errors.Is(err, coordination.ErrWrongOwner) {
			t.Fatalf("invalid authority: %v", err)
		}
	}
}
