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
		for _, resource := range snapshot.Resources {
			if resource.Kind != "message" {
				continue
			}
			var message chat.Message
			if err := json.Unmarshal(resource.Body, &message); err != nil {
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
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "business.db")+"?_txlock=immediate&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"), &gorm.Config{})
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
	kernelDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db")+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
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
	snapshot, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{ConversationID: conversation.ID})
	messageCount := 0
	for _, resource := range snapshot.Resources {
		if resource.Kind == "message" {
			messageCount++
		}
	}
	if err != nil || messageCount != 1 {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
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

func TestMeshDefaultMemoryReadIncludesRoleAndGlobalWrites(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: "device-a", CoreID: "core", RoleOwnerID: "device-a", ResourceOwnerID: "device-a", RoleID: "one", RoleRevision: 3, MemoryRoleID: "one"}
	kernelDB := p.services.KernelContainer.DeviceRegistry.Database()
	for id, role := range map[string]string{"role-memory": "one", "global-memory": coordination.GlobalMemoryRole} {
		_, err := kernelDB.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES(?,'memory',?,?,1,?,?)`, p.ownerID, id, role, []byte(`{"allowContextUse":true,"content":{"value":"shared"}}`), time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{ResourceKind: "memory"})
	if err != nil || len(snapshot.Resources) != 2 {
		t.Fatalf("shared snapshot=%+v err=%v", snapshot.Resources, err)
	}
	scope.MemoryRoleID = coordination.GlobalMemoryRole
	snapshot, err = p.Snapshot(t.Context(), scope, coordination.DataQuery{ResourceKind: "memory"})
	if err != nil || len(snapshot.Resources) != 1 || snapshot.Resources[0].RoleID != coordination.GlobalMemoryRole {
		t.Fatalf("global snapshot=%+v err=%v", snapshot.Resources, err)
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
	historyMessages := 0
	if history != nil {
		for _, resource := range history.Resources {
			if resource.Kind == "message" {
				historyMessages++
			}
		}
	}
	if err != nil || history == nil || history.OwnerID != "device-a" || history.Role.ID != "one" || string(history.Role.Profile) != `{}` || historyMessages != 1 {
		t.Fatalf("history did not preserve owner or leaked device role configuration: %+v %v", history, err)
	}
	if scope.RoleID != "core-role" || scope.ResourceOwnerID != "core" {
		t.Fatal("history changed execution authority")
	}
	if err := p.services.DB.Model(&character.Character{}).Where("id='one'").Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	history, err = p.HistoricalSnapshot(t.Context(), scope, coordination.DataQuery{ConversationID: conversation.ID})
	historyMessages = 0
	if history != nil {
		for _, resource := range history.Resources {
			if resource.Kind == "message" {
				historyMessages++
			}
		}
	}
	if err != nil || history == nil || historyMessages != 1 {
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

func TestMeshSnapshotDeduplicatesOwnedAndLegacyChatByAuthority(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3}
	seedOwnedChatManagement(t, p, "conversation", "shared", "one", `{"id":"shared","title":"Current","characterId":"one","createdAt":"2026-02-01T00:00:00Z"}`)
	seedOwnedChatManagement(t, p, "message", "shared-message", "one", `{"id":"shared-message","conversationId":"shared","role":"user","content":"current","createdAt":"2026-02-01T00:00:01Z"}`)
	if err := p.services.DB.Create(&chat.Conversation{ID: "shared", SpaceID: p.legacySpaceID, Title: "Legacy"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, message := range []chat.Message{
		{ID: "shared-message", ConversationID: "shared", CharacterID: "one", Role: "user", Content: "legacy duplicate", Sequence: 1},
		{ID: "legacy-only", ConversationID: "shared", CharacterID: "one", Role: "assistant", Content: "legacy only", Sequence: 2},
	} {
		if err := p.services.DB.Create(&message).Error; err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{ConversationID: "shared", Limit: 128})
	if err != nil {
		t.Fatal(err)
	}
	conversations, messages := 0, 0
	for _, resource := range snapshot.Resources {
		switch resource.Kind {
		case "conversation":
			conversations++
		case "message":
			messages++
			var body map[string]any
			if err := json.Unmarshal(resource.Body, &body); err != nil {
				t.Fatal(err)
			}
			if resource.ID == "shared-message" && (body["content"] != "current" || resource.Origin == "legacy") {
				t.Fatalf("legacy duplicate overrode owner message: %+v", resource)
			}
			if resource.ID == "legacy-only" && (resource.Origin != "legacy" || !resource.ReadOnly || resource.Revision != 0) {
				t.Fatalf("legacy-only message missing read-only authority: %+v", resource)
			}
		}
	}
	if conversations != 1 || messages != 2 {
		t.Fatalf("canonical snapshot conversations=%d messages=%d", conversations, messages)
	}
	if _, err := p.services.KernelContainer.DeviceRegistry.Database().Exec(`UPDATE kernel_device_owned_resources SET deleted=1,body='null',revision=revision+1 WHERE owner_id=? AND kind='message' AND resource_id='shared-message'`, p.ownerID); err != nil {
		t.Fatal(err)
	}
	snapshot, err = p.Snapshot(t.Context(), scope, coordination.DataQuery{ConversationID: "shared", Limit: 128})
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range snapshot.Resources {
		if resource.Kind == "message" && resource.ID == "shared-message" {
			t.Fatal("legacy message was resurrected after owner tombstone")
		}
	}
}
