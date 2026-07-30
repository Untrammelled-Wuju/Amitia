package bindings

import (
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
)

type CompiledBinding struct {
	Binding    behavior.BehaviorBinding
	Condition  ConditionNode
	CompiledAt time.Time
}

type Evaluator struct {
	compiledBindings []CompiledBinding
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		compiledBindings: make([]CompiledBinding, 0),
	}
}

func (e *Evaluator) AddBinding(binding behavior.BehaviorBinding, condition ConditionNode) {
	e.compiledBindings = append(e.compiledBindings, CompiledBinding{
		Binding:    binding,
		Condition:  condition,
		CompiledAt: time.Now(),
	})
}

func (e *Evaluator) Evaluate(eventType string, ctx EvalContext) []CompiledBinding {
	var matched []CompiledBinding
	for _, cb := range e.compiledBindings {
		if !cb.Binding.Enabled {
			continue
		}
		if cb.Binding.EventType != eventType {
			continue
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

func (e *Evaluator) RemoveBinding(id string) {
	for i, cb := range e.compiledBindings {
		if cb.Binding.ID == id {
			e.compiledBindings = append(e.compiledBindings[:i], e.compiledBindings[i+1:]...)
			return
		}
	}
}

func (e *Evaluator) Clear() {
	e.compiledBindings = nil
}
