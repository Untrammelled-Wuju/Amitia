package webresearch

import "testing"

func TestResearchPlanBuildsFocusDependencies(t *testing.T) {
	runtime := &Runtime{config: DefaultConfig()}
	plan := runtime.buildResearchPlan(ToolInput{
		SearchQuery: []SearchQueryCommand{{Q: "Amitia search architecture"}},
		FocusAreas:  []string{"provider routing", "citation recovery"},
	})
	if len(plan.Questions) != 3 {
		t.Fatalf("questions=%d, want 3", len(plan.Questions))
	}
	root := plan.Questions[0]
	if len(root.Dependencies) != 0 {
		t.Fatalf("root dependencies=%v, want none", root.Dependencies)
	}
	for _, question := range plan.Questions[1:] {
		if len(question.Dependencies) != 1 || question.Dependencies[0] != root.ID {
			t.Fatalf("focus %q dependencies=%v, want [%s]", question.Question, question.Dependencies, root.ID)
		}
	}
}

func TestResearchFollowUpWaitsForDependencies(t *testing.T) {
	runtime := &Runtime{config: DefaultConfig()}
	input := ToolInput{
		SearchQuery: []SearchQueryCommand{{Q: "Amitia search architecture"}},
		FocusAreas:  []string{"provider routing"},
	}
	plan := runtime.buildResearchPlan(input)
	executed := map[string]struct{}{
		researchQueryIdentity(input.SearchQuery[0]): {},
	}
	if got := runtime.buildDeepFollowUps(input, plan, executed, nil); len(got) != 0 {
		t.Fatalf("follow-ups before root evidence=%v, want none", got)
	}
	citations := []Citation{{Title: "Amitia search architecture", Text: "Amitia search architecture runtime design"}}
	got := runtime.buildDeepFollowUps(input, plan, executed, citations)
	if len(got) != 1 || got[0].Q != "Amitia search architecture provider routing" {
		t.Fatalf("follow-ups after dependency=%v", got)
	}
}

func TestResearchDependencyMissingQuestionIsNotSatisfied(t *testing.T) {
	question := ResearchQuestion{ID: "q2", Question: "provider routing", Dependencies: []string{"missing"}}
	plan := ResearchPlan{Questions: []ResearchQuestion{question}}
	if researchQuestionDependenciesSatisfied(plan, question, []Citation{{Title: "provider routing", Text: "covered"}}) {
		t.Fatal("unknown DAG dependency must not be treated as satisfied")
	}
}
