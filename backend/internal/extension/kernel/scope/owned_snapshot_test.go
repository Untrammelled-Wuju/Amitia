package scope_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func TestOwnedAuthorizationSnapshotSurvivesRestartAndRejectsScopeReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	snapshot := scope.CreateSnapshotWithOwner("invocation", []scope.ScopeRef{scope.NewCharacterScope("role")}, "core", "role", "conversation", "extension", "module", 1)
	authority := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", InitiatorDeviceID: "caller", TargetDeviceID: "target", ProviderEpoch: 4, TargetProviderEpoch: 8, PermissionRevision: 2, TargetPermissionRevision: 3, ModeRevision: 5, RoleRevision: 6, RoleOwnerID: "target", ResourceOwnerID: "target", RoleID: "role", RequestID: "request"}
	snapshot.OwnedExecutionScope, _ = json.Marshal(authority)
	store := scope.NewSQLiteScopeStore(db)
	if err := store.SaveSnapshot(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store = scope.NewSQLiteScopeStore(db)
	loaded, err := store.GetSnapshot(t.Context(), snapshot.SnapshotID)
	if err != nil {
		t.Fatal(err)
	}
	var restored coordination.ExecutionScope
	if json.Unmarshal(loaded.OwnedExecutionScope, &restored) != nil || restored != authority || loaded.SpaceID != snapshot.SpaceID || loaded.CharacterID != snapshot.CharacterID || loaded.Generation != snapshot.Generation {
		t.Fatal("restart lost immutable owner, role or permission scope")
	}
	if err := store.SaveSnapshot(t.Context(), loaded); err != nil {
		t.Fatalf("identical persisted authority rejected: %v", err)
	}
	for _, mutate := range []func(*scope.ScopeSnapshot){
		func(value *scope.ScopeSnapshot) { value.OwnedExecutionScope = nil },
		func(value *scope.ScopeSnapshot) { value.CharacterID = "another-role" },
		func(value *scope.ScopeSnapshot) { value.Generation++ },
		func(value *scope.ScopeSnapshot) {
			value.OwnedExecutionScope = json.RawMessage(`{"coreId":"another-core"}`)
		},
	} {
		changed := loaded
		mutate(&changed)
		if err := store.SaveSnapshot(t.Context(), changed); err == nil {
			t.Fatal("persisted task authority replaced or downgraded")
		}
	}
}

func TestMemoryOwnedSnapshotsCannotBeMutatedThroughReadOrWriteSlices(t *testing.T) {
	store := scope.NewMemoryScopeStore()
	snapshot := scope.CreateSnapshot("invocation", []scope.ScopeRef{scope.NewCharacterScope("role")}, "role", "conversation", "extension", "module")
	snapshot.OwnedExecutionScope = json.RawMessage(`{"coreId":"core","targetDeviceId":"target"}`)
	if err := store.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.OwnedExecutionScope[0] = '['
	snapshot.ResolvedScopes[0].CharacterID = "different-role"
	loaded, err := store.GetSnapshot(t.Context(), snapshot.SnapshotID)
	if err != nil || !json.Valid(loaded.OwnedExecutionScope) || loaded.ResolvedScopes[0].CharacterID != "role" {
		t.Fatal("caller mutated stored task authority")
	}
	loaded.OwnedExecutionScope[0] = '['
	if err := store.SaveSnapshot(t.Context(), loaded); err == nil {
		t.Fatal("corrupted authority accepted")
	}
	loaded, _ = store.GetSnapshot(t.Context(), snapshot.SnapshotID)
	loaded.OwnedExecutionScope = nil
	if err := store.SaveSnapshot(t.Context(), loaded); err == nil {
		t.Fatal("memory authority downgraded")
	}
}
