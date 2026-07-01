package interaction

import "sort"

type BudgetPriority int

const (
	BudgetPrioritySafety        BudgetPriority = 0
	BudgetPriorityCurrentIntent BudgetPriority = 1
	BudgetPriorityHighAuthority BudgetPriority = 2
	BudgetPriorityLowAuthority  BudgetPriority = 3
)

func (p BudgetPriority) String() string {
	switch p {
	case BudgetPrioritySafety:
		return "safety"
	case BudgetPriorityCurrentIntent:
		return "current_intent"
	case BudgetPriorityHighAuthority:
		return "high_authority"
	case BudgetPriorityLowAuthority:
		return "low_authority"
	default:
		return "unknown"
	}
}

type TokenBudgetModule struct {
	Name     string
	Tokens   int
	Priority BudgetPriority
}

type TokenBudgetPlan struct {
	Module   TokenBudgetModule
	Allocated int
	Trimmed   bool
}

type TokenBudgetManager struct {
	MaxTotalTokens int
}

func NewTokenBudgetManager(maxTotalTokens int) *TokenBudgetManager {
	return &TokenBudgetManager{MaxTotalTokens: maxTotalTokens}
}

func (m *TokenBudgetManager) Allocate(modules []TokenBudgetModule) []TokenBudgetPlan {
	if len(modules) == 0 {
		return nil
	}

	sorted := make([]TokenBudgetModule, len(modules))
	copy(sorted, modules)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority < sorted[j].Priority
		}
		return sorted[i].Tokens > sorted[j].Tokens
	})

	plan := make([]TokenBudgetPlan, 0, len(sorted))
	remaining := m.MaxTotalTokens
	fullBudget := sumTokens(sorted)

	if fullBudget <= m.MaxTotalTokens {
		for _, mod := range sorted {
			plan = append(plan, TokenBudgetPlan{Module: mod, Allocated: mod.Tokens, Trimmed: false})
		}
		return plan
	}

	for i, mod := range sorted {
		if remaining <= 0 {
			plan = append(plan, TokenBudgetPlan{Module: mod, Allocated: 0, Trimmed: true})
			continue
		}

		untrimmedCount := countRemainingUntrimmed(sorted[i+1:], remaining)
		fairShare := remaining / (untrimmedCount + 1)

		if mod.Tokens <= fairShare {
			plan = append(plan, TokenBudgetPlan{Module: mod, Allocated: mod.Tokens, Trimmed: false})
			remaining -= mod.Tokens
		} else {
			plan = append(plan, TokenBudgetPlan{Module: mod, Allocated: fairShare, Trimmed: true})
			remaining -= fairShare
		}
	}

	return plan
}

func SensitivityToBudgetPriority(sensitivity string) BudgetPriority {
	switch sensitivity {
	case "safety", "critical":
		return BudgetPrioritySafety
	case "intent", "current":
		return BudgetPriorityCurrentIntent
	case "high", "authority":
		return BudgetPriorityHighAuthority
	case "low", "background":
		return BudgetPriorityLowAuthority
	default:
		return BudgetPriorityLowAuthority
	}
}

func (m *TokenBudgetManager) TrimByPriority(modules []PromptSectionRef) []PromptSectionRef {
	if len(modules) == 0 {
		return modules
	}

	budgetModules := make([]TokenBudgetModule, len(modules))
	for i, ref := range modules {
		priority := SensitivityToBudgetPriority(ref.Sensitivity)
		budgetModules[i] = TokenBudgetModule{
			Name:     ref.Type,
			Tokens:   ref.TokenBudget,
			Priority: priority,
		}
	}

	allocations := m.Allocate(budgetModules)
	planByType := make(map[string]TokenBudgetPlan, len(allocations))
	for _, p := range allocations {
		planByType[p.Module.Name] = p
	}

	trimmed := make([]PromptSectionRef, 0, len(modules))
	for _, ref := range modules {
		p, ok := planByType[ref.Type]
		if !ok || p.Trimmed {
			if ref.Trimmable {
				continue
			}
			if p.Allocated == 0 {
				continue
			}
		}
		cloned := ref
		if ok {
			cloned.TokenBudget = p.Allocated
		}
		trimmed = append(trimmed, cloned)
	}

	return trimmed
}

func sumTokens(modules []TokenBudgetModule) int {
	total := 0
	for _, mod := range modules {
		total += mod.Tokens
	}
	return total
}

func countRemainingUntrimmed(modules []TokenBudgetModule, remainingBudget int) int {
	if len(modules) == 0 {
		return 0
	}
	count := 0
	needed := 0
	for _, mod := range modules {
		needed += mod.Tokens
		count++
	}
	if needed <= remainingBudget {
		return count
	}
	return 0
}
