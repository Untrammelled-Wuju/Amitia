package v2

const (
	InstanceStatusAbsent               = "absent"
	InstanceStatusStarting             = "starting"
	InstanceStatusLoadingRelease       = "loading_release"
	InstanceStatusWindowCreated        = "window_created"
	InstanceStatusRendererInitializing = "renderer_initializing"
	InstanceStatusReady                = "ready"
	InstanceStatusStopping             = "stopping"
	InstanceStatusStopped              = "stopped"
	InstanceStatusFailed               = "failed"
)

const (
	WindowStatusAbsent    = "absent"
	WindowStatusHidden    = "hidden"
	WindowStatusVisible   = "visible"
	WindowStatusDestroyed = "destroyed"
	WindowStatusFailed    = "failed"
)

const (
	RendererStatusAbsent       = "absent"
	RendererStatusBootstrapped = "bootstrapped"
	RendererStatusRuntimeReady = "runtime_ready"
	RendererStatusUnresponsive = "unresponsive"
	RendererStatusCrashed      = "crashed"
	RendererStatusFailed       = "failed"
)

const (
	PlaybackStatusIdle    = "idle"
	PlaybackStatusLoading = "loading"
	PlaybackStatusPlaying = "playing"
	PlaybackStatusHolding = "holding"
	PlaybackStatusPaused  = "paused"
	PlaybackStatusStopped = "stopped"
	PlaybackStatusFailed  = "failed"
)

const (
	HealthStatusOffline     = "offline"
	HealthStatusOnlineNoPet = "online_no_pet"
	HealthStatusSyncing     = "syncing"
	HealthStatusHealthy     = "healthy"
	HealthStatusDegraded    = "degraded"
	HealthStatusFailed      = "failed"
)

type RuntimeActualState struct {
	UserID    string `gorm:"column:user_id;type:text;not null;primaryKey" json:"userId"`
	DeviceID  string `gorm:"column:device_id;type:text;not null;primaryKey" json:"deviceId"`
	RuntimeID string `gorm:"column:runtime_id;type:text;not null;primaryKey" json:"runtimeId"`

	RuntimeSessionID     string `gorm:"column:runtime_session_id;type:text" json:"runtimeSessionId"`
	ConnectionGeneration int64  `gorm:"column:connection_generation;type:integer" json:"connectionGeneration"`
	LastEventSequence    int64  `gorm:"column:last_event_sequence;type:integer" json:"lastEventSequence"`

	AppliedDesiredRevision  int64  `gorm:"column:applied_desired_revision;type:integer" json:"appliedDesiredRevision"`
	AppliedDesiredHash      string `gorm:"column:applied_desired_hash;type:text" json:"appliedDesiredHash"`
	AppliedSettingsRevision int64  `gorm:"column:applied_settings_revision;type:integer" json:"appliedSettingsRevision"`

	InstallationID string `gorm:"column:installation_id;type:text" json:"installationId"`
	PetID          string `gorm:"column:pet_id;type:text" json:"petId"`
	ReleaseID      string `gorm:"column:release_id;type:text" json:"releaseId"`

	InstanceStatus string `gorm:"column:instance_status;type:text" json:"instanceStatus"`
	WindowStatus   string `gorm:"column:window_status;type:text" json:"windowStatus"`
	RendererStatus string `gorm:"column:renderer_status;type:text" json:"rendererStatus"`
	PlaybackStatus string `gorm:"column:playback_status;type:text" json:"playbackStatus"`

	Visible bool `gorm:"column:visible;type:integer" json:"visible"`

	StableActionKey    string `gorm:"column:stable_action_key;type:text" json:"stableActionKey"`
	CurrentActionKey   string `gorm:"column:current_action_key;type:text" json:"currentActionKey"`
	PlaybackInstanceID string `gorm:"column:playback_instance_id;type:text" json:"playbackInstanceId,omitempty"`
	CurrentCommandID   string `gorm:"column:current_command_id;type:text" json:"currentCommandId,omitempty"`

	ActualStateHash string `gorm:"column:actual_state_hash;type:text" json:"actualStateHash"`

	HealthStatus  string `gorm:"column:health_status;type:text" json:"healthStatus"`
	LastErrorCode string `gorm:"column:last_error_code;type:text" json:"lastErrorCode,omitempty"`

	UpdatedAt string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (RuntimeActualState) TableName() string {
	return "desktop_pet_runtime_actual_states_v2"
}

func (s *RuntimeActualState) CanUpdate(newGen, newSeq int64) bool {
	if newGen > s.ConnectionGeneration {
		return true
	}
	if newGen == s.ConnectionGeneration && newSeq > s.LastEventSequence {
		return true
	}
	return false
}

type CommandAttempt struct {
	AttemptID string `gorm:"column:attempt_id;primaryKey;type:text" json:"attemptId"`
	CommandID string `gorm:"column:command_id;type:text;not null" json:"commandId"`

	RuntimeSessionID     string `gorm:"column:runtime_session_id;type:text" json:"runtimeSessionId"`
	ConnectionGeneration int64  `gorm:"column:connection_generation;type:integer" json:"connectionGeneration"`

	DispatchedAt       string `gorm:"column:dispatched_at;type:text" json:"dispatchedAt,omitempty"`
	RuntimeReceivedAt  string `gorm:"column:runtime_received_at;type:text" json:"runtimeReceivedAt,omitempty"`
	RuntimeAcceptedAt  string `gorm:"column:runtime_accepted_at;type:text" json:"runtimeAcceptedAt,omitempty"`
	RendererAcceptedAt string `gorm:"column:renderer_accepted_at;type:text" json:"rendererAcceptedAt,omitempty"`
	PlaybackStartedAt  string `gorm:"column:playback_started_at;type:text" json:"playbackStartedAt,omitempty"`
	FinishedAt         string `gorm:"column:finished_at;type:text" json:"finishedAt,omitempty"`

	ErrorCode    string `gorm:"column:error_code;type:text" json:"errorCode,omitempty"`
	ErrorMessage string `gorm:"column:error_message;type:text" json:"errorMessage,omitempty"`

	InsertedAt string `gorm:"column:inserted_at;type:text" json:"insertedAt"`
	UpdatedAt  string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (CommandAttempt) TableName() string {
	return "desktop_pet_runtime_command_attempts"
}

type CommandResult struct {
	ID        string `gorm:"column:id;primaryKey;type:text" json:"id"`
	CommandID string `gorm:"column:command_id;type:text;not null;uniqueIndex:idx_cmd_result_cmd_runtime" json:"commandId"`
	RuntimeID string `gorm:"column:runtime_id;type:text;not null;uniqueIndex:idx_cmd_result_cmd_runtime" json:"runtimeId"`
	AttemptID string `gorm:"column:attempt_id;type:text" json:"attemptId"`

	ResultType string `gorm:"column:result_type;type:text;not null" json:"resultType"`
	ResultJSON string `gorm:"column:result_json;type:text" json:"resultJSON"`
	ResultHash string `gorm:"column:result_hash;type:text" json:"resultHash"`

	RuntimeSessionID     string `gorm:"column:runtime_session_id;type:text" json:"runtimeSessionId"`
	ConnectionGeneration int64  `gorm:"column:connection_generation;type:integer" json:"connectionGeneration"`
	EventSequence        int64  `gorm:"column:event_sequence;type:integer" json:"eventSequence"`

	ErrorCode    string `gorm:"column:error_code;type:text" json:"errorCode,omitempty"`
	ErrorMessage string `gorm:"column:error_message;type:text" json:"errorMessage,omitempty"`

	InsertedAt string `gorm:"column:inserted_at;type:text" json:"insertedAt"`
	UpdatedAt  string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (CommandResult) TableName() string {
	return "desktop_pet_runtime_command_results"
}

type DeviceCommandSequence struct {
	UserID         string `gorm:"column:user_id;type:text;not null;primaryKey" json:"userId"`
	DeviceID       string `gorm:"column:device_id;type:text;not null;primaryKey" json:"deviceId"`
	Sequence       int64  `gorm:"column:sequence;type:integer;not null" json:"sequence"`
	LastReservedAt string `gorm:"column:last_reserved_at;type:text" json:"lastReservedAt"`
	InsertedAt     string `gorm:"column:inserted_at;type:text" json:"insertedAt"`
	UpdatedAt      string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (DeviceCommandSequence) TableName() string {
	return "desktop_pet_runtime_device_command_sequences"
}

type DomainEventOutbox struct {
	ID             string `gorm:"column:id;primaryKey;type:text" json:"id"`
	EventType      string `gorm:"column:event_type;type:text;not null" json:"eventType"`
	AggregateID    string `gorm:"column:aggregate_id;type:text" json:"aggregateId"`
	Payload        []byte `gorm:"column:payload;type:blob" json:"payload"`
	Status         string `gorm:"column:status;type:text;not null" json:"status"`
	Attempt        int    `gorm:"column:attempt;type:integer" json:"attempt"`
	IdempotencyKey string `gorm:"column:idempotency_key;type:text" json:"idempotencyKey"`
	ClaimExpiresAt string `gorm:"column:claim_expires_at;type:text" json:"claimExpiresAt"`
	LastError      string `gorm:"column:last_error;type:text" json:"lastError,omitempty"`
	InsertedAt     string `gorm:"column:inserted_at;type:text" json:"insertedAt"`
	UpdatedAt      string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	PublishedAt    string `gorm:"column:published_at;type:text" json:"publishedAt,omitempty"`
}

func (DomainEventOutbox) TableName() string {
	return "desktop_pet_runtime_domain_event_outbox"
}

type CommandDedup struct {
	ID             string `gorm:"column:id;primaryKey;type:text" json:"id"`
	UserID         string `gorm:"column:user_id;type:text;not null;primaryKey" json:"userId"`
	DeviceID       string `gorm:"column:device_id;type:text;not null" json:"deviceId"`
	IdempotencyKey string `gorm:"column:idempotency_key;type:text;not null" json:"idempotencyKey"`
	NakCount       int    `gorm:"column:nak_count;type:integer" json:"nakCount"`
	LastNakAt      string `gorm:"column:last_nak_at;type:text" json:"lastNakAt"`
	InsertedAt     string `gorm:"column:inserted_at;type:text" json:"insertedAt"`
	UpdatedAt      string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (CommandDedup) TableName() string {
	return "desktop_pet_runtime_command_dedup"
}

type ReconcileLease struct {
	ReconcilerID    string `gorm:"column:reconciler_id;primaryKey;type:text" json:"reconcilerId"`
	LastHeartbeatAt string `gorm:"column:last_heartbeat_at;type:text" json:"lastHeartbeatAt"`
	InsertedAt      string `gorm:"column:inserted_at;type:text" json:"insertedAt"`
	UpdatedAt       string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

const (
	OutboxStatusPending = "pending"
	OutboxStatusClaimed = "claimed"
	OutboxStatusSent    = "sent"
	OutboxStatusFailed  = "failed"
)

func (ReconcileLease) TableName() string {
	return "desktop_pet_runtime_reconcile_leases"
}
