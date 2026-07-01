package prompt

import (
	"sort"
	"strings"
)

type BudgetPolicy struct {
	MaxPromptTokens int
	SectionLimits   map[SectionType]SectionBudget
}

type SectionBudget struct {
	MaxTokens  int
	MinTokens  int
	Priority   int
	TrimReason string
}

func ApplyBudget(ir IR, policy BudgetPolicy) IR {
	policy = normalizeBudgetPolicy(policy)
	next := ir
	next.Sections = make([]Section, 0, len(ir.Sections))
	next.Audit.TrimRecords = cloneTrimRecords(ir.Audit.TrimRecords)
	remaining := policy.MaxPromptTokens

	for _, section := range ir.Sections {
		budget := resolveSectionBudget(policy, section)
		if remaining <= 0 {
			if section.Trimmable {
				next.Audit.TrimRecords = append(next.Audit.TrimRecords, makeTrimRecord(section, 0, budget.TrimReason))
				continue
			}
			kept := trimSectionTokens(section, budget.MinTokens)
			kept.TokenBudget = minInt(kept.TokenBudget, budget.MinTokens)
			next.Sections = append(next.Sections, kept)
			next.Audit.TrimRecords = append(next.Audit.TrimRecords, makeTrimRecord(section, estimateTokens(kept.Content), "prompt_budget_floor"))
			continue
		}

		target := minInt(section.TokenBudget, budget.MaxTokens, remaining)
		if target < budget.MinTokens {
			if section.Trimmable {
				next.Audit.TrimRecords = append(next.Audit.TrimRecords, makeTrimRecord(section, 0, budget.TrimReason))
				continue
			}
			target = minInt(budget.MinTokens, remaining)
			if target <= 0 {
				target = budget.MinTokens
			}
		}

		kept := trimSectionTokens(section, target)
		used := estimateTokens(kept.Content)
		if used == 0 && strings.TrimSpace(section.Content) != "" && !section.Trimmable {
			used = minInt(target, budget.MinTokens)
		}
		kept.TokenBudget = maxInt(used, minInt(section.TokenBudget, target))
		next.Sections = append(next.Sections, kept)
		remaining -= used
		if used < estimateTokens(section.Content) {
			next.Audit.TrimRecords = append(next.Audit.TrimRecords, makeTrimRecord(section, used, budget.TrimReason))
		}
	}

	return next
}

func normalizeBudgetPolicy(policy BudgetPolicy) BudgetPolicy {
	if policy.MaxPromptTokens <= 0 {
		policy.MaxPromptTokens = 2048
	}
	if policy.SectionLimits == nil {
		policy.SectionLimits = map[SectionType]SectionBudget{}
	}
	return policy
}

func resolveSectionBudget(policy BudgetPolicy, section Section) SectionBudget {
	budget, ok := policy.SectionLimits[section.Type]
	if !ok {
		budget = SectionBudget{
			MaxTokens:  section.TokenBudget,
			MinTokens:  defaultSectionMinTokens(section),
			Priority:   section.Priority,
			TrimReason: defaultTrimReason(section),
		}
	}
	if budget.MaxTokens <= 0 {
		budget.MaxTokens = section.TokenBudget
	}
	if budget.MinTokens <= 0 {
		budget.MinTokens = defaultSectionMinTokens(section)
	}
	if budget.MaxTokens < budget.MinTokens {
		budget.MaxTokens = budget.MinTokens
	}
	if budget.TrimReason == "" {
		budget.TrimReason = defaultTrimReason(section)
	}
	return budget
}

func defaultSectionMinTokens(section Section) int {
	switch section.Type {
	case SectionTypeSystem, SectionTypeCurrentInput, SectionTypeBehaviorPlan:
		return minInt(maxInt(section.TokenBudget/2, 16), section.TokenBudget)
	case SectionTypeIdentity, SectionTypePsyche:
		return minInt(maxInt(section.TokenBudget/3, 12), section.TokenBudget)
	default:
		if section.Trimmable {
			return 0
		}
		return minInt(maxInt(section.TokenBudget/4, 8), section.TokenBudget)
	}
}

func defaultTrimReason(section Section) string {
	switch section.Type {
	case SectionTypeMemory:
		return "low_priority_memory_trimmed"
	case SectionTypeHistory:
		return "old_history_trimmed"
	case SectionTypeWorldbook:
		return "low_confidence_context_trimmed"
	default:
		return "prompt_budget_trimmed"
	}
}

func trimSectionTokens(section Section, tokenLimit int) Section {
	if tokenLimit <= 0 {
		section.Content = ""
		return section
	}
	words := strings.Fields(section.Content)
	if len(words) <= tokenLimit {
		return section
	}
	section.Content = strings.Join(words[:tokenLimit], " ")
	return section
}

func makeTrimRecord(section Section, afterTokens int, reason string) TrimRecord {
	beforeTokens := estimateTokens(section.Content)
	return TrimRecord{
		SectionType:  section.Type,
		Source:       section.Source,
		Reason:       reason,
		BeforeTokens: beforeTokens,
		AfterTokens:  afterTokens,
		Summary:      summarizeTrimmed(section.Content, afterTokens),
	}
}

func summarizeTrimmed(content string, keptTokens int) string {
	words := strings.Fields(content)
	if keptTokens < 0 {
		keptTokens = 0
	}
	if keptTokens >= len(words) {
		return ""
	}
	trimmed := words[keptTokens:]
	if len(trimmed) > 12 {
		trimmed = trimmed[:12]
	}
	return strings.Join(trimmed, " ")
}

func estimateTokens(content string) int {
	return len(strings.Fields(content))
}

func cloneTrimRecords(values []TrimRecord) []TrimRecord {
	if len(values) == 0 {
		return nil
	}
	out := make([]TrimRecord, len(values))
	copy(out, values)
	return out
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

func maxInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	result := values[0]
	for _, value := range values[1:] {
		if value > result {
			result = value
		}
	}
	return result
}

func SortSectionsForBudget(sections []Section, policy BudgetPolicy) []Section {
	sorted := make([]Section, len(sections))
	copy(sorted, sections)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := resolveSectionBudget(policy, sorted[i])
		right := resolveSectionBudget(policy, sorted[j])
		if left.Priority == right.Priority {
			return sectionRank(sorted[i].Type) < sectionRank(sorted[j].Type)
		}
		return left.Priority > right.Priority
	})
	return sorted
}
