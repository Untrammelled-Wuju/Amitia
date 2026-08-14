package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

type CapabilitySnapshot struct {
	Values []string `json:"values,omitempty"`
	Hash   string   `json:"hash,omitempty"`
}

func NormalizeCapabilities(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func ComputeCapabilitiesHash(values []string) string {
	normalized := NormalizeCapabilities(values)
	joined := strings.Join(normalized, "\n")
	sum := sha256.Sum256([]byte(joined))
	return "sha256:" + hex.EncodeToString(sum[:])
}
