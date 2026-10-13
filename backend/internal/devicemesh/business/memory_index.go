package business

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) rebuildMemoryInputs(ctx context.Context, scope coordination.ExecutionScope) error {
	memories, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return err
	}
	if len(memories.Resources) == 0 {
		return nil
	}
	model, ok := e.model.(SemanticModel)
	if !ok {
		return errors.New("记忆索引模型入口不可用")
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return coordination.ErrWrongOwner
	}
	graphs, err := e.memoryManagementSnapshot(ctx, scope, "graph", "")
	if err != nil {
		return err
	}
	hasGraph := map[string]bool{}
	for _, graph := range graphs.Resources {
		if coordination.ResourceUsable(graph.Body, time.Now()) {
			hasGraph[graph.SourceID] = true
		}
	}
	for _, memory := range memories.Resources {
		if !coordination.ResourceUsable(memory.Body, time.Now()) {
			continue
		}
		document, content, err := MemoryContent(MemoryConsumerEntry{Resource: memory})
		if err != nil {
			return err
		}
		values, fingerprint, err := model.OwnedQueryVector(ctx, string(body(content)))
		if err != nil {
			return err
		}
		if len(values) == 0 {
			return errors.New("记忆索引模型返回空向量")
		}
		if err := coordination.ValidateQueryVector(coordination.DataQuery{Vector: values, VectorModel: fingerprint}); err != nil {
			return err
		}
		current := scope
		current.RequestID = "memory-index/" + memoryManagementHash([]interface{}{scope.RequestID, memory.ID, memory.Revision})[:48]
		id := "memory-index/" + memoryManagementHash([]string{"vector", memory.ID})[:32]
		previous, err := port.Resource(ctx, current, "vector", id)
		if err != nil {
			return err
		}
		revision := int64(0)
		if previous != nil {
			revision = previous.Revision
		}
		wrapped := body(map[string]interface{}{"key": MemoryText(document, "key"), "executionScope": current, "content": map[string]interface{}{"values": values, "modelFingerprint": fingerprint}})
		mutations := []coordination.Mutation{{Kind: "vector", ID: id, RoleID: coordination.StorageRole(current, "vector"), SourceID: memory.ID, ExpectedRevision: revision, Body: wrapped}}
		if !hasGraph[memory.ID] {
			graphID := "memory-index/" + memoryManagementHash([]string{"graph", memory.ID})[:32]
			previous, err := port.Resource(ctx, current, "graph", graphID)
			if err != nil {
				return err
			}
			graphRevision := int64(0)
			if previous != nil {
				graphRevision = previous.Revision
			}
			target := MemoryText(content, "entityId")
			if target == "" {
				target = MemoryValue(content)
			}
			if target != "" {
				mutations = append(mutations, coordination.Mutation{Kind: "graph", ID: graphID, RoleID: coordination.StorageRole(current, "graph"), SourceID: memory.ID, ExpectedRevision: graphRevision, Body: body(map[string]interface{}{"key": MemoryText(document, "key"), "executionScope": current, "content": map[string]interface{}{"relation": "describes", "target": target, "memoryKind": MemoryText(document, "kind")}})})
			}
		}
		if _, err := e.commit(coordination.WithScope(ctx, current), coordination.Commit{Scope: current, Dependencies: []coordination.ResourceVersion{{Kind: "memory", ID: memory.ID, Revision: memory.Revision}}, Mutations: mutations}); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) countMissingMemoryInputs(ctx context.Context, scope coordination.ExecutionScope) (int64, error) {
	memories, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return 0, err
	}
	vectors, err := e.memoryManagementSnapshot(ctx, scope, "vector", "")
	if err != nil {
		return 0, err
	}
	present := map[string]bool{}
	for _, vector := range vectors.Resources {
		var document struct {
			Content struct {
				Values []float32 `json:"values"`
				Model  string    `json:"modelFingerprint"`
			} `json:"content"`
		}
		if json.Unmarshal(vector.Body, &document) == nil && len(document.Content.Values) > 0 && document.Content.Model != "" {
			present[vector.SourceID] = true
		}
	}
	var missing int64
	for _, memory := range memories.Resources {
		if !present[memory.ID] && coordination.ResourceUsable(memory.Body, time.Now()) {
			missing++
		}
	}
	return missing, nil
}
