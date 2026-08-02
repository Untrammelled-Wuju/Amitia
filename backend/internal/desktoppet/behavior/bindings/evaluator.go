package bindings

import (
	"encoding/json"
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
	key := scopeKey(scope)
	e.compiledBindings[key] = append(e.compiledBindings[key], CompiledBinding{
		Binding:    binding,
		Condition:  condition,
		CompiledAt: time.Now(),
	})
}

func (e *Evaluator) Evaluate(scope EvaluatorScope, eventType string, ctx EvalContext) []CompiledBinding {
	key := scopeKey(scope)
	var matched []CompiledBinding
	for _, cb := range e.compiledBindings[key] {
		if be, ok := cb.Binding.(bindingEntry); ok {
			if !be.IsEnabled() {
				continue
			}
			if be.GetEventType() != eventType {
				continue
			}
		}
		if cb.Condition == nil {
			matched = append(matched, cb)
			continue
		}
		if cb.Condition.Eval(ctx) {
			matched = append(matched, cb)
		}
	}
	return matched
}

func (e *Evaluator) RemoveBinding(scope EvaluatorScope, id string) {
	key := scopeKey(scope)
	bindings := e.compiledBindings[key]
	for i, cb := range bindings {
		if be, ok := cb.Binding.(bindingEntry); ok && be.GetID() == id {
			e.compiledBindings[key] = append(bindings[:i], bindings[i+1:]...)
			return
		}
	}
}

func (e *Evaluator) RemoveScope(scope EvaluatorScope) {
	delete(e.compiledBindings, scopeKey(scope))
}

func (e *Evaluator) Clear() {
	e.compiledBindings = make(map[string][]CompiledBinding)
}
