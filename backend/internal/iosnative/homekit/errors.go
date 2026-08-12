package homekit

const (
	ErrUnsupported                            = "HOMEKIT_UNSUPPORTED"
	ErrDisabledByUser                         = "HOMEKIT_DISABLED_BY_USER"

	ErrNotInitialized                         = "HOMEKIT_NOT_INITIALIZED"
	ErrInitialLoadPending                     = "HOMEKIT_INITIAL_LOAD_PENDING"

	ErrAuthNotDetermined                      = "HOMEKIT_AUTH_NOT_DETERMINED"
	ErrAuthDenied                             = "HOMEKIT_AUTH_DENIED"
	ErrAuthRestricted                         = "HOMEKIT_AUTH_RESTRICTED"
	ErrHomeAccessNotAuthorized                = "HOMEKIT_HOME_ACCESS_NOT_AUTHORIZED"

	ErrHomeNotFound                           = "HOMEKIT_HOME_NOT_FOUND"
	ErrRoomNotFound                           = "HOMEKIT_ROOM_NOT_FOUND"
	ErrZoneNotFound                           = "HOMEKIT_ZONE_NOT_FOUND"
	ErrAccessoryNotFound                      = "HOMEKIT_ACCESSORY_NOT_FOUND"
	ErrServiceNotFound                        = "HOMEKIT_SERVICE_NOT_FOUND"
	ErrCharacteristicNotFound                 = "HOMEKIT_CHARACTERISTIC_NOT_FOUND"

	ErrAccessoryUnreachable                   = "HOMEKIT_ACCESSORY_UNREACHABLE"
	ErrAccessoryBlocked                       = "HOMEKIT_ACCESSORY_BLOCKED"

	ErrCharacteristicNotReadable              = "HOMEKIT_CHARACTERISTIC_NOT_READABLE"
	ErrCharacteristicNotWritable              = "HOMEKIT_CHARACTERISTIC_NOT_WRITABLE"
	ErrCharacteristicEventUnsupported         = "HOMEKIT_CHARACTERISTIC_EVENT_UNSUPPORTED"

	ErrValueTypeInvalid                       = "HOMEKIT_VALUE_TYPE_INVALID"
	ErrValueOutOfRange                        = "HOMEKIT_VALUE_OUT_OF_RANGE"
	ErrValueNotAllowed                        = "HOMEKIT_VALUE_NOT_ALLOWED"

	ErrReadFailed                             = "HOMEKIT_READ_FAILED"
	ErrWriteFailed                            = "HOMEKIT_WRITE_FAILED"
	ErrWriteOutcomeUnknown                    = "HOMEKIT_WRITE_OUTCOME_UNKNOWN"

	ErrSceneNotFound                          = "HOMEKIT_SCENE_NOT_FOUND"
	ErrSceneExecuteFailed                     = "HOMEKIT_SCENE_EXECUTE_FAILED"
	ErrSceneCreateFailed                      = "HOMEKIT_SCENE_CREATE_FAILED"
	ErrSceneUpdateFailed                      = "HOMEKIT_SCENE_UPDATE_FAILED"
	ErrSceneDeleteFailed                      = "HOMEKIT_SCENE_DELETE_FAILED"
	ErrSceneOutcomeUnknown                    = "HOMEKIT_SCENE_OUTCOME_UNKNOWN"

	ErrAutomationNotFound                     = "HOMEKIT_AUTOMATION_NOT_FOUND"
	ErrAutomationTypeUnsupported              = "HOMEKIT_AUTOMATION_TYPE_UNSUPPORTED"
	ErrAutomationCreateFailed                 = "HOMEKIT_AUTOMATION_CREATE_FAILED"
	ErrAutomationUpdateFailed                 = "HOMEKIT_AUTOMATION_UPDATE_FAILED"
	ErrAutomationEnableFailed                 = "HOMEKIT_AUTOMATION_ENABLE_FAILED"
	ErrAutomationDeleteFailed                 = "HOMEKIT_AUTOMATION_DELETE_FAILED"
	ErrAutomationOutcomeUnknown               = "HOMEKIT_AUTOMATION_OUTCOME_UNKNOWN"

	ErrSetupUserCancelled                     = "HOMEKIT_SETUP_USER_CANCELLED"
	ErrSetupFailed                            = "HOMEKIT_SETUP_FAILED"

	ErrAmbiguousTarget                        = "HOMEKIT_AMBIGUOUS_TARGET"

	ErrTimeout                                = "HOMEKIT_TIMEOUT"
	ErrCancelled                              = "HOMEKIT_CANCELLED"

	ErrNativeBridgeUnavailable                = "HOMEKIT_NATIVE_BRIDGE_UNAVAILABLE"
	ErrInvalidResponse                        = "HOMEKIT_INVALID_RESPONSE"
)
