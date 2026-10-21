package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/knowledge"
)

type investigationKnowledgePort struct {
	searchCalls int
	readCalls   int
	searchFn    func(context.Context, string, knowledge.SearchRequest) (knowledge.SearchResponse, error)
	readFn      func(context.Context, string, knowledge.ReadRequest) (knowledge.ReadResponse, error)
}

func (p *investigationKnowledgePort) Search(ctx context.Context, tenantID string, req knowledge.SearchRequest) (knowledge.SearchResponse, error) {
	p.searchCalls++
	if p.searchFn != nil {
		return p.searchFn(ctx, tenantID, req)
	}
	return knowledge.SearchResponse{}, nil
}

func (p *investigationKnowledgePort) Read(ctx context.Context, tenantID string, req knowledge.ReadRequest) (knowledge.ReadResponse, error) {
	p.readCalls++
	if p.readFn != nil {
		return p.readFn(ctx, tenantID, req)
	}
	return knowledge.ReadResponse{}, nil
}

func TestKnowledgeInvestigationReadCanSatisfyLocatorCoverage(t *testing.T) {
	port := &investigationKnowledgePort{}
	port.readFn = func(_ context.Context, tenantID string, req knowledge.ReadRequest) (knowledge.ReadResponse, error) {
		if tenantID != "space-1" {
			t.Fatalf("unexpected tenant %q", tenantID)
		}
		if req.EvidenceID != "chunk-1" {
			t.Fatalf("expected read to stay anchored to current evidence, got %#v", req)
		}
		return knowledge.ReadResponse{
			SourceID: "source-a", SourceRevision: 2, AnchorUnitID: "unit-37", TotalRunes: 120,
			Units: []knowledge.ReadUnit{{ID: "unit-37", SourceID: "source-a", SourceRevision: 2, Ordinal: 37, UnitType: "page", Title: "第37页", Content: "这里是第三十七页的原文。", Locator: map[string]any{"page": 37}, Checksum: "abc"}},
		}, nil
	}
	plannerCalls := 0
	svc := &service{knowledgePort: port}
	svc.investigationPlanner = func(_ context.Context, _ string, _ knowledge.SearchResponse, _ investigationState, _ investigationBudget) (investigationAction, error) {
		plannerCalls++
		return investigationAction{Action: "read", EvidenceID: "chunk-1"}, nil
	}
	result := knowledge.SearchResponse{
		RetrievalLevel: knowledge.RetrievalLevelAgentic, TraceMaxAgenticRounds: 3, TraceBudgetMs: 1000,
		Evidence: []knowledge.Evidence{{ID: "chunk-1", UnitID: "unit-36", SourceID: "source-a", SourceRevision: 2, Title: "文档A", Content: "检索片段", FusionScore: 0.1, Confidence: 0.9}},
	}
	knowledge.ApplySufficiency("核对第37页原文", &result)
	if result.Sufficient {
		t.Fatal("fixture should start insufficient because page coverage is missing")
	}

	got := svc.runKnowledgeInvestigation(context.Background(), "space-1", "核对第37页原文", result, nil, nil)
	if !got.Sufficient || got.SufficiencyReason != "evidence_covered" {
		t.Fatalf("expected canonical read to satisfy deterministic gate: %#v", got)
	}
	if port.readCalls != 1 || plannerCalls != 1 {
		t.Fatalf("unexpected bounded call counts: planner=%d read=%d", plannerCalls, port.readCalls)
	}
	found := false
	for _, ev := range got.Evidence {
		if ev.EvidenceKind == "canonical_read" && ev.UnitID == "unit-37" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("canonical read evidence missing: %#v", got.Evidence)
	}
}

func TestKnowledgeInvestigationStopsOnDuplicateNoProgress(t *testing.T) {
	port := &investigationKnowledgePort{}
	port.searchFn = func(_ context.Context, _ string, req knowledge.SearchRequest) (knowledge.SearchResponse, error) {
		return knowledge.SearchResponse{Evidence: []knowledge.Evidence{{ID: "chunk-1", SourceID: "source-a", SourceRevision: 1, Content: "same", FusionScore: 0.1}}}, nil
	}
	plannerCalls := 0
	svc := &service{knowledgePort: port}
	svc.investigationPlanner = func(_ context.Context, _ string, _ knowledge.SearchResponse, _ investigationState, _ investigationBudget) (investigationAction, error) {
		plannerCalls++
		return investigationAction{Action: "search", Query: "different evidence query"}, nil
	}
	result := knowledge.SearchResponse{
		RetrievalLevel: knowledge.RetrievalLevelAgentic, TraceMaxAgenticRounds: 5, TraceBudgetMs: 1000,
		Evidence: []knowledge.Evidence{{ID: "chunk-1", SourceID: "source-a", SourceRevision: 1, Content: "same", FusionScore: 0.1}},
	}
	knowledge.ApplySufficiency("比较 A 和 B", &result)
	got := svc.runKnowledgeInvestigation(context.Background(), "space-1", "比较 A 和 B", result, nil, nil)
	if got.Sufficient {
		t.Fatal("duplicate/no-progress investigation must not bypass comparison coverage")
	}
	if port.searchCalls != 1 {
		t.Fatalf("duplicate query should not execute twice, search calls=%d", port.searchCalls)
	}
	if plannerCalls > 3 {
		t.Fatalf("planner exceeded bounded rounds: %d", plannerCalls)
	}
	joined := strings.Join(got.Warnings, "\n")
	if !strings.Contains(joined, "duplicate search query") && !strings.Contains(joined, "no_progress") {
		t.Fatalf("expected duplicate/no-progress diagnostic, got %q", joined)
	}
}

func TestBuildInvestigationReadRequestRejectsUnknownSource(t *testing.T) {
	evidence := []knowledge.Evidence{{ID: "e1", SourceID: "known", SourceRevision: 4}}
	_, _, err := buildInvestigationReadRequest(investigationAction{Action: "read", SourceID: "unknown", SourceRevision: 4, Locator: map[string]any{"page": 1}}, evidence, 12000)
	if err == nil {
		t.Fatal("expected unknown source locator read to be rejected")
	}
}

func TestExtractInvestigationJSONObject(t *testing.T) {
	got := extractInvestigationJSONObject("```json\n{\"action\":\"finish_insufficient\"}\n```")
	if got != `{"action":"finish_insufficient"}` {
		t.Fatalf("unexpected JSON extraction: %q", got)
	}
}

func TestKnowledgeInvestigationRejectsUnknownSearchSources(t *testing.T) {
	port := &investigationKnowledgePort{}
	svc := &service{knowledgePort: port}
	svc.investigationPlanner = func(_ context.Context, _ string, _ knowledge.SearchResponse, _ investigationState, _ investigationBudget) (investigationAction, error) {
		return investigationAction{Action: "search", Query: "follow up", SourceIDs: []string{"outside-ledger"}}, nil
	}
	result := knowledge.SearchResponse{
		RetrievalLevel: knowledge.RetrievalLevelAgentic,
		Evidence:       []knowledge.Evidence{{ID: "e1", SourceID: "known", SourceRevision: 1, Content: "partial", FusionScore: 0.1}},
	}
	knowledge.ApplySufficiency("比较 A 和 B", &result)
	got := svc.runKnowledgeInvestigation(context.Background(), "space-1", "比较 A 和 B", result, nil, nil)
	if port.searchCalls != 0 {
		t.Fatalf("unknown source restriction must not broaden to a global search, calls=%d", port.searchCalls)
	}
	if got.Sufficient {
		t.Fatal("rejected search action must not bypass deterministic sufficiency")
	}
	if !strings.Contains(strings.Join(got.Warnings, "\n"), "outside the current evidence ledger") {
		t.Fatalf("missing rejected-source diagnostic: %#v", got.Warnings)
	}
}

func TestBuildInvestigationReadRequestCarriesCanonicalUnitForward(t *testing.T) {
	evidence := []knowledge.Evidence{{
		ID: "read:unit-9:deadbeef", UnitID: "unit-9", EvidenceKind: "canonical_read",
		SourceID: "source-a", SourceRevision: 2, Content: "canonical context",
	}}
	req, parent, err := buildInvestigationReadRequest(investigationAction{Action: "read", EvidenceID: evidence[0].ID}, evidence, 12000)
	if err != nil {
		t.Fatalf("build canonical follow-up read: %v", err)
	}
	if parent.UnitID != "unit-9" || req.UnitID != "unit-9" {
		t.Fatalf("canonical follow-up did not preserve stable unit id: parent=%#v req=%#v", parent, req)
	}
	if req.SourceID != "source-a" || req.SourceRevision != 2 {
		t.Fatalf("canonical follow-up lost source scope: %#v", req)
	}
}

func TestKnowledgeInvestigationRejectedReadCountsSingleNoProgressRound(t *testing.T) {
	port := &investigationKnowledgePort{}
	svc := &service{knowledgePort: port}
	rounds := []int{}
	svc.investigationPlanner = func(_ context.Context, _ string, _ knowledge.SearchResponse, state investigationState, _ investigationBudget) (investigationAction, error) {
		rounds = append(rounds, state.NoProgress)
		return investigationAction{Action: "read", EvidenceID: "invalid-evidence"}, nil
	}
	result := knowledge.SearchResponse{
		RetrievalLevel: knowledge.RetrievalLevelAgentic, TraceMaxAgenticRounds: 5, TraceBudgetMs: 1000,
		Evidence: []knowledge.Evidence{{ID: "chunk-1", SourceID: "source-a", SourceRevision: 1, Content: "partial", FusionScore: 0.1}},
	}
	got := svc.runKnowledgeInvestigation(context.Background(), "space-1", "比较 A 和 B", result, nil, nil)
	if port.readCalls != 0 {
		t.Fatalf("rejected read must not call the reader: %d", port.readCalls)
	}
	if len(rounds) != 2 || rounds[0] != 0 || rounds[1] != 1 {
		t.Fatalf("rejected read must count one no-progress unit per round: %v", rounds)
	}
	if !strings.Contains(strings.Join(got.Warnings, "\n"), "no_progress") {
		t.Fatalf("expected bounded no-progress termination: %v", got.Warnings)
	}
}
