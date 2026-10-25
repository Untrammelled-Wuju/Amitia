package tool

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type toolMemoryTestRuntime struct {
	db     *sql.DB
	engine *business.Engine
	space  string
}

var currentToolMemoryTestRuntime *toolMemoryTestRuntime

func newToolMemoryTestRuntime(t *testing.T, space string) *toolMemoryTestRuntime {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "owned.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close(); SetOwnedMemoryRuntime(nil); currentToolMemoryTestRuntime = nil })
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device',?,'trusted','now','now')`, space); err != nil {
		t.Fatal(err)
	}
	p := &toolMemoryTestRuntime{db: db, space: space}
	p.engine = business.NewEngine(coordination.NewService(db), p, nil)
	SetOwnedMemoryRuntime(p)
	currentToolMemoryTestRuntime = p
	return p
}
func (p *toolMemoryTestRuntime) Roles(context.Context, coordination.ExecutionScope) ([]coordination.Role, error) {
	roles := []coordination.Role{}
	for _, id := range []string{"char-a", "char-b", "char-profile-a", "char-profile-b", "char-memory", "char-invalid", "char-cancel", "char-audit", "char-force", "char-1", "char-2"} {
		roles = append(roles, coordination.Role{ID: id, Revision: 1, Profile: json.RawMessage(`{}`)})
	}
	return roles, nil
}
func (p *toolMemoryTestRuntime) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	roles, _ := p.Roles(ctx, scope)
	role, err := coordination.ResolveRole(scope.RoleID, roles)
	if err != nil {
		return coordination.DataSnapshot{}, err
	}
	snapshot := coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: role}
	for _, kind := range []string{"memory", "fact", "profile", "episodic", "vector", "graph", "checkpoint", "message", "conversation", "working", "summary"} {
		if query.ResourceKind != "" && query.ResourceKind != kind {
			continue
		}
		rows, err := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).List(ctx, kind, coordination.StorageRole(scope, kind), false)
		if err != nil {
			return snapshot, err
		}
		snapshot.Resources = append(snapshot.Resources, rows...)
	}
	return snapshot, nil
}
func (p *toolMemoryTestRuntime) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	return coordination.NewOwnershipStore(p.db, commit.Scope.ResourceOwnerID).Apply(ctx, commit)
}
func (p *toolMemoryTestRuntime) SaveInterrupted(ctx context.Context, reply coordination.InterruptedReply) error {
	return coordination.NewOwnershipStore(p.db, reply.Scope.ResourceOwnerID).SaveInterrupted(ctx, reply)
}
func (p *toolMemoryTestRuntime) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	row, err := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).Get(ctx, kind, id)
	if row != nil && row.RoleID != coordination.StorageRole(scope, kind) {
		return nil, coordination.ErrWrongOwner
	}
	return row, err
}
func (p *toolMemoryTestRuntime) ExecuteMemory(ctx context.Context, execution ToolExecutionContext, name string, args map[string]interface{}) (business.MemoryToolResponse, error) {
	request := business.Request{SpaceID: p.space, DeviceID: "device", CoreID: p.space, RoleID: execution.CharacterID, ConversationID: execution.ConversationID, RequestID: execution.RequestID + "/" + execution.ToolCallID, Message: execution.UserMessage}
	return p.engine.MemoryTool(ctx, request, name, args)
}

func TestOwnedMemoryToolDispatchPersistsAndIsolatesRoles(t *testing.T) {
	_, _, cleanup := setupToolTestDB(t)
	t.Cleanup(cleanup)
	for _, role := range []string{"char-a", "char-b"} {
		ctx := ToolExecutionContext{CharacterID: role, ConversationID: "conv-" + role, RequestID: "req-" + role, ToolCallID: "call"}
		result, ok := ExecuteWithContext(ctx, "save_memory", `{"key":"颜色","value":"`+role+`"}`)
		if !ok || result.Status != ToolStatusSuccess || len(result.SideEffects) < 1 || !result.SideEffects[0].Confirmed {
			t.Fatalf("save=%+v", result)
		}
		read, ok := ExecuteMemoryWithContext(ctx, "query_memory", `{"query":"颜色","mode":"keyword"}`)
		if !ok || read.Status != ToolStatusSuccess || !strings.Contains(read.Content, role) || strings.Contains(read.Content, otherTestRole(role)) {
			t.Fatalf("role leaked: %+v", read)
		}
	}
}
func otherTestRole(role string) string {
	if role == "char-a" {
		return "char-b"
	}
	return "char-a"
}

type unconfirmedMemoryTestRuntime struct{}

func (unconfirmedMemoryTestRuntime) ExecuteMemory(context.Context, ToolExecutionContext, string, map[string]interface{}) (business.MemoryToolResponse, error) {
	return business.MemoryToolResponse{Text: "saved", Saved: false}, nil
}
func TestOwnedMemoryToolRejectsUnconfirmedSave(t *testing.T) {
	SetOwnedMemoryRuntime(unconfirmedMemoryTestRuntime{})
	t.Cleanup(func() { SetOwnedMemoryRuntime(nil) })
	result := executeOwnedMemoryTool(t.Context(), ToolExecutionContext{CharacterID: "role", ConversationID: "chat"}, "save_memory", map[string]interface{}{"key": "a", "value": "b"})
	if result.Status != ToolStatusFailed || len(result.SideEffects) != 0 {
		t.Fatalf("unconfirmed save reported success: %+v", result)
	}
}

func TestOwnedMemoryToolSaveValidationAndReplay(t *testing.T) {
	p := newToolMemoryTestRuntime(t, "space")
	execution := ToolExecutionContext{CharacterID: "char-a", ConversationID: "conv-char-a", RequestID: "request", ToolCallID: "call"}
	args := map[string]interface{}{"key": "计划", "value": "下周出差", "expiresAt": "2026-12-31", "entityId": "550e8400-e29b-41d4-a716-446655440000"}
	first, err := p.ExecuteMemory(t.Context(), execution, "save_memory", args)
	if err != nil || !first.Saved {
		t.Fatalf("save=%+v %v", first, err)
	}
	replayed, err := p.ExecuteMemory(t.Context(), execution, "save_memory", args)
	if err != nil || replayed.Acknowledgement.Versions[first.Resources[0].Kind+"/"+first.Resources[0].ID] != first.Resources[0].Revision {
		t.Fatalf("replay=%+v %v", replayed, err)
	}
	args["value"] = "另一项计划"
	if _, err = p.ExecuteMemory(t.Context(), execution, "save_memory", args); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("changed replay accepted: %v", err)
	}
	execution.RequestID = "invalid"
	args["expiresAt"] = "2026/12/31"
	if _, err = p.ExecuteMemory(t.Context(), execution, "save_memory", args); err == nil {
		t.Fatal("invalid expiry saved")
	}
}

func TestOwnedMemoryToolGlobalScopeUsesOnlyUserRequest(t *testing.T) {
	p := newToolMemoryTestRuntime(t, "space")
	denied := ToolExecutionContext{CharacterID: "char-a", ConversationID: "conv-char-a", RequestID: "global-denied", ToolCallID: "call", UserMessage: "记住我喜欢茶"}
	if _, err := p.ExecuteMemory(t.Context(), denied, "save_memory", map[string]interface{}{"key": "饮品", "value": "茶", "scope": "global"}); err == nil {
		t.Fatal("global memory accepted without explicit user request")
	}
	allowed := ToolExecutionContext{CharacterID: "char-a", ConversationID: "conv-char-a", RequestID: "global-allowed", ToolCallID: "call", UserMessage: "把这条保存到全局记忆"}
	if response, err := p.ExecuteMemory(t.Context(), allowed, "save_memory", map[string]interface{}{"key": "饮品", "value": "茶", "scope": "global"}); err != nil || !response.Saved {
		t.Fatalf("explicit global save failed: %+v %v", response, err)
	}
}
