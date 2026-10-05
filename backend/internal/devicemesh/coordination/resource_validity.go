package coordination

import (
	"encoding/json"
	"time"
)

func ResourceUsable(body json.RawMessage, now time.Time) bool {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(body, &value); err != nil || value == nil {
		return false
	}
	if raw, exists := value["allowContextUse"]; exists {
		var allowed bool
		if json.Unmarshal(raw, &allowed) != nil || !allowed {
			return false
		}
	}
	if raw, exists := value["archivedAt"]; exists && string(raw) != "null" {
		var archived string
		if json.Unmarshal(raw, &archived) != nil || archived != "" {
			return false
		}
	}
	for _, key := range []string{"deleted", "archived", "invalidated", "replaced"} {
		var disabled bool
		if raw, exists := value[key]; exists {
			if err := json.Unmarshal(raw, &disabled); err == nil && disabled {
				return false
			}
		}
	}
	for _, key := range []string{"expiresAt", "validUntil"} {
		var expires string
		if raw, exists := value[key]; exists && string(raw) != "null" {
			if err := json.Unmarshal(raw, &expires); err != nil {
				return false
			}
			if expires != "" {
				until, err := time.Parse(time.RFC3339Nano, expires)
				if err != nil || !until.After(now) {
					return false
				}
			}
		}
	}
	if content, exists := value["content"]; exists && len(content) > 0 && content[0] == '{' {
		return ResourceUsable(content, now)
	}
	return true
}
