// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"errors"
	"sort"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet"
)

func newSucceededAction(key string, supportsIdle int, sortOrder int) desktoppet.GenerationTaskAction {
	return desktoppet.GenerationTaskAction{
		ActionKey:           key,
		ActionNameSnapshot:  key,
		Status:              "succeeded",
		SupportsDefaultIdle: supportsIdle,
		SortOrder:           sortOrder,
	}
}

func newFailedAction(key string, supportsIdle int, sortOrder int) desktoppet.GenerationTaskAction {
	return desktoppet.GenerationTaskAction{
		ActionKey:           key,
		ActionNameSnapshot:  key,
		Status:              "failed",
		SupportsDefaultIdle: supportsIdle,
		SortOrder:           sortOrder,
	}
}

func TestDefaultActionSelectorUserSpecified(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newSucceededAction("idle_blink", 1, 5),
	}
	s := NewDefaultActionSelector("idle_blink")
	got, err := s.SelectDefaultAction(actions)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "idle_blink" {
		t.Errorf("expected idle_blink, got %s", got)
	}
}

func TestDefaultActionSelectorUserSpecifiedNotIdle(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newSucceededAction("happy", 0, 10),
	}
	s := NewDefaultActionSelector("happy")
	got, err := s.SelectDefaultAction(actions)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "idle_normal" {
		t.Errorf("expected idle_normal fallback when user specified happy (not idle), got %s", got)
	}
}

func TestDefaultActionSelectorIdleNormal(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("wave", 0, 10),
		newSucceededAction("idle_normal", 1, 1),
		newSucceededAction("idle_breathing", 1, 2),
	}
	s := NewDefaultActionSelector("")
	got, err := s.SelectDefaultAction(actions)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "idle_normal" {
		t.Errorf("expected idle_normal, got %s", got)
	}
}

func TestDefaultActionSelectorIdleBreathing(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("wave", 0, 10),
		newSucceededAction("idle_breathing", 1, 2),
		newSucceededAction("idle_blink", 1, 5),
	}
	s := NewDefaultActionSelector("")
	got, err := s.SelectDefaultAction(actions)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "idle_breathing" {
		t.Errorf("expected idle_breathing, got %s", got)
	}
}

func TestDefaultActionSelectorOtherIdleAction(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("wave", 0, 10),
		newSucceededAction("idle_blink", 1, 5),
		newSucceededAction("idle_sway", 1, 3),
	}
	s := NewDefaultActionSelector("")
	got, err := s.SelectDefaultAction(actions)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "idle_sway" {
		t.Errorf("expected idle_sway (lowest sort_order), got %s", got)
	}
}

func TestDefaultActionSelectorOtherIdleActionSortedByOrder(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_blink", 1, 5),
		newSucceededAction("idle_sway", 1, 3),
		newSucceededAction("idle_stretch", 1, 7),
	}
	s := NewDefaultActionSelector("")
	got, err := s.SelectDefaultAction(actions)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "idle_sway" {
		t.Errorf("expected idle_sway (sort_order 3), got %s", got)
	}
}

func TestDefaultActionSelectorNoIdleAction(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("wave", 0, 10),
		newSucceededAction("happy", 0, 11),
		newSucceededAction("fall", 0, 12),
	}
	s := NewDefaultActionSelector("")
	got, err := s.SelectDefaultAction(actions)
	if err == nil {
		t.Errorf("expected error when no idle action available, got %s", got)
	}
	if !errors.Is(err, ErrDefaultIdleActionUnavailable) {
		t.Errorf("expected ErrDefaultIdleActionUnavailable, got %v", err)
	}
}

func TestDefaultActionSelectorEmptyActions(t *testing.T) {
	s := NewDefaultActionSelector("")
	_, err := s.SelectDefaultAction(nil)
	if err == nil {
		t.Errorf("expected error for empty actions")
	}
	if !errors.Is(err, ErrDefaultIdleActionUnavailable) {
		t.Errorf("expected ErrDefaultIdleActionUnavailable, got %v", err)
	}
}

func TestDefaultActionSelectorUserSpecifiedNotSucceeded(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newFailedAction("idle_blink", 1, 5),
	}
	s := NewDefaultActionSelector("idle_blink")
	got, err := s.SelectDefaultAction(actions)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "idle_normal" {
		t.Errorf("expected idle_normal fallback when user specified action failed, got %s", got)
	}
}

func TestDefaultActionIsDefaultIdleAction(t *testing.T) {
	cases := []struct {
		name    string
		action  desktoppet.GenerationTaskAction
		expected bool
	}{
		{"succeeded idle", newSucceededAction("idle_normal", 1, 1), true},
		{"failed idle", newFailedAction("idle_normal", 1, 1), false},
		{"succeeded non-idle", newSucceededAction("happy", 0, 1), false},
		{"failed non-idle", newFailedAction("happy", 0, 1), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsDefaultIdleAction(c.action); got != c.expected {
				t.Errorf("expected %v, got %v", c.expected, got)
			}
		})
	}
}

func TestDefaultActionFindSucceededAction(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newFailedAction("idle_blink", 1, 5),
		newSucceededAction("wave", 0, 10),
	}
	a, ok := FindSucceededAction(actions, "idle_normal")
	if !ok {
		t.Errorf("expected to find idle_normal")
	}
	if a == nil || a.ActionKey != "idle_normal" {
		t.Errorf("invalid action returned")
	}
	a, ok = FindSucceededAction(actions, "idle_blink")
	if ok {
		t.Errorf("expected not to find idle_blink (failed status)")
	}
	if a != nil {
		t.Errorf("expected nil action when not found")
	}
	_, ok = FindSucceededAction(actions, "missing")
	if ok {
		t.Errorf("expected not to find missing action")
	}
}

func TestDefaultActionSelectIncludedActionsExclude(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newSucceededAction("wave", 0, 10),
		newSucceededAction("happy", 0, 11),
	}
	included, err := SelectIncludedActions(actions, "idle_normal", []string{"happy"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := map[string]bool{"idle_normal": true, "wave": true}
	if len(included) != 2 {
		t.Fatalf("expected 2 included actions, got %d: %v", len(included), included)
	}
	for _, key := range included {
		if !expected[key] {
			t.Errorf("unexpected action %s in included list", key)
		}
		if key == "happy" {
			t.Errorf("happy should be excluded")
		}
	}
}

func TestDefaultActionSelectIncludedActionsExcludeDefaultFails(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newSucceededAction("happy", 0, 11),
	}
	_, err := SelectIncludedActions(actions, "idle_normal", []string{"idle_normal"})
	if err == nil {
		t.Errorf("expected error when excluding default action")
	}
}

func TestDefaultActionSelectIncludedActionsNoSubstitute(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newSucceededAction("happy", 0, 11),
		newSucceededAction("wave", 0, 12),
	}
	_, err := SelectIncludedActions(actions, "idle_normal", []string{"idle_normal"})
	if err == nil {
		t.Errorf("expected error: should not substitute default idle with happy/wave")
	}
}

func TestDefaultActionSelectIncludedActionsDefaultMissing(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("happy", 0, 11),
		newSucceededAction("wave", 0, 12),
	}
	_, err := SelectIncludedActions(actions, "idle_normal", nil)
	if err == nil {
		t.Errorf("expected error when default action not in succeeded actions")
	}
}

func TestDefaultActionSelectIncludedActionsAllExcludedExceptDefault(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newSucceededAction("wave", 0, 10),
		newSucceededAction("happy", 0, 11),
	}
	included, err := SelectIncludedActions(actions, "idle_normal", []string{"wave", "happy"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(included) != 1 {
		t.Errorf("expected 1 included action, got %d: %v", len(included), included)
	}
	if len(included) > 0 && included[0] != "idle_normal" {
		t.Errorf("expected idle_normal, got %s", included[0])
	}
}

func TestDefaultActionSelectIncludedActionsIgnoresFailed(t *testing.T) {
	actions := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_normal", 1, 1),
		newFailedAction("wave", 0, 10),
	}
	included, err := SelectIncludedActions(actions, "idle_normal", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(included) != 1 {
		t.Errorf("expected 1 included action (failed excluded), got %d: %v", len(included), included)
	}
}

func TestDefaultActionSortCandidatesStable(t *testing.T) {
	candidates := []desktoppet.GenerationTaskAction{
		newSucceededAction("idle_blink", 1, 5),
		newSucceededAction("idle_sway", 1, 3),
		newSucceededAction("idle_stretch", 1, 7),
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].SortOrder < candidates[j].SortOrder
	})
	if candidates[0].ActionKey != "idle_sway" {
		t.Errorf("expected idle_sway first, got %s", candidates[0].ActionKey)
	}
}
