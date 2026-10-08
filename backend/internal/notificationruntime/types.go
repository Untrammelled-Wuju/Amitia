package notificationruntime

import (
	"strings"
	"time"
)

type DeviceEndpoint struct {
	ID                           string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID                      string     `gorm:"column:space_id;not null;index:idx_notification_device_identity,unique" json:"spaceId"`
	DeviceID                     string     `gorm:"column:device_id;not null;index:idx_notification_device_identity,unique" json:"deviceId"`
	Platform                     string     `gorm:"column:platform;not null;index:idx_notification_device_identity,unique" json:"platform"`
	DeviceName                   string     `gorm:"column:device_name;not null;default:''" json:"deviceName"`
	AppVersion                   string     `gorm:"column:app_version;not null;default:''" json:"appVersion"`
	OSVersion                    string     `gorm:"column:os_version;not null;default:''" json:"osVersion"`
	Locale                       string     `gorm:"column:locale;not null;default:''" json:"locale"`
	Appearance                   string     `gorm:"column:appearance;not null;default:''" json:"appearance"`
	Timezone                     string     `gorm:"column:timezone;not null;default:''" json:"timezone"`
	PreferredProvider            string     `gorm:"column:preferred_provider;not null;default:''" json:"preferredProvider"`
	NativeDataProviders          string     `gorm:"column:native_data_providers;not null;default:''" json:"-"`
	FCMToken                     string     `gorm:"column:fcm_token;not null;default:''" json:"-"`
	APNSToken                    string     `gorm:"column:apns_token;not null;default:''" json:"-"`
	VoIPToken                    string     `gorm:"column:voip_token;not null;default:''" json:"-"`
	HMSToken                     string     `gorm:"column:hms_token;not null;default:''" json:"-"`
	MiPushToken                  string     `gorm:"column:mipush_token;not null;default:''" json:"-"`
	OppoToken                    string     `gorm:"column:oppo_token;not null;default:''" json:"-"`
	VivoToken                    string     `gorm:"column:vivo_token;not null;default:''" json:"-"`
	HonorToken                   string     `gorm:"column:honor_token;not null;default:''" json:"-"`
	LiveActivityPushToStartToken string     `gorm:"column:live_activity_push_to_start_token;not null;default:''" json:"-"`
	PushEnabled                  bool       `gorm:"column:push_enabled;not null" json:"pushEnabled"`
	SystemNotificationsEnabled   bool       `gorm:"column:system_notifications_enabled;not null;default:1" json:"systemNotificationsEnabled"`
	MessagePushEnabled           bool       `gorm:"column:message_push_enabled;not null" json:"messagePushEnabled"`
	ExecutionActivityEnabled     bool       `gorm:"column:execution_activity_enabled;not null" json:"executionActivityEnabled"`
	CallPushEnabled              bool       `gorm:"column:call_push_enabled;not null" json:"callPushEnabled"`
	ReminderPushEnabled          bool       `gorm:"column:reminder_push_enabled;not null" json:"reminderPushEnabled"`
	PreviewMode                  string     `gorm:"column:preview_mode;not null;default:'full'" json:"previewMode"`
	SoundEnabled                 bool       `gorm:"column:sound_enabled;not null" json:"soundEnabled"`
	LiveActivitySupported        bool       `gorm:"column:live_activity_supported;not null;default:0" json:"liveActivitySupported"`
	DynamicIslandSupported       bool       `gorm:"column:dynamic_island_supported;not null;default:0" json:"dynamicIslandSupported"`
	ProgressStyleSupported       bool       `gorm:"column:progress_style_supported;not null;default:0" json:"progressStyleSupported"`
	CommunicationSupported       bool       `gorm:"column:communication_supported;not null;default:0" json:"communicationSupported"`
	Foreground                   bool       `gorm:"column:foreground;not null;default:0" json:"foreground"`
	ActiveConversationID         string     `gorm:"column:active_conversation_id;not null;default:''" json:"activeConversationId"`
	PresenceUpdatedAt            time.Time  `gorm:"column:presence_updated_at" json:"presenceUpdatedAt"`
	LastSeenAt                   time.Time  `gorm:"column:last_seen_at;index" json:"lastSeenAt"`
	TokenUpdatedAt               time.Time  `gorm:"column:token_updated_at" json:"tokenUpdatedAt"`
	CreatedAt                    time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                    time.Time  `gorm:"column:updated_at" json:"updatedAt"`
	RevokedAt                    *time.Time `gorm:"column:revoked_at;index" json:"revokedAt,omitempty"`
}

func (DeviceEndpoint) TableName() string { return "notification_devices" }

func (d DeviceEndpoint) SupportsNativeData(provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" || strings.TrimSpace(d.NativeDataProviders) == "" {
		return false
	}
	for _, candidate := range strings.Split(d.NativeDataProviders, ",") {
		if strings.ToLower(strings.TrimSpace(candidate)) == provider {
			return true
		}
	}
	return false
}

type LiveActivityToken struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	SpaceID        string     `gorm:"column:space_id;not null;index:idx_notification_live_scope_run,unique" json:"spaceId"`
	DeviceID       string     `gorm:"column:device_id;not null;index:idx_notification_live_scope_run,unique" json:"deviceId"`
	ConversationID string     `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	RunID          string     `gorm:"column:run_id;not null;index:idx_notification_live_scope_run,unique" json:"runId"`
	ActivityID     string     `gorm:"column:activity_id;not null;default:''" json:"activityId"`
	UpdateToken    string     `gorm:"column:update_token;not null;default:''" json:"-"`
	Revision       int64      `gorm:"column:revision;not null;default:0" json:"revision"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updatedAt"`
	EndedAt        *time.Time `gorm:"column:ended_at" json:"endedAt,omitempty"`
}

func (LiveActivityToken) TableName() string { return "notification_live_activities" }

type DeliveryLog struct {
	ID                string     `gorm:"column:id;primaryKey" json:"id"`
	NotificationID    string     `gorm:"column:notification_id;not null;index" json:"notificationId"`
	SpaceID           string     `gorm:"column:space_id;not null;index" json:"spaceId"`
	DeviceID          string     `gorm:"column:device_id;not null;index" json:"deviceId"`
	Provider          string     `gorm:"column:provider;not null" json:"provider"`
	Type              string     `gorm:"column:type;not null;index" json:"type"`
	Status            string     `gorm:"column:status;not null;index" json:"status"`
	ProviderMessageID string     `gorm:"column:provider_message_id;not null;default:''" json:"providerMessageId"`
	ErrorCode         string     `gorm:"column:error_code;not null;default:''" json:"errorCode"`
	ErrorMessage      string     `gorm:"column:error_message;not null;default:''" json:"errorMessage"`
	RetryCount        int        `gorm:"column:retry_count;not null;default:0" json:"retryCount"`
	QueuedAt          time.Time  `gorm:"column:queued_at" json:"queuedAt"`
	SentAt            *time.Time `gorm:"column:sent_at" json:"sentAt,omitempty"`
	AcceptedAt        *time.Time `gorm:"column:accepted_at" json:"acceptedAt,omitempty"`
	OpenedAt          *time.Time `gorm:"column:opened_at" json:"openedAt,omitempty"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;index" json:"expiresAt"`
}

func (DeliveryLog) TableName() string { return "notification_deliveries" }

type NotificationOutbox struct {
	ID                string    `gorm:"column:id;primaryKey" json:"id"`
	NotificationID    string    `gorm:"column:notification_id;not null;index:idx_notification_outbox_identity,unique" json:"notificationId"`
	SpaceID           string    `gorm:"column:space_id;not null;index:idx_notification_outbox_identity,unique" json:"spaceId"`
	DeviceID          string    `gorm:"column:device_id;not null;index:idx_notification_outbox_identity,unique" json:"deviceId"`
	Type              string    `gorm:"column:type;not null;default:'';index" json:"type"`
	RunID             string    `gorm:"column:run_id;not null;default:'';index" json:"runId"`
	EnvelopeJSON      string    `gorm:"column:envelope_json;not null" json:"-"`
	Status            string    `gorm:"column:status;not null;index" json:"status"`
	Provider          string    `gorm:"column:provider;not null;default:''" json:"provider"`
	ProviderMessageID string    `gorm:"column:provider_message_id;not null;default:''" json:"providerMessageId"`
	AttemptCount      int       `gorm:"column:attempt_count;not null;default:0" json:"attemptCount"`
	LastError         string    `gorm:"column:last_error;not null;default:''" json:"lastError"`
	NextAttemptAt     time.Time `gorm:"column:next_attempt_at;index" json:"nextAttemptAt"`
	ExpiresAt         time.Time `gorm:"column:expires_at;index" json:"expiresAt"`
	CreatedAt         time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (NotificationOutbox) TableName() string { return "notification_outbox" }

type NotificationRunTerminal struct {
	ID       string    `gorm:"column:id;primaryKey" json:"id"`
	SpaceID  string    `gorm:"column:space_id;not null;index:idx_notification_run_terminal,unique" json:"spaceId"`
	DeviceID string    `gorm:"column:device_id;not null;index:idx_notification_run_terminal,unique" json:"deviceId"`
	RunID    string    `gorm:"column:run_id;not null;index:idx_notification_run_terminal,unique" json:"runId"`
	Revision int64     `gorm:"column:revision;not null;default:0" json:"revision"`
	EndedAt  time.Time `gorm:"column:ended_at;not null;index" json:"endedAt"`
}

func (NotificationRunTerminal) TableName() string { return "notification_run_terminals" }

type PushEnvelope struct {
	NotificationID string
	Type           string
	SpaceID        string
	ConversationID string
	CharacterID    string
	MessageID      string
	RunID          string
	Revision       int64
	Title          string
	Body           string
	DeepLink       string
	Priority       string
	TTL            time.Duration
	Badge          int
	Sound          bool
	Data           map[string]string
}

type ExecutionState struct {
	RunID          string
	ConversationID string
	CharacterID    string
	AgentID        string
	Title          string
	Summary        string
	Phase          string
	CurrentStep    int
	TotalSteps     int
	Progress       float64
	TotalTokens    int
	Revision       int64
	StartedAt      time.Time
	UpdatedAt      time.Time
}

type ProviderResult struct {
	Provider          string
	ProviderMessageID string
	Accepted          bool
	InvalidToken      bool
	ErrorCode         string
	ErrorMessage      string
}
