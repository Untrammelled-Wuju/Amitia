// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"strings"
	"time"
)

var toolDB *sql.DB

func SetDB(db *sql.DB) {
	toolDB = db
}

var OnMemorySaved func(id, key, value, memoryType, characterID string)

func SetOnMemorySaved(fn func(id, key, value, memoryType, characterID string)) {
	OnMemorySaved = fn
}

var OnProfileSaved func(id string)

func SetOnProfileSaved(fn func(id string)) {
	OnProfileSaved = fn
}

var OnEpisodicSaved func(id string)

func SetOnEpisodicSaved(fn func(id string)) {
	OnEpisodicSaved = fn
}

func init() {
	Register(Tool{
		Type: "function",
		Function: Function{
			Name:        "create_schedule",
			Description: "创建一条待办日程。当用户提到要做某事、约时间、定闹钟、提醒等时调用。可以创建单次或重复日程。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"title": {
						Type:        "string",
						Description: "日程标题",
					},
					"description": {
						Type:        "string",
						Description: "日程详细描述",
					},
					"due_time": {
						Type:        "string",
						Description: "截止时间，格式 YYYY-MM-DD HH:MM，如 2025-01-15 14:30",
					},
					"repeat": {
						Type:        "string",
						Description: "重复规则：none/daily/weekly/monthly",
					},
					"channel": {
						Type:        "string",
						Description: "发送通知的渠道：wechat/qq/all，默认all",
					},
				},
				Required: []string{"title", "due_time"},
			},
		},
	}, createSchedule)
}

func createSchedule(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopedCtx, scopeErr := requireScopedWrite(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	execCtx = scopedCtx
	if toolDB == nil {
		return ErrorResult("database_not_initialized", "ERROR: database not initialized")
	}

	title, _ := args["title"].(string)
	desc, _ := args["description"].(string)
	dueTime, _ := args["due_time"].(string)
	repeat, _ := args["repeat"].(string)
	channel, _ := args["channel"].(string)
	if channel == "" {
		channel = execCtx.Channel
	}
	if title == "" || dueTime == "" {
		return ErrorResult("invalid_args", "ERROR: title and due_time are required")
	}
	if repeat == "" {
		repeat = "none"
	}
	if channel == "" {
		channel = "all"
	}
	title = strings.TrimSpace(title)
	desc = strings.TrimSpace(desc)
	now := time.Now().Format("2006-01-02 15:04:05")
	id := fmt.Sprintf("sched-%s", uuid.New().String())
	_, err := toolDB.Exec(
		"INSERT INTO schedules (id, title, description, due_time, repeat_mode, channel, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?)",
		id, title, desc, dueTime, repeat, channel, now, now,
	)
	if err != nil {
		return ErrorResult("database_error", fmt.Sprintf("ERROR: %s", err.Error()))
	}
	result := TextResult(fmt.Sprintf("OK 已创建日程：%s（截止 %s）", title, dueTime))
	result.ExternalOperationID = id
	result.SideEffects = []ToolSideEffect{{Type: "schedule_create", TargetID: id, Confirmed: true}}
	result.Audit = map[string]interface{}{"channel": channel, "repeat": repeat, "due_time": dueTime, "character_id": execCtx.CharacterID, "conversation_id": execCtx.ConversationID}
	return result
}

var activeScheduleVar *bool

func SetActiveSchedule(v *bool) {
	activeScheduleVar = v
}
