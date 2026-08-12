package calendar

func MapAuthorizationLevel(level string) (AuthorizationLevel, bool) {
	switch level {
	case string(AuthorizationNotDetermined):
		return AuthorizationNotDetermined, true
	case string(AuthorizationRestricted):
		return AuthorizationRestricted, true
	case string(AuthorizationDenied):
		return AuthorizationDenied, true
	case string(AuthorizationWriteOnly):
		return AuthorizationWriteOnly, true
	case string(AuthorizationFullAccess):
		return AuthorizationFullAccess, true
	case string(AuthorizationLegacyAuthorized):
		return AuthorizationLegacyAuthorized, true
	default:
		return "", false
	}
}

func AuthorizationLevelFromNative(native string) AuthorizationLevel {
	switch native {
	case "notDetermined":
		return AuthorizationNotDetermined
	case "restricted":
		return AuthorizationRestricted
	case "denied":
		return AuthorizationDenied
	case "writeOnly":
		return AuthorizationWriteOnly
	case "fullAccess":
		return AuthorizationFullAccess
	case "authorized":
		return AuthorizationLegacyAuthorized
	default:
		return AuthorizationNotDetermined
	}
}

func CanCreateEvent(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationWriteOnly, AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func CanReadEvents(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func CanUpdateEvents(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func CanDeleteEvents(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func CanListCalendars(level AuthorizationLevel) bool {
	switch level {
	case AuthorizationFullAccess, AuthorizationLegacyAuthorized:
		return true
	default:
		return false
	}
}

func MapEventSpan(span string) (EventSpan, bool) {
	switch span {
	case string(EventSpanThisEvent), "":
		return EventSpanThisEvent, true
	case string(EventSpanFutureEvents):
		return EventSpanFutureEvents, true
	default:
		return "", false
	}
}

func EventSpanToNative(span EventSpan) string {
	switch span {
	case EventSpanThisEvent:
		return "thisEvent"
	case EventSpanFutureEvents:
		return "futureEvents"
	default:
		return "thisEvent"
	}
}
