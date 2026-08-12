package reminders

import "time"

type AuthorizationLevel string

const (
	AuthorizationNotDetermined     AuthorizationLevel = "not_determined"
	AuthorizationRestricted        AuthorizationLevel = "restricted"
	AuthorizationDenied            AuthorizationLevel = "denied"
	AuthorizationFullAccess        AuthorizationLevel = "full_access"
	AuthorizationLegacyAuthorized AuthorizationLevel = "legacy_authorized"
)

type CapabilityState struct {
	Supported bool `json:"supported"`

	Authorization string `json:"authorization"`

	CanRead   bool `json:"canRead"`
	CanCreate bool `json:"canCreate"`
	CanUpdate bool `json:"canUpdate"`
	CanDelete bool `json:"canDelete"`

	CanListReminderLists bool `json:"canListReminderLists"`

	DefaultListAvailable bool `json:"defaultListAvailable"`

	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type ReminderListInfo struct {
	ListID string `json:"listId"`

	Title string `json:"title"`

	SourceID     string `json:"sourceId,omitempty"`
	SourceTitle  string `json:"sourceTitle,omitempty"`

	Type string `json:"type,omitempty"`

	AllowsModifications bool `json:"allowsModifications"`

	IsDefault bool `json:"isDefault"`

	Color string `json:"color,omitempty"`
}

type DateComponentsSpec struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`

	Hour   *int `json:"hour,omitempty"`
	Minute *int `json:"minute,omitempty"`
	Second *int `json:"second,omitempty"`

	TimeZone string `json:"timeZone,omitempty"`
}

type ReminderDateSpec struct {
	Date *time.Time `json:"date,omitempty"`

	HasTime bool `json:"hasTime"`

	TimeZone string `json:"timeZone,omitempty"`
}

type ReminderAlarmSpec struct {
	Type string `json:"type"`

	RelativeOffsetSeconds *int64 `json:"relativeOffsetSeconds,omitempty"`

	AbsoluteAt *time.Time `json:"absoluteAt,omitempty"`
}

type ReminderAlarmInfo struct {
	Type string `json:"type"`

	RelativeOffsetSeconds *int64 `json:"relativeOffsetSeconds,omitempty"`

	AbsoluteAt *time.Time `json:"absoluteAt,omitempty"`
}

type RecurrenceSpec struct {
	Frequency string `json:"frequency"`

	Interval int `json:"interval,omitempty"`

	EndAt *time.Time `json:"endAt,omitempty"`

	OccurrenceCount *int `json:"occurrenceCount,omitempty"`

	DaysOfWeek []string `json:"daysOfWeek,omitempty"`
}

type RecurrenceInfo struct {
	Frequency string `json:"frequency"`

	Interval int `json:"interval,omitempty"`

	EndAt *time.Time `json:"endAt,omitempty"`

	OccurrenceCount *int `json:"occurrenceCount,omitempty"`

	DaysOfWeek []string `json:"daysOfWeek,omitempty"`
}

type RecurrenceUpdate struct {
	Frequency *string `json:"frequency,omitempty"`

	Interval *int `json:"interval,omitempty"`

	EndAt *time.Time `json:"endAt,omitempty"`

	OccurrenceCount *int `json:"occurrenceCount,omitempty"`

	DaysOfWeek []string `json:"daysOfWeek,omitempty"`
}

type Priority string

const (
	PriorityNone   Priority = "none"
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

type ReminderInfo struct {
	ReminderID string `json:"reminderId"`

	ListID string `json:"listId"`

	Title string `json:"title"`

	Notes string `json:"notes,omitempty"`

	URL string `json:"url,omitempty"`

	Priority string `json:"priority,omitempty"`

	StartAt *time.Time `json:"startAt,omitempty"`
	DueAt   *time.Time `json:"dueAt,omitempty"`

	StartHasTime bool `json:"startHasTime,omitempty"`
	DueHasTime   bool `json:"dueHasTime,omitempty"`

	TimeZone string `json:"timeZone,omitempty"`

	Completed bool `json:"completed"`

	CompletionAt *time.Time `json:"completionAt,omitempty"`

	Alarms []ReminderAlarmInfo `json:"alarms,omitempty"`

	Recurring bool `json:"recurring"`

	Editable bool `json:"editable"`
}

type PatchField[T any] struct {
	Set   bool `json:"set"`
	Value *T   `json:"value,omitempty"`
}

type QueryRemindersRequest struct {
	ListIDs []string `json:"listIds,omitempty"`

	Status string `json:"status,omitempty"`

	DueStart *time.Time `json:"dueStart,omitempty"`
	DueEnd   *time.Time `json:"dueEnd,omitempty"`

	CompletionStart *time.Time `json:"completionStart,omitempty"`
	CompletionEnd   *time.Time `json:"completionEnd,omitempty"`

	Search string `json:"search,omitempty"`

	Limit int `json:"limit,omitempty"`

	IncludeNotes  bool `json:"includeNotes,omitempty"`
	IncludeAlarms bool `json:"includeAlarms,omitempty"`
	IncludeURL    bool `json:"includeUrl,omitempty"`

	IncludeCompleted bool `json:"includeCompleted,omitempty"`
}

type GetReminderRequest struct {
	ReminderID string `json:"reminderId"`

	IncludeNotes  bool `json:"includeNotes,omitempty"`
	IncludeAlarms bool `json:"includeAlarms,omitempty"`
}

type CreateReminderRequest struct {
	ListID string `json:"listId,omitempty"`

	Title string `json:"title"`

	Notes string `json:"notes,omitempty"`

	URL string `json:"url,omitempty"`

	Priority string `json:"priority,omitempty"`

	Start *DateComponentsSpec `json:"start,omitempty"`
	Due   *DateComponentsSpec `json:"due,omitempty"`

	Alarms []ReminderAlarmSpec `json:"alarms,omitempty"`

	Recurrence *RecurrenceSpec `json:"recurrence,omitempty"`
}

type UpdateReminderRequest struct {
	ReminderID string `json:"reminderId"`

	ListID *string `json:"listId,omitempty"`

	Title *string `json:"title,omitempty"`

	Notes *string `json:"notes,omitempty"`

	URL *string `json:"url,omitempty"`

	Priority *string `json:"priority,omitempty"`

	Start *DateComponentsSpec `json:"start,omitempty"`
	Due   *DateComponentsSpec `json:"due,omitempty"`

	Alarms *[]ReminderAlarmSpec `json:"alarms,omitempty"`

	Recurrence *RecurrenceUpdate `json:"recurrence,omitempty"`
}

type CompleteReminderRequest struct {
	ReminderID string `json:"reminderId"`

	CompletionAt *time.Time `json:"completionAt,omitempty"`
}

type UncompleteReminderRequest struct {
	ReminderID string `json:"reminderId"`
}

type DeleteReminderRequest struct {
	ReminderID string `json:"reminderId"`
}

type QueryRemindersResult struct {
	Reminders []ReminderInfo `json:"reminders"`
	Count     int            `json:"count"`
	Truncated bool           `json:"truncated"`
}

type CreateReminderResult struct {
	ReminderID string `json:"reminderId"`
	ReminderInfo
}

type UpdateReminderResult struct {
	ReminderID string `json:"reminderId"`
	ReminderInfo
}

type CompleteReminderResult struct {
	ReminderID  string `json:"reminderId"`
	Completed   bool   `json:"completed"`
	CompletionAt *time.Time `json:"completionAt,omitempty"`
}

type UncompleteReminderResult struct {
	ReminderID string `json:"reminderId"`
	Completed  bool   `json:"completed"`
}

type DeleteReminderResult struct {
	ReminderID string `json:"reminderId"`
	Deleted    bool   `json:"deleted"`
}

type ListsListResult struct {
	Lists []ReminderListInfo `json:"lists"`
	Count int                `json:"count"`
}

type AuthorizationStatusResult struct {
	Level            string `json:"level"`
	EffectiveLevel   string `json:"effectiveLevel"`
	CanRead          bool   `json:"canRead"`
	CanCreate        bool   `json:"canCreate"`
	CanUpdate        bool   `json:"canUpdate"`
	CanDelete        bool   `json:"canDelete"`
	CanListReminderLists bool `json:"canListReminderLists"`
}

type AuthorizationRequestResult struct {
	Level    string `json:"level"`
	Granted  bool   `json:"granted"`
}
