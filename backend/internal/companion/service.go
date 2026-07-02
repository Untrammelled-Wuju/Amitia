// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/embedding"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	GetSleepSetting(characterID string) map[string]interface{}
	UpdateSleepSetting(body map[string]interface{}, characterID string) map[string]interface{}
	GetSchedule(date string, characterID string) map[string]interface{}
	GetScheduleConflicts(date string, characterID string) []map[string]interface{}
	GetScheduleToday(characterID string) map[string]interface{}
	GetStateLife(characterID string) map[string]interface{}
	GetState(characterID string) map[string]interface{}
	GetTimelineToday(characterID string) map[string]interface{}
	ListFixedEvents(date string, characterID string) []map[string]interface{}
	GetFixedEvent(id int) map[string]interface{}
	CreateFixedEvent(body map[string]interface{}, characterID string) map[string]interface{}
	UpdateFixedEvent(id int, body map[string]interface{}, characterID string) map[string]interface{}
	DeleteFixedEvent(id int, characterID string) bool
	ToggleFixedEventEnabled(id int) map[string]interface{}
	ListSpecialEvents(characterID string) []map[string]interface{}
	CreateSpecialEvent(body map[string]interface{}, characterID string) map[string]interface{}
	UpdateSpecialEvent(id int, body map[string]interface{}, characterID string) map[string]interface{}
	DeleteSpecialEvent(id int, characterID string) bool
	ToggleSpecialEventEnabled(id int) map[string]interface{}
	ListClassAdjustments(characterID string) []map[string]interface{}
	CreateClassAdjustment(body map[string]interface{}, characterID string) map[string]interface{}
	UpdateClassAdjustment(id int, body map[string]interface{}, characterID string) map[string]interface{}
	DeleteClassAdjustment(id int, characterID string) bool
	GetEffectiveClasses(date string, characterID string) []map[string]interface{}
	GetLifestyleTendency(characterID string) map[string]interface{}
	UpdateLifestyleTendency(body map[string]interface{}, characterID string) map[string]interface{}
	ResetLifestyleTendency(characterID string) map[string]interface{}
	GetWorkProfile(characterID string) map[string]interface{}
	UpdateWorkProfile(body map[string]interface{}, characterID string) map[string]interface{}
	GetActiveMessageSetting(characterID string) map[string]interface{}
	UpdateActiveMessageSetting(body map[string]interface{}, characterID string) map[string]interface{}
	GetActiveMessageTasksToday(characterID string) []map[string]interface{}
	RegenerateActiveMessageTasks(characterID string) map[string]interface{}
	RunActiveMessageTask(id int, characterID string) map[string]interface{}
	CancelActiveMessageTask(id int, characterID string) map[string]interface{}
	ListDelayedReplies(characterID string) []map[string]interface{}
	CancelDelayedReply(id int, characterID string) map[string]interface{}
	ProcessDelayedReplies(characterID string) map[string]interface{}
	ProcessDueActiveMessageTasks(characterID string) map[string]interface{}
	GetDebugOverview(characterID string) map[string]interface{}
	RegenerateAllDebug(characterID string) map[string]interface{}
	ProcessActiveMessagesDebug(characterID string) map[string]interface{}
	ProcessDelayedRepliesDebug(characterID string) map[string]interface{}
	GetRuleLogs(characterID string) []map[string]interface{}
	RegenerateSchedule(characterID string) map[string]interface{}
	RegenerateTimeline(characterID string) map[string]interface{}
	ScheduleBasedGenerator(date string, characterID string) map[string]interface{}
	GenerateSharePrompt(characterID string, taskType string, schedule TodaySchedule, mood string, energy int) string
	GetShareHistory(characterID string) ShareHistory
	TriggerDailyRegeneration(characterID string) map[string]interface{}
	RandomBurstTrigger(characterID string) map[string]interface{}
}

type service struct {
	db              *gorm.DB
	embeddingSvc    *embedding.Service
	lastBurstAt     time.Time
	todayBurstCount int
}

func NewService(ctx *app.AppContext) Service {
	return &service{db: ctx.DB, embeddingSvc: embedding.NewService(ctx.DB)}
}

func toJSON(v interface{}) string { b, _ := json.Marshal(v); return string(b) }

func parseWorkDays(s string) map[int]bool {
	result := map[int]bool{}
	parts := []string{}
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	dayMap := map[string]int{"1": 1, "2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "0": 0, "7": 0}
	for _, p := range parts {
		p = trimSpace(p)
		if d, ok := dayMap[p]; ok {
			result[d] = true
			continue
		}
		if idx := indexOf(p, "-"); idx >= 0 {
			from := trimSpace(p[:idx])
			to := trimSpace(p[idx+1:])
			fd, fok := dayMap[from]
			td, tok := dayMap[to]
			if fok && tok {
				for d := fd; d <= td; d++ {
					result[d] = true
				}
			}
		}
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func hashInt(n int) int {
	n = ((n >> 16) ^ n) * 0x45d9f3b
	n = ((n >> 16) ^ n) * 0x45d9f3b
	n = (n >> 16) ^ n
	if n < 0 {
		n = -n
	}
	return n
}
