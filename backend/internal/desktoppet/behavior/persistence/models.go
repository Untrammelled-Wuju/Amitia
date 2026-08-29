package persistence

type BehaviorContextModel struct {
	UserID                  string `gorm:"column:user_id;primaryKey"`
	CharacterID             string `gorm:"column:character_id;primaryKey"`
	Revision                int64  `gorm:"column:revision"`
	StableStateJSON         string `gorm:"column:stable_state_json"`
	TransientStateJSON      string `gorm:"column:transient_state_json"`
	ActiveToolsJSON         string `gorm:"column:active_tools_json"`
	VoiceStateJSON          string `gorm:"column:voice_state_json"`
	DesktopGestureJSON      string `gorm:"column:desktop_gesture_json"`
	ForegroundJSON          string `gorm:"column:foreground_json"`
	CooldownsJSON           string `gorm:"column:cooldowns_json"`
	RecentSemanticsJSON     string `gorm:"column:recent_semantics_json"`
	DesiredStateJSON        string `gorm:"column:desired_state_json"`
	LastSourceRevisionsJSON string `gorm:"column:last_source_revisions_json"`
	UpdatedAt               string `gorm:"column:updated_at"`
}

func (BehaviorContextModel) TableName() string {
	return "desktop_pet_behavior_contexts"
}

type BehaviorInboxModel struct {
	EventID           string `gorm:"column:event_id;primaryKey"`
	DedupKey          string `gorm:"column:dedup_key;uniqueIndex:ux_desktop_pet_behavior_inbox_dedup"`
	EventType         string `gorm:"column:event_type"`
	SchemaVersion     int    `gorm:"column:schema_version"`
	UserID            string `gorm:"column:user_id"`
	CharacterID       string `gorm:"column:character_id"`
	ConversationID    string `gorm:"column:conversation_id"`
	InteractionID     string `gorm:"column:interaction_id"`
	SessionID         string `gorm:"column:session_id"`
	ToolOperationID   string `gorm:"column:tool_operation_id"`
	InstallationID    string `gorm:"column:installation_id"`
	PetInstanceID     string `gorm:"column:pet_instance_id"`
	ReleaseID         string `gorm:"column:release_id"`
	OccurredAt        string `gorm:"column:occurred_at"`
	ReceivedAt        string `gorm:"column:received_at"`
	ExpiresAt         string `gorm:"column:expires_at"`
	Origin            string `gorm:"column:origin"`
	CorrelationID     string `gorm:"column:correlation_id"`
	CausationID       string `gorm:"column:causation_id"`
	EventEnvelopeJSON string `gorm:"column:event_envelope_json"`
	PayloadJSON       string `gorm:"column:payload_json"`
	PayloadHash       string `gorm:"column:payload_hash"`
	Status            string `gorm:"column:status"`
	AttemptCount      int    `gorm:"column:attempt_count"`
	LeaseOwner        string `gorm:"column:lease_owner"`
	LeaseExpiresAt    string `gorm:"column:lease_expires_at"`
	HeartbeatAt       string `gorm:"column:heartbeat_at"`
	AvailableAt       string `gorm:"column:available_at"`
	LastErrorCode     string `gorm:"column:last_error_code"`
	LastErrorMessage  string `gorm:"column:last_error_message"`
	ProcessedAt       string `gorm:"column:processed_at"`
	CreatedAt         string `gorm:"column:created_at"`
}

func (BehaviorInboxModel) TableName() string {
	return "desktop_pet_behavior_inbox"
}

type BehaviorDecisionModel struct {
	DecisionID             string `gorm:"column:decision_id;primaryKey"`
	EventID                string `gorm:"column:event_id"`
	UserID                 string `gorm:"column:user_id"`
	CharacterID            string `gorm:"column:character_id"`
	InstallationID         string `gorm:"column:installation_id"`
	ContextRevision        int64  `gorm:"column:context_revision"`
	RulesetVersion         int    `gorm:"column:ruleset_version"`
	InterruptPolicy        string `gorm:"column:interrupt_policy"`
	MinimumPlayMS          int64  `gorm:"column:minimum_play_ms"`
	MaximumPlayMS          int64  `gorm:"column:maximum_play_ms"`
	FallbackDepth          int    `gorm:"column:fallback_depth"`
	ReturnPolicy           string `gorm:"column:return_policy"`
	ContextHash            string `gorm:"column:context_hash"`
	Semantic               string `gorm:"column:semantic"`
	ActionKey              string `gorm:"column:action_key"`
	Priority               int    `gorm:"column:priority"`
	Status                 string `gorm:"column:status"`
	ReasonCode             string `gorm:"column:reason_code"`
	RejectedCandidatesJSON string `gorm:"column:rejected_candidates_json"`
	RuntimeCommandID       string `gorm:"column:runtime_command_id"`
	CreatedAt              string `gorm:"column:created_at"`
	StartedAt              string `gorm:"column:started_at"`
	CompletedAt            string `gorm:"column:completed_at"`
}

func (BehaviorDecisionModel) TableName() string {
	return "desktop_pet_behavior_decisions"
}

type BehaviorCooldownModel struct {
	UserID           string `gorm:"column:user_id;primaryKey"`
	CharacterID      string `gorm:"column:character_id;primaryKey"`
	CooldownKey      string `gorm:"column:cooldown_key;primaryKey"`
	UntilAt          string `gorm:"column:until_at"`
	SourceDecisionID string `gorm:"column:source_decision_id"`
	UpdatedAt        string `gorm:"column:updated_at"`
}

func (BehaviorCooldownModel) TableName() string {
	return "desktop_pet_behavior_cooldowns"
}
