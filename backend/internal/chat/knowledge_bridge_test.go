package chat

import (
	"context"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/knowledge"
	"strings"
	"testing"
)

func TestKnowledgeRouteDoesNotLeakIntoCompanionChat(t *testing.T) {
	for _, message := range []string{"早安", "晚安", "陪我", "我累了", "你还记得我昨天说的事吗"} {
		if got := knowledgeRoute(message); got != "" {
			t.Fatalf("social turn routed %q to %q", message, got)
		}
	}
}
func TestKnowledgeRouteExplicitDocuments(t *testing.T) {
	if got := knowledgeRoute("检查PDF第37页的原文"); got == "" {
		t.Fatal("document request should search")
	}
	if got := knowledgeRoute("对比两份知识库资料的冲突"); got != "deep" {
		t.Fatalf("comparison routed %q", got)
	}
}

func TestPhase16ExplicitReadRoutesToAgentic(t *testing.T) {
	for _, question := range []string{"核对PDF第37页原文", "查看文档 page 3000", "查询 Excel 第500行"} {
		if got := knowledgeRoute(question); got != "agentic" {
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
	active, _, _, _ := chooseKnowledgeRoute("请核对PDF第37页的原文", features)
	if active != knowledge.RetrievalLevelAgentic {
		t.Fatalf("new policy ignored explicit file investigation: %q", active)
	}
	features.RetrievalPolicyEnabled = false
	features.RetrievalPolicyShadow = true
	actual, mode, shadow, _ := chooseKnowledgeRoute("请核对PDF第37页的原文", features)
	if actual != knowledge.RetrievalLevelFast || mode != "legacy_shadow" || shadow != knowledge.RetrievalLevelAgentic {
		t.Fatalf("shadow must not alter active legacy routing: %q %q %q", actual, mode, shadow)
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
	value, err := svc.fetchKnowledgeContext(context.Background(), "space-a", "请核对PDF第37页原文", nil)
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
