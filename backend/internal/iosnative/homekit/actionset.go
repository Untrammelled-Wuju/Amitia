package homekit

import "fmt"

const (
	BuiltinActionSetWakeUp        = "HMWakeUp"
	BuiltinActionSetGoodMorning   = "HMGoodMorning"
	BuiltinActionSetGoodNight     = "HMGoodNight"
	BuiltinActionSetHomeDeparture = "HMHomeDeparture"
	BuiltinActionSetHomeArrival   = "HMHomeArrival"
)

var BuiltinActionSetIDs = map[string]bool{
	BuiltinActionSetWakeUp:        true,
	BuiltinActionSetGoodMorning:   true,
	BuiltinActionSetGoodNight:     true,
	BuiltinActionSetHomeDeparture: true,
	BuiltinActionSetHomeArrival:   true,
}

func IsBuiltinActionSet(actionSetName string) bool {
	return BuiltinActionSetIDs[actionSetName]
}

type SceneActionInput struct {
	AccessoryID      string `json:"accessoryId"`
	ServiceID        string `json:"serviceId"`
	CharacteristicID string `json:"characteristicId"`

	TargetValue HomeCharacteristicValue `json:"targetValue"`
}

type CreateSceneInput struct {
	HomeID  string             `json:"homeId"`
	Name    string             `json:"name"`
	Actions []SceneActionInput `json:"actions"`
}

type UpdateSceneInput struct {
	SceneID string  `json:"sceneId"`
	Name    *string `json:"name,omitempty"`
}

func ValidateCreateSceneInput(input CreateSceneInput) error {
	if input.HomeID == "" {
		return NewValidationError("homeId is required")
	}
	if input.Name == "" {
		return NewValidationError("name is required")
	}
	if len(input.Actions) == 0 {
		return NewValidationError("at least one action is required")
	}
	if len(input.Actions) > MaxActionsPerScene {
		return NewValidationError(fmt.Sprintf("too many actions: max %d", MaxActionsPerScene))
	}
	for i, action := range input.Actions {
		if action.AccessoryID == "" {
			return NewValidationError(fmt.Sprintf("action[%d]: accessoryId is required", i))
		}
		if action.ServiceID == "" {
			return NewValidationError(fmt.Sprintf("action[%d]: serviceId is required", i))
		}
		if action.CharacteristicID == "" {
			return NewValidationError(fmt.Sprintf("action[%d]: characteristicId is required", i))
		}
	}
	return nil
}

type ValidationError struct {
	Message string
}

func NewValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}

func (e *ValidationError) Error() string {
	return e.Message
}
