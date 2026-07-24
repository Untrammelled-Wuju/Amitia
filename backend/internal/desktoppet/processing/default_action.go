// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"errors"
	"fmt"
	"sort"

	"github.com/u-ai/backend/internal/desktoppet"
)

const (
	ErrCodeDefaultIdleActionUnavailable = "DEFAULT_IDLE_ACTION_UNAVAILABLE"
	DefaultActionIdleNormal             = "idle_normal"
	DefaultActionIdleBreathing          = "idle_breathing"
)

var ErrDefaultIdleActionUnavailable = errors.New("default idle action unavailable")

type DefaultActionSelector struct {
	userDefaultAction string
}

func NewDefaultActionSelector(userDefaultAction string) *DefaultActionSelector {
	return &DefaultActionSelector{userDefaultAction: userDefaultAction}
}

func (s *DefaultActionSelector) SelectDefaultAction(succeededActions []desktoppet.GenerationTaskAction) (string, error) {
	if s.userDefaultAction != "" {
		if action, ok := FindSucceededAction(succeededActions, s.userDefaultAction); ok {
			if action.SupportsDefaultIdle == 1 {
				return action.ActionKey, nil
			}
		}
	}
	if action, ok := FindSucceededAction(succeededActions, DefaultActionIdleNormal); ok {
		return action.ActionKey, nil
	}
	if action, ok := FindSucceededAction(succeededActions, DefaultActionIdleBreathing); ok {
		return action.ActionKey, nil
	}
	candidates := make([]desktoppet.GenerationTaskAction, 0)
	for _, action := range succeededActions {
		if action.Status != "succeeded" {
			continue
		}
		if action.SupportsDefaultIdle != 1 {
			continue
		}
		if action.ActionKey == DefaultActionIdleNormal || action.ActionKey == DefaultActionIdleBreathing {
			continue
		}
		if action.ActionKey == s.userDefaultAction {
			continue
		}
		candidates = append(candidates, action)
	}
	if len(candidates) == 0 {
		return "", ErrDefaultIdleActionUnavailable
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].SortOrder < candidates[j].SortOrder
	})
	return candidates[0].ActionKey, nil
}

func IsDefaultIdleAction(action desktoppet.GenerationTaskAction) bool {
	return action.SupportsDefaultIdle == 1 && action.Status == "succeeded"
}

func FindSucceededAction(actions []desktoppet.GenerationTaskAction, actionKey string) (*desktoppet.GenerationTaskAction, bool) {
	for i := range actions {
		if actions[i].ActionKey == actionKey && actions[i].Status == "succeeded" {
			return &actions[i], true
		}
	}
	return nil, false
}

func SelectIncludedActions(succeededActions []desktoppet.GenerationTaskAction, defaultAction string, excludedActions []string) ([]string, error) {
	excludedSet := make(map[string]bool)
	for _, key := range excludedActions {
		excludedSet[key] = true
	}
	if excludedSet[defaultAction] {
		return nil, fmt.Errorf("default action %s cannot be excluded", defaultAction)
	}
	included := make([]string, 0, len(succeededActions))
	hasDefault := false
	for _, action := range succeededActions {
		if action.Status != "succeeded" {
			continue
		}
		if excludedSet[action.ActionKey] {
			continue
		}
		if action.ActionKey == defaultAction {
			hasDefault = true
		}
		included = append(included, action.ActionKey)
	}
	if !hasDefault {
		return nil, fmt.Errorf("default action %s not found in succeeded actions", defaultAction)
	}
	return included, nil
}
