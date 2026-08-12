package contacts

import "github.com/u-ai/backend/internal/nativebridge"

func AuthorizationLevelFromNative(native string) AuthorizationLevel {
	switch native {
	case "not_determined", "notDetermined":
		return AuthorizationNotDetermined
	case "restricted":
		return AuthorizationRestricted
	case "denied":
		return AuthorizationDenied
	case "authorized":
		return AuthorizationAuthorized
	case "limited":
		return AuthorizationLimited
	default:
		return AuthorizationNotDetermined
	}
}

func AuthorizationLevelToCanonical(level AuthorizationLevel) string {
	return string(level)
}

func IsAuthorized(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationAuthorized:
		return true
	case AuthorizationLimited:
		return true
	default:
		return false
	}
}

func CanCreateContact(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationAuthorized:
		return true
	case AuthorizationLimited:
		return true
	default:
		return false
	}
}

func CanReadContacts(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationAuthorized:
		return true
	case AuthorizationLimited:
		return true
	default:
		return false
	}
}

func CanUpdateContact(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationAuthorized:
		return true
	case AuthorizationLimited:
		return true
	default:
		return false
	}
}

func CanDeleteContact(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationAuthorized:
		return true
	case AuthorizationLimited:
		return true
	default:
		return false
	}
}

func CanManageLimitedAccess(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationAuthorized:
		return true
	case AuthorizationLimited:
		return true
	default:
		return false
	}
}

func EffectiveLevel(level AuthorizationLevel) string {
	switch level {
	case AuthorizationAuthorized, AuthorizationLimited:
		return "read_write"
	default:
		return "no_access"
	}
}

func ContainerTypeFromNative(native string) string {
	switch native {
	case "local", "carddav", "exchange", "ldap", "subscribed", "unnamed":
		return native
	default:
		return "local"
	}
}

func GroupTypeFromNative(native string) string {
	switch native {
	case "smart", "subscription", "group", "folder":
		return native
	default:
		return "group"
	}
}

func MapAuthorizationStatus(level AuthorizationLevel) AuthorizationStatusResult {
	return AuthorizationStatusResult{
		Level:                  string(level),
		EffectiveLevel:         EffectiveLevel(level),
		Limited:                level == AuthorizationLimited,
		CanRead:                CanReadContacts(level),
		CanCreate:              CanCreateContact(level),
		CanUpdate:              CanUpdateContact(level),
		CanDelete:              CanDeleteContact(level),
		CanManageLimitedAccess: CanManageLimitedAccess(level),
		CanReadNotes:           level == AuthorizationAuthorized,
	}
}

func MapCapabilityState(level AuthorizationLevel, canReadNotes bool) CapabilityState {
	state := CapabilityState{
		Supported:              true,
		Authorization:          string(level),
		Limited:                level == AuthorizationLimited,
		CanRead:                CanReadContacts(level),
		CanCreate:              CanCreateContact(level),
		CanUpdate:              CanUpdateContact(level),
		CanDelete:              CanDeleteContact(level),
		CanManageLimitedAccess: CanManageLimitedAccess(level),
		CanReadNotes:           canReadNotes && level == AuthorizationAuthorized,
	}

	switch level {
	case AuthorizationNotDetermined:
		state.State = "not_determined"
	case AuthorizationRestricted:
		state.State = "restricted"
		state.Reason = "device_restriction"
	case AuthorizationDenied:
		state.State = "denied"
		state.Reason = "user_denied"
	case AuthorizationAuthorized:
		state.State = "authorized"
	case AuthorizationLimited:
		state.State = "limited"
	default:
		state.State = "unknown"
	}

	return state
}

func MapErrorToNativeBridge(errCode string) string {
	switch errCode {
	case ErrPermissionDenied:
		return nativebridge.ErrAuthorizationDenied
	case ErrPermissionRestricted:
		return nativebridge.ErrAuthorizationDenied
	case ErrNotesEntitlementRequired:
		return nativebridge.ErrOperationNotSupported
	case ErrInvalidContactID, ErrInvalidName, ErrInvalidPhone,
		ErrInvalidEmail, ErrInvalidAddress, ErrInvalidDate,
		ErrInvalidQuery, ErrInvalidSearchField:
		return nativebridge.ErrBridgeInvalidResponse
	case ErrPhotoInvalid, ErrPhotoTooLarge:
		return nativebridge.ErrBridgeInvalidResponse
	case ErrContainerNotFound, ErrContainerReadOnly:
		return nativebridge.ErrBridgeInvalidResponse
	default:
		return nativebridge.ErrBridgeInvalidResponse
	}
}
