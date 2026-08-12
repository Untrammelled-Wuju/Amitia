package reminders

func AuthorizationLevelFromNative(native string) AuthorizationLevel {
	switch native {
	case "notDetermined":
		return AuthorizationNotDetermined
	case "restricted":
		return AuthorizationRestricted
	case "denied":
		return AuthorizationDenied
	case "fullAccess":
		return AuthorizationFullAccess
	case "authorized":
		return AuthorizationLegacyAuthorized
	default:
		return AuthorizationNotDetermined
	}
}

func CanReadReminders(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func CanCreateReminders(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func CanUpdateReminders(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func CanDeleteReminders(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func CanListReminderLists(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func PriorityFromNative(native Priority) (Priority, bool) {
	switch native {
	case PriorityNone, PriorityLow, PriorityMedium, PriorityHigh:
		return native, true
	default:
		return PriorityNone, false
	}
}

func PriorityToNative(priority Priority) int {
	switch priority {
	case PriorityNone:
		return 0
	case PriorityLow:
		return 1
	case PriorityMedium:
		return 5
	case PriorityHigh:
		return 9
	default:
		return 0
	}
}

func IsValidAuthorizationLevelForReminders(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}
