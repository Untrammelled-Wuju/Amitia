package prompt

import "testing"

func TestApplyBudgetTrimsLowPrioritySectionsFirst(t *testing.T) {
	ir := CompileIR([]Section{
		{Type: SectionTypeSystem, Priority: 100, TokenBudget: 8, Source: "system", Sensitivity: SensitivityInternal, Content: "follow safety rules and current policy always"},
		{Type: SectionTypeCurrentInput, Priority: 90, TokenBudget: 6, Source: "message", Sensitivity: SensitivityUserData, Content: "please help me plan tomorrow schedule"},
		{Type: SectionTypeMemory, Priority: 30, TokenBudget: 10, Source: "memory", Sensitivity: SensitivityUserData, Trimmable: true, Content: "user likes tea and jogging every weekend with long walks"},
		{Type: SectionTypeHistory, Priority: 20, TokenBudget: 10, Source: "history", Sensitivity: SensitivityUserData, Trimmable: true, Content: "older conversation about books music travel and breakfast preferences"},
	}, CompileOptions{})

	budgeted := ApplyBudget(ir, BudgetPolicy{
		MaxPromptTokens: 14,
		SectionLimits: map[SectionType]SectionBudget{
			SectionTypeSystem:       {MaxTokens: 8, MinTokens: 4, Priority: 100},
			SectionTypeCurrentInput: {MaxTokens: 6, MinTokens: 4, Priority: 90},
			SectionTypeMemory:       {MaxTokens: 6, MinTokens: 0, Priority: 30, TrimReason: "low_priority_memory_trimmed"},
			SectionTypeHistory:      {MaxTokens: 6, MinTokens: 0, Priority: 20, TrimReason: "old_history_trimmed"},
		},
	})

	if len(budgeted.Sections) != 3 {
		t.Fatalf("expected history to be dropped first: %#v", budgeted.Sections)
	}
	if budgeted.Sections[2].Type != SectionTypeMemory {
		t.Fatalf("expected memory to remain after higher priority sections: %#v", budgeted.Sections)
	}
	if got := sectionContentTokens(budgeted.Sections); got > 14 {
		t.Fatalf("expected prompt to stay within budget, got %d", got)
	}
	if len(budgeted.Audit.TrimRecords) != 2 {
		t.Fatalf("expected two trim records: %#v", budgeted.Audit.TrimRecords)
	}
	if budgeted.Audit.TrimRecords[0].SectionType != SectionTypeMemory || budgeted.Audit.TrimRecords[1].SectionType != SectionTypeHistory {
		t.Fatalf("unexpected trim record order: %#v", budgeted.Audit.TrimRecords)
	}
	if budgeted.Audit.TrimRecords[1].AfterTokens != 0 {
		t.Fatalf("expected history to be fully dropped: %#v", budgeted.Audit.TrimRecords[1])
	}
}

func TestApplyBudgetPreservesRequiredSectionsAtMinimum(t *testing.T) {
	ir := CompileIR([]Section{
		{Type: SectionTypeSystem, Priority: 100, TokenBudget: 10, Source: "system", Sensitivity: SensitivityInternal, Content: "follow policy keep user safe stay factual"},
		{Type: SectionTypeBehaviorPlan, Priority: 80, TokenBudget: 8, Source: "decision", Sensitivity: SensitivityInternal, Content: "answer warmly ask one concise clarifying question"},
	}, CompileOptions{})

	budgeted := ApplyBudget(ir, BudgetPolicy{
		MaxPromptTokens: 8,
		SectionLimits: map[SectionType]SectionBudget{
			SectionTypeSystem:       {MaxTokens: 6, MinTokens: 4, Priority: 100},
			SectionTypeBehaviorPlan: {MaxTokens: 4, MinTokens: 2, Priority: 80},
		},
	})

	if len(budgeted.Sections) != 2 {
		t.Fatalf("expected required sections to remain: %#v", budgeted.Sections)
	}
	if estimateTokens(budgeted.Sections[0].Content) < 4 {
		t.Fatalf("system section should keep minimum tokens: %#v", budgeted.Sections[0])
	}
	if estimateTokens(budgeted.Sections[1].Content) < 2 {
		t.Fatalf("behavior plan should keep minimum tokens: %#v", budgeted.Sections[1])
	}
	if got := sectionContentTokens(budgeted.Sections); got > 8 {
		t.Fatalf("expected prompt to stay within budget, got %d", got)
	}
}

func TestApplyBudgetRecordsTrimmedSummary(t *testing.T) {
	ir := CompileIR([]Section{
		{Type: SectionTypeWorldbook, Priority: 20, TokenBudget: 10, Source: "worldbook", Sensitivity: SensitivityInternal, Trimmable: true, Content: "city rules market opens early and festival lanterns appear every spring"},
	}, CompileOptions{})

	budgeted := ApplyBudget(ir, BudgetPolicy{
		MaxPromptTokens: 4,
		SectionLimits: map[SectionType]SectionBudget{
			SectionTypeWorldbook: {MaxTokens: 4, MinTokens: 0, Priority: 20, TrimReason: "low_confidence_context_trimmed"},
		},
	})

	if len(budgeted.Audit.TrimRecords) != 1 {
		t.Fatalf("expected a trim record: %#v", budgeted.Audit.TrimRecords)
	}
	record := budgeted.Audit.TrimRecords[0]
	if record.Reason != "low_confidence_context_trimmed" {
		t.Fatalf("unexpected trim reason: %#v", record)
	}
	if record.Summary == "" {
		t.Fatalf("expected trimmed summary: %#v", record)
	}
	if record.BeforeTokens <= record.AfterTokens {
		t.Fatalf("expected before tokens greater than after tokens: %#v", record)
	}
}

func TestSortSectionsForBudgetUsesBudgetPriority(t *testing.T) {
	sections := []Section{
		{Type: SectionTypeMemory, Priority: 10, TokenBudget: 8},
		{Type: SectionTypeCurrentInput, Priority: 90, TokenBudget: 8},
		{Type: SectionTypeHistory, Priority: 20, TokenBudget: 8},
	}

	sorted := SortSectionsForBudget(sections, BudgetPolicy{
		SectionLimits: map[SectionType]SectionBudget{
			SectionTypeMemory:       {Priority: 40},
			SectionTypeCurrentInput: {Priority: 100},
			SectionTypeHistory:      {Priority: 30},
		},
	})

	if sorted[0].Type != SectionTypeCurrentInput || sorted[1].Type != SectionTypeMemory || sorted[2].Type != SectionTypeHistory {
		t.Fatalf("unexpected sort order: %#v", sorted)
	}
}

func sectionContentTokens(sections []Section) int {
	total := 0
	for _, section := range sections {
		total += estimateTokens(section.Content)
	}
	return total
}
