package coordination

import (
	"bytes"
	"encoding/json"
)

func memoryContentChanged(previous, next json.RawMessage) bool {
	project := func(raw json.RawMessage) []byte {
		var document map[string]json.RawMessage
		if json.Unmarshal(raw, &document) != nil {
			return raw
		}
		for _, key := range []string{"executionScope", "createdAt", "updatedAt", "retentionLevel", "pinned", "memoryStrength", "strengthUpdatedAt", "lastReinforcedAt", "reinforceCount", "retrievedCount", "injectedCount", "decayState", "archivedAt", "expiresAt", "allowContextUse", "effectiveStrength", "effectiveDecayState", "effectiveRetentionLevel"} {
			delete(document, key)
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			return raw
		}
		return encoded
	}
	return !bytes.Equal(project(previous), project(next))
}
