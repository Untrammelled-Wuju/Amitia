// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"fmt"
	"time"
)

func (s *service) moodRecoveryCheck(convID, charID, source string) {
	if source == "proactive" || source == "system" {
		return
	}
	var lastAt string
	err := s.db.Table("messages").Select("created_at").Where("role = 'user' AND conversation_id = ?", convID).Order("created_at DESC").Offset(1).Limit(1).Row().Scan(&lastAt)
	if err != nil || lastAt == "" {
		return
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", lastAt, time.Local)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", lastAt)
	}
	if err != nil {
		return
	}
	idleDur := time.Since(t)
	if idleDur > 6*time.Hour {
		idleHours := int(idleDur.Hours())
		s.db.Exec("INSERT INTO mood_logs (character_id, mood, intensity, source, note, created_at) VALUES (?, 'happy', 7, 'reconnect', ?, datetime('now', 'localtime'))", charID, fmt.Sprintf("用户回来了 (冷落%d小时)", idleHours))
	}
}
