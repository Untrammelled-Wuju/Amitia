// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"context"
	"strings"
	"time"
)

type ConversationWorkingMemoryState struct {
	ConversationID         string
	MessageCount           int
	LastMessageAt          time.Time
	ActiveThreads          []string
	LastInteractionSummary string
}

type ConversationStateStore interface {
	GetWorkingMemoryState(convID string) (ConversationWorkingMemoryState, bool)
	UpsertWorkingMemoryState(convID string, state ConversationWorkingMemoryState)
}

type WorkingMemoryService struct {
	store ConversationStateStore
}

func NewWorkingMemoryService(store ...ConversationStateStore) *WorkingMemoryService {
	s := &WorkingMemoryService{}
	if len(store) > 0 {
		s.store = store[0]
	}
	return s
}

func (s *WorkingMemoryService) Name() string { return "工作记忆" }

func (s *WorkingMemoryService) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.store == nil || strings.TrimSpace(convID) == "" {
		return ErrSkip
	}
	summary := trimRunes(strings.TrimSpace(newReply), 500)
	if summary == "" {
		return ErrSkip
	}
	state, ok := s.store.GetWorkingMemoryState(convID)
	if !ok {
		state = ConversationWorkingMemoryState{ConversationID: convID}
	}
	state.ConversationID = convID
	state.MessageCount = len(messages)
	state.LastMessageAt = time.Now()
	state.LastInteractionSummary = summary
	state.ActiveThreads = mergeThreads(state.ActiveThreads, extractWorkingMemoryThreads(messages, newReply), 20)
	s.store.UpsertWorkingMemoryState(convID, state)
	return nil
}

func trimRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func extractWorkingMemoryThreads(messages []map[string]string, newReply string) []string {
	candidates := make([]string, 0, 6)
	for i := len(messages) - 1; i >= 0 && len(candidates) < 5; i-- {
		content := strings.TrimSpace(messages[i]["content"])
		if content == "" {
			continue
		}
		candidates = append(candidates, trimRunes(content, 100))
	}
	if strings.TrimSpace(newReply) != "" {
		candidates = append(candidates, trimRunes(strings.TrimSpace(newReply), 100))
	}
	return candidates
}

func mergeThreads(existing, incoming []string, limit int) []string {
	seen := map[string]bool{}
	merged := make([]string, 0, len(existing)+len(incoming))
	for _, thread := range existing {
		thread = strings.TrimSpace(thread)
		if thread == "" || seen[thread] {
			continue
		}
		seen[thread] = true
		merged = append(merged, thread)
	}
	for _, thread := range incoming {
		thread = strings.TrimSpace(thread)
		if thread == "" || seen[thread] {
			continue
		}
		seen[thread] = true
		merged = append(merged, thread)
	}
	if limit > 0 && len(merged) > limit {
		return merged[len(merged)-limit:]
	}
	return merged
}
