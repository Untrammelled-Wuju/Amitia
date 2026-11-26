//go:build rsi_gap

package capability

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

var ErrGapInvalid = errors.New("capability: gap request invalid")

type GapState string

const (
	GapExisting   GapState = "existing"
	GapComposable GapState = "composable"
	GapMissing    GapState = "missing"
)

type CompositionRule struct {
	Result      string
	Requires    []string
	Description string
}

type GapEntry struct {
	ToolID          string
	CapabilityID    CapabilityID
	State           GapState
	Reasons         []AvailabilityReason
	MissingRequires []string
	Score           int
}

type CapabilityGapReport struct {
	Goal       string
	Profile    TaskProfile
	Strategy   StrategySnapshot
	Existing   []GapEntry
	Composable []GapEntry
	Missing    []GapEntry
}

func (r CapabilityGapReport) Verdict() GapState {
	switch {
	case len(r.Existing) > 0:
		return GapExisting
	case len(r.Composable) > 0:
		return GapComposable
	default:
		return GapMissing
	}
}

func gapTokens(text string) map[string]bool {
	tokens := map[string]bool{}
	for _, field := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(field)) >= 2 {
			tokens[field] = true
		}
	}
	return tokens
}

func ruleMatchesGoal(rule CompositionRule, goal string, goalTokens map[string]bool) bool {
	lowered := strings.ToLower(strings.TrimSpace(goal))
	result := strings.ToLower(strings.TrimSpace(rule.Result))
	if result == "" {
		return false
	}
	if strings.Contains(lowered, result) {
		return true
	}
	resultTokens := gapTokens(result)
	if len(resultTokens) == 0 {
		return false
	}
	matched := 0
	for token := range resultTokens {
		if goalTokens[token] {
			matched++
		}
	}
	return matched*2 >= len(resultTokens)
}

func AnalyzeCapabilityGap(ctx context.Context, selector *TaskSelector, goal string, rules []CompositionRule) (CapabilityGapReport, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return CapabilityGapReport{}, fmt.Errorf("%w: goal required", ErrGapInvalid)
	}
	if selector == nil {
		return CapabilityGapReport{}, fmt.Errorf("%w: selector required", ErrGapInvalid)
	}
	seenRules := map[string]bool{}
	for _, rule := range rules {
		result := strings.TrimSpace(rule.Result)
		if result == "" {
			return CapabilityGapReport{}, fmt.Errorf("%w: rule result required", ErrGapInvalid)
		}
		if len(rule.Requires) == 0 {
			return CapabilityGapReport{}, fmt.Errorf("%w: rule requires dependencies: %s", ErrGapInvalid, result)
		}
		if seenRules[result] {
			return CapabilityGapReport{}, fmt.Errorf("%w: duplicate rule: %s", ErrGapInvalid, result)
		}
		seenRules[result] = true
	}
	selection, err := selector.Select(ctx, ToolInvocationContext{}, goal, 0)
	if err != nil {
		return CapabilityGapReport{}, err
	}
	report := CapabilityGapReport{Goal: goal, Profile: selection.Profile, Strategy: selection.Strategy}
	selected := make([]GapEntry, 0, len(selection.Selected))
	selectedIDs := map[string]bool{}
	selectedNames := map[string]bool{}
	for _, entry := range selection.Selected {
		selectedIDs[entry.Tool.ID] = true
		selectedNames[strings.ToLower(entry.Tool.Name)] = true
		selected = append(selected, GapEntry{
			ToolID:       entry.Tool.ID,
			CapabilityID: entry.Tool.CapabilityID,
			State:        GapExisting,
			Score:        entry.Score,
		})
	}
	sort.SliceStable(selected, func(left, right int) bool { return selected[left].ToolID < selected[right].ToolID })
	report.Existing = selected

	goalTokens := gapTokens(goal)
	matchedRules := map[string]bool{}
	for _, rule := range rules {
		if !ruleMatchesGoal(rule, goal, goalTokens) {
			continue
		}
		if selectedIDs[rule.Result] || selectedNames[strings.ToLower(rule.Result)] {
			matchedRules[rule.Result] = true
			continue
		}
		missing := make([]string, 0, len(rule.Requires))
		for _, required := range rule.Requires {
			if !selectedIDs[required] && !selectedNames[strings.ToLower(required)] {
				missing = append(missing, required)
			}
		}
		sort.Strings(missing)
		matchedRules[rule.Result] = true
		if len(missing) == 0 {
			report.Composable = append(report.Composable, GapEntry{
				ToolID:       rule.Result,
				State:        GapComposable,
				Reasons:      []AvailabilityReason{},
				Score:        0,
			})
			continue
		}
		report.Missing = append(report.Missing, GapEntry{
			ToolID:          rule.Result,
			State:           GapMissing,
			Reasons:         []AvailabilityReason{ReasonDependencyMissing},
			MissingRequires: missing,
		})
	}

	for _, rejected := range selection.Rejected {
		if matchedRules[rejected.ToolID] {
			continue
		}
		reasons := append([]AvailabilityReason(nil), rejected.Reasons...)
		if len(reasons) == 0 {
			reasons = []AvailabilityReason{ReasonNotInstalled}
		}
		report.Missing = append(report.Missing, GapEntry{
			ToolID:       rejected.ToolID,
			State:        GapMissing,
			Reasons:      reasons,
		})
	}
	if len(report.Existing) == 0 && len(report.Composable) == 0 && len(report.Missing) == 0 {
		report.Missing = append(report.Missing, GapEntry{
			State:   GapMissing,
			Reasons: []AvailabilityReason{ReasonNotInstalled},
		})
	}
	sort.SliceStable(report.Composable, func(left, right int) bool { return report.Composable[left].ToolID < report.Composable[right].ToolID })
	sort.SliceStable(report.Missing, func(left, right int) bool { return report.Missing[left].ToolID < report.Missing[right].ToolID })
	return report, nil
}
