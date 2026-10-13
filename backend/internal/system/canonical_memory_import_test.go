package system

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/gorm"
)

type importMemoryPort struct{ db *sql.DB }

func (p importMemoryPort) Roles(context.Context, coordination.ExecutionScope) ([]coordination.Role, error) {
	return []coordination.Role{{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role","globalMemoryEnabled":false}`)}}, nil
}
func (p importMemoryPort) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	roles, _ := p.Roles(ctx, scope)
	result := coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: roles[0], Resources: []coordination.Resource{}}
	for _, kind := range []string{"message", "checkpoint", "memory", "fact"} {
		rows, err := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).List(ctx, kind, coordination.StorageRole(scope, kind), false)
		if err != nil {
			return result, err
		}
		result.Resources = append(result.Resources, rows...)
	}
	return result, nil
}
func (p importMemoryPort) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	return coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).Get(ctx, kind, id)
}
func (p importMemoryPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	return coordination.NewOwnershipStore(p.db, commit.Scope.ResourceOwnerID).Apply(ctx, commit)
}
func (p importMemoryPort) SaveInterrupted(ctx context.Context, reply coordination.InterruptedReply) error {
	return coordination.NewOwnershipStore(p.db, reply.Scope.ResourceOwnerID).SaveInterrupted(ctx, reply)
}

type importMemoryModel struct{}

func (importMemoryModel) GenerateOwnedReply(context.Context, business.Inference) (business.Generation, error) {
	return business.Generation{Text: "已记录"}, nil
}
func (importMemoryModel) ExtractOwnedMemory(context.Context, business.Inference, business.Generation) ([]business.DerivedMemory, error) {
	return nil, nil
}
func (importMemoryModel) ExtractOwnedMemoryCandidates(context.Context, business.Inference, business.Generation) ([]business.DerivedMemory, error) {
	return []business.DerivedMemory{{Kind: "fact", Key: "饮品", Body: json.RawMessage(`{"value":"喜欢茶","importance":7}`)}}, nil
}

func TestImportCandidatesArePersistedAndConfirmedThroughOwnedMemoryHTTP(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('phone','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	engine := business.NewEngine(coordination.NewService(db), importMemoryPort{db}, importMemoryModel{})
	request := business.Request{SpaceID: "core", DeviceID: "phone", CoreID: "core", RoleID: "role", RequestID: "seed", ConversationID: "imported", Message: "我喜欢喝茶"}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	mainDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "main.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mainSQL, _ := mainDB.DB()
	defer mainSQL.Close()
	for _, query := range []string{`CREATE TABLE conversations(id TEXT PRIMARY KEY,space_id TEXT,source TEXT,deleted_at TEXT)`, `CREATE TABLE messages(id TEXT PRIMARY KEY,conversation_id TEXT,character_id TEXT)`, `INSERT INTO conversations VALUES('imported','core','import',NULL)`, `INSERT INTO messages VALUES('source-message','imported','role')`} {
		if err := mainDB.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	handler := &Handler{db: mainDB, canonicalMemory: engine, memoryCoreID: "core"}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("actorContext", &auth.ActorContext{SpaceID: runtimeidentity.SpaceID("core"), DeviceID: runtimeidentity.DeviceID("phone"), IsLocalTrusted: true})
		c.Next()
	})
	router.POST("/imports/:id/extract", handler.ExtractImportsMemoryCandidates)
	router.POST("/imports/:id/confirm", handler.ConfirmImportsBatchMemories)
	call := func(path, payload string) map[string]any {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		var result map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	extracted := call("/imports/imported/extract", `{}`)
	payload, ok := extracted["data"].(map[string]any)
	if !ok {
		t.Fatalf("extract=%v", extracted)
	}
	candidates, ok := payload["candidates"].([]any)
	if !ok || len(candidates) != 1 {
		t.Fatalf("candidates=%v", extracted)
	}
	id := candidates[0].(map[string]any)["id"].(string)
	confirmed := call("/imports/imported/confirm", `{"selectedIds":["`+id+`"]}`)
	payload, ok = confirmed["data"].(map[string]any)
	if !ok || payload["confirmed"] != true || payload["memoriesCreated"] != float64(1) {
		t.Fatalf("confirm=%v", confirmed)
	}
	rows, err := coordination.NewOwnershipStore(db, "phone").List(t.Context(), "memory", "role", false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if result := call("/imports/imported/confirm", `{"selectedIds":["unknown"]}`); result["code"] == float64(200) {
		t.Fatal("unknown candidate reported success")
	}
}
