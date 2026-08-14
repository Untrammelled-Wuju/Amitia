package event

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func BuildIdempotencyKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "|")))
	return hex.EncodeToString(sum[:])
}
