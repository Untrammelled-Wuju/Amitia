package calendar

import "time"

type AuthorizationLevel string

const (
	AuthorizationNotDetermined     AuthorizationLevel = "not_determined"
	AuthorizationRestricted        AuthorizationLevel = "restricted"
	AuthorizationDenied            AuthorizationLevel = "denied"
	AuthorizationWriteOnly         AuthorizationLevel = "write_only"
	AuthorizationFullAccess        AuthorizationLevel = "full_access"
	AuthorizationLegacyAuthorized AuthorizationLevel = "legacy_authorized"
)

type EventSpan string

const (
	EventSpanThisEvent    EventSpan = "this_event"
	EventSpanFutureEvents EventSpan = "future_events"
)

type CapabilityState struct {
	Supported bool `json:"supported"`

	Authorization string `json:"authorization"`

	CanCreate bool `json:"canCreate"`
	CanRead   bool `json:"canRead"`
	CanUpdate bool `json:"canUpdate"`
	CanDelete bool `json:"canDelete"`

	CanListCalendars bool `json:"canListCalendars"`

	DefaultCalendarAvailable bool `json:"defaultCalendarAvailable"`

	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type CalendarInfo struct {
	CalendarID string `json:"calendarId"`

	Title string `json:"title"`

	SourceID     string `json:"sourceId,omitempty"`
	SourceTitle  string `json:"sourceTitle,omitempty"`

	Type string `json:"type,omitempty"`

	AllowsModifications bool `json:"allowsModifications"`

	IsDefault bool `json:"isDefault"`

	Color string `json:"color,omitempty"`
}

type AlarmInfo struct {
	Type string `json:"type"`

	RelativeOffsetSeconds *int64 `json:"relativeOffsetSeconds,omitempty"`

	AbsoluteAt *time.Time `json:"absoluteAt,omitempty"`
}

type AlarmSpec struct {
	Type string `json:"type"`

	RelativeOffsetSeconds *int64 `json:"relativeOffsetSeconds,omitempty"`

	AbsoluteAt *time.Time `json:"absoluteAt,omitempty"`
}

type RecurrenceInfo struct {
	Frequency string `json:"frequency"`

	Interval int `json:"interval,omitempty"`

	EndAt *time.Time `json:"endAt,omitempty"`

	OccurrenceCount *int `json:"occurrenceCount,omitempty"`

	DaysOfWeek []string `json:"daysOfWeek,omitempty"`
}

type RecurrenceSpec struct {
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

type EventInfo struct {
	EventID string `json:"eventId"`

	CalendarID string `json:"calendarId"`

	Title string `json:"title"`

	StartAt time.Time `json:"startAt"`
	EndAt   time.Time `json:"endAt"`

	AllDay bool `json:"allDay"`

	TimeZone string `json:"timeZone,omitempty"`

	Location string `json:"location,omitempty"`

	Notes string `json:"notes,omitempty"`

	URL string `json:"url,omitempty"`

	Availability string `json:"availability,omitempty"`
	Status       string `json:"status,omitempty"`

	Recurring bool `json:"recurring"`

	OccurrenceAt *time.Time `json:"occurrenceAt,omitempty"`

	Detached bool `json:"detached,omitempty"`

	Alarms []AlarmInfo `json:"alarms,omitempty"`

	Editable bool `json:"editable"`

	HasAttendees bool `json:"hasAttendees,omitempty"`

	OrganizerName string `json:"organizerName,omitempty"`
}

type QueryEventsRequest struct {
	StartAt time.Time `json:"startAt"`
	EndAt   time.Time `json:"endAt"`

	CalendarIDs []string `json:"calendarIds,omitempty"`

	Limit int `json:"limit,omitempty"`

	Search string `json:"search,omitempty"`

	IncludeNotes    bool `json:"includeNotes,omitempty"`
	IncludeAlarms   bool `json:"includeAlarms,omitempty"`
	IncludeOrganizer bool `json:"includeOrganizer,omitempty"`

	IncludeDeclined bool `json:"includeDeclined,omitempty"`
}

type GetEventRequest struct {
	EventID string `json:"eventId"`
}

type CreateEventRequest struct {
	CalendarID string `json:"calendarId,omitempty"`

	Title string `json:"title"`

	StartAt time.Time `json:"startAt"`
	EndAt   time.Time `json:"endAt"`

	AllDay bool `json:"allDay,omitempty"`

	TimeZone string `json:"timeZone,omitempty"`

	Location string `json:"location,omitempty"`
	Notes    string `json:"notes,omitempty"`
	URL      string `json:"url,omitempty"`

	Availability string `json:"availability,omitempty"`

	Alarms []AlarmSpec `json:"alarms,omitempty"`

	Recurrence *RecurrenceSpec `json:"recurrence,omitempty"`
}

type UpdateEventRequest struct {
	EventID string `json:"eventId"`

	Span EventSpan `json:"span,omitempty"`

	CalendarID *string `json:"calendarId,omitempty"`

	Title *string `json:"title,omitempty"`

	StartAt *time.Time `json:"startAt,omitempty"`
	EndAt   *time.Time `json:"endAt,omitempty"`

	AllDay *bool `json:"allDay,omitempty"`

	TimeZone *string `json:"timeZone,omitempty"`

	Location *string `json:"location,omitempty"`
	Notes    *string `json:"notes,omitempty"`
	URL      *string `json:"url,omitempty"`

	Availability *string `json:"availability,omitempty"`

	Alarms *[]AlarmSpec `json:"alarms,omitempty"`

	Recurrence *RecurrenceUpdate `json:"recurrence,omitempty"`
}

type DeleteEventRequest struct {
	EventID string `json:"eventId"`

	Span EventSpan `json:"span,omitempty"`
}

type AuthorizationRequest struct {
	Access string `json:"access"`
}

type QueryEventsResult struct {
	Events    []EventInfo `json:"events"`
	Count     int         `json:"count"`
	Truncated bool        `json:"truncated"`
}

type CreateEventResult struct {
	EventID string `json:"eventId"`
	EventInfo
}

type UpdateEventResult struct {
	EventID string `json:"eventId"`
	EventInfo
}

type DeleteEventResult struct {
	EventID string `json:"eventId"`
	Deleted bool `json:"deleted"`
}

type CalendarsListResult struct {
	Calendars []CalendarInfo `json:"calendars"`
	Count     int            `json:"count"`
}

type AuthorizationStatusResult struct {
	Level          string `json:"level"`
	EffectiveLevel string `json:"effectiveLevel"`
	CanCreate      bool   `json:"canCreate"`
	CanRead        bool   `json:"canRead"`
	CanUpdate      bool   `json:"canUpdate"`
	CanDelete      bool   `json:"canDelete"`
	CanListCalendars bool `json:"canListCalendars"`
}

type AuthorizationRequestResult struct {
	Level          string `json:"level"`
	EffectiveLevel string `json:"effectiveLevel"`
	Granted        bool   `json:"granted"`
}
