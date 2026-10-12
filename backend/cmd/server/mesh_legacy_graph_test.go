package main

import (
	"encoding/json"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/memory"
	"testing"
)

type legacyGraphFixture struct {
	graph.Service
	nodes, edges []map[string]any
}

func (g legacyGraphFixture) GetAllNodes(string) ([]map[string]any, error) { return g.nodes, nil }
func (g legacyGraphFixture) GetAllEdges(string) ([]map[string]any, error) { return g.edges, nil }
func TestMemoryScopeLegacyGraphPreservesRelationsAndPartitions(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	for _, row := range []memory.Memory{{ID: "private", CharacterID: "one", Scope: "character", Key: "私有", SpaceID: p.legacySpaceID}, {ID: "shared", CharacterID: "one", Scope: "user", Key: "全局", SpaceID: p.legacySpaceID}, {ID: "other", CharacterID: "two", Scope: "character", Key: "其他角色", SpaceID: p.legacySpaceID}} {
		if err := p.services.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	p.services.Graph = legacyGraphFixture{Service: graph.NewStubService(), nodes: []map[string]any{{"id": "node:private", "label": "私有", "entity_type": "memory", "properties": map[string]any{"source_memory_id": "private"}}, {"id": "node:shared", "label": "全局", "entity_type": "memory", "properties": map[string]any{"source_memory_id": "shared"}}, {"id": "node:tea", "label": "茶", "entity_type": "entity", "properties": map[string]any{}}}, edges: []map[string]any{{"in": "node:private", "out": "node:tea", "relation_type": "喜欢"}, {"in": "node:shared", "out": "node:tea", "relation_type": "了解"}, {"in": "node:shared", "out": "node:private", "relation_type": "不应跨分区"}}}
	scope := coordination.ExecutionScope{ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, TargetDeviceID: p.ownerID, RoleID: "one", RoleRevision: 3, MemoryRoleID: coordination.GlobalMemoryRole}
	snapshot, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{Management: true, ResourceKind: "graph"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.LegacyGraphs) != 1 {
		t.Fatalf("global graph count %d", len(snapshot.LegacyGraphs))
	}
	var row struct {
		RoleID string `json:"roleId"`
		Body   struct {
			LegacyMemory memory.Memory `json:"legacyMemory"`
			Content      struct {
				Nodes []any            `json:"nodes"`
				Edges []map[string]any `json:"edges"`
			} `json:"content"`
		} `json:"body"`
	}
	if err := json.Unmarshal(snapshot.LegacyGraphs[0], &row); err != nil {
		t.Fatal(err)
	}
	if row.RoleID != coordination.GlobalMemoryRole || row.Body.LegacyMemory.ID != "shared" || len(row.Body.Content.Edges) != 1 || row.Body.Content.Edges[0]["relation"] != "了解" {
		t.Fatalf("graph %+v", row)
	}
	scope.MemoryViewAll = true
	snapshot, err = p.Snapshot(t.Context(), scope, coordination.DataQuery{Management: true, ResourceKind: "graph", Limit: 1})
	if err != nil || len(snapshot.LegacyGraphs) != 1 || snapshot.NextCursors["legacyGraph"] == "" {
		t.Fatalf("paging %+v %v", snapshot, err)
	}
	query := coordination.DataQuery{Management: true, ResourceKind: "graph", Limit: 1, LegacyCursor: snapshot.NextCursors["legacyGraph"]}
	snapshot, err = p.Snapshot(t.Context(), scope, query)
	if err != nil || len(snapshot.LegacyGraphs) != 1 {
		t.Fatalf("next %+v %v", snapshot, err)
	}
}
