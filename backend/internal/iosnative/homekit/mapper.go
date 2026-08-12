package homekit

import "github.com/u-ai/backend/internal/nativebridge"

func AuthorizationStatusFromNative(nativeBits int) AuthorizationStatus {
	determined := nativeBits&1 != 0
	authorized := nativeBits&2 != 0
	restricted := nativeBits&4 != 0

	if restricted {
		return AuthRestricted
	}
	if authorized {
		return AuthAuthorized
	}
	if determined {
		return AuthDenied
	}
	return AuthNotDetermined
}

func AuthorizationStatusToCanonical(status AuthorizationStatus) string {
	return string(status)
}

func MapCapabilityState(status AuthorizationStatus, enabledByUser, initialized, initialLoadCompleted bool, homeCount int) HomeKitState {
	state := HomeKitState{
		Supported:              true,
		EnabledByUser:          enabledByUser,
		Initialized:            initialized,
		InitialLoadCompleted:   initialLoadCompleted,
		Authorization:          string(status),
		HomeCount:              homeCount,
		CanRead:                status == AuthAuthorized,
		CanControl:             status == AuthAuthorized,
	}

	switch status {
	case AuthNotDetermined:
		state.State = "not_determined"
		if !enabledByUser {
			state.Reason = "not_enabled_by_user"
		}
	case AuthAuthorized:
		state.State = "authorized"
	case AuthDenied:
		state.State = "denied"
		state.Reason = "user_denied"
	case AuthRestricted:
		state.State = "restricted"
		state.Reason = "device_restriction"
	default:
		state.State = "unknown"
	}

	return state
}

func MapAccessoryCategory(appleCategory string) string {
	switch appleCategory {
	case "HMAccessoryCategoryLightbulb", "HMAccessoryCategoryLighting":
		return "light"
	case "HMAccessoryCategoryDoorLock", "HMAccessoryCategoryLock":
		return "lock"
	case "HMAccessoryCategoryThermostat":
		return "thermostat"
	case "HMAccessoryCategoryFan":
		return "fan"
	case "HMAccessoryCategoryGarageDoorOpener":
		return "garage_door"
	case "HMAccessoryCategoryCamera", "HMAccessoryCategoryIPCamera", "HMAccessoryCategoryDoorbell":
		return "camera"
	case "HMAccessoryCategorySensor", "HMAccessoryCategoryContactSensor", "HMAccessoryCategoryMotionSensor",
		"HMAccessoryCategoryLeakSensor", "HMAccessoryCategorySmokeSensor", "HMAccessoryCategoryCarbonMonoxideSensor",
		"HMAccessoryCategoryCarbonDioxideSensor", "HMAccessoryCategoryAirQualitySensor",
		"HMAccessoryCategoryHumiditySensor", "HMAccessoryCategoryLightSensor", "HMAccessoryCategoryTemperatureSensor",
		"HMAccessoryCategoryOccupancySensor":
		return "sensor"
	case "HMAccessoryCategoryOutlet":
		return "outlet"
	case "HMAccessoryCategorySwitch":
		return "switch"
	case "HMAccessoryCategoryWindow", "HMAccessoryCategoryWindowCovering":
		return "window"
	case "HMAccessoryCategorySecuritySystem":
		return "security_system"
	case "HMAccessoryCategoryDoor":
		return "door"
	case "HMAccessoryCategorySprinkler":
		return "sprinkler"
	case "HMAccessoryCategoryFaucet", "HMAccessoryCategoryShowerHead":
		return "faucet"
	case "HMAccessoryCategoryTelevision", "HMAccessoryCategoryTelevisionSpeaker", "HMAccessoryCategorySpeaker":
		return "speaker"
	case "HMAccessoryCategoryBridge":
		return "bridge"
	case "HMAccessoryCategoryRangeExtender", "HMAccessoryCategoryWiFiRouter", "HMAccessoryCategoryResidentDevice":
		return "hub"
	default:
		return "accessory"
	}
}

func MapNativeServiceType(appleServiceType string) string {
	return appleServiceType
}

func MapNativeTriggerType(appleTriggerType string) string {
	switch appleTriggerType {
	case "HMEventTrigger":
		return AutomationTypeEvent
	case "HMTimerTrigger":
		return AutomationTypeTimerLegacy
	default:
		return AutomationTypeUnknown
	}
}

func MapErrorToNativeBridge(errCode string) string {
	switch errCode {
	case ErrAuthDenied, ErrAuthRestricted, ErrHomeAccessNotAuthorized:
		return nativebridge.ErrAuthorizationDenied
	case ErrNotInitialized, ErrInitialLoadPending:
		return nativebridge.ErrBridgeDisconnected
	case ErrDisabledByUser:
		return nativebridge.ErrOperationNotSupported
	default:
		return nativebridge.ErrBridgeInvalidResponse
	}
}

type HomeKitState struct {
	Supported              bool   `json:"supported"`
	EnabledByUser          bool   `json:"enabledByUser"`
	Initialized            bool   `json:"initialized"`
	InitialLoadCompleted   bool   `json:"initialLoadCompleted"`
	Authorization          string `json:"authorization"`
	HomeCount              int    `json:"homeCount"`
	CanRead                bool   `json:"canRead"`
	CanControl             bool   `json:"canControl"`
	State                  string `json:"state"`
	Reason                 string `json:"reason,omitempty"`
}
