// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

// SchedulerRunning is set by main.go when the proactive/reminder cron scheduler starts.
var SchedulerRunning = false

type ProactiveRule struct {
	ID             int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name           string  `gorm:"column:name;not null" json:"name"`
	Enabled        int     `gorm:"column:enabled;default:1" json:"enabled"`
	Channel        string  `gorm:"column:channel;default:web" json:"channel"`
	ConversationID string  `gorm:"column:conversation_id" json:"conversationId"`
	CharacterID    string  `gorm:"column:character_id" json:"characterId"`
	RuleType       string  `gorm:"column:rule_type;default:cron" json:"ruleType"`
	ScheduleCron   string  `gorm:"column:schedule_cron" json:"scheduleCron"`
	QuietStart     string  `gorm:"column:quiet_start" json:"quietStart"`
	QuietEnd       string  `gorm:"column:quiet_end" json:"quietEnd"`
	MaxPerDay      int     `gorm:"column:max_per_day;default:1" json:"maxPerDay"`
	LastSentAt     *string `gorm:"column:last_sent_at" json:"lastSentAt"`
	SentCountToday int     `gorm:"column:sent_count_today;default:0" json:"sentCountToday"`
	PromptTemplate string  `gorm:"column:prompt_template" json:"promptTemplate"`
	RandomMinutes  int     `gorm:"column:random_minutes;default:30" json:"randomMinutes"`
	CreatedAt      string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (ProactiveRule) TableName() string { return "proactive_rules" }

type Reminder struct {
	ID                int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title             string  `gorm:"column:title;not null" json:"title"`
	Content           string  `gorm:"column:content" json:"content"`
	Channel           string  `gorm:"column:channel;default:web" json:"channel"`
	ConversationID    string  `gorm:"column:conversation_id" json:"conversationId"`
	CharacterID       string  `gorm:"column:character_id" json:"characterId"`
	RemindAt          string  `gorm:"column:remind_at;not null" json:"remindAt"`
	RepeatRule        string  `gorm:"column:repeat_rule;default:none" json:"repeatRule"`
	Enabled           int     `gorm:"column:enabled;default:1" json:"enabled"`
	LastTriggeredAt   *string `gorm:"column:last_triggered_at" json:"lastTriggeredAt"`
	CreatedAt         string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         string  `gorm:"column:updated_at" json:"updatedAt"`
	ConversationTitle string  `gorm:"-" json:"conversationTitle"`
	CharacterName     string  `gorm:"-" json:"characterName"`
}

func (Reminder) TableName() string { return "reminders" }

type CreateRuleRequest struct {
	Name           string `json:"name" binding:"required"`
	Enabled        *bool  `json:"enabled"`
	Channel        string `json:"channel"`
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	RuleType       string `json:"ruleType"`
	ScheduleCron   string `json:"scheduleCron"`
	QuietStart     string `json:"quietStart"`
	QuietEnd       string `json:"quietEnd"`
	MaxPerDay      int    `json:"maxPerDay"`
	PromptTemplate string `json:"promptTemplate"`
	RandomMinutes  int    `json:"randomMinutes"`
}

type CreateReminderRequest struct {
	Title          string `json:"title" binding:"required"`
	Content        string `json:"content"`
	Channel        string `json:"channel"`
	ConversationID string `json:"conversationId"`
	CharacterID    string `json:"characterId"`
	RemindAt       string `json:"remindAt" binding:"required"`
	RepeatRule     string `json:"repeatRule"`
}
