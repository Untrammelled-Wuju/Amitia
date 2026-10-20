package knowledge

import "time"

const (
	SourceStatusPending       = "pending"
	SourceStatusProcessing    = "processing"
	SourceStatusReady         = "ready"
	SourceStatusReadyDegraded = "ready_degraded"
	SourceStatusFailed        = "failed"
	SourceStatusCancelled     = "cancelled"
	SourceStatusDeleted       = "deleted"

	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
	JobStatusCancelled = "cancelled"

	JobKindIngest   = "ingest"
	JobKindReindex  = "reindex"
	JobKindRollback = "rollback"

	RevisionStatusPending       = "pending"
	RevisionStatusProcessing    = "processing"
	RevisionStatusReady         = "ready"
	RevisionStatusReadyDegraded = "ready_degraded"
	RevisionStatusFailed        = "failed"
	RevisionStatusCancelled     = "cancelled"
	RevisionStatusPurged        = "purged"

	ParserVersion       = "knowledge-parser-v1"
	ChunkSchemaVersion  = "knowledge-chunk-v1"
	DefaultIndexVersion = "v1"
)

const DefaultRevisionRetention = 7 * 24 * time.Hour

type Source struct {
	ID                 string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID           string     `gorm:"column:tenant_id;not null;index" json:"-"`
	LogicalName        string     `gorm:"column:logical_name;not null" json:"logicalName"`
	Filename           string     `gorm:"column:filename;not null" json:"filename"`
	MimeType           string     `gorm:"column:mime_type;not null" json:"mimeType"`
	SourceType         string     `gorm:"column:source_type;not null" json:"sourceType"`
	StorageKey         string     `gorm:"column:storage_key;not null" json:"-"`
	SizeBytes          int64      `gorm:"column:size_bytes;not null" json:"sizeBytes"`
	SHA256             string     `gorm:"column:sha256;not null;index" json:"sha256"`
	Revision           int64      `gorm:"column:revision;not null" json:"revision"`
	ParserVersion      string     `gorm:"column:parser_version;not null" json:"parserVersion"`
	ChunkSchemaVersion string     `gorm:"column:chunk_schema_version;not null" json:"chunkSchemaVersion"`
	ConfigHash         string     `gorm:"column:config_hash;not null" json:"configHash"`
	EmbeddingVersion   string     `gorm:"column:embedding_version;not null" json:"embeddingVersion"`
	IndexVersion       string     `gorm:"column:index_version;not null" json:"indexVersion"`
	Status             string     `gorm:"column:status;not null;index" json:"status"`
	ExpectedUnits      int64      `gorm:"column:expected_units;not null" json:"expectedUnits"`
	ProcessedUnits     int64      `gorm:"column:processed_units;not null" json:"processedUnits"`
	FailedUnits        int64      `gorm:"column:failed_units;not null" json:"failedUnits"`
	IndexedChunks      int64      `gorm:"column:indexed_chunks;not null" json:"indexedChunks"`
	LastError          string     `gorm:"column:last_error;not null" json:"lastError,omitempty"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	DeletedAt          *time.Time `gorm:"column:deleted_at" json:"deletedAt,omitempty"`
}

func (Source) TableName() string { return "knowledge_sources" }

type SourceRevision struct {
	ID                 string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID           string     `gorm:"column:tenant_id;not null;index" json:"-"`
	SourceID           string     `gorm:"column:source_id;not null;index" json:"sourceId"`
	Revision           int64      `gorm:"column:revision;not null" json:"revision"`
	Filename           string     `gorm:"column:filename;not null" json:"filename"`
	MimeType           string     `gorm:"column:mime_type;not null" json:"mimeType"`
	SourceType         string     `gorm:"column:source_type;not null" json:"sourceType"`
	StorageKey         string     `gorm:"column:storage_key;not null" json:"-"`
	SizeBytes          int64      `gorm:"column:size_bytes;not null" json:"sizeBytes"`
	SHA256             string     `gorm:"column:sha256;not null;index" json:"sha256"`
	ParserVersion      string     `gorm:"column:parser_version;not null" json:"parserVersion"`
	ChunkSchemaVersion string     `gorm:"column:chunk_schema_version;not null" json:"chunkSchemaVersion"`
	ConfigHash         string     `gorm:"column:config_hash;not null" json:"configHash"`
	EmbeddingVersion   string     `gorm:"column:embedding_version;not null" json:"embeddingVersion"`
	IndexVersion       string     `gorm:"column:index_version;not null" json:"indexVersion"`
	Status             string     `gorm:"column:status;not null;index" json:"status"`
	ExpectedUnits      int64      `gorm:"column:expected_units;not null" json:"expectedUnits"`
	ProcessedUnits     int64      `gorm:"column:processed_units;not null" json:"processedUnits"`
	FailedUnits        int64      `gorm:"column:failed_units;not null" json:"failedUnits"`
	IndexedChunks      int64      `gorm:"column:indexed_chunks;not null" json:"indexedChunks"`
	LastError          string     `gorm:"column:last_error;not null" json:"lastError,omitempty"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	ActivatedAt        *time.Time `gorm:"column:activated_at" json:"activatedAt,omitempty"`
	RetainUntil        *time.Time `gorm:"column:retain_until" json:"retainUntil,omitempty"`
	PurgedAt           *time.Time `gorm:"column:purged_at" json:"purgedAt,omitempty"`
	IsActive           bool       `gorm:"-" json:"isActive"`
	CanRollback        bool       `gorm:"-" json:"canRollback"`
	CanPurge           bool       `gorm:"-" json:"canPurge"`
}

func (SourceRevision) TableName() string { return "knowledge_source_revisions" }

type DocumentUnit struct {
	ID             string         `gorm:"column:id;primaryKey" json:"id"`
	TenantID       string         `gorm:"column:tenant_id;not null;index" json:"-"`
	SourceID       string         `gorm:"column:source_id;not null;index" json:"sourceId"`
	SourceRevision int64          `gorm:"column:source_revision;not null" json:"sourceRevision"`
	Ordinal        int64          `gorm:"column:ordinal;not null" json:"ordinal"`
	UnitType       string         `gorm:"column:unit_type;not null" json:"unitType"`
	Title          string         `gorm:"column:title;not null" json:"title"`
	Content        string         `gorm:"column:content;not null" json:"content"`
	LocatorJSON    string         `gorm:"column:locator_json;not null" json:"locatorJson"`
	MetadataJSON   string         `gorm:"column:metadata_json;not null" json:"metadataJson"`
	Checksum       string         `gorm:"column:checksum;not null" json:"checksum"`
	ValidFrom      *time.Time     `gorm:"column:valid_from" json:"validFrom,omitempty"`
	ValidTo        *time.Time     `gorm:"column:valid_to" json:"validTo,omitempty"`
	SupersedesJSON string         `gorm:"column:supersedes_json;not null" json:"supersedesJson,omitempty"`
	ConflictGroup  string         `gorm:"column:conflict_group;not null;index" json:"conflictGroup,omitempty"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null" json:"createdAt"`
	Locator        map[string]any `gorm:"-" json:"locator,omitempty"`
	Metadata       map[string]any `gorm:"-" json:"metadata,omitempty"`
}

func (DocumentUnit) TableName() string { return "knowledge_units" }

type Chunk struct {
	ID               string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID         string     `gorm:"column:tenant_id;not null;index" json:"-"`
	SourceID         string     `gorm:"column:source_id;not null;index" json:"sourceId"`
	SourceRevision   int64      `gorm:"column:source_revision;not null" json:"sourceRevision"`
	UnitID           string     `gorm:"column:unit_id;not null;index" json:"unitId"`
	ChunkIndex       int        `gorm:"column:chunk_index;not null" json:"chunkIndex"`
	ParentChunkID    string     `gorm:"column:parent_chunk_id;not null" json:"parentChunkId,omitempty"`
	Title            string     `gorm:"column:title;not null" json:"title"`
	Content          string     `gorm:"column:content;not null" json:"content"`
	EmbeddingText    string     `gorm:"column:embedding_text;not null" json:"-"`
	TokenCount       int        `gorm:"column:token_count;not null" json:"tokenCount"`
	LocatorJSON      string     `gorm:"column:locator_json;not null" json:"locatorJson"`
	MetadataJSON     string     `gorm:"column:metadata_json;not null" json:"metadataJson"`
	Checksum         string     `gorm:"column:checksum;not null" json:"checksum"`
	EmbeddingVersion string     `gorm:"column:embedding_version;not null" json:"embeddingVersion"`
	IndexVersion     string     `gorm:"column:index_version;not null" json:"indexVersion"`
	Status           string     `gorm:"column:status;not null" json:"status"`
	ValidFrom        *time.Time `gorm:"column:valid_from" json:"validFrom,omitempty"`
	ValidTo          *time.Time `gorm:"column:valid_to" json:"validTo,omitempty"`
	SupersedesJSON   string     `gorm:"column:supersedes_json;not null" json:"supersedesJson,omitempty"`
	ConflictGroup    string     `gorm:"column:conflict_group;not null;index" json:"conflictGroup,omitempty"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (Chunk) TableName() string { return "knowledge_chunks" }

type IngestJob struct {
	ID                   string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID             string     `gorm:"column:tenant_id;not null;index" json:"-"`
	SourceID             string     `gorm:"column:source_id;not null;index" json:"sourceId"`
	SourceRevision       int64      `gorm:"column:source_revision;not null" json:"sourceRevision"`
	Kind                 string     `gorm:"column:kind;not null" json:"kind"`
	Stage                string     `gorm:"column:stage;not null" json:"stage"`
	Status               string     `gorm:"column:status;not null;index" json:"status"`
	CursorJSON           string     `gorm:"column:cursor_json;not null" json:"cursorJson"`
	LastCommittedOrdinal int64      `gorm:"column:last_committed_ordinal;not null" json:"lastCommittedOrdinal"`
	ExpectedUnits        int64      `gorm:"column:expected_units;not null" json:"expectedUnits"`
	ProcessedUnits       int64      `gorm:"column:processed_units;not null" json:"processedUnits"`
	FailedUnits          int64      `gorm:"column:failed_units;not null" json:"failedUnits"`
	IndexedChunks        int64      `gorm:"column:indexed_chunks;not null" json:"indexedChunks"`
	RetryCount           int        `gorm:"column:retry_count;not null" json:"retryCount"`
	HeartbeatAt          *time.Time `gorm:"column:heartbeat_at" json:"heartbeatAt,omitempty"`
	LastError            string     `gorm:"column:last_error;not null" json:"lastError,omitempty"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	FinishedAt           *time.Time `gorm:"column:finished_at" json:"finishedAt,omitempty"`
}

func (IngestJob) TableName() string { return "knowledge_ingest_jobs" }

type IndexOutbox struct {
	ID             string     `gorm:"column:id;primaryKey"`
	TenantID       string     `gorm:"column:tenant_id;not null"`
	AggregateType  string     `gorm:"column:aggregate_type;not null"`
	AggregateID    string     `gorm:"column:aggregate_id;not null"`
	SourceRevision int64      `gorm:"column:source_revision;not null;index"`
	Operation      string     `gorm:"column:operation;not null"`
	PayloadJSON    string     `gorm:"column:payload_json;not null"`
	Status         string     `gorm:"column:status;not null"`
	RetryCount     int        `gorm:"column:retry_count;not null"`
	LastError      string     `gorm:"column:last_error;not null"`
	NextAttemptAt  *time.Time `gorm:"column:next_attempt_at"`
	LeaseToken     string     `gorm:"column:lease_token;not null"`
	LeaseUntil     *time.Time `gorm:"column:lease_until"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
}

func (IndexOutbox) TableName() string { return "knowledge_index_outbox" }

type IndexAlias struct {
	TenantID        string    `gorm:"column:tenant_id;primaryKey" json:"-"`
	ActiveVersion   string    `gorm:"column:active_version;not null" json:"activeVersion"`
	PreviousVersion string    `gorm:"column:previous_version;not null" json:"previousVersion"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (IndexAlias) TableName() string { return "knowledge_index_aliases" }

type Evidence struct {
	ID             string         `json:"id"`
	UnitID         string         `json:"unitId,omitempty"`
	EvidenceKind   string         `json:"evidenceKind,omitempty"`
	CoverageKeys   []string       `json:"coverageKeys,omitempty"`
	Checksum       string         `json:"checksum,omitempty"`
	SourceType     string         `json:"sourceType"`
	SourceID       string         `json:"sourceId"`
	SourceRevision int64          `json:"sourceRevision"`
	Title          string         `json:"title"`
	Content        string         `json:"content"`
	Locator        map[string]any `json:"locator"`
	DenseScore     float64        `json:"denseScore"`
	SparseScore    float64        `json:"sparseScore"`
	FusionScore    float64        `json:"fusionScore"`
	RerankScore    float64        `json:"rerankScore"`
	Authority      float64        `json:"authority"`
	Confidence     float64        `json:"confidence"`
	Trust          string         `json:"trust"`
	ValidFrom      *time.Time     `json:"validFrom,omitempty"`
	ValidTo        *time.Time     `json:"validTo,omitempty"`
	Supersedes     []string       `json:"supersedes,omitempty"`
	ConflictGroup  string         `json:"conflictGroup,omitempty"`
	Metadata       map[string]any `json:"metadata"`
}

// ReadRequest asks the canonical reader for already-parsed source units. It never
// accepts a filesystem path: callers must identify evidence, a unit, or a
// source/revision + structured locator inside the authenticated tenant.
type UnitOutline struct {
	ID             string         `json:"id"`
	SourceID       string         `json:"sourceId"`
	SourceRevision int64          `json:"sourceRevision"`
	Ordinal        int64          `json:"ordinal"`
	UnitType       string         `json:"unitType"`
	Title          string         `json:"title"`
	Locator        map[string]any `json:"locator,omitempty"`
	Checksum       string         `json:"checksum,omitempty"`
}

type UnitOutlineResponse struct {
	SourceID         string        `json:"sourceId"`
	SourceRevision   int64         `json:"sourceRevision"`
	Items            []UnitOutline `json:"items"`
	NextAfterOrdinal *int64        `json:"nextAfterOrdinal,omitempty"`
}

type ReadRequest struct {
	EvidenceID     string         `json:"evidenceId,omitempty"`
	UnitID         string         `json:"unitId,omitempty"`
	SourceID       string         `json:"sourceId,omitempty"`
	SourceRevision int64          `json:"sourceRevision,omitempty"`
	Locator        map[string]any `json:"locator,omitempty"`
	BeforeUnits    int            `json:"beforeUnits,omitempty"`
	AfterUnits     int            `json:"afterUnits,omitempty"`
	MaxUnits       int            `json:"maxUnits,omitempty"`
	MaxRunes       int            `json:"maxRunes,omitempty"`
}

type ReadUnit struct {
	ID             string         `json:"id"`
	SourceID       string         `json:"sourceId"`
	SourceRevision int64          `json:"sourceRevision"`
	Ordinal        int64          `json:"ordinal"`
	UnitType       string         `json:"unitType"`
	Title          string         `json:"title"`
	Content        string         `json:"content"`
	Locator        map[string]any `json:"locator,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Checksum       string         `json:"checksum,omitempty"`
	Truncated      bool           `json:"truncated,omitempty"`
}

type ReadResponse struct {
	SourceID       string     `json:"sourceId"`
	SourceRevision int64      `json:"sourceRevision"`
	AnchorUnitID   string     `json:"anchorUnitId,omitempty"`
	Units          []ReadUnit `json:"units"`
	TotalRunes     int        `json:"totalRunes"`
	Truncated      bool       `json:"truncated"`
}

type SearchRequest struct {
	Query           string     `json:"query" binding:"required"`
	ContextualQuery string     `json:"contextualQuery,omitempty"`
	RetrievalLevel  string     `json:"retrievalLevel,omitempty"`
	Limit           int        `json:"limit"`
	SourceIDs       []string   `json:"sourceIds"`
	DenseWeight     float64    `json:"denseWeight"`
	SparseWeight    float64    `json:"sparseWeight"`
	MinScore        float64    `json:"minScore"`
	TemporalMode    string     `json:"temporalMode,omitempty"`
	AsOf            *time.Time `json:"asOf,omitempty"`
	SkipTrace       bool       `json:"-"`
	PolicyMode      string     `json:"-"`
	RouteReason     string     `json:"-"`
	ShadowLevel     string     `json:"-"`
	ShadowReason    string     `json:"-"`
}

type SearchResponse struct {
	Evidence          []Evidence `json:"evidence"`
	Sufficient        bool       `json:"sufficient"`
	AnswerState       string     `json:"answerState"`
	SufficiencyReason string     `json:"sufficiencyReason"`
	Query             string     `json:"query"`
	OriginalQuery     string     `json:"originalQuery"`
	EffectiveQuery    string     `json:"effectiveQuery"`
	QueryComplexity   string     `json:"queryComplexity"`
	RetrievalLevel    string     `json:"retrievalLevel"`
	QueryHistory      []string   `json:"queryHistory,omitempty"`
	Degraded          bool       `json:"degraded"`
	TemporalMode      string     `json:"temporalMode"`
	AsOf              *time.Time `json:"asOf,omitempty"`
	Ambiguous         bool       `json:"ambiguous"`
	ConflictGroups    []string   `json:"conflictGroups,omitempty"`
	Warnings          []string   `json:"warnings,omitempty"`

	TraceConfigHash            string `json:"-"`
	TraceBudgetMs              int64  `json:"-"`
	TraceCandidateLimit        int    `json:"-"`
	TraceParentExpansionTokens int    `json:"-"`
	TraceRerankCandidateLimit  int    `json:"-"`
	TraceMaxAgenticRounds      int    `json:"-"`
	TraceSLOTargetMs           int64  `json:"-"`
	TracePolicyMode            string `json:"-"`
	TraceRouteReason           string `json:"-"`
	TraceShadowLevel           string `json:"-"`
	TraceShadowReason          string `json:"-"`
}

// RetrievalTraceSummary is a privacy-safe developer view of a retrieval run.
// Raw user queries, document content and locator values are intentionally never exposed.
type RetrievalTraceSummary struct {
	ID             string                    `json:"id"`
	RequestID      string                    `json:"requestId"`
	QueryHash      string                    `json:"queryHash"`
	CandidateCount int                       `json:"candidateCount"`
	SelectedCount  int                       `json:"selectedCount"`
	Sufficient     bool                      `json:"sufficient"`
	DurationMs     int64                     `json:"durationMs"`
	ErrorCode      string                    `json:"errorCode,omitempty"`
	TemporalMode   string                    `json:"temporalMode"`
	AsOf           *time.Time                `json:"asOf,omitempty"`
	Degraded       bool                      `json:"degraded"`
	Ambiguous      bool                      `json:"ambiguous"`
	Warnings       []string                  `json:"warnings,omitempty"`
	Candidates     []RetrievalCandidateTrace `json:"candidates,omitempty"`
	Evidence       []RetrievalEvidenceTrace  `json:"evidence,omitempty"`
	QueryPlan      RetrievalQueryPlanTrace   `json:"queryPlan"`
	ConfigHash     string                    `json:"configHash"`
	SLOTargetMs    int64                     `json:"sloTargetMs"`
	SLOExceeded    bool                      `json:"sloExceeded"`
	PolicyMode     string                    `json:"policyMode,omitempty"`
	RouteReason    string                    `json:"routeReason,omitempty"`
	ShadowLevel    string                    `json:"shadowLevel,omitempty"`
	ShadowReason   string                    `json:"shadowReason,omitempty"`
	CreatedAt      time.Time                 `json:"createdAt"`
}

type RetrievalCandidateTrace struct {
	Rank           int        `json:"rank"`
	MatchedChunkID string     `json:"matchedChunkId"`
	EvidenceID     string     `json:"evidenceId"`
	SourceID       string     `json:"sourceId"`
	SourceRevision int64      `json:"sourceRevision"`
	DenseScore     float64    `json:"denseScore"`
	SparseScore    float64    `json:"sparseScore"`
	ExactScore     float64    `json:"exactScore"`
	FusionScore    float64    `json:"fusionScore"`
	Relevance      float64    `json:"relevance"`
	RerankScore    float64    `json:"rerankScore"`
	MMRScore       float64    `json:"mmrScore"`
	ConflictGroup  string     `json:"conflictGroup,omitempty"`
	ValidFrom      *time.Time `json:"validFrom,omitempty"`
	ValidTo        *time.Time `json:"validTo,omitempty"`
	LocatorHash    string     `json:"locatorHash,omitempty"`
}

type RetrievalQueryPlanTrace struct {
	Level                 string   `json:"level"`
	Complexity            string   `json:"complexity"`
	OriginalHash          string   `json:"originalHash"`
	EffectiveHash         string   `json:"effectiveHash"`
	QueryHistory          []string `json:"queryHistoryHashes"`
	QueryCount            int      `json:"queryCount"`
	TimeoutMs             int64    `json:"timeoutMs"`
	CandidateLimit        int      `json:"candidateLimit"`
	ParentExpansionTokens int      `json:"parentExpansionTokens"`
	RerankCandidateLimit  int      `json:"rerankCandidateLimit"`
	MaxAgenticRounds      int      `json:"maxAgenticRounds"`
}

type RetrievalEvidenceTrace struct {
	Rank           int        `json:"rank"`
	EvidenceID     string     `json:"evidenceId"`
	SourceID       string     `json:"sourceId"`
	SourceRevision int64      `json:"sourceRevision"`
	DenseScore     float64    `json:"denseScore"`
	SparseScore    float64    `json:"sparseScore"`
	FusionScore    float64    `json:"fusionScore"`
	RerankScore    float64    `json:"rerankScore"`
	Confidence     float64    `json:"confidence"`
	Trust          string     `json:"trust"`
	ConflictGroup  string     `json:"conflictGroup,omitempty"`
	ValidFrom      *time.Time `json:"validFrom,omitempty"`
	ValidTo        *time.Time `json:"validTo,omitempty"`
	LocatorHash    string     `json:"locatorHash,omitempty"`
}

type RetrievalSLOLevelSummary struct {
	Level         string `json:"level"`
	Samples       int    `json:"samples"`
	ErrorCount    int    `json:"errorCount"`
	ExceededCount int    `json:"exceededCount"`
	P50Ms         int64  `json:"p50Ms"`
	P95Ms         int64  `json:"p95Ms"`
	TargetMs      int64  `json:"targetMs"`
	EnoughSamples bool   `json:"enoughSamples"`
	TargetMet     bool   `json:"targetMet"`
}

type RetrievalSLOSummary struct {
	ConfigHash    string                     `json:"configHash"`
	WindowMinutes int                        `json:"windowMinutes"`
	MinSamples    int                        `json:"minSamples"`
	From          time.Time                  `json:"from"`
	To            time.Time                  `json:"to"`
	Levels        []RetrievalSLOLevelSummary `json:"levels"`
}

type RuntimeStatus struct {
	ConfigHash                   string `json:"configHash"`
	RetrievalPolicyEnabled       bool   `json:"retrievalPolicyEnabled"`
	RetrievalPolicyShadow        bool   `json:"retrievalPolicyShadow"`
	CanonicalReaderEnabled       bool   `json:"canonicalReaderEnabled"`
	AgenticInvestigationEnabled  bool   `json:"agenticInvestigationEnabled"`
	ReaderWorkers                int    `json:"readerWorkers"`
	RetrievalTraceRetentionHours int64  `json:"retrievalTraceRetentionHours"`
	BenchmarkRetentionHours      int64  `json:"benchmarkRetentionHours"`
	DiagnosticCleanupBatchSize   int    `json:"diagnosticCleanupBatchSize"`
}

// BenchmarkRelevantEvidence is a deterministic human-labelled relevance target.
// Either EvidenceID or SourceID must be provided. Grade is clamped to [1, 3].
type BenchmarkRelevantEvidence struct {
	EvidenceID     string `json:"evidenceId,omitempty"`
	SourceID       string `json:"sourceId,omitempty"`
	SourceRevision int64  `json:"sourceRevision,omitempty"`
	Grade          int    `json:"grade,omitempty"`
}

type BenchmarkCaseExpectation struct {
	ExpectedRetrievalLevel string   `json:"expectedRetrievalLevel,omitempty"`
	ExpectedAnswerState    string   `json:"expectedAnswerState,omitempty"`
	MinRecallAtK           *float64 `json:"minRecallAtK,omitempty"`
	MinContextRecall       *float64 `json:"minContextRecall,omitempty"`
	MaxLatencyMs           int64    `json:"maxLatencyMs,omitempty"`
}

type BenchmarkGate struct {
	MinRecallAtK        *float64 `json:"minRecallAtK,omitempty"`
	MinMRR              *float64 `json:"minMrr,omitempty"`
	MinNDCGAtK          *float64 `json:"minNdcgAtK,omitempty"`
	MinContextRecall    *float64 `json:"minContextRecall,omitempty"`
	MinNoAnswerAccuracy *float64 `json:"minNoAnswerAccuracy,omitempty"`
	MaxP95LatencyMs     int64    `json:"maxP95LatencyMs,omitempty"`
	MaxErrorRate        *float64 `json:"maxErrorRate,omitempty"`
}

type BenchmarkCase struct {
	ID                  string                      `json:"id"`
	Scenario            string                      `json:"scenario,omitempty"`
	Query               string                      `json:"query"`
	ContextualQuery     string                      `json:"contextualQuery,omitempty"`
	RetrievalLevel      string                      `json:"retrievalLevel,omitempty"`
	Limit               int                         `json:"limit,omitempty"`
	SourceIDs           []string                    `json:"sourceIds,omitempty"`
	TemporalMode        string                      `json:"temporalMode,omitempty"`
	AsOf                *time.Time                  `json:"asOf,omitempty"`
	ExpectedNoAnswer    bool                        `json:"expectedNoAnswer"`
	Relevant            []BenchmarkRelevantEvidence `json:"relevant,omitempty"`
	CitationEvidenceIDs []string                    `json:"citationEvidenceIds,omitempty"`
	Expect              BenchmarkCaseExpectation    `json:"expect,omitempty"`
}

type BenchmarkRequest struct {
	K     int             `json:"k,omitempty"`
	Gate  BenchmarkGate   `json:"gate,omitempty"`
	Cases []BenchmarkCase `json:"cases" binding:"required"`
}

type BenchmarkCaseResult struct {
	ID                string   `json:"id"`
	Scenario          string   `json:"scenario"`
	RetrievalLevel    string   `json:"retrievalLevel"`
	AnswerState       string   `json:"answerState"`
	LatencyMs         int64    `json:"latencyMs"`
	RecallAtK         float64  `json:"recallAtK"`
	MRR               float64  `json:"mrr"`
	NDCGAtK           float64  `json:"ndcgAtK"`
	ContextPrecision  float64  `json:"contextPrecision"`
	ContextRecall     float64  `json:"contextRecall"`
	NoAnswerCorrect   bool     `json:"noAnswerCorrect"`
	CitationPrecision *float64 `json:"citationPrecision,omitempty"`
	CitationRecall    *float64 `json:"citationRecall,omitempty"`
	Groundedness      *float64 `json:"groundedness,omitempty"`
	EvidenceIDs       []string `json:"evidenceIds"`
	Passed            bool     `json:"passed"`
	Violations        []string `json:"violations,omitempty"`
	Error             string   `json:"error,omitempty"`
}

type BenchmarkSummary struct {
	ConfigHash            string         `json:"configHash"`
	K                     int            `json:"k"`
	Cases                 int            `json:"cases"`
	SuccessfulCases       int            `json:"successfulCases"`
	ErrorCases            int            `json:"errorCases"`
	FailedCases           int            `json:"failedCases"`
	PositiveCases         int            `json:"positiveCases"`
	NoAnswerCases         int            `json:"noAnswerCases"`
	CitationObservedCases int            `json:"citationObservedCases"`
	ScenarioCounts        map[string]int `json:"scenarioCounts"`
	RecallAtK             float64        `json:"recallAtK"`
	MRR                   float64        `json:"mrr"`
	NDCGAtK               float64        `json:"ndcgAtK"`
	ContextPrecision      float64        `json:"contextPrecision"`
	ContextRecall         float64        `json:"contextRecall"`
	NoAnswerAccuracy      float64        `json:"noAnswerAccuracy"`
	CitationPrecision     float64        `json:"citationPrecision"`
	CitationRecall        float64        `json:"citationRecall"`
	Groundedness          float64        `json:"groundedness"`
	GroundednessMode      string         `json:"groundednessMode"`
	ErrorRate             float64        `json:"errorRate"`
	P50LatencyMs          int64          `json:"p50LatencyMs"`
	P95LatencyMs          int64          `json:"p95LatencyMs"`
	Passed                bool           `json:"passed"`
	Violations            []string       `json:"violations,omitempty"`
}

type BenchmarkResponse struct {
	Summary BenchmarkSummary      `json:"summary"`
	Results []BenchmarkCaseResult `json:"results"`
}

// BenchmarkRunRecord stores only aggregate benchmark telemetry. Queries,
// evidence content, locators and source names are deliberately excluded so
// performance history cannot become a second copy of user knowledge.
type BenchmarkRunRecord struct {
	ID                    string    `gorm:"column:id;primaryKey" json:"-"`
	TenantID              string    `gorm:"column:tenant_id;index" json:"-"`
	ConfigHash            string    `gorm:"column:config_hash" json:"-"`
	K                     int       `gorm:"column:k" json:"-"`
	Cases                 int       `gorm:"column:cases" json:"-"`
	SuccessfulCases       int       `gorm:"column:successful_cases" json:"-"`
	ErrorCases            int       `gorm:"column:error_cases" json:"-"`
	FailedCases           int       `gorm:"column:failed_cases" json:"-"`
	PositiveCases         int       `gorm:"column:positive_cases" json:"-"`
	NoAnswerCases         int       `gorm:"column:no_answer_cases" json:"-"`
	CitationObservedCases int       `gorm:"column:citation_observed_cases" json:"-"`
	ScenarioCountsJSON    string    `gorm:"column:scenario_counts_json" json:"-"`
	RecallAtK             float64   `gorm:"column:recall_at_k" json:"-"`
	MRR                   float64   `gorm:"column:mrr" json:"-"`
	NDCGAtK               float64   `gorm:"column:ndcg_at_k" json:"-"`
	ContextPrecision      float64   `gorm:"column:context_precision" json:"-"`
	ContextRecall         float64   `gorm:"column:context_recall" json:"-"`
	NoAnswerAccuracy      float64   `gorm:"column:no_answer_accuracy" json:"-"`
	CitationPrecision     float64   `gorm:"column:citation_precision" json:"-"`
	CitationRecall        float64   `gorm:"column:citation_recall" json:"-"`
	Groundedness          float64   `gorm:"column:groundedness" json:"-"`
	ErrorRate             float64   `gorm:"column:error_rate" json:"-"`
	P50LatencyMs          int64     `gorm:"column:p50_latency_ms" json:"-"`
	P95LatencyMs          int64     `gorm:"column:p95_latency_ms" json:"-"`
	Passed                bool      `gorm:"column:passed" json:"-"`
	ViolationsJSON        string    `gorm:"column:violations_json" json:"-"`
	CreatedAt             time.Time `gorm:"column:created_at" json:"-"`
}

func (BenchmarkRunRecord) TableName() string { return "knowledge_benchmark_runs" }

type BenchmarkRunSummary struct {
	ID                    string         `json:"id"`
	ConfigHash            string         `json:"configHash"`
	K                     int            `json:"k"`
	Cases                 int            `json:"cases"`
	SuccessfulCases       int            `json:"successfulCases"`
	ErrorCases            int            `json:"errorCases"`
	FailedCases           int            `json:"failedCases"`
	PositiveCases         int            `json:"positiveCases"`
	NoAnswerCases         int            `json:"noAnswerCases"`
	CitationObservedCases int            `json:"citationObservedCases"`
	ScenarioCounts        map[string]int `json:"scenarioCounts"`
	RecallAtK             float64        `json:"recallAtK"`
	MRR                   float64        `json:"mrr"`
	NDCGAtK               float64        `json:"ndcgAtK"`
	ContextPrecision      float64        `json:"contextPrecision"`
	ContextRecall         float64        `json:"contextRecall"`
	NoAnswerAccuracy      float64        `json:"noAnswerAccuracy"`
	CitationPrecision     float64        `json:"citationPrecision"`
	CitationRecall        float64        `json:"citationRecall"`
	Groundedness          float64        `json:"groundedness"`
	ErrorRate             float64        `json:"errorRate"`
	P50LatencyMs          int64          `json:"p50LatencyMs"`
	P95LatencyMs          int64          `json:"p95LatencyMs"`
	Passed                bool           `json:"passed"`
	Violations            []string       `json:"violations,omitempty"`
	CreatedAt             time.Time      `json:"createdAt"`
}

type SourceDetail struct {
	Source       Source           `json:"source"`
	Revisions    []SourceRevision `json:"revisions"`
	LatestJob    *IngestJob       `json:"latestJob,omitempty"`
	UnitCount    int64            `json:"unitCount"`
	ChunkCount   int64            `json:"chunkCount"`
	IndexHealth  string           `json:"indexHealth"`
	HasActiveJob bool             `json:"hasActiveJob"`
}
