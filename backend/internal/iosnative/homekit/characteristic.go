package homekit

const (
	CharacteristicTypePowerState              = "power_state"
	CharacteristicTypeBrightness              = "brightness"
	CharacteristicTypeHue                     = "hue"
	CharacteristicTypeSaturation              = "saturation"

	CharacteristicTypeCurrentTemperature      = "current_temperature"
	CharacteristicTypeTargetTemperature       = "target_temperature"
	CharacteristicTypeHeatingCoolingState     = "heating_cooling_state"

	CharacteristicTypeLockCurrentState        = "lock_current_state"
	CharacteristicTypeLockTargetState         = "lock_target_state"

	CharacteristicTypeCurrentDoorState        = "current_door_state"
	CharacteristicTypeTargetDoorState         = "target_door_state"

	CharacteristicTypeOccupancyDetected       = "occupancy_detected"
	CharacteristicTypeMotionDetected          = "motion_detected"
	CharacteristicTypeContactState            = "contact_state"

	CharacteristicTypeAirQuality              = "air_quality"
	CharacteristicTypeCarbonDioxideDetected   = "carbon_dioxide_detected"
	CharacteristicTypeCarbonMonoxideDetected  = "carbon_monoxide_detected"
	CharacteristicTypeSmokeDetected           = "smoke_detected"

	CharacteristicTypeBatteryLevel            = "battery_level"
	CharacteristicTypeChargingState           = "charging_state"
	CharacteristicTypeStatusLowBattery        = "status_low_battery"

	CharacteristicTypeActive                  = "active"
	CharacteristicTypeRotationSpeed           = "rotation_speed"
	CharacteristicTypePositionState           = "position_state"
	CharacteristicTypeCurrentPosition         = "current_position"
	CharacteristicTypeTargetPosition          = "target_position"

	CharacteristicTypeGarageDoorTargetState   = "garage_door_target_state"
	CharacteristicTypeSecuritySystemTarget    = "security_system_target_state"

	CharacteristicTypeUnknown                 = "unknown"
)

var AppleTypeToCanonical = map[string]string{
	"HMCharacteristicTypePowerState":            CharacteristicTypePowerState,
	"HMCharacteristicTypeBrightness":           CharacteristicTypeBrightness,
	"HMCharacteristicTypeHue":                  CharacteristicTypeHue,
	"HMCharacteristicTypeSaturation":           CharacteristicTypeSaturation,

	"HMCharacteristicTypeCurrentTemperature":   CharacteristicTypeCurrentTemperature,
	"HMCharacteristicTypeTargetTemperature":    CharacteristicTypeTargetTemperature,
	"HMCharacteristicTypeHeatingCoolingState":  CharacteristicTypeHeatingCoolingState,

	"HMCharacteristicTypeLockCurrentState":     CharacteristicTypeLockCurrentState,
	"HMCharacteristicTypeLockTargetState":      CharacteristicTypeLockTargetState,

	"HMCharacteristicTypeCurrentDoorState":     CharacteristicTypeCurrentDoorState,
	"HMCharacteristicTypeTargetDoorState":      CharacteristicTypeTargetDoorState,

	"HMCharacteristicTypeOccupancyDetected":    CharacteristicTypeOccupancyDetected,
	"HMCharacteristicTypeMotionDetected":       CharacteristicTypeMotionDetected,
	"HMCharacteristicTypeContactSensorState":   CharacteristicTypeContactState,

	"HMCharacteristicTypeAirQuality":           CharacteristicTypeAirQuality,
	"HMCharacteristicTypeCarbonDioxideDetected": CharacteristicTypeCarbonDioxideDetected,
	"HMCharacteristicTypeCarbonMonoxideDetected": CharacteristicTypeCarbonMonoxideDetected,
	"HMCharacteristicTypeSmokeDetected":        CharacteristicTypeSmokeDetected,

	"HMCharacteristicTypeBatteryLevel":         CharacteristicTypeBatteryLevel,
	"HMCharacteristicTypeChargingState":        CharacteristicTypeChargingState,
	"HMCharacteristicTypeStatusLowBattery":     CharacteristicTypeStatusLowBattery,

	"HMCharacteristicTypeActive":               CharacteristicTypeActive,
	"HMCharacteristicTypeRotationSpeed":        CharacteristicTypeRotationSpeed,
	"HMCharacteristicTypePositionState":        CharacteristicTypePositionState,
	"HMCharacteristicTypeCurrentPosition":      CharacteristicTypeCurrentPosition,
	"HMCharacteristicTypeTargetPosition":       CharacteristicTypeTargetPosition,

	"HMCharacteristicTypeSecuritySystemTargetState": CharacteristicTypeSecuritySystemTarget,
}

var AppleTypeToValue = map[string]string{
	"HMCharacteristicTypePowerState":            "bool",
	"HMCharacteristicTypeBrightness":           "integer",
	"HMCharacteristicTypeHue":                  "float",
	"HMCharacteristicTypeSaturation":           "float",
	"HMCharacteristicTypeCurrentTemperature":   "float",
	"HMCharacteristicTypeTargetTemperature":    "float",
	"HMCharacteristicTypeHeatingCoolingState":  "integer",
	"HMCharacteristicTypeLockCurrentState":     "integer",
	"HMCharacteristicTypeLockTargetState":      "integer",
	"HMCharacteristicTypeCurrentDoorState":     "integer",
	"HMCharacteristicTypeTargetDoorState":      "integer",
	"HMCharacteristicTypeOccupancyDetected":    "bool",
	"HMCharacteristicTypeMotionDetected":       "bool",
	"HMCharacteristicTypeContactSensorState":   "bool",
	"HMCharacteristicTypeAirQuality":           "integer",
	"HMCharacteristicTypeBatteryLevel":         "integer",
	"HMCharacteristicTypeChargingState":        "integer",
	"HMCharacteristicTypeActive":               "bool",
	"HMCharacteristicTypeRotationSpeed":        "float",
	"HMCharacteristicTypeCurrentPosition":      "integer",
	"HMCharacteristicTypeTargetPosition":       "integer",
}

var AppleUnitToCanonical = map[string]string{
	"percentage":     "percent",
	"celsius":        "celsius",
	"fahrenheit":     "fahrenheit",
	"kelvin":         "kelvin",
	"lux":            "lux",
	"arcdegrees":     "arcdegrees",
	"seconds":        "seconds",
	"minutes":        "minutes",
	"hours":          "hours",
	"micrograms/m^3": "micrograms_per_cubic_meter",
	"ppm":            "ppm",
	"ppb":            "ppb",
}

func MapCharacteristicType(appleType string) string {
	if canonical, ok := AppleTypeToCanonical[appleType]; ok {
		return canonical
	}
	return CharacteristicTypeUnknown + ":" + appleType
}

func MapValueType(appleType string) string {
	if vt, ok := AppleTypeToValue[appleType]; ok {
		return vt
	}
	return "string"
}

func MapUnit(appleUnit string) string {
	if unit, ok := AppleUnitToCanonical[appleUnit]; ok {
		return unit
	}
	return appleUnit
}
