// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

var toolDB *sql.DB

func SetDB(db *sql.DB) {
	toolDB = db
}

var CurrentCharacterID string

func SetCurrentCharacterID(id string) {
	CurrentCharacterID = id
}

var CurrentConversationID string

func SetCurrentConversationID(id string) {
	CurrentConversationID = id
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

func createSchedule(args map[string]interface{}) string {
	if toolDB == nil {
		return "ERROR: database not initialized"
	}

	title, _ := args["title"].(string)
	desc, _ := args["description"].(string)
	dueTime, _ := args["due_time"].(string)
	repeat, _ := args["repeat"].(string)
	channel, _ := args["channel"].(string)
	if title == "" || dueTime == "" {
		return "ERROR: title and due_time are required"
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
	id := fmt.Sprintf("sched-%d", time.Now().UnixNano())
	_, err := toolDB.Exec(
		"INSERT INTO schedules (id, title, description, due_time, repeat_mode, channel, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?)",
		id, title, desc, dueTime, repeat, channel, now, now,
	)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}
	return fmt.Sprintf("OK 已创建日程：%s（截止 %s）", title, dueTime)
}

var activeScheduleVar *bool

func SetActiveSchedule(v *bool) {
	activeScheduleVar = v
}
