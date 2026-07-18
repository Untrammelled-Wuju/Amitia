package temporal

import (
	"context"
	"time"
)

const (
	OwnerUser            = "user"
	OwnerCharacter       = "character"
	TimezoneFollowDevice = "follow_device"
	TimezoneFixed        = "fixed"
	TimezoneFollowUser   = "follow_user"
	TimezoneNarrative    = "narrative"
	DefaultTimezone      = "Asia/Shanghai"
	SnapshotVersion      = "temporal-snapshot-v1"
	DefaultUserOwnerID   = "default"
)

type Profile struct {
	ID                     string    `gorm:"column:id;primaryKey" json:"id"`
	OwnerType              string    `gorm:"column:owner_type;uniqueIndex:idx_temporal_profile_owner" json:"ownerType"`
	OwnerID                string    `gorm:"column:owner_id;uniqueIndex:idx_temporal_profile_owner" json:"ownerId"`
	TimezoneMode           string    `gorm:"column:timezone_mode" json:"timezoneMode"`
	Timezone               string    `gorm:"column:timezone" json:"timezone"`
	Locale                 string    `gorm:"column:locale" json:"locale"`
	CalendarSystem         string    `gorm:"column:calendar_system" json:"calendarSystem"`
	WeekStart              int       `gorm:"column:week_start" json:"weekStart"`
	HolidayRegion          string    `gorm:"column:holiday_region" json:"holidayRegion"`
	Hemisphere             string    `gorm:"column:hemisphere" json:"hemisphere"`
	DaypartConfigJSON      string    `gorm:"column:daypart_config_json" json:"daypartConfigJson"`
	QuietHoursJSON         string    `gorm:"column:quiet_hours_json" json:"quietHoursJson"`
	AutoDetectTimezone     bool      `gorm:"column:auto_detect_timezone" json:"autoDetectTimezone"`
	TravelMode             bool      `gorm:"column:travel_mode" json:"travelMode"`
	AwarenessLevel         int       `gorm:"column:awareness_level" json:"awarenessLevel"`
	Source                 string    `gorm:"column:source" json:"source"`
	Confidence             int       `gorm:"column:confidence" json:"confidence"`
	PendingTimezone        string    `gorm:"column:pending_timezone" json:"pendingTimezoneSuggestion,omitempty"`
	Enabled                bool      `gorm:"column:enabled" json:"enabled"`
	HolidayAwareness       bool      `gorm:"column:holiday_awareness" json:"holidayAwareness"`
	DaypartAwareness       bool      `gorm:"column:daypart_awareness" json:"daypartAwareness"`
	AnniversaryAwareness   bool      `gorm:"column:anniversary_awareness" json:"anniversaryAwareness"`
	MemoryResonance        bool      `gorm:"column:memory_resonance" json:"memoryResonance"`
	AllowSharedDateMention bool      `gorm:"column:allow_shared_date_mention" json:"allowSharedDateMention"`
	Version                int       `gorm:"column:version" json:"version"`
	CreatedAtUTC           time.Time `gorm:"column:created_at_utc" json:"createdAtUtc"`
	UpdatedAtUTC           time.Time `gorm:"column:updated_at_utc" json:"updatedAtUtc"`
}

func (Profile) TableName() string { return "temporal_profiles" }

type ProfilePatch struct {
	TimezoneMode           *string `json:"timezoneMode"`
	Timezone               *string `json:"timezone"`
	Locale                 *string `json:"locale"`
	CalendarSystem         *string `json:"calendarSystem"`
	WeekStart              *int    `json:"weekStart"`
	HolidayRegion          *string `json:"holidayRegion"`
	Hemisphere             *string `json:"hemisphere"`
	DaypartConfigJSON      *string `json:"daypartConfigJson"`
	QuietHoursJSON         *string `json:"quietHoursJson"`
	AutoDetectTimezone     *bool   `json:"autoDetectTimezone"`
	TravelMode             *bool   `json:"travelMode"`
	AwarenessLevel         *int    `json:"awarenessLevel"`
	Source                 *string `json:"source"`
	Confidence             *int    `json:"confidence"`
	Enabled                *bool   `json:"enabled"`
	HolidayAwareness       *bool   `json:"holidayAwareness"`
	DaypartAwareness       *bool   `json:"daypartAwareness"`
	AnniversaryAwareness   *bool   `json:"anniversaryAwareness"`
	MemoryResonance        *bool   `json:"memoryResonance"`
	AllowSharedDateMention *bool   `json:"allowSharedDateMention"`
}

type Anchor struct {
	ID                    string     `gorm:"column:id;primaryKey" json:"id"`
	ScopeType             string     `gorm:"column:scope_type;index" json:"scopeType"`
	UserID                string     `gorm:"column:user_id;index" json:"userId"`
	CharacterID           string     `gorm:"column:character_id;index" json:"characterId"`
	AnchorType            string     `gorm:"column:anchor_type;index" json:"anchorType"`
	Title                 string     `gorm:"column:title" json:"title"`
	Description           string     `gorm:"column:description" json:"description"`
	TimeKind              string     `gorm:"column:time_kind" json:"timeKind"`
	InstantAtUTC          *time.Time `gorm:"column:instant_at_utc" json:"instantAtUtc,omitempty"`
	EndAtUTC              *time.Time `gorm:"column:end_at_utc" json:"endAtUtc,omitempty"`
	LocalDate             string     `gorm:"column:local_date" json:"localDate,omitempty"`
	LocalTime             string     `gorm:"column:local_time" json:"localTime,omitempty"`
	Timezone              string     `gorm:"column:timezone" json:"timezone,omitempty"`
	RRule                 string     `gorm:"column:rrule" json:"rrule,omitempty"`
	DurationSeconds       int64      `gorm:"column:duration_seconds" json:"durationSeconds"`
	PreWindowSeconds      int64      `gorm:"column:pre_window_seconds" json:"preWindowSeconds"`
	PostWindowSeconds     int64      `gorm:"column:post_window_seconds" json:"postWindowSeconds"`
	Importance            int        `gorm:"column:importance" json:"importance"`
	Confidence            int        `gorm:"column:confidence" json:"confidence"`
	SensitivityLevel      string     `gorm:"column:sensitivity_level" json:"sensitivityLevel"`
	AllowPromptMention    bool       `gorm:"column:allow_prompt_mention" json:"allowPromptMention"`
	AllowProactiveMention bool       `gorm:"column:allow_proactive_mention" json:"allowProactiveMention"`
	RequiresConfirmation  bool       `gorm:"column:requires_confirmation" json:"requiresConfirmation"`
	Source                string     `gorm:"column:source" json:"source"`
	SourceRef             string     `gorm:"column:source_ref" json:"sourceRef,omitempty"`
	PayloadJSON           string     `gorm:"column:payload_json" json:"payloadJson,omitempty"`
	Status                string     `gorm:"column:status;index" json:"status"`
	NextOccurrenceAtUTC   *time.Time `gorm:"column:next_occurrence_at_utc;index" json:"nextOccurrenceAtUtc,omitempty"`
	LastOccurrenceAtUTC   *time.Time `gorm:"column:last_occurrence_at_utc" json:"lastOccurrenceAtUtc,omitempty"`
	CreatedAtUTC          time.Time  `gorm:"column:created_at_utc" json:"createdAtUtc"`
	UpdatedAtUTC          time.Time  `gorm:"column:updated_at_utc" json:"updatedAtUtc"`
}

func (Anchor) TableName() string { return "temporal_anchors" }

type Event struct {
	ID                 string    `gorm:"column:id;primaryKey" json:"id"`
	EventType          string    `gorm:"column:event_type;index" json:"eventType"`
	UserID             string    `gorm:"column:user_id;index" json:"userId"`
	CharacterID        string    `gorm:"column:character_id;index" json:"characterId"`
	AnchorID           string    `gorm:"column:anchor_id;index" json:"anchorId,omitempty"`
	OccurredAtUTC      time.Time `gorm:"column:occurred_at_utc;index" json:"occurredAtUtc"`
	EffectiveLocalDate string    `gorm:"column:effective_local_date" json:"effectiveLocalDate"`
	Timezone           string    `gorm:"column:timezone" json:"timezone"`
	Salience           float64   `gorm:"column:salience" json:"salience"`
	Source             string    `gorm:"column:source" json:"source"`
	SourceEventID      string    `gorm:"column:source_event_id" json:"sourceEventId,omitempty"`
	IdempotencyKey     string    `gorm:"column:idempotency_key;uniqueIndex" json:"idempotencyKey"`
	PayloadJSON        string    `gorm:"column:payload_json" json:"payloadJson,omitempty"`
	CreatedAtUTC       time.Time `gorm:"column:created_at_utc" json:"createdAtUtc"`
}

func (Event) TableName() string { return "temporal_events" }

type MemoryTemporalMetadata struct {
	MemoryID          string     `gorm:"column:memory_id;primaryKey" json:"memoryId"`
	OccurredAtUTC     *time.Time `gorm:"column:occurred_at_utc;index" json:"occurredAtUtc,omitempty"`
	EndedAtUTC        *time.Time `gorm:"column:ended_at_utc" json:"endedAtUtc,omitempty"`
	Timezone          string     `gorm:"column:timezone" json:"timezone,omitempty"`
	LocalDate         string     `gorm:"column:local_date" json:"localDate,omitempty"`
	Daypart           string     `gorm:"column:daypart" json:"daypart,omitempty"`
	TemporalPrecision string     `gorm:"column:temporal_precision" json:"temporalPrecision"`
	ValidFromUTC      *time.Time `gorm:"column:valid_from_utc" json:"validFromUtc,omitempty"`
	ValidToUTC        *time.Time `gorm:"column:valid_to_utc" json:"validToUtc,omitempty"`
	AnchorIDsJSON     string     `gorm:"column:anchor_ids_json" json:"anchorIdsJson,omitempty"`
	SourceTimeText    string     `gorm:"column:source_time_text" json:"sourceTimeText,omitempty"`
	CreatedAtUTC      time.Time  `gorm:"column:created_at_utc" json:"createdAtUtc"`
	UpdatedAtUTC      time.Time  `gorm:"column:updated_at_utc" json:"updatedAtUtc"`
}

func (MemoryTemporalMetadata) TableName() string { return "memory_temporal_metadata" }

type CivilTimeSnapshot struct {
	Timezone      string    `json:"timezone"`
	LocalTime     time.Time `json:"localTime"`
	Weekday       string    `json:"weekday"`
	Daypart       string    `json:"daypart"`
	Season        string    `json:"season"`
	OffsetSeconds int       `json:"offsetSeconds"`
	IsWeekend     bool      `json:"isWeekend"`
	IsWorkday     bool      `json:"isWorkday"`
}

type ScheduleTemporalSnapshot struct {
	CurrentState string `json:"currentState,omitempty"`
	Busy         bool   `json:"busy"`
}

type AnchorOccurrence struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	DistanceDays int     `json:"distanceDays"`
	Salience     float64 `json:"salience"`
}

type TemporalSignals struct {
	TimezoneDiffers        bool   `json:"timezoneDiffers"`
	QuietHours             bool   `json:"quietHours"`
	DayChanged             bool   `json:"dayChanged"`
	UserTimezoneSource     string `json:"userTimezoneSource"`
	UserTimezoneConfidence int    `json:"userTimezoneConfidence"`
	UserTimezoneConfirmed  bool   `json:"userTimezoneConfirmed"`
}

type TemporalBehaviorPolicy struct {
	MentionTime         string `json:"mentionTime"`
	AllowProactive      bool   `json:"allowProactive"`
	MaxTemporalMentions int    `json:"maxTemporalMentions"`
}

type Snapshot struct {
	Version          string                   `json:"version"`
	NowUTC           time.Time                `json:"nowUtc"`
	UserTime         CivilTimeSnapshot        `json:"userTime"`
	CharacterTime    CivilTimeSnapshot        `json:"characterTime"`
	RelationshipTime *RelationshipTimeContext `json:"relationshipTime,omitempty"`
	Schedule         ScheduleTemporalSnapshot `json:"schedule"`
	CalendarEvents   []CalendarEvent          `json:"calendarEvents,omitempty"`
	SalientAnchors   []AnchorOccurrence       `json:"salientAnchors"`
	Signals          TemporalSignals          `json:"signals"`
	Policy           TemporalBehaviorPolicy   `json:"policy"`
	GeneratedAt      time.Time                `json:"generatedAt"`
}

type SnapshotInput struct {
	UserID         string
	CharacterID    string
	Channel        string
	DeviceTimezone string
}

type NarrativeClockInput struct {
	UserID      string `json:"userId"`
	CharacterID string `json:"characterId"`
}

type NarrativeTime struct {
	Enabled bool      `json:"enabled"`
	Now     time.Time `json:"now,omitempty"`
}

type NarrativeClockProvider interface {
	Resolve(ctx context.Context, input NarrativeClockInput) (NarrativeTime, error)
}

type AnchorQuery struct {
	UserID      string
	CharacterID string
	Status      string
	Limit       int
}
