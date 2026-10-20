package knowledge

import (
	"strings"
	"testing"
)

func TestQueryPlanPreservesOriginalAndOnlySupplementsShortReference(t *testing.T) {
	plan, err := buildQueryPlan(SearchRequest{
		Query: "这个呢？", ContextualQuery: "上一轮讨论的是 Amitia Knowledge 的索引版本与回滚策略",
	}, 8)
	if err != nil {
		t.Fatalf("build query plan: %v", err)
	}
	if plan.OriginalQuery != "这个呢？" {
		t.Fatalf("original query changed: %q", plan.OriginalQuery)
	}
	if plan.EffectiveQuery == plan.OriginalQuery || !strings.Contains(plan.EffectiveQuery, plan.OriginalQuery) {
		t.Fatalf("contextual query did not supplement original: %q", plan.EffectiveQuery)
	}
	if len(plan.QueryHistory) < 2 || plan.QueryHistory[0] != plan.OriginalQuery {
		t.Fatalf("original query must remain first in history: %#v", plan.QueryHistory)
	}
}

func TestQueryPlanDoesNotRewriteNormalStandaloneQuestion(t *testing.T) {
	plan, err := buildQueryPlan(SearchRequest{
		Query: "Amitia 的知识库如何进行 revision 回滚？", ContextualQuery: "unrelated context",
	}, 8)
	if err != nil {
		t.Fatalf("build query plan: %v", err)
	}
	if plan.EffectiveQuery != plan.OriginalQuery {
		t.Fatalf("standalone query must not be silently rewritten: %q", plan.EffectiveQuery)
	}
}

func TestQueryPlanRetrievalLevelsHaveBoundedBudgets(t *testing.T) {
	cases := []struct {
		level string
		max   int
	}{
		{RetrievalLevelFast, 100},
		{RetrievalLevelDeep, 240},
		{RetrievalLevelAgentic, 300},
	}
	for _, tc := range cases {
		plan, err := buildQueryPlan(SearchRequest{Query: "比较 A 和 B 的差异", RetrievalLevel: tc.level}, 30)
		if err != nil {
			t.Fatalf("%s: %v", tc.level, err)
		}
		if plan.CandidateLimit <= 0 || plan.CandidateLimit > tc.max {
			t.Fatalf("%s candidate budget out of range: %d", tc.level, plan.CandidateLimit)
		}
		if plan.Timeout <= 0 {
			t.Fatalf("%s missing timeout", tc.level)
		}
	}
}

func TestQueryPlanOnlyDecomposesComplexOrComparison(t *testing.T) {
	simple, err := buildQueryPlan(SearchRequest{Query: "RAG 是什么", RetrievalLevel: RetrievalLevelDeep}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(simple.QueryHistory) != 1 {
		t.Fatalf("simple query should not decompose: %#v", simple.QueryHistory)
	}
	complex, err := buildQueryPlan(SearchRequest{Query: "A vs B", RetrievalLevel: RetrievalLevelDeep}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(complex.QueryHistory) < 3 {
		t.Fatalf("comparison should retain original and subqueries: %#v", complex.QueryHistory)
	}
	if len(complex.QueryHistory) > 5 {
		t.Fatalf("subquery count exceeded hard budget: %#v", complex.QueryHistory)
	}
}

func TestQueryPlanDecomposesExplicitChineseComparison(t *testing.T) {
	plan, err := buildQueryPlan(SearchRequest{Query: "豆包 TTS 和 MiniMax TTS 有什么区别？", RetrievalLevel: RetrievalLevelDeep}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.QueryHistory) < 3 {
		t.Fatalf("expected original plus two explicit comparison sides: %#v", plan.QueryHistory)
	}
	if plan.QueryHistory[0] != plan.OriginalQuery {
		t.Fatalf("original query must stay first: %#v", plan.QueryHistory)
	}
}

func TestQueryPlanRejectsInvalidLevel(t *testing.T) {
	if _, err := buildQueryPlan(SearchRequest{Query: "x", RetrievalLevel: "unbounded"}, 8); err == nil {
		t.Fatal("expected invalid retrieval level error")
	}
}

func TestSafeQueryPlanTraceHashesQueriesOnly(t *testing.T) {
	result := SearchResponse{
		OriginalQuery: "TOP SECRET ORIGINAL", EffectiveQuery: "PRIVATE CONTEXT\nTOP SECRET ORIGINAL",
		RetrievalLevel: RetrievalLevelDeep, QueryComplexity: QueryComplexityComplex,
		QueryHistory: []string{"TOP SECRET ORIGINAL", "PRIVATE SUBQUERY"},
	}
	trace := safeQueryPlanTrace(result)
	joined := strings.Join(append([]string{trace.OriginalHash, trace.EffectiveHash}, trace.QueryHistory...), " ")
	for _, secret := range []string{"TOP SECRET", "PRIVATE", "ORIGINAL", "SUBQUERY"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("query plan trace leaked raw query content: %q", joined)
		}
	}
	if trace.QueryCount != 2 || len(trace.OriginalHash) != 64 || len(trace.EffectiveHash) != 64 {
		t.Fatalf("unexpected safe trace: %#v", trace)
	}
}
