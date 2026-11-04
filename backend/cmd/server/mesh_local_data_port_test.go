package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/agentplatform"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func TestProjectGroupResourceForSnapshotRemovesDuplicatedAvatars(t *testing.T) {
	avatar := strings.Repeat("x", 256<<10)
	round := map[string]any{
		"roundId":        "round",
		"conversationId": "chat",
		"deliveries": []any{
			map[string]any{"deliveryId": "one", "characterAvatar": avatar},
			map[string]any{"deliveryId": "two", "characterAvatar": avatar},
		},
	}
	encoded, err := json.Marshal(round)
	if err != nil {
		t.Fatal(err)
	}
	projected, err := projectGroupResourceForSnapshot(coordination.Resource{Kind: "group-round", Body: encoded})
	if err != nil {
		t.Fatal(err)
	}
	var restored map[string]any
	if err := json.Unmarshal(projected.Body, &restored); err != nil {
		t.Fatal(err)
	}
	deliveries, _ := restored["deliveries"].([]any)
	if len(projected.Body) > 4096 || len(deliveries) != 2 {
		t.Fatalf("群轮次快照仍包含重复头像: bytes=%d deliveries=%d", len(projected.Body), len(deliveries))
	}
	for _, raw := range deliveries {
		delivery, _ := raw.(map[string]any)
		if delivery["characterAvatar"] != nil {
			t.Fatal("群轮次快照保留了投递头像")
		}
	}
	conversation := map[string]any{
		"conversationId":   "chat",
		"conversationType": "group",
		"members": []any{
			map[string]any{"characterId": "one", "characterAvatar": avatar},
			map[string]any{"characterId": "two", "characterAvatar": avatar},
		},
	}
	encoded, err = json.Marshal(conversation)
	if err != nil {
		t.Fatal(err)
	}
	projected, err = projectGroupResourceForSnapshot(coordination.Resource{Kind: "conversation", Body: encoded})
	if err != nil {
		t.Fatal(err)
	}
	var projectedConversation map[string]any
	if err := json.Unmarshal(projected.Body, &projectedConversation); err != nil {
		t.Fatal(err)
	}
	projectedMembers, _ := projectedConversation["members"].([]any)
	if len(projected.Body) > 4096 || len(projectedMembers) != 2 {
		t.Fatalf("群会话快照仍包含重复头像: bytes=%d members=%d", len(projected.Body), len(projectedMembers))
	}
	for _, raw := range projectedMembers {
		member, _ := raw.(map[string]any)
		if member["characterAvatar"] != nil {
			t.Fatal("群会话快照保留了成员头像")
		}
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
	platform, err := agentplatform.NewService(db, agentplatform.ServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []character.Character{{ID: "one", Name: "one", SpaceID: p.spaceID, Status: "enabled", Revision: 3}, {ID: "two", Name: "two", SpaceID: p.spaceID, Status: "enabled", Revision: 1}, {ID: "disabled", Name: "disabled", SpaceID: p.spaceID, Status: "disabled", Revision: 1}, {ID: "foreign", Name: "foreign", SpaceID: "another-space", Status: "enabled", Revision: 1}} {
		if err := db.Create(&role).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := platform.SyncRoleAgent(db, agentplatform.RoleAgentSyncInput{SpaceID: role.SpaceID, CharacterID: role.ID, Name: role.Name, Status: role.Status}); err != nil {
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
	seedOwnedChatManagement(t, p, "conversation", "current-chat", "one", `{"id":"current-chat","title":"current","characterId":"one","createdAt":"2026-10-01T00:00:00Z"}`)
	seedOwnedChatManagement(t, p, "message", "first", "one", `{"id":"first","conversationId":"current-chat","role":"user","content":"allowed","createdAt":"2026-10-01T00:00:01Z"}`)
	seedOwnedChatManagement(t, p, "message", "second", "two", `{"id":"second","conversationId":"current-chat","role":"assistant","content":"other-role","createdAt":"2026-10-01T00:00:02Z"}`)
	snapshot, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{ConversationID: "current-chat"})
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
