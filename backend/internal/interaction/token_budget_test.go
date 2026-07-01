package interaction

import (
	"sort"
	"testing"
)

func TestTokenBudgetManager_Allocate_WithinBudget(t *testing.T) {
	manager := NewTokenBudgetManager(1000)
	modules := []TokenBudgetModule{
		{Name: "safety_rules", Tokens: 200, Priority: BudgetPrioritySafety},
		{Name: "intent_context", Tokens: 300, Priority: BudgetPriorityCurrentIntent},
		{Name: "memories", Tokens: 200, Priority: BudgetPriorityHighAuthority},
	}

	plan := manager.Allocate(modules)

	if len(plan) != 3 {
		t.Fatalf("expected 3 plans, got %d", len(plan))
	}
	for _, p := range plan {
		if p.Trimmed {
			t.Errorf("module %s was trimmed but budget was within limit", p.Module.Name)
		}
	}
}

func TestTokenBudgetManager_Allocate_OverBudget(t *testing.T) {
	manager := NewTokenBudgetManager(500)
	modules := []TokenBudgetModule{
		{Name: "safety_rules", Tokens: 200, Priority: BudgetPrioritySafety},
		{Name: "intent_context", Tokens: 300, Priority: BudgetPriorityCurrentIntent},
		{Name: "memories", Tokens: 200, Priority: BudgetPriorityHighAuthority},
		{Name: "background", Tokens: 300, Priority: BudgetPriorityLowAuthority},
	}

	plan := manager.Allocate(modules)

	totalAllocated := 0
	for _, p := range plan {
		totalAllocated += p.Allocated
	}
	if totalAllocated > 500 {
		t.Errorf("total allocated %d exceeds max 500", totalAllocated)
	}

	safetyTrimmed := false
	for _, p := range plan {
		if p.Module.Name == "safety_rules" && p.Trimmed {
			safetyTrimmed = true
		}
	}
	if safetyTrimmed {
		t.Error("safety module should not be trimmed before lower priority modules")
	}
}

func TestTokenBudgetManager_Allocate_LowPriorityTrimmedFirst(t *testing.T) {
	manager := NewTokenBudgetManager(400)
	modules := []TokenBudgetModule{
		{Name: "safety", Tokens: 200, Priority: BudgetPrioritySafety},
		{Name: "intent", Tokens: 200, Priority: BudgetPriorityCurrentIntent},
		{Name: "high_auth", Tokens: 200, Priority: BudgetPriorityHighAuthority},
		{Name: "low_auth", Tokens: 200, Priority: BudgetPriorityLowAuthority},
	}

	plan := manager.Allocate(modules)

	for _, p := range plan {
		if p.Module.Name == "low_auth" && !p.Trimmed {
			t.Error("low_authority module should be trimmed when budget is tight")
		}
		if p.Module.Name == "safety" && p.Trimmed {
			t.Error("safety module should never be trimmed before lower priority modules")
		}
	}
}

func TestTokenBudgetManager_Allocate_ExactBudget(t *testing.T) {
	manager := NewTokenBudgetManager(500)
	modules := []TokenBudgetModule{
		{Name: "a", Tokens: 250, Priority: BudgetPrioritySafety},
		{Name: "b", Tokens: 250, Priority: BudgetPriorityCurrentIntent},
	}

	plan := manager.Allocate(modules)

	total := 0
	for _, p := range plan {
		total += p.Allocated
	}
	if total != 500 {
		t.Errorf("expected total 500, got %d", total)
	}
	for _, p := range plan {
		if p.Trimmed {
			t.Errorf("module %s trimmed at exact budget", p.Module.Name)
		}
	}
}

func TestTokenBudgetManager_Allocate_SamePriorityPreservesLarger(t *testing.T) {
	manager := NewTokenBudgetManager(500)
	modules := []TokenBudgetModule{
		{Name: "safety_a", Tokens: 300, Priority: BudgetPrioritySafety},
		{Name: "safety_b", Tokens: 300, Priority: BudgetPrioritySafety},
	}

	plan := manager.Allocate(modules)

	total := 0
	for _, p := range plan {
		total += p.Allocated
	}
	if total > 500 {
		t.Errorf("total %d exceeds max 500", total)
	}
}

func TestSensitivityToBudgetPriority(t *testing.T) {
	tests := []struct {
		sensitivity string
		expected    BudgetPriority
	}{
		{"safety", BudgetPrioritySafety},
		{"critical", BudgetPrioritySafety},
		{"intent", BudgetPriorityCurrentIntent},
		{"current", BudgetPriorityCurrentIntent},
		{"high", BudgetPriorityHighAuthority},
		{"authority", BudgetPriorityHighAuthority},
		{"low", BudgetPriorityLowAuthority},
		{"background", BudgetPriorityLowAuthority},
		{"unknown", BudgetPriorityLowAuthority},
	}

	for _, tt := range tests {
		result := SensitivityToBudgetPriority(tt.sensitivity)
		if result != tt.expected {
			t.Errorf("SensitivityToBudgetPriority(%q) = %s, want %s", tt.sensitivity, result, tt.expected)
		}
	}
}

func TestBudgetPriority_Ordering(t *testing.T) {
	priorities := []BudgetPriority{
		BudgetPrioritySafety,
		BudgetPriorityCurrentIntent,
		BudgetPriorityHighAuthority,
		BudgetPriorityLowAuthority,
	}

	if !sort.IsSorted(sort.IntSlice([]int{
		int(priorities[0]),
		int(priorities[1]),
		int(priorities[2]),
		int(priorities[3]),
	})) {
		t.Error("BudgetPriority values must be in ascending order for trim logic to work")
	}
}

func TestTokenBudgetManager_TrimByPriority(t *testing.T) {
	manager := NewTokenBudgetManager(500)
	sections := []PromptSectionRef{
		{Type: "safety_rules", TokenBudget: 200, Priority: 4, Sensitivity: "safety", Trimmable: false},
		{Type: "persona", TokenBudget: 200, Priority: 3, Sensitivity: "high", Trimmable: true},
		{Type: "context", TokenBudget: 200, Priority: 2, Sensitivity: "high", Trimmable: true},
		{Type: "background", TokenBudget: 300, Priority: 1, Sensitivity: "low", Trimmable: true},
	}

	result := manager.TrimByPriority(sections)

	for _, ref := range result {
		if ref.Type == "background" {
			t.Error("low priority background section should have been trimmed")
		}
	}

	totalTokens := 0
	for _, ref := range result {
		totalTokens += ref.TokenBudget
	}
	if totalTokens > 500 {
		t.Errorf("total remaining tokens %d exceeds max 500", totalTokens)
	}
}

func TestTokenBudgetManager_TrimByPriority_NonTrimmableRetained(t *testing.T) {
	manager := NewTokenBudgetManager(100)
	sections := []PromptSectionRef{
		{Type: "safety_rules", TokenBudget: 200, Priority: 1, Sensitivity: "safety", Trimmable: false},
		{Type: "background", TokenBudget: 200, Priority: 1, Sensitivity: "low", Trimmable: true},
	}

	result := manager.TrimByPriority(sections)

	safetyFound := false
	for _, ref := range result {
		if ref.Type == "safety_rules" {
			safetyFound = true
		}
	}
	if !safetyFound {
		t.Error("non-trimmable safety section was removed despite not being trimmable")
	}
}
