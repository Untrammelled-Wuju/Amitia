package notificationruntime

import "strings"

var ackableNotificationTypes = map[string]struct{}{
	"reminder.triggered": {},
	"proactive.message":  {},
	"attention.required": {},
	"system.test":        {},
}

var outboxTerminalStatuses = []string{"delivered", "displayed", "clicked", "display_failed", "expired"}

func AckableNotificationType(notificationType string) bool {
	_, ok := ackableNotificationTypes[strings.ToLower(strings.TrimSpace(notificationType))]
	return ok
}

func outboxAcceptedStatus(notificationType string) string {
	if AckableNotificationType(notificationType) {
		return "accepted"
	}
	return "delivered"
}
