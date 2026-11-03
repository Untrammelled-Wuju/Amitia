package business

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestWorkerInferenceSnapshotRestrictsRoleMemoryAndRuntimeContext(t *testing.T) {
	snapshot := coordination.DataSnapshot{
		Resources: []coordination.Resource{
			{Kind: "message", ID: "current"},
			{Kind: "summary", ID: "summary"},
			{Kind: "project", ID: "project"},
			{Kind: "profile", ID: "profile"},
			{Kind: "episodic", ID: "episodic"},
			{Kind: "relationship", ID: "relationship"},
			{Kind: "memory", ID: "memory-profile", Body: json.RawMessage(`{"content":{"memoryType":"profile","value":"private"}}`)},
			{Kind: "fact", ID: "fact"},
			{Kind: "vector", ID: "vector"},
			{Kind: "worldbook", ID: "worldbook"},
		},
		RuntimeContext: map[string]json.RawMessage{
			"psyche":     json.RawMessage(`{"status":"ready"}`),
			"continuity": json.RawMessage(`{"status":"ready"}`),
		},
		RecallEvidence: []coordination.ResourceVersion{{Kind: "memory", ID: "profile"}},
	}
	coding := workerInferenceSnapshot("coding_worker", snapshot)
	if len(coding.RuntimeContext) != 1 || coding.RuntimeContext["continuity"] == nil || len(coding.RecallEvidence) != 0 {
		t.Fatalf("coding worker retained private runtime context: %+v", coding)
	}
	if resourceKinds(coding.Resources)["profile"] || resourceKinds(coding.Resources)["episodic"] || resourceKinds(coding.Resources)["worldbook"] {
		t.Fatalf("coding worker retained unrelated context: %+v", coding.Resources)
	}
	if !resourceKinds(coding.Resources)["message"] || !resourceKinds(coding.Resources)["project"] {
		t.Fatalf("coding worker lost task context: %+v", coding.Resources)
	}
	research := workerInferenceSnapshot("research_worker", snapshot)
	if !resourceKinds(research.Resources)["fact"] || !resourceKinds(research.Resources)["vector"] || !resourceKinds(research.Resources)["worldbook"] {
		t.Fatalf("research worker lost knowledge context: %+v", research.Resources)
	}
	if resourceKinds(research.Resources)["memory-profile"] || resourceKinds(research.Resources)["profile"] || resourceKinds(research.Resources)["episodic"] {
		t.Fatalf("research worker retained private context: %+v", research.Resources)
	}
}

func resourceKinds(resources []coordination.Resource) map[string]bool {
	result := make(map[string]bool, len(resources))
	for _, resource := range resources {
		result[resource.Kind] = true
		result[resource.ID] = true
	}
	return result
}
