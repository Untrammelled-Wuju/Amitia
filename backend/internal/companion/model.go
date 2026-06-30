// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import "time"

type SleepSetting struct {
	ID                int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CharacterID       string `gorm:"column:character_id;index" json:"characterId"`
	BedTime           string `gorm:"column:bed_time;default:23:00" json:"bedTime"`
	WakeTime          string `gorm:"column:wake_time;default:07:00" json:"wakeTime"`
	Enabled           int    `gorm:"column:enabled;default:1" json:"enabled"`
	SleepReplyEnabled int    `gorm:"column:sleep_reply_enabled;default:0" json:"sleepReplyEnabled"`
	SleepReplyMode    string `gorm:"column:sleep_reply_mode;default:NO_REPLY" json:"sleepReplyMode"`
	CreatedAt         string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         string `gorm:"column:updated_at" json:"updatedAt"`
}

func (SleepSetting) TableName() string { return "sleep_settings" }

type FixedEvent struct {
	ID                int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CharacterID       string `gorm:"column:character_id;index" json:"characterId"`
	Title             string `gorm:"column:title;not null" json:"title"`
	Description       string `gorm:"column:description" json:"description"`
	WeekDay           int    `gorm:"column:week_day;default:-1" json:"weekDay"`
	StartTime         string `gorm:"column:start_time" json:"startTime"`
	EndTime           string `gorm:"column:end_time" json:"endTime"`
	EventType         string `gorm:"column:event_type;default:CUSTOM_BUSY" json:"eventType"`
	RepeatType        string `gorm:"column:repeat_type;default:weekly" json:"repeatType"`
	RepeatDays        string `gorm:"column:repeat_days" json:"repeatDays"`
	PrepareMinMinutes int    `gorm:"column:prepare_min_minutes;default:10" json:"prepareMinMinutes"`
	PrepareMaxMinutes int    `gorm:"column:prepare_max_minutes;default:40" json:"prepareMaxMinutes"`
	ReplyMode         string `gorm:"column:reply_mode;default:SHORT_REPLY" json:"replyMode"`
	Enabled           int    `gorm:"column:enabled;default:1" json:"enabled"`
	CreatedAt         string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         string `gorm:"column:updated_at" json:"updatedAt"`
}

func (FixedEvent) TableName() string { return "fixed_events" }

type SpecialEvent struct {
	ID                   int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CharacterID          string `gorm:"column:character_id;index" json:"characterId"`
	Title                string `gorm:"column:title;not null" json:"title"`
	Description          string `gorm:"column:description" json:"description"`
	StartDate            string `gorm:"column:start_date" json:"startDate"`
	EndDate              string `gorm:"column:end_date" json:"endDate"`
	StartTime            string `gorm:"column:start_time" json:"startTime"`
	EndTime              string `gorm:"column:end_time" json:"endTime"`
	EventType            string `gorm:"column:event_type;default:CUSTOM" json:"eventType"`
	RepeatType           string `gorm:"column:repeat_type;default:none" json:"repeatType"`
	RepeatDays           string `gorm:"column:repeat_days" json:"repeatDays"`
	Enabled              int    `gorm:"column:enabled;default:1" json:"enabled"`
	Priority             int    `gorm:"column:priority;default:0" json:"priority"`
	ActiveMessageAllowed int    `gorm:"column:active_message_allowed;default:1" json:"activeMessageAllowed"`
	ReplyMode            string `gorm:"column:reply_mode;default:SHORT_REPLY" json:"replyMode"`
	AffectSchedule       int    `gorm:"column:affect_schedule;default:0" json:"affectSchedule"`
	AffectSleep          int    `gorm:"column:affect_sleep;default:0" json:"affectSleep"`
	AffectMeal           int    `gorm:"column:affect_meal;default:0" json:"affectMeal"`
	AffectEnergy         int    `gorm:"column:affect_energy;default:0" json:"affectEnergy"`
	Payload              string `gorm:"column:payload" json:"payload"`
	CreatedAt            string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at" json:"updatedAt"`
}

func (SpecialEvent) TableName() string { return "special_events" }

type ClassAdjustment struct {
	ID          int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CharacterID string `gorm:"column:character_id;index" json:"characterId"`
	Date        string `gorm:"column:date" json:"date"`
	SlotIndex   int    `gorm:"column:slot_index" json:"slotIndex"`
	ClassName   string `gorm:"column:class_name" json:"className"`
	AdjustType  string `gorm:"column:adjust_type;default:swap" json:"adjustType"`
	Description string `gorm:"column:description" json:"description"`
	CreatedAt   string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   string `gorm:"column:updated_at" json:"updatedAt"`
}

func (ClassAdjustment) TableName() string { return "class_adjustments" }

type LifestyleTendency struct {
	ID                     int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CharacterID            string `gorm:"column:character_id;uniqueIndex" json:"characterId"`
	PunctualityTendency    int    `gorm:"column:punctuality_tendency;default:50" json:"punctualityTendency"`
	EarlyPrepareTendency   int    `gorm:"column:early_prepare_tendency;default:50" json:"earlyPrepareTendency"`
	SelfDisciplineTendency int    `gorm:"column:self_discipline_tendency;default:50" json:"selfDisciplineTendency"`
	SleepinessTendency     int    `gorm:"column:sleepiness_tendency;default:50" json:"sleepinessTendency"`
	RandomnessTendency     int    `gorm:"column:randomness_tendency;default:50" json:"randomnessTendency"`
	ActivityEnergy         int    `gorm:"column:activity_energy;default:50" json:"activityEnergy"`
	SocialEnergy           int    `gorm:"column:social_energy;default:50" json:"socialEnergy"`
	CareTendency           int    `gorm:"column:care_tendency;default:50" json:"careTendency"`
	DailyShareTendency     int    `gorm:"column:daily_share_tendency;default:50" json:"dailyShareTendency"`
	ManuallyConfigured     int    `gorm:"column:manually_configured;default:0" json:"manuallyConfigured"`
	UpdatedAt              string `gorm:"column:updated_at" json:"updatedAt"`
}

func (LifestyleTendency) TableName() string { return "lifestyle_tendencies" }

type WorkProfile struct {
	ID                          int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CharacterID                 string `gorm:"column:character_id;uniqueIndex" json:"characterId"`
	Enabled                     int    `gorm:"column:enabled;default:0" json:"enabled"`
	WorkDays                    string `gorm:"column:work_days;default:MON,TUE,WED,THU,FRI" json:"workDays"`
	WorkStartTime               string `gorm:"column:work_start_time;default:09:00" json:"workStartTime"`
	WorkEndTime                 string `gorm:"column:work_end_time;default:18:00" json:"workEndTime"`
	LunchBreakStartTime         string `gorm:"column:lunch_break_start_time;default:12:00" json:"lunchBreakStartTime"`
	LunchBreakEndTime           string `gorm:"column:lunch_break_end_time;default:13:30" json:"lunchBreakEndTime"`
	CommuteMinMinutes           int    `gorm:"column:commute_min_minutes;default:15" json:"commuteMinMinutes"`
	CommuteMaxMinutes           int    `gorm:"column:commute_max_minutes;default:45" json:"commuteMaxMinutes"`
	PrepareMinMinutes           int    `gorm:"column:prepare_min_minutes;default:20" json:"prepareMinMinutes"`
	PrepareMaxMinutes           int    `gorm:"column:prepare_max_minutes;default:60" json:"prepareMaxMinutes"`
	ReplyMode                   string `gorm:"column:reply_mode;default:SHORT_REPLY" json:"replyMode"`
	AllowOvertime               int    `gorm:"column:allow_overtime;default:0" json:"allowOvertime"`
	OvertimeProbability         int    `gorm:"column:overtime_probability;default:10" json:"overtimeProbability"`
	OvertimeMinMinutes          int    `gorm:"column:overtime_min_minutes;default:30" json:"overtimeMinMinutes"`
	OvertimeMaxMinutes          int    `gorm:"column:overtime_max_minutes;default:180" json:"overtimeMaxMinutes"`
	OvertimeReplyMode           string `gorm:"column:overtime_reply_mode;default:SHORT_REPLY" json:"overtimeReplyMode"`
	DelayedReplyEnabled         int    `gorm:"column:delayed_reply_enabled;default:0" json:"delayedReplyEnabled"`
	CommuteHomeShareEnabled     int    `gorm:"column:commute_home_share_enabled;default:1" json:"commuteHomeShareEnabled"`
	CommuteHomeShareProbability int    `gorm:"column:commute_home_share_probability;default:60" json:"commuteHomeShareProbability"`
	UpdatedAt                   string `gorm:"column:updated_at" json:"updatedAt"`
}

func (WorkProfile) TableName() string { return "work_profiles" }

type ActiveMessageSetting struct {
	CharacterID   string `gorm:"column:character_id;uniqueIndex" json:"characterId"`
	Enabled       int    `gorm:"column:enabled;default:1" json:"enabled"`
	MinInterval   int    `gorm:"column:min_interval;default:60" json:"minInterval"`
	QuietStart    string `gorm:"column:quiet_start;default:23:00" json:"quietStart"`
	QuietEnd      string `gorm:"column:quiet_end;default:07:00" json:"quietEnd"`
	MaxPerDay     int    `gorm:"column:max_per_day;default:6" json:"maxPerDay"`
	MaxDailyCalls int    `gorm:"column:max_daily_calls;default:10" json:"maxDailyCalls"`
	Channel       string `gorm:"column:channel;default:all" json:"channel"`
}

func (ActiveMessageSetting) TableName() string { return "active_message_settings" }

type ScheduleSlot struct {
	DayOfWeek int    `json:"dayOfWeek"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Name      string `json:"name"`
	Type      string `json:"type"`
}

type TodaySchedule struct {
	WakeTime     time.Time  `json:"wakeTime"`
	LunchTime    time.Time  `json:"lunchTime"`
	DinnerTime   time.Time  `json:"dinnerTime"`
	HasNap       bool       `json:"hasNap"`
	NapStartTime *time.Time `json:"napStartTime,omitempty"`
	NapEndTime   *time.Time `json:"napEndTime,omitempty"`
	SleepTime    time.Time  `json:"sleepTime"`
	IsRestDay    bool       `json:"isRestDay"`
}

type TimelineEntry struct {
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
	State      string    `json:"state"`
	SourceType string    `json:"sourceType"`
	Priority   int       `json:"priority"`
	Reason     string    `json:"reason"`
}

type ShareTask struct {
	Type    string    `json:"type"`
	DueTime time.Time `json:"dueTime"`
	Prompt  string    `json:"prompt"`
	Reason  string    `json:"reason"`
}

type ScheduleGenerateResult struct {
	Generated         bool        `json:"generated"`
	Tasks             []ShareTask `json:"tasks"`
	TaskCount         int         `json:"taskCount"`
	EstimatedLLMCalls int         `json:"estimatedLLMCalls"`
}

type ShareHistory struct {
	RecentTopics []string `json:"recentTopics"`
	LastShareAt  string   `json:"lastShareAt"`
}
