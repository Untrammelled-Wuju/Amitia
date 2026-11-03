package business

import (
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

var workerCommonResourceKinds = map[string]struct{}{
	"conversation":     {},
	"message":          {},
	"summary":          {},
	"working":          {},
	"continuity":       {},
	"checkpoint":       {},
	"project":          {},
	"tool-result":      {},
	"group-round":      {},
	"group-delivery":   {},
	"group-invitation": {},
}

func workerResourceKinds(role string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(workerCommonResourceKinds)+6)
	for kind := range workerCommonResourceKinds {
		allowed[kind] = struct{}{}
	}
	switch strings.TrimSpace(role) {
	case "research_worker":
		for _, kind := range []string{"memory", "fact", "vector", "graph", "worldbook"} {
			allowed[kind] = struct{}{}
		}
	case "operator_worker":
		allowed["worldbook"] = struct{}{}
	}
	return allowed
}

func workerMemoryKindAllowed(kind string, body json.RawMessage) bool {
	if kind != "memory" {
		return true
	}
	var document struct {
		Content struct {
			MemoryType string `json:"memoryType"`
			Type       string `json:"type"`
			Kind       string `json:"kind"`
		} `json:"content"`
	}
	if json.Unmarshal(body, &document) != nil {
		return false
	}
	for _, value := range []string{document.Content.MemoryType, document.Content.Type, document.Content.Kind} {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "profile", "episodic", "working":
			return false
		}
	}
	return true
}

func workerInferenceSnapshot(role string, snapshot coordination.DataSnapshot) coordination.DataSnapshot {
	if strings.TrimSpace(role) == "" {
		return snapshot
	}
	allowed := workerResourceKinds(role)
	resources := make([]coordination.Resource, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		if _, ok := allowed[resource.Kind]; !ok {
			continue
		}
		if !workerMemoryKindAllowed(resource.Kind, resource.Body) {
			continue
		}
		resources = append(resources, resource)
	}
	snapshot.Resources = resources
	snapshot.RecallEvidence = nil
	if continuity, ok := snapshot.RuntimeContext["continuity"]; ok {
		snapshot.RuntimeContext = map[string]json.RawMessage{"continuity": continuity}
	} else {
		snapshot.RuntimeContext = nil
	}
	if snapshot.Group != nil {
		group := *snapshot.Group
		group.Members = append([]coordination.GroupMember(nil), snapshot.Group.Members...)
		snapshot.Group = &group
	}
	return snapshot
}

func workerHistoricalSnapshot(role string, snapshot *coordination.DataSnapshot) *coordination.DataSnapshot {
	if snapshot == nil || strings.TrimSpace(role) == "" {
		return snapshot
	}
	filtered := workerInferenceSnapshot(role, *snapshot)
	return &filtered
}
