package chat

import (
	"encoding/json"
	"strings"
)

func validateAgentToolArguments(raw string) (string, string) {
	if strings.TrimSpace(raw) == "" {
		return "{}", ""
	}
	if len(raw) > 256*1024 {
		return raw, "tool arguments exceed the 256 KiB limit"
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &object); err != nil || object == nil {
		return raw, "tool arguments must be a valid JSON object"
	}
	return raw, ""
}
