package alarms

type AuthorizationStatus string

const (
	AuthNotDetermined AuthorizationStatus = "not_determined"
	AuthAuthorized    AuthorizationStatus = "authorized"
	AuthDenied        AuthorizationStatus = "denied"
)

type AlarmState string

const (
	StateScheduled AlarmState = "scheduled"
	StateCountdown AlarmState = "countdown"
	StatePaused    AlarmState = "paused"
	StateAlerting  AlarmState = "alerting"
)

type Recurrence string

const (
	RecurrenceNever  Recurrence = "never"
	RecurrenceWeekly Recurrence = "weekly"
)

type AlarmStatus struct {
	Supported              bool               `json:"supported"`
	Authorization          AuthorizationStatus `json:"authorization"`
	Count                  int                `json:"count"`
	LimitReached           bool               `json:"limitReached"`
	LiveActivitySupported  bool               `json:"liveActivitySupported"`
	SilentModeOverride     bool               `json:"silentModeOverride"`
	FocusModeOverride      bool               `json:"focusModeOverride"`
	CanAdd                 bool               `json:"canAdd"`
	CanRead                bool               `json:"canRead"`
	CanUpdate              bool               `json:"canUpdate"`
	CanRemove              bool               `json:"canRemove"`
	Generation             uint64             `json:"generation"`
}

type IOSAlarmSchedule struct {
	FireAt    *string `json:"fireAt,omitempty"`
	Hour      *int    `json:"hour,omitempty"`
	Minute    *int    `json:"minute,omitempty"`
	Recurrence string `json:"recurrence,omitempty"`
	Weekdays  []string `json:"weekdays,omitempty"`
}

type IOSAlarmCountdown struct {
	PreAlertSeconds  *int64 `json:"preAlertSeconds,omitempty"`
	PostAlertSeconds *int64 `json:"postAlertSeconds,omitempty"`
}

type IOSAlarmPresentation struct {
	AlertTitle       string `json:"alertTitle"`
	CountdownTitle   string `json:"countdownTitle,omitempty"`
	PausedTitle      string `json:"pausedTitle,omitempty"`
	TintColor        string `json:"tintColor,omitempty"`
	SecondaryAction  string `json:"secondaryAction,omitempty"`
}

type IOSAlarmSound struct {
	Kind    string `json:"kind"`
	SoundID string `json:"soundId,omitempty"`
}

type IOSAlarmMetadata struct {
	Kind      string `json:"kind"`
	Icon      string `json:"icon,omitempty"`
	OwnerRef  string `json:"ownerRef,omitempty"`
}

type IOSAlarmScheduleRequest struct {
	Kind         string              `json:"kind"`
	Title        string              `json:"title"`
	Schedule     *IOSAlarmSchedule   `json:"schedule,omitempty"`
	Countdown    *IOSAlarmCountdown  `json:"countdown,omitempty"`
	Presentation IOSAlarmPresentation `json:"presentation"`
	Sound        IOSAlarmSound       `json:"sound,omitempty"`
	Action       string              `json:"action,omitempty"`
	Metadata     *IOSAlarmMetadata   `json:"metadata,omitempty"`
}

type IOSAlarmInfo struct {
	AlarmID          string                 `json:"alarmId"`
	State            string                 `json:"state"`
	Title            string                 `json:"title"`
	Schedule         *IOSAlarmScheduleInfo  `json:"schedule,omitempty"`
	Countdown        *IOSAlarmCountdownInfo `json:"countdown,omitempty"`
	OwnerRef         string                 `json:"ownerRef,omitempty"`
	SupportedActions []string               `json:"supportedActions"`
}

type IOSAlarmScheduleInfo struct {
	FireAt     *string  `json:"fireAt,omitempty"`
	Hour       *int     `json:"hour,omitempty"`
	Minute     *int     `json:"minute,omitempty"`
	Recurrence string   `json:"recurrence,omitempty"`
	Weekdays   []string `json:"weekdays,omitempty"`
}

type IOSAlarmCountdownInfo struct {
	PreAlertSeconds  *int64 `json:"preAlertSeconds,omitempty"`
	PostAlertSeconds *int64 `json:"postAlertSeconds,omitempty"`
}

type AlarmLifecycleResult struct {
	AlarmID         string   `json:"alarmId"`
	State           string   `json:"state"`
	StateChanged    bool     `json:"stateChanged"`
	SupportedActions []string `json:"supportedActions"`
}

type IOSAlarmLink struct {
	AlarmID   string `json:"alarmId"`
	OwnerType string `json:"ownerType,omitempty"`
	OwnerID   string `json:"ownerId,omitempty"`
	Purpose   string `json:"purpose,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
}
