package knowledge

import (
	"time"

	"github.com/u-ai/backend/config"
)

type runtimeSettings struct {
	ConfigHash string

	RetrievalPolicyEnabled      bool
	RetrievalPolicyShadow       bool
	CanonicalReaderEnabled      bool
	AgenticInvestigationEnabled bool

	ChunkTargetRunes      int
	ChunkOverlapRunes     int
	ParentMaxRunes        int
	MaxResultLimit        int
	FastTimeout           time.Duration
	FastMinCandidates     int
	FastMaxCandidates     int
	FastParentTokens      int
	DeepTimeout           time.Duration
	DeepMinCandidates     int
	DeepMaxCandidates     int
	DeepParentTokens      int
	AgenticTimeout        time.Duration
	AgenticMinCandidates  int
	AgenticMaxCandidates  int
	AgenticParentTokens   int
	MaxAgenticRounds      int
	MaxDenseCandidates    int
	RerankCandidateLimit  int
	RerankTimeout         time.Duration
	RerankMaxTitleRunes   int
	RerankMaxContentRunes int

	MaxSourceBytes            int64
	OnlineRetrievalWorkers    int
	ReaderWorkers             int
	IngestWorkers             int
	IndexWorkers              int
	OCRWorkers                int
	EmbeddingBatchSize        int
	QdrantBatchSize           int
	StreamBufferBytes         int
	UploadBufferBytes         int
	TextScannerMaxBytes       int
	TextUnitMaxBytes          int
	JSONUnitMaxBytes          int
	MaxPDFPages               int
	MaxOCRImageBytes          int64
	OCRRasterDPI              int
	PDFInfoTimeout            time.Duration
	PDFTextPageTimeout        time.Duration
	OCRRasterTimeout          time.Duration
	OCRTimeout                time.Duration
	LegacyDocTimeout          time.Duration
	MaxOOXMLFiles             int
	MaxOOXMLUncompressedBytes int64
	MaxSharedStrings          int
	MaxSharedStringBytes      int64
	MaxWorksheetRowCells      int
	MaxCellTextBytes          int

	MaintenanceInitialDelay      time.Duration
	MaintenanceInterval          time.Duration
	MaintenanceTenantPassTimeout time.Duration
	JobHeartbeatInterval         time.Duration
	JobStaleAfter                time.Duration
	OutboxLease                  time.Duration
	OutboxMaxRetries             int
	OutboxRetryBase              time.Duration
	OutboxRetryMax               time.Duration
	MaxOutboxesPerPass           int
	ManualMaxOutboxes            int
	MaxOutboxesPerJob            int
	RevisionRetention            time.Duration
	RetrievalTraceRetention      time.Duration
	BenchmarkRetention           time.Duration
	DiagnosticCleanupBatchSize   int

	FastSLOTarget    time.Duration
	DeepSLOTarget    time.Duration
	AgenticSLOTarget time.Duration
	SLOWindow        time.Duration
	SLOMinSamples    int
}

func defaultRuntimeSettings() runtimeSettings {
	return runtimeSettingsFromConfig(config.DefaultKnowledgeRuntimeConfig())
}

func loadRuntimeSettings() runtimeSettings {
	c := config.DefaultKnowledgeRuntimeConfig()
	if config.AppCfg != nil && !knowledgeRuntimeConfigUnset(config.AppCfg.Knowledge) {
		c = config.AppCfg.Knowledge
	}
	return runtimeSettingsFromConfig(c)
}

func runtimeSettingsFromConfig(c config.KnowledgeRuntimeConfig) runtimeSettings {
	return runtimeSettings{
		ConfigHash:             c.Hash(),
		RetrievalPolicyEnabled: c.Features.RetrievalPolicyEnabled, RetrievalPolicyShadow: c.Features.RetrievalPolicyShadow,
		CanonicalReaderEnabled: c.Features.CanonicalReaderEnabled, AgenticInvestigationEnabled: c.Features.AgenticInvestigationEnabled,
		ChunkTargetRunes: c.Chunk.TargetRunes, ChunkOverlapRunes: c.Chunk.OverlapRunes, ParentMaxRunes: c.Chunk.ParentMaxRunes,
		MaxResultLimit: c.Retrieval.MaxResultLimit,
		FastTimeout:    time.Duration(c.Retrieval.FastTimeoutMs) * time.Millisecond, FastMinCandidates: c.Retrieval.FastMinCandidates, FastMaxCandidates: c.Retrieval.FastMaxCandidates, FastParentTokens: c.Retrieval.FastParentExpansionTokens,
		DeepTimeout: time.Duration(c.Retrieval.DeepTimeoutMs) * time.Millisecond, DeepMinCandidates: c.Retrieval.DeepMinCandidates, DeepMaxCandidates: c.Retrieval.DeepMaxCandidates, DeepParentTokens: c.Retrieval.DeepParentExpansionTokens,
		AgenticTimeout: time.Duration(c.Retrieval.AgenticTimeoutMs) * time.Millisecond, AgenticMinCandidates: c.Retrieval.AgenticMinCandidates, AgenticMaxCandidates: c.Retrieval.AgenticMaxCandidates, AgenticParentTokens: c.Retrieval.AgenticParentExpansionTokens, MaxAgenticRounds: c.Retrieval.MaxAgenticRounds,
		MaxDenseCandidates:   c.Retrieval.MaxDenseCandidates,
		RerankCandidateLimit: c.Rerank.CandidateLimit, RerankTimeout: time.Duration(c.Rerank.TimeoutMs) * time.Millisecond, RerankMaxTitleRunes: c.Rerank.MaxTitleRunes, RerankMaxContentRunes: c.Rerank.MaxContentRunes,
		MaxSourceBytes: c.Resources.MaxSourceBytes, OnlineRetrievalWorkers: c.Resources.OnlineRetrievalWorkers, ReaderWorkers: c.Resources.ReaderWorkers, IngestWorkers: c.Resources.IngestWorkers, IndexWorkers: c.Resources.IndexWorkers, OCRWorkers: c.Resources.OCRWorkers, EmbeddingBatchSize: c.Resources.EmbeddingBatchSize, QdrantBatchSize: c.Resources.QdrantBatchSize,
		StreamBufferBytes: c.Resources.StreamBufferBytes, UploadBufferBytes: c.Resources.UploadBufferBytes, TextScannerMaxBytes: c.Resources.TextScannerMaxBytes, TextUnitMaxBytes: c.Resources.TextUnitMaxBytes, JSONUnitMaxBytes: c.Resources.JSONUnitMaxBytes,
		MaxPDFPages: c.Resources.MaxPDFPages, MaxOCRImageBytes: c.Resources.MaxOCRImageBytes, OCRRasterDPI: c.Resources.OCRRasterDPI,
		PDFInfoTimeout: time.Duration(c.Resources.PDFInfoTimeoutSec) * time.Second, PDFTextPageTimeout: time.Duration(c.Resources.PDFTextPageTimeoutSec) * time.Second, OCRRasterTimeout: time.Duration(c.Resources.OCRRasterTimeoutSec) * time.Second, OCRTimeout: time.Duration(c.Resources.OCRTimeoutSec) * time.Second, LegacyDocTimeout: time.Duration(c.Resources.LegacyDocTimeoutSec) * time.Second,
		MaxOOXMLFiles: c.Resources.MaxOOXMLFiles, MaxOOXMLUncompressedBytes: c.Resources.MaxOOXMLUncompressedBytes, MaxSharedStrings: c.Resources.MaxSharedStrings, MaxSharedStringBytes: c.Resources.MaxSharedStringBytes, MaxWorksheetRowCells: c.Resources.MaxWorksheetRowCells, MaxCellTextBytes: c.Resources.MaxCellTextBytes,
		MaintenanceInitialDelay: time.Duration(c.Maintenance.InitialDelaySec) * time.Second, MaintenanceInterval: time.Duration(c.Maintenance.IntervalSec) * time.Second, MaintenanceTenantPassTimeout: time.Duration(c.Maintenance.TenantPassTimeoutSec) * time.Second,
		JobHeartbeatInterval: time.Duration(c.Maintenance.JobHeartbeatSec) * time.Second, JobStaleAfter: time.Duration(c.Maintenance.JobStaleSec) * time.Second, OutboxLease: time.Duration(c.Maintenance.OutboxLeaseSec) * time.Second,
		OutboxMaxRetries: c.Maintenance.OutboxMaxRetries, OutboxRetryBase: time.Duration(c.Maintenance.OutboxRetryBaseSec) * time.Second, OutboxRetryMax: time.Duration(c.Maintenance.OutboxRetryMaxSec) * time.Second,
		MaxOutboxesPerPass: c.Maintenance.MaxOutboxesPerPass, ManualMaxOutboxes: c.Maintenance.ManualMaxOutboxes, MaxOutboxesPerJob: c.Maintenance.MaxOutboxesPerJob, RevisionRetention: time.Duration(c.Maintenance.RevisionRetentionHours) * time.Hour,
		RetrievalTraceRetention: time.Duration(c.Maintenance.RetrievalTraceRetentionHours) * time.Hour, BenchmarkRetention: time.Duration(c.Maintenance.BenchmarkRetentionHours) * time.Hour, DiagnosticCleanupBatchSize: c.Maintenance.DiagnosticCleanupBatchSize,
		FastSLOTarget: time.Duration(c.SLO.FastP95TargetMs) * time.Millisecond, DeepSLOTarget: time.Duration(c.SLO.DeepP95TargetMs) * time.Millisecond, AgenticSLOTarget: time.Duration(c.SLO.AgenticP95TargetMs) * time.Millisecond,
		SLOWindow: time.Duration(c.SLO.WindowMinutes) * time.Minute, SLOMinSamples: c.SLO.MinSamples,
	}
}

func knowledgeRuntimeConfigUnset(c config.KnowledgeRuntimeConfig) bool {
	return c.Chunk.TargetRunes == 0 && c.Chunk.ParentMaxRunes == 0 &&
		c.Retrieval.MaxResultLimit == 0 && c.Retrieval.FastTimeoutMs == 0 && c.Retrieval.DeepTimeoutMs == 0 && c.Retrieval.AgenticTimeoutMs == 0 &&
		c.Rerank.CandidateLimit == 0 && c.Resources.MaxSourceBytes == 0 && c.Resources.OnlineRetrievalWorkers == 0 && c.Resources.IngestWorkers == 0 &&
		c.Maintenance.IntervalSec == 0 && c.Maintenance.JobStaleSec == 0 && c.SLO.FastP95TargetMs == 0
}

func (s runtimeSettings) outboxRetryDelay(retryCount int) time.Duration {
	base := s.OutboxRetryBase
	maxDelay := s.OutboxRetryMax
	if base <= 0 {
		base = time.Minute
	}
	if maxDelay < base {
		maxDelay = 4 * time.Hour
	}
	if retryCount < 0 {
		retryCount = 0
	}
	shift := retryCount
	if shift > 20 {
		shift = 20
	}
	delay := base * time.Duration(1<<shift)
	if delay > maxDelay || delay < 0 {
		delay = maxDelay
	}
	return delay
}

func (s runtimeSettings) sloTarget(level string) time.Duration {
	switch level {
	case RetrievalLevelFast:
		return s.FastSLOTarget
	case RetrievalLevelDeep:
		return s.DeepSLOTarget
	case RetrievalLevelAgentic:
		return s.AgenticSLOTarget
	default:
		return 0
	}
}

func runtimeFeatures(settings runtimeSettings) config.KnowledgeFeatureConfig {
	return config.KnowledgeFeatureConfig{
		RetrievalPolicyEnabled:      settings.RetrievalPolicyEnabled,
		RetrievalPolicyShadow:       settings.RetrievalPolicyShadow,
		CanonicalReaderEnabled:      settings.CanonicalReaderEnabled,
		AgenticInvestigationEnabled: settings.AgenticInvestigationEnabled,
	}
}
