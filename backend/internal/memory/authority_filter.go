package memory

import (
	"strings"
	"time"

	"github.com/u-ai/backend/internal/mindruntime"
)

type retrievalAuthorityPolicy struct {
	CharacterID      string
	UserID           string
	ProactiveMention bool
	Now              time.Time
}

func memoryAllowedBySQLiteAuthority(m Memory, policy retrievalAuthorityPolicy) bool {
	if !memoryMatchesRetrievalScope(m, policy.CharacterID, policy.UserID) {
		return false
	}
	if !memoryAllowedForDerivedContent(m, policy.Now) {
		return false
	}
	if policy.ProactiveMention && !m.AllowProactiveMention {
		return false
	}
	return true
}

func tombstoneTargetsFromMemorySearch(coordinator *mindruntime.DataLifecycleCoordinator, characterID string) map[string]bool {
	if coordinator == nil {
		return nil
	}
	blocked := make(map[string]bool)
	if coordinator.IsRetrievalBlocked(characterID) {
		blocked[characterID] = true
	}
	for _, id := range coordinator.BlockedEntityIDsByType("memory") {
		blocked[id] = true
	}
	if len(blocked) == 0 {
		return nil
	}
	return blocked
}

func memoryAllowedForDerivedContent(m Memory, now time.Time) bool {
	if memoryStatusBlocksRetrieval(m.VerifiedStatus) {
		return false
	}
	if memoryExpired(m.ExpiresAt, now) {
		return false
	}
	return true
}

func memoryMatchesRetrievalScope(m Memory, characterID, userID string) bool {
	characterID = strings.TrimSpace(characterID)
	userID = strings.TrimSpace(userID)
	scope := strings.ToLower(strings.TrimSpace(m.Scope))
	if scope == "user" || scope == "user_global" {
		if userID != "" {
			return m.CharacterID == userID
		}
		if characterID != "" {
			return m.CharacterID == characterID
		}
		return true
	}
	if characterID != "" {
		return m.CharacterID == characterID
	}
	if userID != "" {
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
