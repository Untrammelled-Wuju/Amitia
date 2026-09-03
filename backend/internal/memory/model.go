// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

type Memory struct {
	ID                    string  `gorm:"column:id;primaryKey" json:"id"`
	CharacterID           string  `gorm:"column:character_id" json:"characterId"`
	MemoryType            string  `gorm:"column:memory_type;default:custom" json:"memoryType"`
	MemorySubtype         string  `gorm:"column:memory_subtype;default:''" json:"memorySubtype"`
	Source                string  `gorm:"column:source;default:manual" json:"source"`
	Scope                 string  `gorm:"column:scope;default:character" json:"scope"`
	Key                   string  `gorm:"column:key;not null" json:"key"`
	Value                 string  `gorm:"column:value;not null" json:"value"`
	Importance            int     `gorm:"column:importance;default:0" json:"importance"`
	Confidence            int     `gorm:"column:confidence;default:50" json:"confidence"`
	ExpiresAt             *string `gorm:"column:expires_at" json:"expiresAt"`
	EntityID              string  `gorm:"column:entity_id" json:"entityId"`
	EntityType            string  `gorm:"column:entity_type" json:"entityType"`
	SourceMsgID           string  `gorm:"column:source_msg_id" json:"sourceMsgId"`
	SourceConvID          string  `gorm:"column:source_conv_id" json:"sourceConvId"`
	VerifiedStatus        string  `gorm:"column:verified_status;default:unverified" json:"verifiedStatus"`
	LastVerifiedAt        *string `gorm:"column:last_verified_at" json:"lastVerifiedAt"`
	UseCount              int     `gorm:"column:use_count;default:0" json:"useCount"`
	LastUsedAt            *string `gorm:"column:last_used_at" json:"lastUsedAt"`
	RetentionLevel        int     `gorm:"column:retention_level;default:3" json:"retentionLevel"`
	MemoryStrength        float64 `gorm:"column:memory_strength;default:0.68" json:"memoryStrength"`
	StrengthUpdatedAt     *string `gorm:"column:strength_updated_at" json:"strengthUpdatedAt"`
	LastReinforcedAt      *string `gorm:"column:last_reinforced_at" json:"lastReinforcedAt"`
	ReinforceCount        int     `gorm:"column:reinforce_count;default:0" json:"reinforceCount"`
	RetrievedCount        int     `gorm:"column:retrieved_count;default:0" json:"retrievedCount"`
	InjectedCount         int     `gorm:"column:injected_count;default:0" json:"injectedCount"`
	DecayState            string  `gorm:"column:decay_state;default:active" json:"decayState"`
	Pinned                bool    `gorm:"column:pinned;default:0" json:"pinned"`
	ArchivedAt            *string `gorm:"column:archived_at" json:"archivedAt"`
	SupersededBy          string  `gorm:"column:superseded_by;default:''" json:"supersededBy"`
	SensitivityLevel      string  `gorm:"column:sensitivity_level;default:internal" json:"sensitivityLevel"`
	AllowProactiveMention bool    `gorm:"column:allow_proactive_mention;default:1" json:"allowProactiveMention"`
	RequiresConfirmation  bool    `gorm:"column:requires_confirmation;default:0" json:"requiresConfirmation"`
	Version               int     `gorm:"column:version;default:1" json:"version"`
	DerivationKey         string  `gorm:"column:derivation_key;default:''" json:"derivationKey"`
	CreatedAt             string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt             string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Memory) TableName() string { return "memories" }

type CreateMemoryRequest struct {
	CharacterID           string  `json:"characterId"`
	MemoryType            string  `json:"memoryType"`
	MemorySubtype         string  `json:"memorySubtype"`
	Key                   string  `json:"key" binding:"required"`
	Value                 string  `json:"value" binding:"required"`
	Importance            int     `json:"importance"`
	Confidence            int     `json:"confidence"`
	ExpiresAt             string  `json:"expiresAt"`
	EntityID              string  `json:"entityId"`
	EntityType            string  `json:"entityType"`
	SourceMsgID           string  `json:"sourceMsgId"`
	SourceConvID          string  `json:"sourceConvId"`
	VerifiedStatus        string  `json:"verifiedStatus"`
	Source                string  `json:"source"`
	SensitivityLevel      string  `json:"sensitivityLevel"`
	AllowProactiveMention bool    `json:"allowProactiveMention"`
	RequiresConfirmation  bool    `json:"requiresConfirmation"`
	Scope                 string  `json:"scope"`
	RetentionLevel        int     `json:"retentionLevel"`
	MemoryStrength        float64 `json:"memoryStrength"`
	Pinned                bool    `json:"pinned"`
}

type UpdateMemoryRequest struct {
	Key                   *string  `json:"key"`
	Value                 *string  `json:"value"`
	MemoryType            *string  `json:"memoryType"`
	MemorySubtype         *string  `json:"memorySubtype"`
	CharacterID           *string  `json:"characterId"`
	Importance            *int     `json:"importance"`
	Confidence            *int     `json:"confidence"`
	ExpiresAt             *string  `json:"expiresAt"`
	EntityID              *string  `json:"entityId"`
	EntityType            *string  `json:"entityType"`
	VerifiedStatus        *string  `json:"verifiedStatus"`
	SensitivityLevel      *string  `json:"sensitivityLevel"`
	AllowProactiveMention *bool    `json:"allowProactiveMention"`
	RequiresConfirmation  *bool    `json:"requiresConfirmation"`
	Scope                 *string  `json:"scope"`
	RetentionLevel        *int     `json:"retentionLevel"`
	MemoryStrength        *float64 `json:"memoryStrength"`
	Pinned                *bool    `json:"pinned"`
}

type SearchMemoryRequest struct {
	Keyword          string            `json:"keyword"`
	CharacterID      string            `json:"characterId"`
	UserID           string            `json:"userId"`
	SensitivityLevel string            `json:"sensitivityLevel"`
	Limit            int               `json:"limit"`
	Layers           []MemoryLayer     `json:"layers"`
	Types            []string          `json:"types"`
	Time             *MemoryTimeFilter `json:"time"`
	Cursor           string            `json:"cursor"`
	Sort             MemorySort        `json:"sort"`
}

type MemoryTimeFilter struct {
	Basis          MemoryTimeBasis `json:"basis"`
	FromUTC        *string         `json:"from"`
	ToUTC          *string         `json:"to"`
	AtUTC          *string         `json:"at"`
	LocalDateFrom  string          `json:"localDateFrom"`
	LocalDateTo    string          `json:"localDateTo"`
	Dayparts       []string        `json:"dayparts"`
	Precisions     []string        `json:"precisions"`
	IncludeUnknown bool            `json:"includeUnknown"`
}

type MemorySort string

const (
	MemorySortRelevance    MemorySort = "relevance"
	MemorySortTimeDesc     MemorySort = "time_desc"
	MemorySortTimeAsc      MemorySort = "time_asc"
	MemorySortImportance   MemorySort = "importance"
	MemorySortRecentlyUsed MemorySort = "recently_used"
)

type VectorSearchRequest struct {
	Keyword          string `json:"keyword"`
	Query            string `json:"query"`
	CharacterID      string `json:"characterId"`
	UserID           string `json:"userId"`
	Limit            int    `json:"limit"`
	ConversationID   string `json:"conversationId"`
	RequestID        string `json:"requestId"`
	Channel          string `json:"channel"`
	ProactiveMention bool   `json:"proactiveMention"`
}

type MemoryListQuery struct {
	Page           int    `form:"page"`
	PageSize       int    `form:"pageSize"`
	CharacterID    string `form:"characterId"`
	UserID         string `form:"userId"`
	Source         string `form:"source"`
	MemoryType     string `form:"memoryType"`
	Type           string `form:"type"`
	Keyword        string `form:"keyword"`
	SortBy         string `form:"sortBy"`
	Sort           string `form:"sort"`
	VerifiedStatus string `form:"verifiedStatus"`
	MinConfidence  int    `form:"minConfidence"`
	RetentionLevel int    `form:"retentionLevel"`
	DecayState     string `form:"decayState"`
	Pinned         *bool  `form:"pinned"`
	ScopeType      string `form:"scopeType"`
}

type MemoryListResponse struct {
	Items    []Memory `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

type MemoryCandidateModel struct {
	ID                    string  `gorm:"column:id;primaryKey" json:"id"`
	Key                   string  `gorm:"column:key" json:"key"`
	Value                 string  `gorm:"column:value" json:"value"`
	MemoryType            string  `gorm:"column:memory_type;default:custom" json:"memoryType"`
	MemorySubtype         string  `gorm:"column:memory_subtype;default:''" json:"memorySubtype"`
	Importance            int     `gorm:"column:importance;default:5" json:"importance"`
	RetentionLevel        int     `gorm:"column:retention_level;default:0" json:"retentionLevel"`
	MemoryStrength        float64 `gorm:"column:memory_strength;default:0" json:"memoryStrength"`
	StrengthUpdatedAt     *string `gorm:"column:strength_updated_at" json:"strengthUpdatedAt,omitempty"`
	LastReinforcedAt      *string `gorm:"column:last_reinforced_at" json:"lastReinforcedAt,omitempty"`
	ReinforceCount        int     `gorm:"column:reinforce_count;default:0" json:"reinforceCount"`
	DecayState            string  `gorm:"column:decay_state;default:''" json:"decayState"`
	Pinned                bool    `gorm:"column:pinned;default:0" json:"pinned"`
	ArchivedAt            *string `gorm:"column:archived_at" json:"archivedAt,omitempty"`
	Scope                 string  `gorm:"column:scope;default:character" json:"scope"`
	SensitivityLevel      string  `gorm:"column:sensitivity_level;default:internal" json:"sensitivityLevel"`
	AllowProactiveMention bool    `gorm:"column:allow_proactive_mention;default:1" json:"allowProactiveMention"`
	RequiresConfirmation  bool    `gorm:"column:requires_confirmation;default:0" json:"requiresConfirmation"`
	SourceText            string  `gorm:"column:source_text" json:"sourceText"`
	CharacterID           string  `gorm:"column:character_id" json:"characterId"`
	CreatedAt             string  `gorm:"column:created_at" json:"createdAt"`
	ConversationID        string  `gorm:"column:conversation_id" json:"conversationId"`
	CandidateKind         string  `gorm:"column:candidate_kind;default:'extracted'" json:"candidateKind"`
	ConfidenceReal        float64 `gorm:"column:confidence;default:0" json:"confidenceReal"`
	TargetMemoryID        string  `gorm:"column:target_memory_id;default:''" json:"targetMemoryId"`
	ProposedAction        string  `gorm:"column:proposed_action;default:''" json:"proposedAction"`
	SourceMemoryIDsJSON   string  `gorm:"column:source_memory_ids_json;default:''" json:"sourceMemoryIdsJson"`
	SourceVersionsJSON    string  `gorm:"column:source_versions_json;default:''" json:"sourceVersionsJson"`
	DerivationKey         string  `gorm:"column:derivation_key;default:''" json:"derivationKey"`
	Reason                string  `gorm:"column:reason;default:''" json:"reason"`
}

func (MemoryCandidateModel) TableName() string { return "memory_candidates" }

type MemoryDerivation struct {
	ID                string `gorm:"column:id;primaryKey" json:"id"`
	OutputMemoryID    string `gorm:"column:output_memory_id" json:"outputMemoryId"`
	InputMemoryID     string `gorm:"column:input_memory_id" json:"inputMemoryId"`
	InputVersion      int    `gorm:"column:input_version" json:"inputVersion"`
	InputSnapshotHash string `gorm:"column:input_snapshot_hash" json:"inputSnapshotHash"`
	DerivationKind    string `gorm:"column:derivation_kind" json:"derivationKind"`
	Ordinal           int    `gorm:"column:ordinal" json:"ordinal"`
	OperationID       string `gorm:"column:operation_id" json:"operationId"`
	CreatedAt         string `gorm:"column:created_at" json:"createdAt"`
}

func (MemoryDerivation) TableName() string { return "memory_derivations" }

type MemorySummaryMode string

const (
	MemorySummaryModeDeterministic MemorySummaryMode = "deterministic"
	MemorySummaryModeModel         MemorySummaryMode = "model"
	MemorySummaryModeAuto          MemorySummaryMode = "auto"
)

type MemoryEvidenceRef struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Layer string `json:"layer"`
}

type MemorySummaryRequest struct {
	CharacterID     string
	Topic           string
	Layers          []MemoryLayer
	Types           []string
	Time            *MemoryTimeFilter
	MinImportance   int
	MinConfidence   int
	Limit           int
	IncludeEvidence bool
	Mode            MemorySummaryMode
}

type MemorySummaryResult struct {
	Summary       string              `json:"summary"`
	EvidenceCount int                 `json:"evidenceCount"`
	Evidence      []MemoryEvidenceRef `json:"evidence,omitempty"`
	Topic         string              `json:"topic"`
	Layers        []MemoryLayer       `json:"layers,omitempty"`
	Types         []string            `json:"types,omitempty"`
	GeneratedBy   string              `json:"generatedBy"`
	Truncated     bool                `json:"truncated"`
	Warnings      []string            `json:"warnings,omitempty"`
}

type DerivationKind string

const (
	DerivationKindReinforce          DerivationKind = "reinforce"
	DerivationKindMerge              DerivationKind = "merge"
	DerivationKindSummary            DerivationKind = "summary"
	DerivationKindReflection         DerivationKind = "reflection"
	DerivationKindConflictResolution DerivationKind = "conflict_resolution"
)

type CandidateKind string

const (
	CandidateKindExtracted          CandidateKind = "extracted"
	CandidateKindConsolidated       CandidateKind = "consolidated"
	CandidateKindConflictResolution CandidateKind = "conflict_resolution"
	CandidateKindReflection         CandidateKind = "reflection"
)

type ProposedAction string

const (
	ProposedActionCreate    ProposedAction = "create"
	ProposedActionReinforce ProposedAction = "reinforce"
	ProposedActionMerge     ProposedAction = "merge"
	ProposedActionKeepOld   ProposedAction = "keep_old"
	ProposedActionKeepNew   ProposedAction = "keep_new"
	ProposedActionFlag      ProposedAction = "flag"
	ProposedActionNoop      ProposedAction = "noop"
)

type ConsolidationRequest struct {
	CharacterID     string
	Scope           string
	Source          string
	MaxOutputs      int
	PolicyVersion   string
	PromptVersion   string
	IncludeConflict bool
}

type ConsolidationCandidateProposal struct {
	CandidateKind   string   `json:"candidateKind"`
	Key             string   `json:"key"`
	Value           string   `json:"value"`
	MemoryType      string   `json:"memoryType"`
	MemorySubtype   string   `json:"memorySubtype"`
	Importance      int      `json:"importance"`
	Confidence      float64  `json:"confidence"`
	ProposedAction  string   `json:"proposedAction"`
	SourceMemoryIDs []string `json:"sourceMemoryIds"`
	SourceVersions  []int    `json:"sourceVersions"`
	DerivationKey   string   `json:"derivationKey"`
	TargetMemoryID  string   `json:"targetMemoryId"`
	Reason          string   `json:"reason"`
}

type ConsolidationResult struct {
	OperationID      string                           `json:"operationId"`
	Candidates       []ConsolidationCandidateProposal `json:"candidates"`
	CommittedCount   int                              `json:"committedCount"`
	SkippedDuplicate int                              `json:"skippedDuplicate"`
	RequiresReview   int                              `json:"requiresReview"`
	Errors           []string                         `json:"errors,omitempty"`
}

type MemoryRecordV1 struct {
	ID                    string  `json:"id"`
	Key                   string  `json:"key"`
	Value                 string  `json:"value"`
	MemoryLayer           string  `json:"memoryLayer"`
	MemoryType            string  `json:"memoryType"`
	MemorySubtype         string  `json:"memorySubtype,omitempty"`
	Importance            int     `json:"importance"`
	Confidence            int     `json:"confidence"`
	Source                string  `json:"source"`
	Scope                 string  `json:"scope"`
	CharacterID           string  `json:"characterId,omitempty"`
	EntityID              string  `json:"entityId,omitempty"`
	EntityType            string  `json:"entityType,omitempty"`
	SourceMessageID       string  `json:"sourceMessageId,omitempty"`
	SourceConversationID  string  `json:"sourceConversationId,omitempty"`
	VerifiedStatus        string  `json:"verifiedStatus"`
	LastVerifiedAt        *string `json:"lastVerifiedAt,omitempty"`
	ExpiresAt             *string `json:"expiresAt,omitempty"`
	SensitivityLevel      string  `json:"sensitivityLevel"`
	AllowProactiveMention bool    `json:"allowProactiveMention"`
	RequiresConfirmation  bool    `json:"requiresConfirmation"`
	Version               int     `json:"version"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
	UseCount              int     `json:"useCount,omitempty"`
	LastUsedAt            *string `json:"lastUsedAt,omitempty"`
	RetentionLevel        int     `json:"retentionLevel,omitempty"`
	MemoryStrength        float64 `json:"memoryStrength,omitempty"`
	StrengthUpdatedAt     *string `json:"strengthUpdatedAt,omitempty"`
	LastReinforcedAt      *string `json:"lastReinforcedAt,omitempty"`
	ReinforceCount        int     `json:"reinforceCount,omitempty"`
	RetrievedCount        int     `json:"retrievedCount,omitempty"`
	InjectedCount         int     `json:"injectedCount,omitempty"`
	DecayState            string  `json:"decayState,omitempty"`
	Pinned                bool    `json:"pinned,omitempty"`
	ArchivedAt            *string `json:"archivedAt,omitempty"`
	SupersededBy          string  `json:"supersededBy,omitempty"`
	DerivationKey         string  `json:"derivationKey,omitempty"`
}

type MemoryEventV1 struct {
	ID           string `json:"id"`
	MemoryID     string `json:"memoryId"`
	Version      int    `json:"version"`
	EventType    string `json:"eventType"`
	OperationID  string `json:"operationId"`
	SnapshotHash string `json:"snapshotHash"`
	EventReason  string `json:"eventReason"`
	CreatedAt    string `json:"createdAt"`
}

type MemoryTemporalV1 struct {
	MemoryID          string   `json:"memoryId"`
	OccurredAtUTC     *string  `json:"occurredAtUtc,omitempty"`
	EndedAtUTC        *string  `json:"endedAtUtc,omitempty"`
	Timezone          string   `json:"timezone,omitempty"`
	LocalDate         string   `json:"localDate,omitempty"`
	Daypart           string   `json:"daypart,omitempty"`
	TemporalPrecision string   `json:"temporalPrecision,omitempty"`
	ValidFromUTC      *string  `json:"validFromUtc,omitempty"`
	ValidToUTC        *string  `json:"validToUtc,omitempty"`
	AnchorIDs         []string `json:"anchorIds,omitempty"`
	SourceTimeText    string   `json:"sourceTimeText,omitempty"`
	CreatedAtUTC      string   `json:"createdAtUtc"`
	UpdatedAtUTC      string   `json:"updatedAtUtc"`
}

type MemoryDerivationV1 struct {
	ID                string `json:"id"`
	OutputMemoryID    string `json:"outputMemoryId"`
	InputMemoryID     string `json:"inputMemoryId"`
	InputVersion      int    `json:"inputVersion"`
	InputSnapshotHash string `json:"inputSnapshotHash"`
	DerivationKind    string `json:"derivationKind"`
	Ordinal           int    `json:"ordinal"`
	OperationID       string `json:"operationId"`
	CreatedAt         string `json:"createdAt"`
}

type MemoryExportDataset struct {
	Records     []MemoryRecordV1     `json:"records,omitempty"`
	Events      []MemoryEventV1      `json:"events,omitempty"`
	Temporal    []MemoryTemporalV1   `json:"temporal,omitempty"`
	Derivations []MemoryDerivationV1 `json:"derivations,omitempty"`
}

type ExportScope string

const (
	ExportScopeAll        ExportScope = "all_memory"
	ExportScopeCharacter  ExportScope = "character"
	ExportScopeUserGlobal ExportScope = "user_global"
	ExportScopeSelected   ExportScope = "selected"
)

type HistoryMode string

const (
	HistoryModeCurrentPlusEvents HistoryMode = "current_plus_events"
	HistoryModeCurrentOnly       HistoryMode = "current_only"
)

type MemoryExportRequest struct {
	Scope               ExportScope `json:"scope"`
	CharacterID         string      `json:"characterId,omitempty"`
	SelectedIDs         []string    `json:"selectedIds,omitempty"`
	HistoryMode         HistoryMode `json:"historyMode"`
	IncludeCandidates   bool        `json:"includeCandidates"`
	IncludeSourceMemory string      `json:"includeSourceMemory"`
}

type MemoryExportResult struct {
	OperationID     string `json:"operationId"`
	RecordCount     int    `json:"recordCount"`
	EventCount      int    `json:"eventCount"`
	TemporalCount   int    `json:"temporalCount"`
	DerivationCount int    `json:"derivationCount"`
	CandidateCount  int    `json:"candidateCount"`
	Scope           string `json:"scope"`
}

type MemoryImportRequest struct {
	ImportID          string   `json:"importId"`
	TargetCharacterID string   `json:"targetCharacterId"`
	ScopeMode         string   `json:"scopeMode"`
	CollisionPolicy   string   `json:"collisionPolicy"`
	HistoryPolicy     string   `json:"historyPolicy"`
	ProvenancePolicy  string   `json:"provenancePolicy"`
	IncludeCandidates bool     `json:"includeCandidates"`
	ResumeToken       string   `json:"resumeToken"`
	SourceDatasetIDs  []string `json:"sourceDatasetIds"`
}

type MemoryCollisionPolicy string

const (
	CollisionPolicyRemap      MemoryCollisionPolicy = "remap"
	CollisionPolicyKeepBoth   MemoryCollisionPolicy = "keep_both"
	CollisionPolicyReview     MemoryCollisionPolicy = "review"
	CollisionPolicyExactDedup MemoryCollisionPolicy = "exact_dedupe"
)

type MemoryPreviewResult struct {
	DatasetVersion           string         `json:"datasetVersion"`
	RecordCount              int            `json:"recordCount"`
	EventCount               int            `json:"eventCount"`
	TemporalCount            int            `json:"temporalCount"`
	DerivationCount          int            `json:"derivationCount"`
	CandidateCount           int            `json:"candidateCount"`
	CharacterScopes          []string       `json:"characterScopes"`
	TypeDistribution         map[string]int `json:"typeDistribution"`
	SensitivityDistribution  map[string]int `json:"sensitivityDistribution"`
	IDCollisions             int            `json:"idCollisions"`
	SemanticDuplicates       int            `json:"semanticDuplicates"`
	UnresolvedCharacters     []string       `json:"unresolvedCharacters"`
	UnresolvedSourceMessages []string       `json:"unresolvedSourceMessages"`
	UnresolvedAnchors        []string       `json:"unresolvedAnchors"`
	BrokenDerivations        []string       `json:"brokenDerivations"`
	CycleRiskCount           int            `json:"cycleRiskCount"`
	ReindexRequired          bool           `json:"reindexRequired"`
	EstimatedSize            int64          `json:"estimatedSize"`
	Warnings                 []string       `json:"warnings"`
}

type MemoryImportResult struct {
	OperationID         string `json:"operationId"`
	ImportedRecords     int    `json:"importedRecords"`
	ImportedEvents      int    `json:"importedEvents"`
	ImportedTemporal    int    `json:"importedTemporal"`
	ImportedDerivations int    `json:"importedDerivations"`
	Deduplicated        int    `json:"deduplicated"`
	IDRemapped          int    `json:"idRemapped"`
	Collisions          int    `json:"collisions"`
	Warnings            int    `json:"warnings"`
	ReindexState        string `json:"reindexState"`
	ResumeToken         string `json:"resumeToken,omitempty"`
}

type MemoryRecoveryCheck struct {
	RecordsValid          int64  `json:"recordsValid"`
	RecordsInvalid        int64  `json:"recordsInvalid"`
	TemporalValid         int64  `json:"temporalValid"`
	TemporalInvalid       int64  `json:"temporalInvalid"`
	HistoryIssues         int64  `json:"historyIssues"`
	ProvenanceIssues      int64  `json:"provenanceIssues"`
	VectorState           string `json:"vectorState"`
	GraphState            string `json:"graphState"`
	PendingReconciliation int64  `json:"pendingReconciliation"`
	Ready                 bool   `json:"ready"`
}
