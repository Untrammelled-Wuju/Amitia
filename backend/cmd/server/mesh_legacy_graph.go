package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memory"
)

func legacyGraphID(value any) string {
	if value == nil {
		return ""
	}
	if record, ok := value.(map[string]any); ok {
		if record["tb"] != nil && record["id"] != nil {
			return fmt.Sprint(record["tb"]) + ":" + fmt.Sprint(record["id"])
		}
	}
	return fmt.Sprint(value)
}

func (p *meshLocalDataPort) legacyGraphRows(ctx context.Context, memories []json.RawMessage) ([]json.RawMessage, error) {
	nodes, edges := []map[string]any{}, []map[string]any{}
	if p.services.Graph != nil {
		var err error
		nodes, err = p.services.Graph.GetAllNodes(p.legacySpaceID)
		if err != nil {
			return nil, err
		}
		edges, err = p.services.Graph.GetAllEdges(p.legacySpaceID)
		if err != nil {
			return nil, err
		}
	}
	byID := map[string]map[string]any{}
	roots := map[string]map[string]any{}
	for _, node := range nodes {
		id := legacyGraphID(node["id"])
		if id == "" {
			continue
		}
		byID[id] = node
		if props, ok := node["properties"].(map[string]any); ok && props["source_memory_id"] != nil {
			roots[fmt.Sprint(props["source_memory_id"])] = node
		}
	}
	rows := []json.RawMessage{}
	for _, raw := range memories {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var row memory.Memory
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		partition := row.CharacterID
		if row.Scope == "user" || row.Scope == "user_global" {
			partition = coordination.GlobalMemoryRole
		}
		if partition == "" {
			continue
		}
		root := roots[row.ID]
		rootID := "memory:" + row.ID
		if root != nil {
			rootID = legacyGraphID(root["id"])
		}
		graphNodes := []map[string]any{{"id": rootID, "label": row.Key}}
		graphEdges := []map[string]any{}
		seen := map[string]bool{rootID: true}
		for _, edge := range edges {
			source, target := legacyGraphID(edge["in"]), legacyGraphID(edge["out"])
			if source != rootID && target != rootID {
				continue
			}
			other := source
			if other == rootID {
				other = target
			}
			node := byID[other]
			if node == nil {
				continue
			}
			if node["entity_type"] == "memory" {
				continue
			}
			props, _ := node["properties"].(map[string]any)
			if props["source_memory_id"] != nil && fmt.Sprint(props["source_memory_id"]) != row.ID {
				continue
			}
			if actor, ok := props["character_id"].(string); ok && actor != "" && actor != row.CharacterID {
				continue
			}
			if !seen[other] {
				seen[other] = true
				graphNodes = append(graphNodes, map[string]any{"id": other, "label": node["label"]})
			}
			graphEdges = append(graphEdges, map[string]any{"source": source, "target": target, "relation": edge["relation_type"]})
			if len(graphEdges) >= 128 {
				break
			}
		}
		body, _ := json.Marshal(map[string]any{"id": "legacy/" + row.ID, "kind": "graph", "ownerId": p.ownerID, "roleId": partition, "revision": 0, "body": map[string]any{"key": row.Key, "legacyMemory": json.RawMessage(raw), "content": map[string]any{"nodes": graphNodes, "edges": graphEdges}}})
		rows = append(rows, body)
	}
	return rows, nil
}
