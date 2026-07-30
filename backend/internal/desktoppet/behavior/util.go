package behavior

import "github.com/google/uuid"

func UUIDNew() string {
	return uuid.NewString()
}

func dedupKey(parts ...string) string {
	result := ""
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i > 0 && result != "" {
			result += ":"
		}
		result += p
	}
	return result
}
