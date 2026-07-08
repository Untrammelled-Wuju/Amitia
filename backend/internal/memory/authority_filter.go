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

func shouldExtractFromMessage(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	return role == "user"
}

func isTransientContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}
	greetings := []string{"你好", "在吗", "早安", "晚安", "午安", "早上好", "晚上好", "下午好", "hi", "hello", "hey"}
	for _, g := range greetings {
		if strings.EqualFold(trimmed, g) {
			return true
		}
	}
	pureEmoticons := []string{"哈哈哈", "哈哈", "嘿嘿", "嘻嘻", "呵呵", "嗯嗯", "哦哦", "好的", "ok", "OK", "嗯", "哦", "啊"}
	for _, e := range pureEmoticons {
		if trimmed == e {
			return true
		}
	}
	return false
}

func filterExtractableMessages(messages []map[string]string) []map[string]string {
	filtered := make([]map[string]string, 0, len(messages))
	hasUser := false
	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg["role"]))
		if role == "user" && !isTransientContent(msg["content"]) {
			hasUser = true
		}
		filtered = append(filtered, msg)
	}
	if !hasUser {
		return nil
	}
	return filtered
}
