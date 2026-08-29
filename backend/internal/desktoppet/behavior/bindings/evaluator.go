package bindings

import (
	"encoding/json"
	"sync"
	"time"
)

type BehaviorBinding struct {
	ID              string          `json:"id"`
	UserID          string          `json:"userId"`
	CharacterID     string          `json:"characterId,omitempty"`
	InstallationID  string          `json:"installationId,omitempty"`
	EventType       string          `json:"eventType"`
	ConditionsJSON  json.RawMessage `json:"conditions"`
	Semantic        string          `json:"semantic"`
	PreferredAction string          `json:"preferredAction,omitempty"`
	PriorityOffset  int             `json:"priorityOffset"`
	CooldownMS      int64           `json:"cooldownMs"`
	Enabled         bool            `json:"enabled"`
	Version         int             `json:"version"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

func (b BehaviorBinding) GetID() string              { return b.ID }
func (b BehaviorBinding) GetEventType() string       { return b.EventType }
func (b BehaviorBinding) IsEnabled() bool            { return b.Enabled }
func (b BehaviorBinding) GetSemantic() string        { return b.Semantic }
func (b BehaviorBinding) GetPreferredAction() string { return b.PreferredAction }
func (b BehaviorBinding) GetPriorityOffset() int     { return b.PriorityOffset }
func (b BehaviorBinding) GetCooldownMS() int64       { return b.CooldownMS }
func (b BehaviorBinding) GetUserID() string          { return b.UserID }
func (b BehaviorBinding) GetCharacterID() string     { return b.CharacterID }
func (b BehaviorBinding) GetInstallationID() string  { return b.InstallationID }

type bindingEntry interface {
	GetID() string
	GetEventType() string
	IsEnabled() bool
}

type CompiledBinding struct {
	Binding    interface{}
	Condition  ConditionNode
	CompiledAt time.Time
}

type EvaluatorScope struct {
	UserID         string
	CharacterID    string
	InstallationID string
}

type Evaluator struct {
	mu               sync.RWMutex
	compiledBindings map[string][]CompiledBinding
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		compiledBindings: make(map[string][]CompiledBinding),
	}
}

func scopeKey(scope EvaluatorScope) string {
	return scope.UserID + "/" + scope.CharacterID + "/" + scope.InstallationID
}

func (e *Evaluator) AddBinding(scope EvaluatorScope, binding interface{}, condition ConditionNode) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := scopeKey(scope)
	e.compiledBindings[key] = append(e.compiledBindings[key], CompiledBinding{
		Binding:    binding,
		Condition:  condition,
		CompiledAt: time.Now(),
	})
}

// ReplaceScope atomically swaps the compiled set for a single persisted scope.
// Readers therefore never observe a half-reloaded binding set.
func (e *Evaluator) ReplaceScope(scope EvaluatorScope, bindings []CompiledBinding) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := scopeKey(scope)
	if len(bindings) == 0 {
		delete(e.compiledBindings, key)
		return
	}
	copied := make([]CompiledBinding, len(bindings))
	copy(copied, bindings)
	e.compiledBindings[key] = copied
}

// ReplaceCharacterScopes atomically replaces every installation-specific scope
// for one user/character pair. It also removes scopes whose last binding was
// deleted, preventing stale evaluator entries after CRUD operations.
func (e *Evaluator) ReplaceCharacterScopes(userID, characterID string, replacements map[EvaluatorScope][]CompiledBinding) {
	e.mu.Lock()
	defer e.mu.Unlock()
	prefix := userID + "/" + characterID + "/"
	for key := range e.compiledBindings {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(e.compiledBindings, key)
		}
	}
	for scope, entries := range replacements {
		if scope.UserID != userID || scope.CharacterID != characterID || len(entries) == 0 {
			continue
		}
		copied := make([]CompiledBinding, len(entries))
		copy(copied, entries)
		e.compiledBindings[scopeKey(scope)] = copied
	}
}

func evaluatorLookupScopes(scope EvaluatorScope) []EvaluatorScope {
	result := make([]EvaluatorScope, 0, 4)
	seen := make(map[string]struct{}, 4)
	add := func(candidate EvaluatorScope) {
		key := scopeKey(candidate)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, candidate)
	}

	// Most specific first, then installation-generic, character-generic, and
	// finally user-global bindings. This preserves optional scope semantics.
	add(scope)
	if scope.InstallationID != "" {
		add(EvaluatorScope{UserID: scope.UserID, CharacterID: scope.CharacterID})
	}
	if scope.CharacterID != "" {
		add(EvaluatorScope{UserID: scope.UserID, InstallationID: scope.InstallationID})
	}
	add(EvaluatorScope{UserID: scope.UserID})
	return result
}

func (e *Evaluator) Evaluate(scope EvaluatorScope, eventType string, ctx EvalContext) []CompiledBinding {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var matched []CompiledBinding
	for _, lookupScope := range evaluatorLookupScopes(scope) {
		for _, cb := range e.compiledBindings[scopeKey(lookupScope)] {
			if be, ok := cb.Binding.(bindingEntry); ok {
				if !be.IsEnabled() {
					continue
				}
				if be.GetEventType() != eventType {
					continue
				}
			}
			if cb.Condition == nil || cb.Condition.Eval(ctx) {
				matched = append(matched, cb)
			}
		}
	}
	return matched
}

func (e *Evaluator) RemoveBinding(scope EvaluatorScope, id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := scopeKey(scope)
	entries := e.compiledBindings[key]
	for i, cb := range entries {
		if be, ok := cb.Binding.(bindingEntry); ok && be.GetID() == id {
			e.compiledBindings[key] = append(entries[:i], entries[i+1:]...)
			if len(e.compiledBindings[key]) == 0 {
				delete(e.compiledBindings, key)
			}
			return
		}
	}
}

func (e *Evaluator) RemoveScope(scope EvaluatorScope) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.compiledBindings, scopeKey(scope))
}

func (e *Evaluator) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.compiledBindings = make(map[string][]CompiledBinding)
}
