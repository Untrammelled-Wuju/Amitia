package business

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
)

func seedLongHorizonMemories(t testing.TB, count int) (*Engine, Request) {
	t.Helper()
	engine, db, _, _ := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "long-horizon"}
	view, err := engine.Query(t.Context(), request, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	mutations := make([]coordination.Mutation, 0, count)
	for index := 0; index < count; index++ {
		key, value := fmt.Sprintf("topic-%03d", index), fmt.Sprintf("长期记忆内容 %03d 用于跨会话召回测试", index)
		if index < 10 {
			key, value = fmt.Sprintf("project-alpha-%02d", index), fmt.Sprintf("项目代号星海 alpha 证据 %02d", index)
		}
		document := map[string]json.RawMessage{
			"key": json.RawMessage(fmt.Sprintf("%q", key)), "kind": json.RawMessage(`"fact"`),
			"content": json.RawMessage(fmt.Sprintf(`{"value":%q,"memoryType":"fact","importance":8,"confidence":100,"source":"reviewed","verifiedStatus":"verified"}`, value)),
		}
		memorypolicy.WriteLifecycle(document, memorypolicy.NewLifecycleWithPolicy(memorypolicy.DecayPolicyDurable, 3, false, 8, time.Now()))
		mutations = append(mutations, coordination.Mutation{Kind: "memory", ID: fmt.Sprintf("memory-%03d", index), RoleID: "role", Body: body(document)})
	}
	if _, err := coordination.NewOwnershipStore(db, "a").Apply(t.Context(), coordination.Commit{Scope: view.Scope, Mutations: mutations}); err != nil {
		t.Fatal(err)
	}
	return engine, request
}

func TestMemoryLongHorizonRecallGate(t *testing.T) {
	engine, request := seedLongHorizonMemories(t, 240)
	result, err := engine.SearchOwnedMemories(t.Context(), request, MemoryManagementQuery{Query: "星海 alpha", Mode: "keyword", Limit: 10, ContextUsable: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 10 || len(result.Resources) != 10 {
		t.Fatalf("long-horizon recall total=%d resources=%d", result.Total, len(result.Resources))
	}
	for _, resource := range result.Resources {
		details := result.ScoreDetails[resource.ID]
		if details.FinalScore <= 0 || !details.KeywordMatch {
			t.Fatalf("invalid score details resource=%s details=%+v", resource.ID, details)
		}
	}
}

func BenchmarkMemoryLongHorizonRetrieval(b *testing.B) {
	weights := defaultMemoryRankingWeights()
	documents := make([]map[string]json.RawMessage, 500)
	content := make([]map[string]json.RawMessage, 500)
	now := time.Now()
	for index := range documents {
		documents[index] = map[string]json.RawMessage{
			"key": json.RawMessage(fmt.Sprintf(`"topic-%03d"`, index)), "kind": json.RawMessage(`"fact"`),
			"content":   json.RawMessage(fmt.Sprintf(`{"value":"长期记忆内容 %03d 用于跨会话召回测试","memoryType":"fact","importance":8,"confidence":100,"source":"reviewed","verifiedStatus":"verified"}`, index)),
			"createdAt": json.RawMessage(fmt.Sprintf("%q", now.Add(-time.Duration(index)*time.Hour).Format(time.RFC3339Nano))),
		}
		_ = json.Unmarshal(documents[index]["content"], &content[index])
	}
	state := memorypolicy.NewLifecycleWithPolicy(memorypolicy.DecayPolicyDurable, 3, false, 8, now)
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		for index := range documents {
			details := memoryRetrievalScore("keyword", weights, .5, true, state, content[index], documents[index], now)
			if details.FinalScore <= 0 {
				b.Fatal("invalid score")
			}
		}
	}
}
