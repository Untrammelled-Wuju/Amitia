package homekit

import "fmt"

const (
	AutomationTypeEvent         = "event"
	AutomationTypeCalendar      = "calendar"
	AutomationTypePresence      = "presence"
	AutomationTypeCharacteristic = "characteristic"
	AutomationTypeTimerLegacy   = "timer_legacy"
	AutomationTypeUnknown       = "unknown"
)

var SupportedAutomationTypes = map[string]bool{
	AutomationTypeEvent:          true,
	AutomationTypeCalendar:       true,
	AutomationTypePresence:       true,
	AutomationTypeCharacteristic: true,
}

var PresenceEventScopes = map[string]bool{
	"anyone":       true,
	"everyone":     true,
	"current_user": true,
}

var PresenceEventTypes = map[string]bool{
	"arrives":  true,
	"leaves":   true,
}

type TriggerConditionInput struct {
	Type string `json:"type"`

	Characteristic *CharacteristicEventCondition `json:"characteristic,omitempty"`
	Calendar       *CalendarEventCondition       `json:"calendar,omitempty"`
	Presence       *PresenceEventCondition       `json:"presence,omitempty"`
}

type CharacteristicEventCondition struct {
	AccessoryID      string `json:"accessoryId"`
	ServiceID        string `json:"serviceId"`
	CharacteristicID string `json:"characteristicId"`

	TargetValue HomeCharacteristicValue `json:"targetValue,omitempty"`
}

type CalendarEventCondition struct {
	FireAt         string `json:"fireAt"`
	TimezoneOffset *int   `json:"timezoneOffset,omitempty"`
	Recurrence     string `json:"recurrence,omitempty"`
}

type PresenceEventCondition struct {
	Event     string `json:"event"`
	UserScope string `json:"userScope"`
}

func ValidateAutomationInput(input CreateAutomationInput) error {
	if input.HomeID == "" {
		return NewValidationError("homeId is required")
	}
	if input.Name == "" {
		return NewValidationError("name is required")
	}
	if !SupportedAutomationTypes[input.Type] {
		return NewValidationError(fmt.Sprintf("unsupported automation type: %s", input.Type))
	}

	switch input.Type {
	case AutomationTypeCharacteristic:
		if input.CharacteristicEvent == nil {
			return NewValidationError("characteristicEvent is required for characteristic automation")
		}
		if input.CharacteristicEvent.AccessoryID == "" {
			return NewValidationError("characteristicEvent.accessoryId is required")
		}
		if input.CharacteristicEvent.CharacteristicID == "" {
			return NewValidationError("characteristicEvent.characteristicId is required")
		}
	case AutomationTypeCalendar:
		if input.CalendarEvent == nil {
			return NewValidationError("calendarEvent is required for calendar automation")
		}
		if input.CalendarEvent.FireAt == "" {
			return NewValidationError("calendarEvent.fireAt is required")
		}
	case AutomationTypePresence:
		if input.PresenceEvent == nil {
			return NewValidationError("presenceEvent is required for presence automation")
		}
		if !PresenceEventTypes[input.PresenceEvent.Event] {
			return NewValidationError(fmt.Sprintf("unsupported presence event: %s", input.PresenceEvent.Event))
		}
		if !PresenceEventScopes[input.PresenceEvent.UserScope] {
			return NewValidationError(fmt.Sprintf("unsupported presence scope: %s", input.PresenceEvent.UserScope))
		}
	}

	return nil
}
