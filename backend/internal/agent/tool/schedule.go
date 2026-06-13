package tool

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var toolDB *sql.DB

func SetDB(db *sql.DB) {
	toolDB = db
}

func init() {
	Register(Tool{
		Type: "function",
		Function: Function{
			Name:        "create_schedule",
			Description: "为用户创建日程提醒，写入 reminders 表由后端定时调度器自动触发。支持绝对时间和相对时间。当用户说'X分钟后提醒'、'明天X点'、'每天X点提醒我'等时使用。工具内置当前时间，自行计算 remindAt，无需先查时间。",
			Parameters: Parameters{
				Type: "object",
				Properties: map[string]Property{
					"title": {
						Type:        "string",
						Description: "提醒标题，例如'喝水提醒'、'开会'",
					},
					"remindAt": {
						Type:        "string",
						Description: "提醒触发时间。支持绝对格式 YYYY-MM-DD HH:mm:ss 或相对格式 +Ns/+Nm/+Nh（秒/分/小时后）。例如 '+1m' 表示 1 分钟后。当前服务器时间会自动获取。",
					},
					"repeatRule": {
						Type:        "string",
						Description: "重复规则：none（仅一次）、daily（每天）、weekly（每周）、monthly（每月）。默认 none。",
					},
					"content": {
						Type:        "string",
						Description: "提醒时发送的消息内容，留空则使用标题",
					},
				},
				Required: []string{"title", "remindAt"},
			},
		},
	}, createSchedule)
}

func createSchedule(args map[string]interface{}) string {
	if toolDB == nil {
		return "ERROR: database not initialized"
	}

	now := time.Now()

	title, _ := args["title"].(string)
	remindAt, _ := args["remindAt"].(string)
	content, _ := args["content"].(string)
	repeatRule, _ := args["repeatRule"].(string)

	if title == "" || remindAt == "" {
		return "ERROR: missing required fields (title, remindAt)"
	}
	if repeatRule == "" {
		repeatRule = "none"
	}

	resolvedTime, err := resolveTime(remindAt, now)
	if err != nil {
		return fmt.Sprintf("ERROR: %s. 当前时间: %s", err.Error(), now.Format("2006-01-02 15:04:05"))
	}

	remindAtStr := resolvedTime.Format("2006-01-02 15:04:05")

	if repeatRule == "none" && resolvedTime.Before(now) {
		return fmt.Sprintf("ERROR: 提醒时间 %s 已过期。当前时间: %s", remindAtStr, now.Format("2006-01-02 15:04:05"))
	}

	conversationID, _ := args["conversation_id"].(string)
	characterID, _ := args["character_id"].(string)
	channel, _ := args["channel"].(string)
	if channel == "" {
		channel = "web"
	}
	_, err = toolDB.Exec(
		`INSERT INTO reminders (title, content, channel, conversation_id, character_id, remind_at, repeat_rule, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, datetime('now'), datetime('now'))`,
		title, content, channel, conversationID, characterID, remindAtStr, repeatRule,
	)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}

	repeatLabel := "仅一次"
	switch repeatRule {
	case "daily":
		repeatLabel = "每天"
	case "weekly":
		repeatLabel = "每周"
	case "monthly":
		repeatLabel = "每月"
	}

	return fmt.Sprintf("OK %s | %s | %s", title, remindAtStr, repeatLabel)
}

func resolveTime(input string, now time.Time) (time.Time, error) {
	input = strings.TrimSpace(input)

	if strings.HasPrefix(input, "+") {
		numStr := strings.TrimLeft(input[1:], " ")
		if len(numStr) < 2 {
			return now, fmt.Errorf("invalid relative time: %s", input)
		}
		unit := strings.ToLower(numStr[len(numStr)-1:])
		val, err := strconv.Atoi(numStr[:len(numStr)-1])
		if err != nil {
			return now, fmt.Errorf("invalid relative time value: %s", input)
		}
		switch unit {
		case "s":
			return now.Add(time.Duration(val) * time.Second), nil
		case "m":
			return now.Add(time.Duration(val) * time.Minute), nil
		case "h":
			return now.Add(time.Duration(val) * time.Hour), nil
		default:
			return now, fmt.Errorf("invalid relative time unit: %s (use s/m/h)", unit)
		}
	}

	t, err := time.ParseInLocation("2006-01-02 15:04:05", input, time.Local)
	if err == nil {
		return t, nil
	}

	t, err = time.ParseInLocation("2006-01-02 15:04", input, time.Local)
	if err == nil {
		return t, nil
	}

	return now, fmt.Errorf("invalid time format: %s (use YYYY-MM-DD HH:mm:ss or +Nm/+Ns/+Nh)", input)
}
