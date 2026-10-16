package reminder

import (
	"encoding/json"
	"time"
)

type Caller struct {
	RoleScope         string `json:"-"`
	ConversationScope string `json:"-"`
	SpaceID           string `json:"spaceId"`
	DeviceID          string `json:"deviceId"`
	PluginID          string `json:"pluginId,omitempty"`
	ModuleID          string `json:"moduleId,omitempty"`
}

type Recurrence struct {
	Type            string `json:"type"`
	Weekdays        []int  `json:"weekdays,omitempty"`
	IntervalSeconds int64  `json:"intervalSeconds,omitempty"`
	Cron            string `json:"cron,omitempty"`
}

type ProactivePolicy struct {
	RandomMinutes               int     `json:"randomMinutes"`
	RuleQuietStart              string  `json:"ruleQuietStart"`
	RuleQuietEnd                string  `json:"ruleQuietEnd"`
	Enabled                     bool    `json:"enabled"`
	ActiveLevel                 int     `json:"activeLevel"`
	QuietStart                  string  `json:"quietStart"`
	QuietEnd                    string  `json:"quietEnd"`
	MinInterval                 int     `json:"minInterval"`
	MaxPerDay                   int     `json:"maxPerDay"`
	MaxDailyCalls               int     `json:"maxDailyCalls"`
	UnrepliedSlowdownEnabled    bool    `json:"unrepliedSlowdownEnabled"`
	UnrepliedSlowdownAfter      int     `json:"unrepliedSlowdownAfter"`
	UnrepliedCooldownMultiplier float64 `json:"unrepliedCooldownMultiplier"`
	UnrepliedRecoveryOnReply    bool    `json:"unrepliedRecoveryOnReply"`
	RuleMaxPerDay               int     `json:"ruleMaxPerDay"`
	RuleType                    string  `json:"ruleType"`
}

type Reminder struct {
	ArchiveHistory     []TriggerHistory `json:"archiveHistory,omitempty"`
	LocalAnchorAt      string           `json:"localAnchorAt"`
	LocalRemindAt      string           `json:"localRemindAt"`
	ID                 string           `json:"id"`
	Caller             Caller           `json:"caller"`
	CreatedBy          Caller           `json:"createdBy"`
	OwnerID            string           `json:"ownerId"`
	ExecutionMode      string           `json:"executionMode"`
	Kind               string           `json:"kind"`
	Source             string           `json:"source"`
	Title              string           `json:"title"`
	Content            string           `json:"content"`
	Channel            string           `json:"channel"`
	Channels           []string         `json:"channels"`
	ChannelTargets     []ChannelTarget  `json:"channelTargets"`
	SystemNotification bool             `json:"systemNotification"`
	ConversationID     string           `json:"conversationId"`
	CharacterID        string           `json:"characterId"`
	TargetDeviceID     string           `json:"targetDeviceId,omitempty"`
	RemindAt           string           `json:"remindAt"`
	AnchorAt           string           `json:"anchorAt"`
	RepeatRule         string           `json:"repeatRule"`
	Recurrence         Recurrence       `json:"recurrence"`
	Timezone           string           `json:"timezone"`
	EndAt              string           `json:"endAt,omitempty"`
	MaxOccurrences     int              `json:"maxOccurrences,omitempty"`
	Enabled            int              `json:"enabled"`
	Paused             bool             `json:"paused"`
	State              string           `json:"state"`
	Generation         int64            `json:"generation"`
	RequestID          string           `json:"requestId"`
	Binding            json.RawMessage  `json:"binding"`
	Policy             *ProactivePolicy `json:"policy,omitempty"`
	CreatedAt          string           `json:"createdAt"`
	UpdatedAt          string           `json:"updatedAt"`
	LastTriggeredAt    *string          `json:"lastTriggeredAt,omitempty"`
	ConversationTitle  string           `json:"conversationTitle,omitempty"`
	CharacterName      string           `json:"characterName,omitempty"`
	Saved              bool             `json:"saved"`
}

type CreateReminderRequest struct {
	Title              string           `json:"title"`
	Content            string           `json:"content"`
	Channel            string           `json:"channel"`
	Channels           []string         `json:"channels"`
	ChannelTargets     []ChannelTarget  `json:"channelTargets"`
	SystemNotification *bool            `json:"systemNotification"`
	ConversationID     string           `json:"conversationId"`
	CharacterID        string           `json:"characterId"`
	TargetDeviceID     string           `json:"targetDeviceId"`
	RemindAt           string           `json:"remindAt"`
	RepeatRule         string           `json:"repeatRule"`
	Recurrence         Recurrence       `json:"recurrence"`
	Timezone           string           `json:"timezone"`
	EndAt              string           `json:"endAt"`
	MaxOccurrences     int              `json:"maxOccurrences"`
	Enabled            *bool            `json:"enabled"`
	Kind               string           `json:"kind"`
	Policy             *ProactivePolicy `json:"policy"`
	RequestID          string           `json:"requestId"`
}

type ChannelTarget struct {
	ChannelID string `json:"channelId"`
	PeerID    string `json:"peerId"`
}

type TriggerHistory struct {
	Archived     bool            `json:"archived,omitempty"`
	ID           string          `json:"id"`
	TriggerID    string          `json:"triggerId"`
	ReminderID   string          `json:"reminderId"`
	TriggerType  string          `json:"triggerType"`
	Title        string          `json:"title"`
	Channel      string          `json:"channel"`
	State        string          `json:"state"`
	Priority     string          `json:"priority"`
	Reason       string          `json:"reason"`
	AttemptCount int             `json:"attemptCount"`
	LastError    string          `json:"lastError"`
	ScheduledAt  time.Time       `json:"scheduledAt"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	Result       json.RawMessage `json:"result,omitempty"`
}

type DeliveryResult struct {
	MessageID          string         `json:"messageId"`
	ConversationID     string         `json:"conversationId"`
	MessageSaved       bool           `json:"messageSaved"`
	GenerationDegraded bool           `json:"generationDegraded,omitempty"`
	Targets            []TargetResult `json:"targets"`
}

type TargetResult struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}
