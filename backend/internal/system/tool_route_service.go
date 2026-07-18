// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/temporal"
)

func (s *service) ToolRoute(body map[string]interface{}) map[string]interface{} {
	tool, _ := body["tool"].(string)
	if tool == "" {
		tool = "unknown"
	}
	switch tool {
	case "system_time":
		return s.systemTimeResult(body)
	default:
		return map[string]interface{}{"routed": true, "tool": tool}
	}
}

func (s *service) systemTimeResult(body map[string]interface{}) map[string]interface{} {
	now := time.Now()
	fallback := map[string]interface{}{
		"routed":            true,
		"tool":              "system_time",
		"source":            "system-fallback",
		"timezoneConfirmed": false,
		"utc":               now.UTC().Format(time.RFC3339),
		"local":             now.Format(time.RFC3339),
		"weekday":           now.Weekday().String(),
		"timestamp_unix_ms": now.UnixMilli(),
	}
	if s.temporalSvc == nil {
		return fallback
	}
	characterID, _ := body["characterId"].(string)
	userID, _ := body["userId"].(string)
	if userID == "" {
		userID = "default"
	}
	channel, _ := body["channel"].(string)
	if channel == "" {
		channel = "web"
	}
	snapshot, err := s.temporalSvc.ResolveSnapshot(context.Background(), temporal.SnapshotInput{
		UserID:      userID,
		CharacterID: characterID,
		Channel:     channel,
	})
	if err != nil {
		return fallback
	}
	return map[string]interface{}{
		"routed":            true,
		"tool":              "system_time",
		"source":            "temporal-runtime",
		"timezoneConfirmed": true,
		"utc":               snapshot.NowUTC.Format(time.RFC3339),
		"userLocal":         fmt.Sprintf("%s %s %s", snapshot.UserTime.LocalTime.Format("15:04:05"), snapshot.UserTime.Weekday, snapshot.UserTime.Daypart),
		"userWeekday":       snapshot.UserTime.Weekday,
		"userDaypart":       snapshot.UserTime.Daypart,
		"characterLocal":    fmt.Sprintf("%s %s %s", snapshot.CharacterTime.LocalTime.Format("15:04:05"), snapshot.CharacterTime.Weekday, snapshot.CharacterTime.Daypart),
		"weekday":           snapshot.NowUTC.Weekday().String(),
		"timestamp_unix_ms": snapshot.NowUTC.UnixMilli(),
		"snapshotVersion":   snapshot.Version,
	}
}
