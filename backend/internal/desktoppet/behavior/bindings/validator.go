package bindings

import (
	"fmt"
)

const (
	ErrCodeBindingInvalid       = "binding_invalid"
	ErrCodeBindingActionMissing = "binding_action_missing"
)

func NewBindingError(code, msg string) error {
	return fmt.Errorf("%s: %s", code, msg)
}

const (
	MaxUserPriority         = 900
	MinCooldownMS     int64 = 500
	MinPriorityOffset       = -100
	MaxPriorityOffset       = 100
)

type Validator struct {
	allowedFields map[string]map[string]string
}

func NewValidator() *Validator {
	v := &Validator{
		allowedFields: make(map[string]map[string]string),
	}
	v.registerDefaults()
	return v
}

func (v *Validator) registerDefaults() {
	defaultEventTypes := []string{
		"chat.message.received", "chat.context.loading",
		"chat.response.started", "chat.response.ready",
		"chat.response.completed", "chat.response.failed",
		"chat.response.cancelled",
		"delivery.started", "delivery.completed", "delivery.failed",
		"voice.session.started", "voice.listening.started",
		"voice.listening.activity", "voice.listening.ended",
		"voice.processing.started", "voice.speaking.started",
		"voice.speaking.ended", "voice.turn.interrupted",
		"voice.session.ended",
		"agent.tool.started", "agent.tool.progress",
		"agent.tool.completed", "agent.tool.failed",
		"agent.tool.cancelled",
		"character.affect.changed", "character.activity.changed",
		"character.time_period.changed",
		"proactive.message.started", "proactive.message.completed",
		"proactive.message.suppressed",
		"runtime.pointer.clicked", "runtime.pointer.double_clicked",
		"runtime.pointer.hovered", "runtime.drag.started",
		"runtime.drag.moved", "runtime.drag.completed", "runtime.drag.cancelled",
		"runtime.pet.fall.started", "runtime.pet.edge.reached",
		"runtime.pet.interacted",
		"runtime.playback.action_started", "runtime.playback.action_completed",
		"runtime.playback.action_interrupted", "runtime.playback.action_failed",
		"runtime.connected", "runtime.disconnected",
		"installation.active.changed", "manual.action.requested",
	}
	for _, et := range defaultEventTypes {
		v.allowedFields[et] = map[string]string{
			"any": "any",
		}
	}
}

func (v *Validator) RegisterEventFields(eventType string, fields map[string]string) {
	v.allowedFields[eventType] = fields
}

func (v *Validator) Validate(binding BehaviorBinding, condition ConditionNode) error {
	if binding.EventType == "" {
		return NewBindingError(ErrCodeBindingInvalid, "eventType is required")
	}
	if _, ok := v.allowedFields[binding.EventType]; !ok {
		return NewBindingError(ErrCodeBindingInvalid,
			fmt.Sprintf("eventType %q is not registered in schema", binding.EventType))
	}
	if binding.Semantic == "" {
		return NewBindingError(ErrCodeBindingInvalid, "semantic is required")
	}
	if binding.PriorityOffset < MinPriorityOffset || binding.PriorityOffset > MaxPriorityOffset {
		return NewBindingError(ErrCodeBindingInvalid,
			fmt.Sprintf("priorityOffset must be between %d and %d", MinPriorityOffset, MaxPriorityOffset))
	}
	if binding.PriorityOffset > MaxUserPriority {
		return NewBindingError(ErrCodeBindingInvalid,
			fmt.Sprintf("priorityOffset must not exceed system safety band of %d", MaxUserPriority))
	}
	// Zero means "use the resolver default". Explicit non-zero values must
	// respect the minimum guard interval that prevents animation storms.
	if binding.CooldownMS != 0 && binding.CooldownMS < MinCooldownMS {
		return NewBindingError(ErrCodeBindingInvalid,
			fmt.Sprintf("cooldownMs must be 0 or at least %d", MinCooldownMS))
	}
	if condition != nil {
		allowedFields := v.allowedFields[binding.EventType]
		if err := validateConditionFields(condition, allowedFields); err != nil {
			return NewBindingError(ErrCodeBindingInvalid, err.Error())
		}
	}
	return nil
}

func (v *Validator) ValidateActionAvailable(preferredAction string, availableActions []string) error {
	if preferredAction == "" {
		return nil
	}
	for _, a := range availableActions {
		if a == preferredAction {
			return nil
		}
	}
	return NewBindingError(ErrCodeBindingActionMissing,
		fmt.Sprintf("preferred action %q is not in available actions", preferredAction))
}

func validateConditionFields(node ConditionNode, allowedFields map[string]string) error {
	switch n := node.(type) {
	case *EqNode:
		return validateFieldRef(n.Key, allowedFields)
	case *InNode:
		return validateFieldRef(n.Key, allowedFields)
	case *RangeNode:
		return validateFieldRef(n.Key, allowedFields)
	case *ExistsNode:
		return validateFieldRef(n.Key, allowedFields)
	case *AndNode:
		for _, c := range n.Children {
			if err := validateConditionFields(c, allowedFields); err != nil {
				return err
			}
		}
	case *OrNode:
		for _, c := range n.Children {
			if err := validateConditionFields(c, allowedFields); err != nil {
				return err
			}
		}
	case *NotNode:
		return validateConditionFields(n.Child, allowedFields)
	}
	return nil
}

func validateFieldRef(key string, allowedFields map[string]string) error {
	if !isFieldAllowed(key, allowedFields) {
		return fmt.Errorf("field %q is not in the allowed fields for this event type", key)
	}
	return nil
}
