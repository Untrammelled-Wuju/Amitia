package chat

import (
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/memory"
)

type ConversationStateProvider struct {
	mu    sync.RWMutex
	cache *WorkingMemoryCache
	store map[string]*ConversationStateEntry
}

type ConversationStateEntry struct {
	State     *interaction.ConversationState
	Version   int64
	UpdatedAt time.Time
}

func NewConversationStateProvider(cache *WorkingMemoryCache) *ConversationStateProvider {
	return &ConversationStateProvider{
		cache: cache,
		store: make(map[string]*ConversationStateEntry),
	}
}

func (p *ConversationStateProvider) GetState(convID string) *interaction.ConversationState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	entry, ok := p.store[convID]
	if !ok {
		return nil
	}
	return entry.State
}

func (p *ConversationStateProvider) GetVersionedState(convID string) (*interaction.ConversationState, int64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	entry, ok := p.store[convID]
	if !ok {
		return nil, 0
	}
	return entry.State, entry.Version
}

func (p *ConversationStateProvider) UpsertState(convID string, state *interaction.ConversationState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.store[convID]
	if !ok {
		entry = &ConversationStateEntry{}
		p.store[convID] = entry
	}
	entry.Version++
	entry.UpdatedAt = time.Now()
	state.StateVersion = fmt.Sprintf("%d", entry.Version)
	entry.State = state
}

func (p *ConversationStateProvider) GetWorkingMemoryState(convID string) (memory.ConversationWorkingMemoryState, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	entry, ok := p.store[convID]
	if !ok || entry.State == nil {
		return memory.ConversationWorkingMemoryState{}, false
	}
	return memory.ConversationWorkingMemoryState{
		ConversationID:         entry.State.ConversationID,
		MessageCount:           entry.State.MessageCount,
		LastMessageAt:          entry.State.LastMessageAt,
		ActiveThreads:          append([]string(nil), entry.State.ActiveThreads...),
		LastInteractionSummary: entry.State.LastInteractionSummary,
	}, true
}

func (p *ConversationStateProvider) UpsertWorkingMemoryState(convID string, state memory.ConversationWorkingMemoryState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.store[convID]
	if !ok {
		entry = &ConversationStateEntry{State: &interaction.ConversationState{ConversationID: convID}}
		p.store[convID] = entry
	}
	if entry.State == nil {
		entry.State = &interaction.ConversationState{ConversationID: convID}
	}
	entry.State.ConversationID = convID
	entry.State.MessageCount = state.MessageCount
	entry.State.LastMessageAt = state.LastMessageAt
	entry.State.ActiveThreads = append([]string(nil), state.ActiveThreads...)
	entry.State.LastInteractionSummary = state.LastInteractionSummary
	entry.Version++
	entry.UpdatedAt = time.Now()
	entry.State.StateVersion = fmt.Sprintf("%d", entry.Version)
}

func (p *ConversationStateProvider) BuildFromWorkingMemory(convID string, scope interaction.InteractionScope) *interaction.ConversationState {
	cs := &interaction.ConversationState{
		ConversationID: convID,
		StateVersion:   "0",
		Scope:          &scope,
	}
	wm := p.cache.Get(convID)
	if wm != nil && wm.State != nil {
		cs.LastInteractionSummary = wm.State.Summary
		if len(wm.State.KeyPoints) > 0 {
			cs.ActiveThreads = wm.State.KeyPoints
		}
	}
	return cs
}

func (p *ConversationStateProvider) SetAttention(convID string, attention *interaction.AttentionState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.store[convID]
	if !ok {
		entry = &ConversationStateEntry{State: &interaction.ConversationState{ConversationID: convID}}
		p.store[convID] = entry
	}
	entry.State.AttentionState = attention
	entry.Version++
	entry.UpdatedAt = time.Now()
	entry.State.StateVersion = fmt.Sprintf("%d", entry.Version)
}

func (p *ConversationStateProvider) SetEmotionSnapshot(convID string, snapshot *interaction.EmotionSnapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.store[convID]
	if !ok {
		entry = &ConversationStateEntry{State: &interaction.ConversationState{ConversationID: convID}}
		p.store[convID] = entry
	}
	entry.State.EmotionSnapshot = snapshot
	entry.Version++
	entry.UpdatedAt = time.Now()
	entry.State.StateVersion = fmt.Sprintf("%d", entry.Version)
}

func (p *ConversationStateProvider) SetRelationshipSnapshot(convID string, snapshot *interaction.RelationshipSnapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.store[convID]
	if !ok {
		entry = &ConversationStateEntry{State: &interaction.ConversationState{ConversationID: convID}}
		p.store[convID] = entry
	}
	entry.State.RelationshipSnapshot = snapshot
	entry.Version++
	entry.UpdatedAt = time.Now()
	entry.State.StateVersion = fmt.Sprintf("%d", entry.Version)
}

func (p *ConversationStateProvider) UpdateTopic(convID, topic string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.store[convID]
	if !ok {
		entry = &ConversationStateEntry{State: &interaction.ConversationState{ConversationID: convID}}
		p.store[convID] = entry
	}
	entry.State.CurrentTopic = topic
	entry.Version++
	entry.UpdatedAt = time.Now()
	entry.State.StateVersion = fmt.Sprintf("%d", entry.Version)
}

func (p *ConversationStateProvider) UpdateSummary(convID, summary string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.store[convID]
	if !ok {
		entry = &ConversationStateEntry{State: &interaction.ConversationState{ConversationID: convID}}
		p.store[convID] = entry
	}
	entry.State.LastInteractionSummary = summary
	entry.Version++
	entry.UpdatedAt = time.Now()
	entry.State.StateVersion = fmt.Sprintf("%d", entry.Version)
}

func (p *ConversationStateProvider) RemoveState(convID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.store, convID)
}

func (p *ConversationStateProvider) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.store)
}
