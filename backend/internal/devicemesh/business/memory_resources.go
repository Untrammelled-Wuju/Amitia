package business

import (
	"context"
	"encoding/json"
	"github.com/u-ai/backend/internal/memorypolicy"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) prepareMemoryMutations(ctx context.Context, scope coordination.ExecutionScope, conversation string, derived []DerivedMemory, snapshot *coordination.DataSnapshot) ([]coordination.Mutation, error) {
	derived = append([]DerivedMemory(nil), derived...)
	derived = partitionMemoryDerived(derived)
	protected := map[string]bool{}
	for _, row := range snapshot.Resources {
		if row.Kind != "memory" || row.Deleted {
			continue
		}
		var document map[string]json.RawMessage
		if json.Unmarshal(row.Body, &document) != nil || !memorypolicy.ReadLifecycle(document, time.Now()).Pinned {
			continue
		}
		protected[MemoryText(document, "kind")+"/"+MemoryText(document, "key")] = true
	}
	filtered := derived[:0]
	for _, item := range derived {
		kind := item.Kind
		if kind == "vector" || kind == "graph" {
			kind = item.SourceKind
			if kind == "" {
				kind = "fact"
			}
		}
		if protected[kind+"/"+item.Key] {
			var content map[string]json.RawMessage
			_ = json.Unmarshal(item.Body, &content)
			source := MemoryText(content, "source")
			if source != "manual" && source != "reviewed" {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	derived = filtered
	existing := map[string]bool{}
	indexKey := func(kind, key, sourceKind string) string {
		if sourceKind == "" {
			sourceKind = "fact"
		}
		return kind + "/" + key + "/" + sourceKind
	}
	for _, item := range derived {
		existing[indexKey(item.Kind, item.Key, item.SourceKind)] = true
	}
	for _, item := range append([]DerivedMemory(nil), derived...) {
		if item.Kind != "fact" && item.Kind != "profile" && item.Kind != "episodic" {
			continue
		}
		if !existing[indexKey("vector", item.Key, item.Kind)] {
			if model, ok := e.model.(SemanticModel); ok {
				values, fingerprint, err := model.OwnedQueryVector(ctx, string(item.Body))
				if err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
				}
				if err == nil {
					if err := coordination.ValidateQueryVector(coordination.DataQuery{Vector: values, VectorModel: fingerprint}); err != nil {
						return nil, err
					}
				}
				if err == nil && len(values) > 0 {
					derived = append(derived, DerivedMemory{Kind: "vector", Key: item.Key, SourceKind: item.Kind, Body: body(map[string]interface{}{"values": values, "modelFingerprint": fingerprint})})
					existing[indexKey("vector", item.Key, item.Kind)] = true
				}
			}
		}
		if !existing[indexKey("graph", item.Key, item.Kind)] {
			var content map[string]json.RawMessage
			if json.Unmarshal(item.Body, &content) == nil {
				target := MemoryText(content, "entityId")
				if target == "" {
					target = MemoryText(content, "value")
				}
				if target != "" {
					derived = append(derived, DerivedMemory{Kind: "graph", Key: item.Key, SourceKind: item.Kind, Body: body(map[string]interface{}{"relation": "describes", "target": target, "memoryKind": item.Kind})})
					existing[indexKey("graph", item.Key, item.Kind)] = true
				}
			}
		}
	}
	mutations, err := memoryMutations(scope, conversation, derived, *snapshot)
	if err != nil {
		return nil, err
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return mutations, nil
	}
	seen := make(map[string]bool, len(snapshot.Resources))
	initialCount := len(snapshot.Resources)
	for _, resource := range snapshot.Resources {
		seen[resource.Kind+"/"+resource.ID] = true
	}
	for _, mutation := range mutations {
		key := mutation.Kind + "/" + mutation.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		resource, err := port.Resource(ctx, scope, mutation.Kind, mutation.ID)
		if err != nil {
			return nil, err
		}
		if resource == nil {
			continue
		}
		if resource.Deleted {
			return nil, coordination.ErrResourceVersion
		}
		if resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != coordination.StorageRole(scope, mutation.Kind) || resource.Kind != mutation.Kind || resource.ID != mutation.ID {
			return nil, coordination.ErrWrongOwner
		}
		snapshot.Resources = append(snapshot.Resources, *resource)
	}
	if err := coordination.ValidateSnapshot(scope, *snapshot); err != nil {
		return nil, err
	}
	if len(snapshot.Resources) > initialCount {
		return e.prepareMemoryMutations(ctx, scope, conversation, derived, snapshot)
	}
	return memoryMutations(scope, conversation, derived, *snapshot)
}

func partitionMemoryDerived(input []DerivedMemory) []DerivedMemory {
	result := append([]DerivedMemory(nil), input...)
	mapped := map[string]string{}
	for index, item := range result {
		if item.Kind == "fact" || item.Kind == "profile" || item.Kind == "episodic" {
			var content map[string]json.RawMessage
			if json.Unmarshal(item.Body, &content) == nil && content != nil {
				if value := MemoryValue(content); value != "" {
					content["value"] = body(value)
					result[index].Body = body(content)
					item.Body = result[index].Body
				}
			}
		}
		if item.Kind != "profile" {
			continue
		}
		var content map[string]json.RawMessage
		if json.Unmarshal(item.Body, &content) != nil {
			continue
		}
		category := MemoryText(content, "category")
		if category == "" {
			category = MemoryText(content, "memoryType")
		}
		if category == "" {
			category = "custom"
		}
		attribute := MemoryText(content, "attributeName")
		if attribute == "" {
			attribute = strings.TrimPrefix(item.Key, category+"/")
		}
		key := category + "/" + attribute
		mapped[item.Key] = key
		result[index].Key, result[index].Body = key, body(content)
	}
	for index, item := range result {
		if item.Kind != "vector" && item.Kind != "graph" {
			continue
		}
		if key, ok := mapped[item.Key]; ok && item.SourceKind == "profile" {
			result[index].Key = key
		}
	}
	return result
}
