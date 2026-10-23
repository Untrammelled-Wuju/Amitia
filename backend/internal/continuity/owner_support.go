package continuity

import (
	"encoding/json"
	"strings"
	"time"
)

type BindingLookup struct {
	Type string
	ID   string
}

type RankedThread struct {
	Thread Thread
	Score  float64
}

func BindingLookupMatches(bindings []ThreadBinding, lookups ...BindingLookup) bool {
	for _, lookup := range lookups {
		if strings.TrimSpace(lookup.Type) == "" || strings.TrimSpace(lookup.ID) == "" {
			continue
		}
		for _, binding := range bindings {
			if binding.BindingType == lookup.Type && binding.BindingID == lookup.ID {
				return true
			}
		}
	}
	return false
}

func NewThreadBinding(threadID, bindingType, bindingID, role, source string, confidence float64) ThreadBinding {
	now := time.Now().UTC()
	if role == "" {
		role = "context"
	}
	if confidence <= 0 {
		confidence = 1
	}
	return ThreadBinding{
		ID:           "binding/" + strings.TrimSpace(threadID) + "/" + strings.TrimSpace(bindingType) + "/" + strings.TrimSpace(bindingID),
		ThreadID:     strings.TrimSpace(threadID),
		BindingType:  strings.TrimSpace(bindingType),
		BindingID:    strings.TrimSpace(bindingID),
		Role:         strings.TrimSpace(role),
		Confidence:   confidence,
		Source:       strings.TrimSpace(source),
		CreatedAt:    now,
		LastActiveAt: now,
	}
}

func MergeThreadBinding(bindings []ThreadBinding, binding ThreadBinding) []ThreadBinding {
	if binding.ThreadID == "" || binding.BindingType == "" || binding.BindingID == "" {
		return bindings
	}
	for index := range bindings {
		if bindings[index].BindingType != binding.BindingType || bindings[index].BindingID != binding.BindingID {
			continue
		}
		bindings[index].Role = binding.Role
		bindings[index].Source = binding.Source
		bindings[index].Confidence = binding.Confidence
		bindings[index].LastActiveAt = binding.LastActiveAt
		return bindings
	}
	return append(bindings, binding)
}

func ApplyThreadPatch(thread *Thread, patch ThreadPatch, now time.Time) {
	if thread == nil {
		return
	}
	if patch.StateChanged {
		if patch.Title != "" {
			thread.Title = patch.Title
		}
		if patch.Goal != "" {
			thread.Goal = patch.Goal
		}
		if patch.Summary != "" {
			thread.Summary = patch.Summary
		}
		if patch.CurrentState != "" {
			thread.CurrentState = patch.CurrentState
		}
		if patch.NextAction != "" || patch.Status.IsTerminal() {
			thread.NextAction = patch.NextAction
		}
		if patch.Confidence > 0 {
			thread.Confidence = patch.Confidence
		}
		if patch.Status != "" && ValidThreadStatus(patch.Status) {
			thread.Status = patch.Status
			if patch.Status.IsTerminal() {
				completed := now.UTC()
				thread.CompletedAt = &completed
			}
		}
		thread.LastActiveAt = now
	}
}

func WaitFromPatch(threadID string, patch WaitPatch, sourceExecutionID string, now time.Time) (Wait, bool) {
	if patch.Action != "create" || !ValidWaitType(patch.Type) {
		return Wait{}, false
	}
	condition := patch.Condition
	if len(condition) == 0 {
		condition = json.RawMessage(`{}`)
	}
	if !SafeWaitCondition(patch.Type, condition) {
		return Wait{}, false
	}
	autoResume := patch.Type != WaitTypeUser
	if patch.AutoResume != nil {
		autoResume = *patch.AutoResume
	}
	var dueAt *time.Time
	if strings.TrimSpace(patch.DueAt) != "" {
		parsed, err := time.Parse(time.RFC3339, patch.DueAt)
		if err != nil {
			return Wait{}, false
		}
		parsed = parsed.UTC()
		dueAt = &parsed
	}
	if patch.Type == WaitTypeTime && dueAt == nil {
		return Wait{}, false
	}
	return Wait{
		ID:                "wait/" + shortText(threadID+"|"+patch.Type+"|"+patch.Description, 180),
		ThreadID:          threadID,
		WaitType:          patch.Type,
		Status:            WaitStatusWaiting,
		Description:       patch.Description,
		ConditionJSON:     string(condition),
		ResumeHint:        patch.ResumeHint,
		DueAt:             dueAt,
		SourceExecutionID: sourceExecutionID,
		AutoResume:        autoResume,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, true
}

func SafeWaitCondition(waitType string, raw json.RawMessage) bool {
	return safeWaitCondition(waitType, raw)
}

func ThreadCandidates(documents []Thread) []Thread {
	result := make([]Thread, 0, len(documents))
	for _, thread := range documents {
		if thread.Status.IsTerminal() {
			continue
		}
		result = append(result, thread)
	}
	return result
}
