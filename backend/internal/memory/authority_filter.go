package memory

import (
	"strings"
	"time"
)

type retrievalAuthorityPolicy struct {
	CharacterID      string
	ProactiveMention bool
	Now              time.Time
}

func memoryAllowedBySQLiteAuthority(m Memory, policy retrievalAuthorityPolicy) bool {
	if policy.CharacterID != "" && m.CharacterID != policy.CharacterID && m.Scope != "user" {
		return false
	}
	if memoryStatusBlocksRetrieval(m.VerifiedStatus) {
		return false
	}
	if memoryExpired(m.ExpiresAt, policy.Now) {
		return false
	}
	if policy.ProactiveMention && !m.AllowProactiveMention {
		return false
	}
	return true
}

func memoryStatusBlocksRetrieval(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "deleted", "invalidated", "expired", "rejected", "tombstone", "tombstoned", "inactive":
		return true
	default:
		return false
	}
}

func memoryExpired(expiresAt *string, now time.Time) bool {
	if expiresAt == nil || strings.TrimSpace(*expiresAt) == "" {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	raw := strings.TrimSpace(*expiresAt)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"} {
		t, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return !t.After(now)
		}
	}
	return true
}
