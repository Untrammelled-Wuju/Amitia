package modelprotocol

import "encoding/json"

func UsageReported(data []byte, keys ...string) bool {
	var response map[string]json.RawMessage
	if json.Unmarshal(data, &response) != nil {
		return false
	}
	for _, key := range keys {
		raw := response[key]
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		if raw[0] == '{' {
			var fields map[string]json.RawMessage
			if json.Unmarshal(raw, &fields) == nil && len(fields) > 0 {
				return true
			}
		} else {
			return true
		}
	}
	return false
}
