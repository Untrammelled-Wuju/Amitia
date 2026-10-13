package business

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) LinkOwnedMemories(ctx context.Context, request Request, sourceID, targetID, relation string, weight float64) (coordination.Acknowledgement, error) {
	if sourceID == "" || targetID == "" || sourceID == targetID || len(relation) > 128 || math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 1 {
		return coordination.Acknowledgement{}, errors.New("记忆关联参数无效")
	}
	if relation == "" {
		relation = "related_to"
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return coordination.Acknowledgement{}, coordination.ErrWrongOwner
	}
	dependencies := []coordination.ResourceVersion{}
	for _, id := range []string{sourceID, targetID} {
		row, err := port.Resource(ctx, scope, "memory", id)
		if err != nil {
			return coordination.Acknowledgement{}, err
		}
		if row == nil || !MemoryActive(MemoryConsumerEntry{Resource: *row}) {
			return coordination.Acknowledgement{}, coordination.ErrResourceVersion
		}
		dependencies = append(dependencies, coordination.ResourceVersion{Kind: "memory", ID: id, Revision: row.Revision})
	}
	id := "memory-relation/" + memoryManagementHash([]string{sourceID, targetID, relation})[:32]
	previous, err := port.Resource(ctx, scope, "graph", id)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	revision := int64(0)
	if previous != nil {
		revision = previous.Revision
	}
	document := map[string]interface{}{"key": id, "executionScope": scope, "content": map[string]interface{}{"relation": relation, "target": targetID, "weight": weight, "targetRevision": dependencies[1].Revision, "sourceRevision": dependencies[0].Revision, "targetKind": "memory"}}
	return e.commit(ctx, coordination.Commit{Scope: scope, Dependencies: dependencies, Mutations: []coordination.Mutation{{Kind: "graph", ID: id, RoleID: coordination.StorageRole(scope, "graph"), SourceID: sourceID, ExpectedRevision: revision, Body: body(document)}}})
}

func (e *Engine) OwnedMemoryLinks(ctx context.Context, request Request, id string, depth int) ([]coordination.Resource, error) {
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return nil, err
	}
	defer finish()
	memories, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return nil, err
	}
	active := map[string]int64{}
	for _, row := range memories.Resources {
		if coordination.ResourceUsable(row.Body, time.Now()) {
			active[row.ID] = row.Revision
		}
	}
	if active[id] == 0 {
		return nil, coordination.ErrResourceVersion
	}
	graphs, err := e.memoryManagementSnapshot(ctx, scope, "graph", "")
	if err != nil {
		return nil, err
	}
	depth = max(1, min(depth, 3))
	frontier := map[string]bool{id: true}
	visited := map[string]bool{}
	selected := []coordination.Resource{}
	for step := 0; step < depth; step++ {
		next := map[string]bool{}
		for _, graph := range graphs.Resources {
			if visited[graph.ID] || active[graph.SourceID] == 0 || !coordination.ResourceUsable(graph.Body, time.Now()) {
				continue
			}
			var document struct {
				Content struct {
					Target         string `json:"target"`
					TargetKind     string `json:"targetKind"`
					TargetRevision int64  `json:"targetRevision"`
					SourceRevision int64  `json:"sourceRevision"`
				} `json:"content"`
			}
			if json.Unmarshal(graph.Body, &document) != nil {
				return nil, errors.New("记忆关联内容无效")
			}
			target := strings.TrimPrefix(document.Content.Target, "memory:")
			if document.Content.TargetKind == "memory" && active[target] == 0 {
				continue
			}
			if !frontier[graph.SourceID] && !frontier[target] {
				continue
			}
			visited[graph.ID] = true
			selected = append(selected, graph)
			next[graph.SourceID], next[target] = true, true
		}
		frontier = next
	}
	return selected, e.coordination.Validate(ctx, scope)
}
