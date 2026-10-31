package chat

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/knowledge"
	"strings"
	"testing"
)

func TestKnowledgeRouteDoesNotLeakIntoCompanionChat(t *testing.T) {
	for _, message := range []string{"早安", "晚安", "陪我", "我累了", "你还记得我昨天说的事吗"} {
		if got := knowledge.ResolveChatRoute(message, config.DefaultKnowledgeRuntimeConfig().Features).EffectiveLevel; got != knowledge.RetrievalLevelNone {
			t.Fatalf("social turn routed %q to %q", message, got)
		}
	}
}
func TestKnowledgeRouteExplicitDocuments(t *testing.T) {
	if got := knowledge.ResolveChatRoute("检查PDF第37页的原文", config.DefaultKnowledgeRuntimeConfig().Features).EffectiveLevel; got == knowledge.RetrievalLevelNone {
		t.Fatal("document request should search")
	}
	if got := knowledge.ResolveChatRoute("对比两份知识库资料的冲突", config.DefaultKnowledgeRuntimeConfig().Features).EffectiveLevel; got != "deep" {
		t.Fatalf("comparison routed %q", got)
	}
}

func TestPhase16ExplicitReadRoutesToAgentic(t *testing.T) {
	for _, question := range []string{"核对PDF第37页原文", "查看文档 page 3000", "查询 Excel 第500行"} {
		if got := knowledge.ResolveChatRoute(question, config.DefaultKnowledgeRuntimeConfig().Features).EffectiveLevel; got != "agentic" {
			t.Fatalf("expected bounded agentic for explicit file read %q, got %q", question, got)
		}
	}
}

func TestPhase16CanonicalEvidenceComesFirst(t *testing.T) {
	got := prioritizeCanonicalEvidence([]knowledge.Evidence{
		{ID: "chunk", EvidenceKind: "chunk"},
		{ID: "read", EvidenceKind: "canonical_read"},
	})
	if len(got) != 2 || got[0].ID != "read" {
		t.Fatalf("original context must not displace canonical read: %#v", got)
	}
}

func TestPhase16RouteRolloutModes(t *testing.T) {
	features := config.DefaultKnowledgeRuntimeConfig().Features
	decision := knowledge.ResolveChatRoute("请核对PDF第37页的原文", features)
	if decision.EffectiveLevel != knowledge.RetrievalLevelAgentic {
		t.Fatalf("new policy ignored explicit file investigation: %q", decision.EffectiveLevel)
	}
	features.RetrievalPolicyEnabled = false
	features.RetrievalPolicyShadow = true
	decision = knowledge.ResolveChatRoute("请核对PDF第37页的原文", features)
	if decision.EffectiveLevel != knowledge.RetrievalLevelFast || decision.PolicyMode != knowledge.PolicyModeLegacyShadow || decision.ShadowLevel != knowledge.RetrievalLevelAgentic {
		t.Fatalf("shadow must not alter active legacy routing: %#v", decision)
	}
}

func TestPhase16PreciseReadStaysTenantScoped(t *testing.T) {
	port := &investigationKnowledgePort{}
	port.searchFn = func(_ context.Context, tenant string, req knowledge.SearchRequest) (knowledge.SearchResponse, error) {
		if tenant != "space-a" || req.RetrievalLevel != knowledge.RetrievalLevelAgentic {
			t.Fatalf("search context changed: %s, %#v", tenant, req)
		}
		return knowledge.SearchResponse{RetrievalLevel: knowledge.RetrievalLevelAgentic,
			Evidence: []knowledge.Evidence{{ID: "c1", SourceID: "s1", SourceRevision: 3, UnitID: "u11", Title: "A.pdf", Content: "旧片段", FusionScore: 0.1}},
		}, nil
	}
	port.readFn = func(_ context.Context, tenant string, req knowledge.ReadRequest) (knowledge.ReadResponse, error) {
		if tenant != "space-a" || req.SourceID != "s1" || req.SourceRevision != 3 || req.Locator["page"] != 37 {
			t.Fatalf("read escaped tenant/revision/locator scope: %s, %#v", tenant, req)
		}
		return knowledge.ReadResponse{SourceID: "s1", SourceRevision: 3, AnchorUnitID: "u37", TotalRunes: 7, Units: []knowledge.ReadUnit{{ID: "u37", SourceID: "s1", SourceRevision: 3, Title: "第37页", Content: "正确原文第37页", Locator: map[string]any{"page": 37}, Checksum: "ab"}}}, nil
	}
	svc := &service{knowledgePort: port}
	value, err := svc.fetchKnowledgeContext(context.Background(), "space-a", "请核对PDF第37页原文", nil, nil)
	if err != nil || !strings.Contains(value, "正确原文第37页") || !strings.Contains(value, "[K1]") {
		t.Fatalf("canonical read should enter prompt: %v, %s", err, value)
	}
	if port.searchCalls != 1 || port.readCalls != 1 {
		t.Fatalf("expected exactly one bounded search and one canonical read: %d %d", port.searchCalls, port.readCalls)
	}
}

func TestPhase16CanonicalReadFailureDoesNotClaimVerified(t *testing.T) {
	result := knowledge.SearchResponse{Sufficient: true, AnswerState: "sufficient", Evidence: []knowledge.Evidence{{ID: "c", SourceID: "src", SourceRevision: 1, EvidenceKind: "chunk", Content: "snippet", FusionScore: 0.1}}}
	markMissingCanonicalRead("核对PDF第37页的原文", &result)
	if result.Sufficient || result.SufficiencyReason != "canonical_read_unverified" || !result.Degraded {
		t.Fatalf("chunk evidence cannot claim verified canonical read: %+v", result)
	}
}

func TestOwnedChatAutomaticallyInjectsKnowledgeAndPersistsCitationSnapshot(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	port := &investigationKnowledgePort{
		searchFn: func(_ context.Context, tenantID string, req knowledge.SearchRequest) (knowledge.SearchResponse, error) {
			if tenantID != "space" {
				t.Fatalf("unexpected tenant %q", tenantID)
			}
			return knowledge.SearchResponse{
				Query: req.Query, Sufficient: true, AnswerState: "sufficient", RetrievalLevel: knowledge.RetrievalLevelFast,
				Evidence: []knowledge.Evidence{{
					ID: "evidence-a", SourceID: "source-a", SourceRevision: 1, UnitID: "unit-a",
					Title: "项目资料.pdf", Content: "项目代号是晨星。",
				}},
			}, nil
		},
	}
	f.model.knowledgePort = port
	var captured []map[string]interface{}
	f.model.llmWithTools = func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		captured = messages
		return "项目代号是晨星。[K1]", "", nil, 12, nil
	}

	request := f.request("knowledge-auto")
	request.Message = "请检索知识库资料并告诉我项目代号"
	response, err := f.engine.Run(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if port.searchCalls != 1 || len(captured) == 0 || !strings.Contains(strings.Join(promptContents(captured), "\n"), "项目代号是晨星") {
		t.Fatalf("knowledge context was not injected: calls=%d messages=%#v", port.searchCalls, captured)
	}
	var responseSnapshot []CitationSnapshot
	if err := json.Unmarshal(response.CitationSnapshot, &responseSnapshot); err != nil || len(responseSnapshot) != 1 || responseSnapshot[0].Marker != "[K1]" {
		t.Fatalf("response snapshot invalid: %s %v", response.CitationSnapshot, err)
	}
	messages := f.messages(t)
	if len(messages) != 2 || len(messages[1].CitationSnapshot) != 1 || messages[1].CitationSnapshot[0].SourceID != "source-a" {
		t.Fatalf("persisted snapshot invalid: %#v", messages)
	}
	_, replay, _, cancel := business.RuntimeManager().Subscribe(business.RuntimeKey(response.Scope, response.ConversationID), "knowledge-audit", 0)
	defer cancel()
	found := false
	for _, event := range replay {
		if event.Type == "retrieval.completed" && event.Payload["blockType"] == "agent_task" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("retrieval task was not published to the owned turn: %#v", replay)
	}
}

func TestOwnedChatSkipsKnowledgeForSocialMessage(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	port := &investigationKnowledgePort{}
	f.model.knowledgePort = port
	f.model.llmWithTools = ownedFixedReply("你好。")
	response, err := f.engine.Run(t.Context(), f.request("knowledge-social"))
	if err != nil || !response.Saved {
		t.Fatalf("social chat failed: %+v %v", response, err)
	}
	if port.searchCalls != 0 {
		t.Fatalf("social chat unexpectedly searched knowledge %d times", port.searchCalls)
	}
}

func TestOwnedChatDegradesWhenKnowledgeRetrievalFails(t *testing.T) {
	f := newOwnedExecutionFixture(t)
	port := &investigationKnowledgePort{
		searchFn: func(context.Context, string, knowledge.SearchRequest) (knowledge.SearchResponse, error) {
			return knowledge.SearchResponse{}, errors.New("qdrant unavailable")
		},
	}
	f.model.knowledgePort = port
	var captured []map[string]interface{}
	f.model.llmWithTools = func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		captured = messages
		return "当前无法核实知识库资料。", "", nil, 8, nil
	}
	request := f.request("knowledge-failure")
	request.Message = "请检索知识库资料并告诉我项目代号"
	response, err := f.engine.Run(t.Context(), request)
	if err != nil || !response.Saved {
		t.Fatalf("retrieval failure should degrade, not fail chat: %+v %v", response, err)
	}
	if !strings.Contains(strings.Join(promptContents(captured), "\n"), "知识库检索失败") {
		t.Fatalf("degraded prompt missing: %#v", captured)
	}
}

func promptContents(messages []map[string]interface{}) []string {
	result := make([]string, 0, len(messages))
	for _, message := range messages {
		result = append(result, strings.TrimSpace(strings.Join([]string{message["role"].(string), message["content"].(string)}, ": ")))
	}
	return result
}
