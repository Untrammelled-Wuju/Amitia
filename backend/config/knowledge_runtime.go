package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// KnowledgeRuntimeConfig is the single typed source of truth for knowledge
// ingestion, retrieval and resource budgets. Keep this free of credentials so
// Hash() can be safely persisted in retrieval traces and benchmark artifacts.
type KnowledgeRuntimeConfig struct {
	Features    KnowledgeFeatureConfig     `mapstructure:"features" json:"features"`
	Chunk       KnowledgeChunkConfig       `mapstructure:"chunk" json:"chunk"`
	Retrieval   KnowledgeRetrievalConfig   `mapstructure:"retrieval" json:"retrieval"`
	Rerank      KnowledgeRerankConfig      `mapstructure:"rerank" json:"rerank"`
	Resources   KnowledgeResourceConfig    `mapstructure:"resources" json:"resources"`
	Maintenance KnowledgeMaintenanceConfig `mapstructure:"maintenance" json:"maintenance"`
	SLO         KnowledgeSLOConfig         `mapstructure:"slo" json:"slo"`
}

type KnowledgeFeatureConfig struct {
	RetrievalPolicyEnabled      bool `mapstructure:"retrievalPolicyEnabled" json:"retrievalPolicyEnabled"`
	RetrievalPolicyShadow       bool `mapstructure:"retrievalPolicyShadow" json:"retrievalPolicyShadow"`
	CanonicalReaderEnabled      bool `mapstructure:"canonicalReaderEnabled" json:"canonicalReaderEnabled"`
	AgenticInvestigationEnabled bool `mapstructure:"agenticInvestigationEnabled" json:"agenticInvestigationEnabled"`
}

type KnowledgeChunkConfig struct {
	TargetRunes    int `mapstructure:"targetRunes" json:"targetRunes"`
	OverlapRunes   int `mapstructure:"overlapRunes" json:"overlapRunes"`
	ParentMaxRunes int `mapstructure:"parentMaxRunes" json:"parentMaxRunes"`
}

type KnowledgeRetrievalConfig struct {
	MaxResultLimit               int `mapstructure:"maxResultLimit" json:"maxResultLimit"`
	FastTimeoutMs                int `mapstructure:"fastTimeoutMs" json:"fastTimeoutMs"`
	FastMinCandidates            int `mapstructure:"fastMinCandidates" json:"fastMinCandidates"`
	FastMaxCandidates            int `mapstructure:"fastMaxCandidates" json:"fastMaxCandidates"`
	FastParentExpansionTokens    int `mapstructure:"fastParentExpansionTokens" json:"fastParentExpansionTokens"`
	DeepTimeoutMs                int `mapstructure:"deepTimeoutMs" json:"deepTimeoutMs"`
	DeepMinCandidates            int `mapstructure:"deepMinCandidates" json:"deepMinCandidates"`
	DeepMaxCandidates            int `mapstructure:"deepMaxCandidates" json:"deepMaxCandidates"`
	DeepParentExpansionTokens    int `mapstructure:"deepParentExpansionTokens" json:"deepParentExpansionTokens"`
	AgenticTimeoutMs             int `mapstructure:"agenticTimeoutMs" json:"agenticTimeoutMs"`
	AgenticMinCandidates         int `mapstructure:"agenticMinCandidates" json:"agenticMinCandidates"`
	AgenticMaxCandidates         int `mapstructure:"agenticMaxCandidates" json:"agenticMaxCandidates"`
	AgenticParentExpansionTokens int `mapstructure:"agenticParentExpansionTokens" json:"agenticParentExpansionTokens"`
	MaxAgenticRounds             int `mapstructure:"maxAgenticRounds" json:"maxAgenticRounds"`
	MaxDenseCandidates           int `mapstructure:"maxDenseCandidates" json:"maxDenseCandidates"`
}

type KnowledgeRerankConfig struct {
	CandidateLimit  int `mapstructure:"candidateLimit" json:"candidateLimit"`
	TimeoutMs       int `mapstructure:"timeoutMs" json:"timeoutMs"`
	MaxTitleRunes   int `mapstructure:"maxTitleRunes" json:"maxTitleRunes"`
	MaxContentRunes int `mapstructure:"maxContentRunes" json:"maxContentRunes"`
}

type KnowledgeResourceConfig struct {
	MaxSourceBytes            int64 `mapstructure:"maxSourceBytes" json:"maxSourceBytes"`
	OnlineRetrievalWorkers    int   `mapstructure:"onlineRetrievalWorkers" json:"onlineRetrievalWorkers"`
	ReaderWorkers             int   `mapstructure:"readerWorkers" json:"readerWorkers"`
	IngestWorkers             int   `mapstructure:"ingestWorkers" json:"ingestWorkers"`
	IndexWorkers              int   `mapstructure:"indexWorkers" json:"indexWorkers"`
	OCRWorkers                int   `mapstructure:"ocrWorkers" json:"ocrWorkers"`
	EmbeddingBatchSize        int   `mapstructure:"embeddingBatchSize" json:"embeddingBatchSize"`
	QdrantBatchSize           int   `mapstructure:"qdrantBatchSize" json:"qdrantBatchSize"`
	StreamBufferBytes         int   `mapstructure:"streamBufferBytes" json:"streamBufferBytes"`
	UploadBufferBytes         int   `mapstructure:"uploadBufferBytes" json:"uploadBufferBytes"`
	TextScannerMaxBytes       int   `mapstructure:"textScannerMaxBytes" json:"textScannerMaxBytes"`
	TextUnitMaxBytes          int   `mapstructure:"textUnitMaxBytes" json:"textUnitMaxBytes"`
	JSONUnitMaxBytes          int   `mapstructure:"jsonUnitMaxBytes" json:"jsonUnitMaxBytes"`
	MaxPDFPages               int   `mapstructure:"maxPdfPages" json:"maxPdfPages"`
	MaxOCRImageBytes          int64 `mapstructure:"maxOcrImageBytes" json:"maxOcrImageBytes"`
	OCRRasterDPI              int   `mapstructure:"ocrRasterDpi" json:"ocrRasterDpi"`
	PDFInfoTimeoutSec         int   `mapstructure:"pdfInfoTimeoutSec" json:"pdfInfoTimeoutSec"`
	PDFTextPageTimeoutSec     int   `mapstructure:"pdfTextPageTimeoutSec" json:"pdfTextPageTimeoutSec"`
	OCRRasterTimeoutSec       int   `mapstructure:"ocrRasterTimeoutSec" json:"ocrRasterTimeoutSec"`
	OCRTimeoutSec             int   `mapstructure:"ocrTimeoutSec" json:"ocrTimeoutSec"`
	LegacyDocTimeoutSec       int   `mapstructure:"legacyDocTimeoutSec" json:"legacyDocTimeoutSec"`
	MaxOOXMLFiles             int   `mapstructure:"maxOoxmlFiles" json:"maxOoxmlFiles"`
	MaxOOXMLUncompressedBytes int64 `mapstructure:"maxOoxmlUncompressedBytes" json:"maxOoxmlUncompressedBytes"`
	MaxSharedStrings          int   `mapstructure:"maxSharedStrings" json:"maxSharedStrings"`
	MaxSharedStringBytes      int64 `mapstructure:"maxSharedStringBytes" json:"maxSharedStringBytes"`
	MaxWorksheetRowCells      int   `mapstructure:"maxWorksheetRowCells" json:"maxWorksheetRowCells"`
	MaxCellTextBytes          int   `mapstructure:"maxCellTextBytes" json:"maxCellTextBytes"`
}

type KnowledgeMaintenanceConfig struct {
	InitialDelaySec              int `mapstructure:"initialDelaySec" json:"initialDelaySec"`
	IntervalSec                  int `mapstructure:"intervalSec" json:"intervalSec"`
	TenantPassTimeoutSec         int `mapstructure:"tenantPassTimeoutSec" json:"tenantPassTimeoutSec"`
	JobHeartbeatSec              int `mapstructure:"jobHeartbeatSec" json:"jobHeartbeatSec"`
	JobStaleSec                  int `mapstructure:"jobStaleSec" json:"jobStaleSec"`
	OutboxLeaseSec               int `mapstructure:"outboxLeaseSec" json:"outboxLeaseSec"`
	OutboxMaxRetries             int `mapstructure:"outboxMaxRetries" json:"outboxMaxRetries"`
	OutboxRetryBaseSec           int `mapstructure:"outboxRetryBaseSec" json:"outboxRetryBaseSec"`
	OutboxRetryMaxSec            int `mapstructure:"outboxRetryMaxSec" json:"outboxRetryMaxSec"`
	MaxOutboxesPerPass           int `mapstructure:"maxOutboxesPerPass" json:"maxOutboxesPerPass"`
	ManualMaxOutboxes            int `mapstructure:"manualMaxOutboxes" json:"manualMaxOutboxes"`
	MaxOutboxesPerJob            int `mapstructure:"maxOutboxesPerJob" json:"maxOutboxesPerJob"`
	RevisionRetentionHours       int `mapstructure:"revisionRetentionHours" json:"revisionRetentionHours"`
	RetrievalTraceRetentionHours int `mapstructure:"retrievalTraceRetentionHours" json:"retrievalTraceRetentionHours"`
	BenchmarkRetentionHours      int `mapstructure:"benchmarkRetentionHours" json:"benchmarkRetentionHours"`
	DiagnosticCleanupBatchSize   int `mapstructure:"diagnosticCleanupBatchSize" json:"diagnosticCleanupBatchSize"`
}

type KnowledgeSLOConfig struct {
	FastP95TargetMs    int `mapstructure:"fastP95TargetMs" json:"fastP95TargetMs"`
	DeepP95TargetMs    int `mapstructure:"deepP95TargetMs" json:"deepP95TargetMs"`
	AgenticP95TargetMs int `mapstructure:"agenticP95TargetMs" json:"agenticP95TargetMs"`
	WindowMinutes      int `mapstructure:"windowMinutes" json:"windowMinutes"`
	MinSamples         int `mapstructure:"minSamples" json:"minSamples"`
}

func DefaultKnowledgeRuntimeConfig() KnowledgeRuntimeConfig {
	return KnowledgeRuntimeConfig{
		Features: KnowledgeFeatureConfig{RetrievalPolicyEnabled: true, RetrievalPolicyShadow: false, CanonicalReaderEnabled: true, AgenticInvestigationEnabled: true},
		Chunk:    KnowledgeChunkConfig{TargetRunes: 1200, OverlapRunes: 160, ParentMaxRunes: 6000},
		Retrieval: KnowledgeRetrievalConfig{
			MaxResultLimit: 30,
			FastTimeoutMs:  2500, FastMinCandidates: 24, FastMaxCandidates: 100, FastParentExpansionTokens: 3000,
			DeepTimeoutMs: 6000, DeepMinCandidates: 40, DeepMaxCandidates: 240, DeepParentExpansionTokens: 7000,
			AgenticTimeoutMs: 10000, AgenticMinCandidates: 60, AgenticMaxCandidates: 300, AgenticParentExpansionTokens: 10000,
			MaxAgenticRounds: 3, MaxDenseCandidates: 400,
		},
		Rerank: KnowledgeRerankConfig{CandidateLimit: 24, TimeoutMs: 1200, MaxTitleRunes: 512, MaxContentRunes: 6000},
		Resources: KnowledgeResourceConfig{
			MaxSourceBytes: 2 << 30, OnlineRetrievalWorkers: 32, ReaderWorkers: 16, IngestWorkers: 2, IndexWorkers: 2, OCRWorkers: 1, EmbeddingBatchSize: 32, QdrantBatchSize: 64,
			StreamBufferBytes: 128 << 10, UploadBufferBytes: 256 << 10, TextScannerMaxBytes: 4 << 20, TextUnitMaxBytes: 16 << 10, JSONUnitMaxBytes: 8 << 20,
			MaxPDFPages: 20000, MaxOCRImageBytes: 64 << 20, OCRRasterDPI: 180,
			PDFInfoTimeoutSec: 30, PDFTextPageTimeoutSec: 45, OCRRasterTimeoutSec: 45, OCRTimeoutSec: 60, LegacyDocTimeoutSec: 90,
			MaxOOXMLFiles: 20000, MaxOOXMLUncompressedBytes: 512 << 20, MaxSharedStrings: 500000, MaxSharedStringBytes: 128 << 20,
			MaxWorksheetRowCells: 20000, MaxCellTextBytes: 2 << 20,
		},
		Maintenance: KnowledgeMaintenanceConfig{
			InitialDelaySec: 10, IntervalSec: 60, TenantPassTimeoutSec: 45, JobHeartbeatSec: 30, JobStaleSec: 120, OutboxLeaseSec: 300,
			OutboxMaxRetries: 10, OutboxRetryBaseSec: 60, OutboxRetryMaxSec: 15360,
			MaxOutboxesPerPass: 50, ManualMaxOutboxes: 200, MaxOutboxesPerJob: 100000, RevisionRetentionHours: 168,
			RetrievalTraceRetentionHours: 168, BenchmarkRetentionHours: 2160, DiagnosticCleanupBatchSize: 500,
		},
		SLO: KnowledgeSLOConfig{FastP95TargetMs: 1500, DeepP95TargetMs: 4000, AgenticP95TargetMs: 8000, WindowMinutes: 60, MinSamples: 20},
	}
}

func EffectiveKnowledgeFeatureConfig() KnowledgeFeatureConfig {
	defaults := DefaultKnowledgeRuntimeConfig().Features
	if AppCfg == nil {
		return defaults
	}
	k := AppCfg.Knowledge
	if k.Chunk.TargetRunes == 0 && k.Retrieval.MaxResultLimit == 0 && k.Resources.MaxSourceBytes == 0 && k.Maintenance.IntervalSec == 0 {
		return defaults
	}
	return k.Features
}

func (c KnowledgeRuntimeConfig) Hash() string {
	b, _ := json.Marshal(c)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func validateKnowledgeRuntimeConfig(cfg KnowledgeRuntimeConfig) error {
	c := cfg.Chunk
	if c.TargetRunes < 256 || c.TargetRunes > 16000 || c.OverlapRunes < 0 || c.OverlapRunes >= c.TargetRunes || c.ParentMaxRunes < c.TargetRunes || c.ParentMaxRunes > 64000 {
		return fmt.Errorf("chunk 参数超出安全范围")
	}
	r := cfg.Retrieval
	if r.MaxResultLimit < 1 || r.MaxResultLimit > 100 ||
		r.FastTimeoutMs < 100 || r.FastTimeoutMs > 30000 || r.DeepTimeoutMs < 500 || r.DeepTimeoutMs > 60000 || r.AgenticTimeoutMs < 1000 || r.AgenticTimeoutMs > 120000 ||
		r.FastMinCandidates < 1 || r.FastMaxCandidates < r.FastMinCandidates || r.FastMaxCandidates > 1000 ||
		r.DeepMinCandidates < 1 || r.DeepMaxCandidates < r.DeepMinCandidates || r.DeepMaxCandidates > 2000 ||
		r.AgenticMinCandidates < 1 || r.AgenticMaxCandidates < r.AgenticMinCandidates || r.AgenticMaxCandidates > 3000 ||
		r.MaxAgenticRounds < 1 || r.MaxAgenticRounds > 8 || r.MaxDenseCandidates < 10 || r.MaxDenseCandidates > 5000 ||
		r.FastParentExpansionTokens < 0 || r.FastParentExpansionTokens > 50000 || r.DeepParentExpansionTokens < 0 || r.DeepParentExpansionTokens > 100000 || r.AgenticParentExpansionTokens < 0 || r.AgenticParentExpansionTokens > 150000 {
		return fmt.Errorf("retrieval 参数超出安全范围")
	}
	rr := cfg.Rerank
	if rr.CandidateLimit < 1 || rr.CandidateLimit > 256 || rr.TimeoutMs < 50 || rr.TimeoutMs > 30000 || rr.MaxTitleRunes < 32 || rr.MaxTitleRunes > 4096 || rr.MaxContentRunes < 256 || rr.MaxContentRunes > 50000 {
		return fmt.Errorf("rerank 参数超出安全范围")
	}
	res := cfg.Resources
	if res.MaxSourceBytes < 1<<20 || res.MaxSourceBytes > 32<<30 ||
		res.OnlineRetrievalWorkers < 1 || res.OnlineRetrievalWorkers > 512 || res.ReaderWorkers < 1 || res.ReaderWorkers > 256 || res.IngestWorkers < 1 || res.IngestWorkers > 32 ||
		res.IndexWorkers < 1 || res.IndexWorkers > 32 || res.OCRWorkers < 1 || res.OCRWorkers > 16 ||
		res.EmbeddingBatchSize < 1 || res.EmbeddingBatchSize > 512 || res.QdrantBatchSize < 1 || res.QdrantBatchSize > 512 ||
		res.StreamBufferBytes < 4<<10 || res.StreamBufferBytes > 8<<20 || res.UploadBufferBytes < 4<<10 || res.UploadBufferBytes > 8<<20 ||
		res.TextScannerMaxBytes < 64<<10 || res.TextScannerMaxBytes > 64<<20 || res.TextUnitMaxBytes < 1<<10 || res.TextUnitMaxBytes > 4<<20 ||
		res.JSONUnitMaxBytes < 64<<10 || res.JSONUnitMaxBytes > 64<<20 || res.MaxPDFPages < 1 || res.MaxPDFPages > 100000 ||
		res.MaxOCRImageBytes < 1<<20 || res.MaxOCRImageBytes > 1<<30 || res.OCRRasterDPI < 72 || res.OCRRasterDPI > 600 ||
		res.PDFInfoTimeoutSec < 1 || res.PDFInfoTimeoutSec > 300 || res.PDFTextPageTimeoutSec < 1 || res.PDFTextPageTimeoutSec > 300 || res.OCRRasterTimeoutSec < 1 || res.OCRRasterTimeoutSec > 300 || res.OCRTimeoutSec < 1 || res.OCRTimeoutSec > 600 || res.LegacyDocTimeoutSec < 1 || res.LegacyDocTimeoutSec > 600 ||
		res.MaxOOXMLFiles < 100 || res.MaxOOXMLFiles > 200000 || res.MaxOOXMLUncompressedBytes < 1<<20 || res.MaxOOXMLUncompressedBytes > 8<<30 ||
		res.MaxSharedStrings < 100 || res.MaxSharedStrings > 5000000 || res.MaxSharedStringBytes < 1<<20 || res.MaxSharedStringBytes > 2<<30 || res.MaxWorksheetRowCells < 10 || res.MaxWorksheetRowCells > 100000 || res.MaxCellTextBytes < 1024 || res.MaxCellTextBytes > 64<<20 {
		return fmt.Errorf("resource limit 参数超出安全范围")
	}
	m := cfg.Maintenance
	if m.InitialDelaySec < 0 || m.InitialDelaySec > 300 || m.IntervalSec < 5 || m.IntervalSec > 3600 ||
		m.TenantPassTimeoutSec < 5 || m.TenantPassTimeoutSec > 3600 || m.JobHeartbeatSec < 5 || m.JobHeartbeatSec > 3600 ||
		m.JobStaleSec < 30 || m.JobStaleSec > 24*3600 || m.JobHeartbeatSec >= m.JobStaleSec ||
		m.OutboxLeaseSec < 30 || m.OutboxLeaseSec > 24*3600 || m.OutboxMaxRetries < 1 || m.OutboxMaxRetries > 100 ||
		m.OutboxRetryBaseSec < 1 || m.OutboxRetryBaseSec > 3600 || m.OutboxRetryMaxSec < m.OutboxRetryBaseSec || m.OutboxRetryMaxSec > 24*3600 ||
		m.MaxOutboxesPerPass < 1 || m.MaxOutboxesPerPass > 10000 ||
		m.ManualMaxOutboxes < 1 || m.ManualMaxOutboxes > 50000 || m.MaxOutboxesPerJob < 1 || m.MaxOutboxesPerJob > 1000000 ||
		m.RevisionRetentionHours < 1 || m.RevisionRetentionHours > 24*365 ||
		m.RetrievalTraceRetentionHours < 1 || m.RetrievalTraceRetentionHours > 24*90 ||
		m.BenchmarkRetentionHours < 24 || m.BenchmarkRetentionHours > 24*730 ||
		m.DiagnosticCleanupBatchSize < 10 || m.DiagnosticCleanupBatchSize > 10000 {
		return fmt.Errorf("maintenance 参数超出安全范围")
	}
	slo := cfg.SLO
	if slo.FastP95TargetMs < 50 || slo.FastP95TargetMs > r.FastTimeoutMs ||
		slo.DeepP95TargetMs < 100 || slo.DeepP95TargetMs > r.DeepTimeoutMs ||
		slo.AgenticP95TargetMs < 250 || slo.AgenticP95TargetMs > r.AgenticTimeoutMs ||
		slo.WindowMinutes < 1 || slo.WindowMinutes > 24*60 || slo.MinSamples < 1 || slo.MinSamples > 100000 {
		return fmt.Errorf("slo 参数必须为正数且不能超过对应 retrieval timeout")
	}
	return nil
}
