package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type ownedExecutionFixture struct {
	engine *business.Engine
	model  *service
	db     *sql.DB
	port   *ownedExecutionData
}

type ownedExecutionData struct {
	db   *sql.DB
	role coordination.Role
}

func (p *ownedExecutionData) Roles(context.Context, coordination.ExecutionScope) ([]coordination.Role, error) {
	return []coordination.Role{p.role}, nil
}
func (p *ownedExecutionData) Snapshot(ctx context.Context, scope coordination.ExecutionScope, _ coordination.DataQuery) (coordination.DataSnapshot, error) {
	snapshot := coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID, Role: p.role}
	for _, kind := range []string{"conversation", "message", "checkpoint", "memory", "fact", "summary", "working", "profile", "episodic", "vector", "graph", "psyche", "relationship", "needs"} {
		rows, err := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).List(ctx, kind, coordination.StorageRole(scope, kind), false)
		if err != nil {
			return snapshot, err
		}
		snapshot.Resources = append(snapshot.Resources, rows...)
	}
	return snapshot, nil
}
func (p *ownedExecutionData) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	return coordination.NewOwnershipStore(p.db, commit.Scope.ResourceOwnerID).Apply(ctx, commit)
}
func (p *ownedExecutionData) SaveInterrupted(ctx context.Context, reply coordination.InterruptedReply) error {
	return coordination.NewOwnershipStore(p.db, reply.Scope.ResourceOwnerID).SaveInterrupted(ctx, reply)
}
func (p *ownedExecutionData) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	row, err := coordination.NewOwnershipStore(p.db, scope.ResourceOwnerID).Get(ctx, kind, id)
	if row != nil && row.RoleID != coordination.StorageRole(scope, kind) {
		return nil, coordination.ErrWrongOwner
	}
	return row, err
}

type ownedExecutionModel struct{ *service }

func (m ownedExecutionModel) ExtractOwnedMemory(context.Context, business.Inference, business.Generation) ([]business.DerivedMemory, error) {
	return nil, nil
}
func (m ownedExecutionModel) PlanOwnedTitle(context.Context, business.Inference, business.Generation) (string, error) {
	return "", nil
}

func newOwnedExecutionFixture(t *testing.T) *ownedExecutionFixture {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "owned.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err = kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('device','space','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	modelDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "models.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = modelDB.AutoMigrate(&ModelConfig{}); err != nil {
		t.Fatal(err)
	}
	if err = modelDB.Exec(`CREATE TABLE model_scenario_routes (scenario TEXT PRIMARY KEY, model_config_id INTEGER NOT NULL DEFAULT 0)`).Error; err != nil {
		t.Fatal(err)
	}
	if err = modelDB.Create(&ModelConfig{Name: "owned-test", APIType: "openai-compatible", BaseURL: "http://127.0.0.1", ModelName: "owned-test", IsActive: 1, MaxTokens: 128}).Error; err != nil {
		t.Fatal(err)
	}
	sqlModelDB, _ := modelDB.DB()
	t.Cleanup(func() { _ = sqlModelDB.Close() })
	model := &service{repo: NewRepository(app.NewAppContext(modelDB, nil)), db: modelDB}
	port := &ownedExecutionData{db: db, role: coordination.Role{ID: "role", Revision: 1, Profile: json.RawMessage(`{"characterId":"role","name":"测试角色","identity":"伙伴","globalMemoryEnabled":false}`)}}
	return &ownedExecutionFixture{engine: business.NewEngine(coordination.NewService(db), port, ownedExecutionModel{model}), model: model, db: db, port: port}
}

func (f *ownedExecutionFixture) request(id string) business.Request {
	return business.Request{SpaceID: "space", DeviceID: "device", CoreID: "space", RoleID: "role", ConversationID: "conversation", RequestID: id, Message: "你好"}
}

type ownedTestMessage struct {
	ID                  string
	Role                string             `json:"role"`
	Content             string             `json:"content"`
	CitationSnapshot    []CitationSnapshot `json:"citationSnapshot"`
	RequestID           string             `json:"requestId"`
	ConversationID      string             `json:"conversationId"`
	Sequence            int64              `json:"sequence"`
	Reasoning           string             `json:"reasoningContent"`
	ReasoningDurationMS int64              `json:"reasoningDurationMs"`
	Tokens              int                `json:"tokens"`
}

func (f *ownedExecutionFixture) messages(t *testing.T) []ownedTestMessage {
	t.Helper()
	rows, err := coordination.NewOwnershipStore(f.db, "device").List(t.Context(), "message", "role", false)
	if err != nil {
		t.Fatal(err)
	}
	result := make([]ownedTestMessage, 0, len(rows))
	for _, row := range rows {
		var item ownedTestMessage
		if err = json.Unmarshal(row.Body, &item); err != nil {
			t.Fatal(err)
		}
		item.ID = row.ID
		result = append(result, item)
	}
	return result
}

func (f *ownedExecutionFixture) runtimeState(t *testing.T, kind, id string) map[string]any {
	t.Helper()
	resource, err := coordination.NewOwnershipStore(f.db, "device").Get(t.Context(), kind, id)
	if err != nil {
		t.Fatal(err)
	}
	if resource == nil {
		t.Fatalf("%s/%s not found", kind, id)
	}
	var value map[string]any
	if err := json.Unmarshal(resource.Body, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
