package temporal

import "time"

const (
	CanonicalRelationshipChannel = "*"
	CanonicalRelationshipType    = "user_character"
	RelationshipTimeVersion      = "relationship-time-v1"
	SessionBreakThreshold        = 45 * time.Minute
	ReunionClaimTTL              = 5 * time.Minute
	DefaultExpectedGap           = 24 * time.Hour
	MaximumCadenceSamples        = 60
)

type ReunionKind string

const (
	ReunionKindGlobalReturn          ReunionKind = "global_return"
	ReunionKindRelationshipReconnect ReunionKind = "relationship_reconnect"
	ReunionKindReplyToProactive      ReunionKind = "reply_to_recent_proactive"
)

type ReunionLevel string

const (
	ReunionLevelNone       ReunionLevel = "none"
	ReunionLevelNoticeable ReunionLevel = "noticeable"
	ReunionLevelLong       ReunionLevel = "long"
	ReunionLevelExtended   ReunionLevel = "extended"
	ReunionLevelDormant    ReunionLevel = "dormant"
)

type ReunionState string

const (
	ReunionStatePending    ReunionState = "pending"
	ReunionStateClaimed    ReunionState = "claimed"
	ReunionStateHandled    ReunionState = "handled"
	ReunionStateSuppressed ReunionState = "suppressed"
	ReunionStateExpired    ReunionState = "expired"
)

type InteractionReceiptStatus string

const (
	InteractionReceiptObserved   InteractionReceiptStatus = "observed"
	InteractionReceiptCommitted  InteractionReceiptStatus = "committed"
	InteractionReceiptFailed     InteractionReceiptStatus = "failed"
	InteractionReceiptSuperseded InteractionReceiptStatus = "superseded"
)

type RelationshipTimeContext struct {
	Version string `json:"version"`

	UserID      string `json:"userId"`
	CharacterID string `json:"characterId"`

	NowUTC time.Time `json:"nowUtc"`

	FirstInteractionAt          time.Time `json:"firstInteractionAt,omitempty"`
	GlobalLastCommittedAt       time.Time `json:"globalLastCommittedAt,omitempty"`
	RelationshipLastCommittedAt time.Time `json:"relationshipLastCommittedAt,omitempty"`
	LastSuccessfulExchangeAt    time.Time `json:"lastSuccessfulExchangeAt,omitempty"`
	LastAssistantContactAt      time.Time `json:"lastAssistantContactAt,omitempty"`

	GlobalGapSeconds       float64 `json:"globalGapSeconds"`
	RelationshipGapSeconds float64 `json:"relationshipGapSeconds"`
	ExpectedGapSeconds     float64 `json:"expectedGapSeconds"`
	GapDeviationScore      float64 `json:"gapDeviationScore"`
	NormalizedGap          float64 `json:"normalizedGap"`

	RelationshipAgeDays int `json:"relationshipAgeDays"`
	InteractionCount    int `json:"interactionCount"`
	SessionCount        int `json:"sessionCount"`

	Reunion *ReunionContext `json:"reunion,omitempty"`

	ContinuityScore           float64 `json:"continuityScore"`
	ReacclimationTurnsLeft    int     `json:"reacclimationTurnsLeft"`
	EffectiveTension          float64 `json:"effectiveTension"`
	StoredTension             float64 `json:"storedTension"`
	HasRecentAssistantContact bool    `json:"hasRecentAssistantContact"`

	Diagnostics []string `json:"diagnostics,omitempty"`
}

type ReunionContext struct {
	EpisodeID string `json:"episodeId"`

	Kind  ReunionKind  `json:"kind"`
	Level ReunionLevel `json:"level"`
	State ReunionState `json:"state"`

	RelationshipGapSeconds float64 `json:"relationshipGapSeconds"`
	GlobalGapSeconds       float64 `json:"globalGapSeconds"`
	ExpectedGapSeconds     float64 `json:"expectedGapSeconds"`
	NormalizedGap          float64 `json:"normalizedGap"`

	ClaimedByInteractionID string    `json:"claimedByInteractionId,omitempty"`
	ClaimExpiresAt         time.Time `json:"claimExpiresAt,omitempty"`

	ShouldExpress bool `json:"shouldExpress"`
}

type PrepareInboundInput struct {
	UserID         string
	CharacterID    string
	ConversationID string
	Channel        string
	PeerID         string
	RequestID      string
	InteractionID  string
	ObservedAt     time.Time
	IsInternal     bool
	Source         string
}

type ObservePresenceInput struct {
	UserID      string
	CharacterID string
	Channel     string
	ObservedAt  time.Time
}

type FinalizeInteractionInput struct {
	UserID              string
	CharacterID         string
	InteractionID       string
	CommittedAt         time.Time
	ReunionEpisodeID    string
	SuppressReunion     bool
	SuppressionReason   string
	ReacclimationTurns  int
	ExpectedGapSeconds  float64
	GapMADSeconds       float64
	CadenceSample       *CadenceSample
	EffectLedgerEntries []TemporalEffectLedgerEntry
	AssistantInitiated  bool
}

type RelationshipTimeSettings struct {
	CharacterID             string `gorm:"column:character_id;primaryKey" json:"characterId"`
	Enabled                 bool   `gorm:"column:enabled" json:"enabled"`
	ReunionEnabled          bool   `gorm:"column:reunion_enabled" json:"reunionEnabled"`
	Sensitivity             string `gorm:"column:sensitivity" json:"sensitivity"`
	AllowMemoryRecall       bool   `gorm:"column:allow_memory_recall" json:"allowMemoryRecall"`
	AllowRelationshipAge    bool   `gorm:"column:allow_relationship_age" json:"allowRelationshipAge"`
	AllowReunionMention     bool   `gorm:"column:allow_reunion_mention" json:"allowReunionMention"`
	AllowProactiveReference bool   `gorm:"column:allow_proactive_reference" json:"allowProactiveReference"`
	MaxMentionSentences     int    `gorm:"column:max_mention_sentences" json:"maxMentionSentences"`
	UpdatedAt               string `gorm:"column:updated_at_utc" json:"updatedAt"`
}

func (RelationshipTimeSettings) TableName() string {
	return "temporal_relationship_time_settings"
}

func DefaultRelationshipTimeSettings(characterID string) RelationshipTimeSettings {
	return RelationshipTimeSettings{CharacterID: characterID, Enabled: true, ReunionEnabled: true, Sensitivity: "balanced", AllowMemoryRecall: true, AllowRelationshipAge: true, AllowReunionMention: true, AllowProactiveReference: true, MaxMentionSentences: 1}
}

type GlobalPresenceState struct {
	UserID                            string `gorm:"column:user_id;primaryKey" json:"userId"`
	FirstUserActivityAtUTC            string `gorm:"column:first_user_activity_at_utc" json:"firstUserActivityAtUtc"`
	LastObservedUserActivityAtUTC     string `gorm:"column:last_observed_user_activity_at_utc" json:"lastObservedUserActivityAtUtc"`
	LastCommittedUserInteractionAtUTC string `gorm:"column:last_committed_user_interaction_at_utc" json:"lastCommittedUserInteractionAtUtc"`
	LastChannel                       string `gorm:"column:last_channel" json:"lastChannel"`
	LastCharacterID                   string `gorm:"column:last_character_id" json:"lastCharacterId"`
	InteractionCount                  int    `gorm:"column:interaction_count" json:"interactionCount"`
	SessionCount                      int    `gorm:"column:session_count" json:"sessionCount"`
	StateVersion                      int    `gorm:"column:state_version" json:"stateVersion"`
	CreatedAtUTC                      string `gorm:"column:created_at_utc" json:"createdAtUtc"`
	UpdatedAtUTC                      string `gorm:"column:updated_at_utc" json:"updatedAtUtc"`
}

func (GlobalPresenceState) TableName() string { return "temporal_global_presence_states" }

type RelationshipPresenceState struct {
	ID                                string  `gorm:"column:id;primaryKey" json:"id"`
	UserID                            string  `gorm:"column:user_id" json:"userId"`
	CharacterID                       string  `gorm:"column:character_id" json:"characterId"`
	FirstInteractionAtUTC             string  `gorm:"column:first_interaction_at_utc" json:"firstInteractionAtUtc"`
	LastObservedUserActivityAtUTC     string  `gorm:"column:last_observed_user_activity_at_utc" json:"lastObservedUserActivityAtUtc"`
	LastCommittedUserInteractionAtUTC string  `gorm:"column:last_committed_user_interaction_at_utc" json:"lastCommittedUserInteractionAtUtc"`
	LastSuccessfulExchangeAtUTC       string  `gorm:"column:last_successful_exchange_at_utc" json:"lastSuccessfulExchangeAtUtc"`
	LastAssistantContactAtUTC         string  `gorm:"column:last_assistant_contact_at_utc" json:"lastAssistantContactAtUtc"`
	InteractionCount                  int     `gorm:"column:interaction_count" json:"interactionCount"`
	SessionCount                      int     `gorm:"column:session_count" json:"sessionCount"`
	CadenceSampleCount                int     `gorm:"column:cadence_sample_count" json:"cadenceSampleCount"`
	ExpectedGapSeconds                float64 `gorm:"column:expected_gap_seconds" json:"expectedGapSeconds"`
	GapMADSeconds                     float64 `gorm:"column:gap_mad_seconds" json:"gapMadSeconds"`
	ContinuityScore                   float64 `gorm:"column:continuity_score" json:"continuityScore"`
	ReacclimationTurnsRemaining       int     `gorm:"column:reacclimation_turns_remaining" json:"reacclimationTurnsRemaining"`
	ActiveReunionEpisodeID            string  `gorm:"column:active_reunion_episode_id" json:"activeReunionEpisodeId"`
	StateVersion                      int     `gorm:"column:state_version" json:"stateVersion"`
	CreatedAtUTC                      string  `gorm:"column:created_at_utc" json:"createdAtUtc"`
	UpdatedAtUTC                      string  `gorm:"column:updated_at_utc" json:"updatedAtUtc"`
}

func (RelationshipPresenceState) TableName() string {
	return "temporal_relationship_presence_states"
}

type CadenceSample struct {
	ID                       string  `gorm:"column:id;primaryKey" json:"id"`
	UserID                   string  `gorm:"column:user_id" json:"userId"`
	CharacterID              string  `gorm:"column:character_id" json:"characterId"`
	InteractionID            string  `gorm:"column:interaction_id" json:"interactionId"`
	PreviousInteractionAtUTC string  `gorm:"column:previous_interaction_at_utc" json:"previousInteractionAtUtc"`
	CurrentInteractionAtUTC  string  `gorm:"column:current_interaction_at_utc" json:"currentInteractionAtUtc"`
	GapSeconds               float64 `gorm:"column:gap_seconds" json:"gapSeconds"`
	SampleKind               string  `gorm:"column:sample_kind" json:"sampleKind"`
	Included                 bool    `gorm:"column:included" json:"included"`
	CreatedAtUTC             string  `gorm:"column:created_at_utc" json:"createdAtUtc"`
}

func (CadenceSample) TableName() string { return "temporal_cadence_samples" }

type ReunionEpisode struct {
	ID                                   string       `gorm:"column:id;primaryKey" json:"id"`
	UserID                               string       `gorm:"column:user_id" json:"userId"`
	CharacterID                          string       `gorm:"column:character_id" json:"characterId"`
	ReunionKind                          ReunionKind  `gorm:"column:reunion_kind" json:"reunionKind"`
	ReunionLevel                         ReunionLevel `gorm:"column:reunion_level" json:"reunionLevel"`
	Status                               ReunionState `gorm:"column:status" json:"status"`
	PreviousRelationshipInteractionAtUTC string       `gorm:"column:previous_relationship_interaction_at_utc" json:"previousRelationshipInteractionAtUtc"`
	PreviousGlobalInteractionAtUTC       string       `gorm:"column:previous_global_interaction_at_utc" json:"previousGlobalInteractionAtUtc"`
	DetectedAtUTC                        string       `gorm:"column:detected_at_utc" json:"detectedAtUtc"`
	RelationshipGapSeconds               float64      `gorm:"column:relationship_gap_seconds" json:"relationshipGapSeconds"`
	GlobalGapSeconds                     float64      `gorm:"column:global_gap_seconds" json:"globalGapSeconds"`
	ExpectedGapSeconds                   float64      `gorm:"column:expected_gap_seconds" json:"expectedGapSeconds"`
	NormalizedGap                        float64      `gorm:"column:normalized_gap" json:"normalizedGap"`
	DeviationScore                       float64      `gorm:"column:deviation_score" json:"deviationScore"`
	ContinuityBefore                     float64      `gorm:"column:continuity_before" json:"continuityBefore"`
	ClaimInteractionID                   string       `gorm:"column:claim_interaction_id" json:"claimInteractionId"`
	ClaimExpiresAtUTC                    string       `gorm:"column:claim_expires_at_utc" json:"claimExpiresAtUtc"`
	HandledInteractionID                 string       `gorm:"column:handled_interaction_id" json:"handledInteractionId"`
	HandledAtUTC                         string       `gorm:"column:handled_at_utc" json:"handledAtUtc"`
	SuppressionReason                    string       `gorm:"column:suppression_reason" json:"suppressionReason"`
	PolicyJSON                           string       `gorm:"column:policy_json" json:"policy"`
	IdempotencyKey                       string       `gorm:"column:idempotency_key" json:"idempotencyKey"`
	CreatedAtUTC                         string       `gorm:"column:created_at_utc" json:"createdAtUtc"`
	UpdatedAtUTC                         string       `gorm:"column:updated_at_utc" json:"updatedAtUtc"`
}

func (ReunionEpisode) TableName() string { return "temporal_reunion_episodes" }

type InteractionReceipt struct {
	ID                                 string                   `gorm:"column:id;primaryKey" json:"id"`
	RequestID                          string                   `gorm:"column:request_id" json:"requestId"`
	InteractionID                      string                   `gorm:"column:interaction_id" json:"interactionId"`
	UserID                             string                   `gorm:"column:user_id" json:"userId"`
	CharacterID                        string                   `gorm:"column:character_id" json:"characterId"`
	Channel                            string                   `gorm:"column:channel" json:"channel"`
	PeerID                             string                   `gorm:"column:peer_id" json:"peerId"`
	ObservedAtUTC                      string                   `gorm:"column:observed_at_utc" json:"observedAtUtc"`
	PreviousGlobalCommittedAtUTC       string                   `gorm:"column:previous_global_committed_at_utc" json:"previousGlobalCommittedAtUtc"`
	PreviousRelationshipCommittedAtUTC string                   `gorm:"column:previous_relationship_committed_at_utc" json:"previousRelationshipCommittedAtUtc"`
	ReunionEpisodeID                   string                   `gorm:"column:reunion_episode_id" json:"reunionEpisodeId"`
	Status                             InteractionReceiptStatus `gorm:"column:status" json:"status"`
	CreatedAtUTC                       string                   `gorm:"column:created_at_utc" json:"createdAtUtc"`
	UpdatedAtUTC                       string                   `gorm:"column:updated_at_utc" json:"updatedAtUtc"`
}

func (InteractionReceipt) TableName() string { return "temporal_interaction_receipts" }

type TemporalEffectLedgerEntry struct {
	ID               string `gorm:"column:id;primaryKey" json:"id"`
	EffectKey        string `gorm:"column:effect_key" json:"effectKey"`
	EffectType       string `gorm:"column:effect_type" json:"effectType"`
	UserID           string `gorm:"column:user_id" json:"userId"`
	CharacterID      string `gorm:"column:character_id" json:"characterId"`
	ReunionEpisodeID string `gorm:"column:reunion_episode_id" json:"reunionEpisodeId"`
	InteractionID    string `gorm:"column:interaction_id" json:"interactionId"`
	PayloadJSON      string `gorm:"column:payload_json" json:"payload"`
	AppliedAtUTC     string `gorm:"column:applied_at_utc" json:"appliedAtUtc"`
}

func (TemporalEffectLedgerEntry) TableName() string { return "temporal_effect_ledger" }

func FormatRelationshipTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Round(0).Format(time.RFC3339Nano)
}

func ParseRelationshipTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
