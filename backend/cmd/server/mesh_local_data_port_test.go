package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func TestMeshLegacyHistoryPagesAndCursorOwnership(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: "device-a", CoreID: "core", RoleOwnerID: "device-a", ResourceOwnerID: "device-a", RoleID: "one", RoleRevision: 3}
	conversation := chat.Conversation{ID: "paged-chat", SpaceID: p.legacySpaceID, Title: "paged", Channel: "web"}
	if err := p.services.DB.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 131; i++ {
		message := chat.Message{ID: fmt.Sprintf("legacy-%04d", i), ConversationID: conversation.ID, CharacterID: "one", Role: "user", Content: "history", Sequence: int64(i)}
		if err := p.services.DB.Create(&message).Error; err != nil {
			t.Fatal(err)
		}
	}
	query := coordination.DataQuery{ConversationID: conversation.ID, ResourceKind: "message", Limit: 50}
	seen := map[string]bool{}
	firstCursor := ""
	for page := 0; page < 4; page++ {
		snapshot, err := p.Snapshot(t.Context(), scope, query)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range snapshot.LegacyMessages {
			var message chat.Message
			if err := json.Unmarshal(raw, &message); err != nil {
				t.Fatal(err)
			}
			if seen[message.ID] {
				t.Fatalf("duplicate %s", message.ID)
			}
			seen[message.ID] = true
		}
		query.LegacyCursor = snapshot.NextCursors["legacyMessage"]
		if page == 0 {
			firstCursor = query.LegacyCursor
		}
		if query.LegacyCursor == "" {
			break
		}
	}
	if len(seen) != 131 || firstCursor == "" {
		t.Fatalf("legacy history truncated: %d", len(seen))
	}
	query.LegacyCursor = firstCursor
	query.ConversationID = "other-chat"
	if _, err := p.Snapshot(t.Context(), scope, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign conversation accepted: %v", err)
	}
	query.ConversationID = conversation.ID
	scope.RoleID, scope.RoleRevision = "two", 1
	if _, err := p.Snapshot(t.Context(), scope, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign role accepted: %v", err)
	}
}

func setupMeshLocalDataPort(t *testing.T) *meshLocalDataPort {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "business.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	kernelDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	kernelDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = kernelDB.Close() })
	if err := kernelsqlite.Migrate(t.Context(), kernelDB); err != nil {
		t.Fatal(err)
	}
	p, err := newMeshLocalDataPort(&AppServices{DB: db, KernelContainer: &kernel.Container{DeviceRegistry: host_registry.NewRegistry(kernelDB)}}, t.TempDir(), "device-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []character.Character{{ID: "one", Name: "one", SpaceID: p.legacySpaceID, Status: "enabled", Revision: 3}, {ID: "two", Name: "two", SpaceID: p.legacySpaceID, Status: "enabled", Revision: 1}, {ID: "disabled", Name: "disabled", SpaceID: p.legacySpaceID, Status: "disabled", Revision: 1}, {ID: "foreign", Name: "foreign", SpaceID: "another-space", Status: "enabled", Revision: 1}} {
		if err := db.Create(&role).Error; err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func TestMeshLocalSnapshotReadsOnlySelectedDeviceRoleAndLiveMemories(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: "device-a", CoreID: "core", RoleOwnerID: "device-a", ResourceOwnerID: "device-a", RoleID: "one", RoleRevision: 3}
	roles, err := p.Roles(t.Context(), scope)
	if err != nil || len(roles) != 2 {
		t.Fatalf("roles=%+v err=%v", roles, err)
	}
	conversation := chat.Conversation{ID: "old-chat", SpaceID: p.legacySpaceID, Title: "old", Channel: "web"}
	if err := p.services.DB.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	for _, message := range []chat.Message{{ID: "first", ConversationID: conversation.ID, CharacterID: "one", Role: "user", Content: "allowed", Sequence: 1}, {ID: "second", ConversationID: conversation.ID, CharacterID: "two", Role: "assistant", Content: "other-role", Sequence: 2}} {
		if err := p.services.DB.Create(&message).Error; err != nil {
			t.Fatal(err)
		}
	}
	expired := time.Now().Add(-time.Hour).Format(time.RFC3339)
	for _, item := range []memory.Memory{{ID: "live", SpaceID: p.legacySpaceID, CharacterID: "one", Key: "drink", Value: "tea", VerifiedStatus: "verified"}, {ID: "dead", SpaceID: p.legacySpaceID, CharacterID: "one", Key: "drink", Value: "coffee", VerifiedStatus: "tombstone"}, {ID: "expired", SpaceID: p.legacySpaceID, CharacterID: "one", Key: "temporary", Value: "expired", ExpiresAt: &expired, VerifiedStatus: "verified"}, {ID: "other", SpaceID: p.legacySpaceID, CharacterID: "two", Key: "private", Value: "other-role", VerifiedStatus: "verified"}} {
		if err := p.services.DB.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{ConversationID: conversation.ID})
	if err != nil || len(snapshot.LegacyMessages) != 1 || len(snapshot.LegacyMemories) != 1 {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	var remembered memory.Memory
	if err := json.Unmarshal(snapshot.LegacyMemories[0], &remembered); err != nil || remembered.ID != "live" {
		t.Fatalf("memory=%+v err=%v", remembered, err)
	}
	scope.ResourceOwnerID = "device-b"
	if _, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{}); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("cross-owner read: %v", err)
	}
}

func TestMeshMemoryManagementIncludesInactiveWithoutLeakingIntoInference(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: "device-a", CoreID: "core", RoleOwnerID: "device-a", ResourceOwnerID: "device-a", RoleID: "one", RoleRevision: 3}
	kernelDB := p.services.KernelContainer.DeviceRegistry.Database()
	for id, role := range map[string]string{"disabled": "one", "foreign": "two"} {
		_, err := kernelDB.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES(?,'memory',?,?,1,?,?)`, p.ownerID, id, role, []byte(`{"allowContextUse":false,"content":{"value":"private"}}`), time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			t.Fatal(err)
		}
	}
	visible, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{ResourceKind: "memory", Management: true})
	if err != nil || len(visible.Resources) != 1 || visible.Resources[0].ID != "disabled" {
		t.Fatalf("management visibility: %+v %v", visible.Resources, err)
	}
	context, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{ResourceKind: "memory"})
	if err != nil || len(context.Resources) != 0 {
		t.Fatalf("disabled memory entered inference: %+v %v", context.Resources, err)
	}
}

func TestMeshLocalCommitRejectsDeletedOrRevisedRole(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	commit := coordination.Commit{Scope: coordination.ExecutionScope{RoleOwnerID: "device-a", ResourceOwnerID: "device-a", RoleID: "one", RoleRevision: 2, RequestID: "write"}, Mutations: []coordination.Mutation{{Kind: "memory", ID: "memory", RoleID: "one", Body: json.RawMessage(`{"value":"tea"}`)}}}
	if _, err := p.Commit(t.Context(), commit); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("revised role accepted: %v", err)
	}
	commit.Scope.RoleRevision = 3
	if err := p.services.DB.Model(&character.Character{}).Where("id='one'").Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := p.Commit(t.Context(), commit); !errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("disabled role fell back: %v", err)
	}
	resources, err := p.store.List(t.Context(), "memory", "one", true)
	if err != nil || len(resources) != 0 {
		t.Fatalf("invalid role left data: %+v %v", resources, err)
	}
}

func TestMeshHistoricalReadKeepsDeviceRoleSeparateFromCoreRole(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	conversation := chat.Conversation{ID: "historical-chat", SpaceID: p.legacySpaceID, Title: "history", Channel: "web"}
	if err := p.services.DB.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Create(&chat.Message{ID: "old", ConversationID: conversation.ID, CharacterID: "one", Role: "user", Content: "device context", Sequence: 1}).Error; err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{TargetDeviceID: "device-a", CoreID: "core", RoleOwnerID: "core", ResourceOwnerID: "core", RoleID: "core-role", RoleRevision: 9, Coordinated: true}
	history, err := p.HistoricalSnapshot(t.Context(), scope, coordination.DataQuery{ConversationID: conversation.ID})
	if err != nil || history == nil || history.OwnerID != "device-a" || history.Role.ID != "one" || string(history.Role.Profile) != `{}` || len(history.LegacyMessages) != 1 {
		t.Fatalf("history did not preserve owner or leaked device role configuration: %+v %v", history, err)
	}
	if scope.RoleID != "core-role" || scope.ResourceOwnerID != "core" {
		t.Fatal("history changed execution authority")
	}
	if err := p.services.DB.Model(&character.Character{}).Where("id='one'").Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	history, err = p.HistoricalSnapshot(t.Context(), scope, coordination.DataQuery{ConversationID: conversation.ID})
	if err != nil || history == nil || len(history.LegacyMessages) != 1 {
		t.Fatal("disabled old role prevented history access", err)
	}
	conversations, err := p.HistoricalConversations(t.Context(), scope)
	if err != nil || len(conversations) != 1 {
		t.Fatal("old device conversation disappeared in coordinated mode", err)
	}
	if _, err := p.HistoricalSnapshot(t.Context(), scope, coordination.DataQuery{ConversationID: conversation.ID, HistoricalRoleID: "two"}); !errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("foreign history role accepted: %v", err)
	}
	missing, err := p.HistoricalSnapshot(t.Context(), scope, coordination.DataQuery{ConversationID: "new-core-chat"})
	if err != nil || missing != nil {
		t.Fatalf("new Core conversation requested nonexistent history: %+v %v", missing, err)
	}
}
