package coordination

import (
	"encoding/json"
	"github.com/u-ai/backend/internal/memorypolicy"
	amitialog "log"
	"time"
)

func ResourceUsable(body json.RawMessage, now time.Time) bool {
	return ResourceAvailable(body, now, false)
}

func ResourceRecallable(body json.RawMessage, now time.Time) bool {
	return ResourceAvailable(body, now, true)
}

func ResourceAvailable(body json.RawMessage, now time.Time, includeRecoverable bool) bool {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(body, &value); err != nil {
		amitialog.Printf("[original-error] %v", err)
		return false
	}
	if value == nil {
		return false
	}
	if _, exists := value["memoryStrength"]; exists && value["retentionLevel"] != nil {
		switch memorypolicy.ReadLifecycle(value, now).Effective(now).DecayState {
		case "archived", "dormant":
			if !includeRecoverable {
				return false
			}
		}
	}
	if raw, exists := value["allowContextUse"]; exists {
		var allowed bool
		if err := json.Unmarshal(raw, &allowed); err != nil {
			amitialog.Printf("[original-error] %v", err)
			return false
		}
		if !allowed {
			return false
		}
	}
	if raw, exists := value["archivedAt"]; exists && string(raw) != "null" {
		var archived string
		if err := json.Unmarshal(raw, &archived); err != nil {
			amitialog.Printf("[original-error] %v", err)
			return false
		}
		if archived != "" {
			return false
		}
	}
	for _, key := range []string{"deleted", "archived", "invalidated", "replaced"} {
		var disabled bool
		if raw, exists := value[key]; exists {
			if err := json.Unmarshal(raw, &disabled); err != nil {
				amitialog.Printf("[original-error] %v", err)
				return false
			}
			if disabled {
				return false
			}
		}
	}
	for _, key := range []string{"validFromUtc", "validFrom"} {
		if raw, exists := value[key]; exists && string(raw) != "null" {
			var starts string
			if err := json.Unmarshal(raw, &starts); err != nil {
				amitialog.Printf("[original-error] %v", err)
				return false
			}
			if starts != "" {
				from, err := parseResourceTime(starts)
				if err != nil {
					amitialog.Printf("[original-error] %v", err)
					return false
				}
				if now.Before(from) {
					return false
				}
			}
		}
	}
	for _, key := range []string{"expiresAt", "validUntil", "validToUtc", "validTo"} {
		var expires string
		if raw, exists := value[key]; exists && string(raw) != "null" {
			if err := json.Unmarshal(raw, &expires); err != nil {
				amitialog.Printf("[original-error] %v", err)
				return false
			}
			if expires != "" {
				until, err := parseResourceTime(expires)
				if err != nil || !until.After(now) {
					if err != nil {
						amitialog.Printf("[original-error] %v", err)
					}
					return false
				}
			}
		}
	}
	if content, exists := value["content"]; exists && len(content) > 0 && content[0] == '{' {
		return ResourceAvailable(content, now, includeRecoverable)
	}
	return true
}

func parseResourceTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		if layout == "2006-01-02" {
			return time.Time{}, err
		}
	}
	return time.Time{}, nil
}
