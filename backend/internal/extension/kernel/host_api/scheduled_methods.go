package host_api

import "strings"

func scheduledMethod(method Method) (string, string, bool) {
	value := string(method)
	for _, kind := range []string{"reminder", "proactive_task"} {
		prefix := "host." + kind + "."
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		action := strings.TrimPrefix(value, prefix)
		grant := "manage"
		switch action {
		case "list", "get", "history", "capabilities", "status", "queue", "preview":
			grant = "read"
		case "create":
			grant = "create"
		case "execute":
			grant = "execute"
		case "update", "pause", "resume", "cancel", "clear":
		default:
			return "", "", false
		}
		return kind + "." + grant, grant, true
	}
	return "", "", false
}

func ScheduledMethods() []Method {
	methods := []Method{}
	for _, kind := range []string{"reminder", "proactive_task"} {
		for _, action := range []string{"create", "list", "get", "update", "pause", "resume", "cancel", "execute", "history", "capabilities", "status", "queue", "preview", "clear"} {
			methods = append(methods, Method("host."+kind+"."+action))
		}
	}
	return methods
}
