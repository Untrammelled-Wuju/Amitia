package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/embedding"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
)

const rrfK = 60.0

type RetrievalRuntime struct {
	repo          *Repository
	embedding     *embedding.Service
	reranker      Reranker
	rerankTimeout time.Duration
	settings      runtimeSettings
	onlineGate    *resourceGate
}

func NewRetrievalRuntime(repo *Repository, embeddingSvc *embedding.Service, runtime ...runtimeSettings) *RetrievalRuntime {
	settings := loadRuntimeSettings()
	if len(runtime) > 0 {
		settings = runtime[0]
	}
	timeout := settings.RerankTimeout
	if timeout <= 0 {
		timeout = defaultRuntimeSettings().RerankTimeout
	}
	return &RetrievalRuntime{
		repo: repo, embedding: embeddingSvc, rerankTimeout: timeout, settings: settings,
		onlineGate: newResourceGate(settings.OnlineRetrievalWorkers),
	}
}

type rankedCandidate struct {
	chunkID string
	score   float64
}

type fusionState struct {
	chunkID     string
	denseScore  float64
	sparseScore float64
	exactScore  float64
	fusion      float64
}

func clampWeight(v, fallback float64) float64 {
	if v == 0 {
		return fallback
	}
	if v < 0.1 {
		return 0.1
	}
	if v > 3 {
		return 3
	}
	return v
}

type variantRetrievalResult struct {
	index    int
	query    string
	dense    []rankedCandidate
	sparse   []Chunk
	exact    []Chunk
	warnings []string
	err      error
}

func (r *RetrievalRuntime) retrieveQueryVariant(ctx context.Context, tenantID string, query string, sourceIDs []string, candidateLimit int, temporal temporalQuery, useDense bool) variantRetrievalResult {
	result := variantRetrievalResult{query: query, dense: []rankedCandidate{}, sparse: []Chunk{}, exact: []Chunk{}, warnings: []string{}}
	var wg sync.WaitGroup
	var sparseErr, exactErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		result.sparse, sparseErr = r.repo.LexicalCandidatesAtContext(ctx, tenantID, query, sourceIDs, candidateLimit, temporal)
	}()
	go func() {
		defer wg.Done()
		result.exact, exactErr = r.repo.ExactCandidatesAtContext(ctx, tenantID, query, sourceIDs, candidateLimit, temporal)
	}()
	wg.Wait()
	if sparseErr != nil {
		result.err = sparseErr
		return result
	}
	if exactErr != nil {
		result.err = exactErr
		return result
	}
	if !useDense {
		return result
	}
	if r.embedding == nil || qdrantDB.Client == nil {
		result.warnings = append(result.warnings, "向量检索不可用，已降级为 Exact + Sparse 检索")
		return result
	}
	if err := ctx.Err(); err != nil {
		result.err = err
		return result
	}
	vector, embedErr := r.embedding.Embed(query)
	if embedErr != nil {
		result.warnings = append(result.warnings, "查询向量化失败，已降级为 Exact + Sparse 检索: "+embedErr.Error())
		return result
	}
	alias, aliasErr := r.repo.EnsureAlias(tenantID)
	if aliasErr != nil {
		result.err = aliasErr
		return result
	}
	denseLimit := candidateLimit
	if temporal.Mode != TemporalModeAll {
		denseLimit *= 2
		if maxDense := r.settings.MaxDenseCandidates; maxDense > 0 && denseLimit > maxDense {
			denseLimit = maxDense
		}
	}
	points, searchErr := qdrantDB.SearchVectors(vector, denseLimit, map[string]interface{}{
		"tenant_id": tenantID, "index_version": alias.ActiveVersion,
	}, knowledgeCollectionName(alias.ActiveVersion))
	if searchErr != nil {
		result.warnings = append(result.warnings, "Dense 检索失败，已继续使用 Exact + Sparse: "+searchErr.Error())
		return result
	}
	allowedSources := make(map[string]struct{}, len(sourceIDs))
	for _, id := range sourceIDs {
		allowedSources[strings.TrimSpace(id)] = struct{}{}
	}
	for _, point := range points {
		if point == nil {
			continue
		}
		chunkID, sourceID := "", ""
		if v, ok := point.Payload["chunk_id"]; ok {
			chunkID = v.GetStringValue()
		}
		if v, ok := point.Payload["source_id"]; ok {
			sourceID = v.GetStringValue()
		}
		if chunkID == "" {
			continue
		}
		if len(allowedSources) > 0 {
			if _, ok := allowedSources[sourceID]; !ok {
				continue
			}
		}
		result.dense = append(result.dense, rankedCandidate{chunkID: chunkID, score: float64(point.Score)})
	}
	return result
}

func (r *RetrievalRuntime) Search(ctx context.Context, tenantID string, req SearchRequest) (resp SearchResponse, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	originalQuery := strings.TrimSpace(req.Query)
	resp = SearchResponse{
		Evidence: []Evidence{}, Query: originalQuery, OriginalQuery: originalQuery, Warnings: []string{}, TraceConfigHash: r.settings.ConfigHash,
		TracePolicyMode: strings.TrimSpace(req.PolicyMode), TraceRouteReason: strings.TrimSpace(req.RouteReason),
		TraceShadowLevel: strings.TrimSpace(req.ShadowLevel), TraceShadowReason: strings.TrimSpace(req.ShadowReason),
	}
	traceCandidateCount := 0
	traceCandidates := []evidenceCandidate{}
	if !req.SkipTrace {
		defer func() {
			errorCode := ""
			if err != nil {
				errorCode = "retrieval_failed"
				if errors.Is(err, context.DeadlineExceeded) {
					errorCode = "retrieval_timeout"
				} else if errors.Is(err, context.Canceled) || (ctx != nil && ctx.Err() != nil) {
					errorCode = "retrieval_cancelled"
				}
			}
			_ = r.repo.RecordRetrievalTrace(tenantID, originalQuery, traceCandidateCount, traceCandidates, resp, time.Since(started), errorCode)
		}()
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 8
	}
	maxResultLimit := r.settings.MaxResultLimit
	if maxResultLimit <= 0 {
		maxResultLimit = 30
	}
	if limit > maxResultLimit {
		limit = maxResultLimit
	}
	plan, err := buildQueryPlan(req, limit, r.settings)
	if err != nil {
		return resp, err
	}
	resp.OriginalQuery = plan.OriginalQuery
	resp.EffectiveQuery = plan.EffectiveQuery
	resp.QueryComplexity = plan.Complexity
	resp.RetrievalLevel = plan.Level
	resp.QueryHistory = append([]string(nil), plan.QueryHistory...)
	resp.TraceBudgetMs = plan.Timeout.Milliseconds()
	resp.TraceCandidateLimit = plan.CandidateLimit
	resp.TraceParentExpansionTokens = plan.ParentExpansionTokenBudget
	if plan.UseRerank {
		resp.TraceRerankCandidateLimit = r.settings.RerankCandidateLimit
	}
	resp.TraceMaxAgenticRounds = plan.MaxAgenticRounds
	resp.TraceSLOTargetMs = r.settings.sloTarget(plan.Level).Milliseconds()
	if plan.Level == RetrievalLevelNone {
		resp.AnswerState = AnswerStateInsufficient
		resp.SufficiencyReason = "retrieval_disabled"
		resp.Warnings = []string{"知识检索级别为 NONE，本次未执行知识召回"}
		return resp, nil
	}
	if len(req.SourceIDs) > 0 {
		validated, validateErr := r.repo.ValidateSourceIDs(tenantID, req.SourceIDs)
		if validateErr != nil {
			return resp, validateErr
		}
		req.SourceIDs = validated
	}
	temporal, err := resolveTemporalQuery(req)
	if err != nil {
		return resp, err
	}
	resp.TemporalMode = temporal.Mode
	if temporal.Mode != TemporalModeAll {
		asOf := temporal.AsOf
		resp.AsOf = &asOf
	}

	searchCtx := ctx
	var cancel context.CancelFunc
	if plan.Timeout > 0 {
		searchCtx, cancel = context.WithTimeout(ctx, plan.Timeout)
		defer cancel()
	}
	releaseOnline, gateErr := r.onlineGate.Acquire(searchCtx)
	if gateErr != nil {
		err = gateErr
		return resp, err
	}
	defer releaseOnline()
	results := make([]variantRetrievalResult, len(plan.QueryHistory))
	var wg sync.WaitGroup
	for i, query := range plan.QueryHistory {
		i, query := i, query
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := r.retrieveQueryVariant(searchCtx, tenantID, query, req.SourceIDs, plan.CandidateLimit, temporal, plan.UseDense)
			result.index = i
			results[i] = result
		}()
	}
	wg.Wait()

	states := map[string]*fusionState{}
	state := func(id string) *fusionState {
		s := states[id]
		if s == nil {
			s = &fusionState{chunkID: id}
			states[id] = s
		}
		return s
	}
	denseWeight := clampWeight(req.DenseWeight, 1)
	sparseWeight := clampWeight(req.SparseWeight, 1)
	for i, result := range results {
		if result.err != nil {
			if i == 0 {
				return resp, result.err
			}
			resp.Degraded = true
			resp.Warnings = append(resp.Warnings, fmt.Sprintf("子查询 %d 召回失败，已回退保留 Original Query 结果", i+1))
			continue
		}
		if len(result.warnings) > 0 {
			resp.Degraded = true
			resp.Warnings = append(resp.Warnings, result.warnings...)
		}
		variantWeight := 1.0
		if i > 0 {
			variantWeight = 0.85
		}
		for rank, c := range result.dense {
			s := state(c.chunkID)
			if c.score > s.denseScore {
				s.denseScore = c.score
			}
			s.fusion += variantWeight * denseWeight / (rrfK + float64(rank+1))
		}
		for rank, c := range result.sparse {
			s := state(c.ID)
			score := 1.0 / float64(rank+1)
			if score > s.sparseScore {
				s.sparseScore = score
			}
			s.fusion += variantWeight * sparseWeight / (rrfK + float64(rank+1))
		}
		for rank, c := range result.exact {
			s := state(c.ID)
			score := 1.0 / float64(rank+1)
			if score > s.exactScore {
				s.exactScore = score
			}
			s.fusion += variantWeight * 1.5 / (rrfK + float64(rank+1))
		}
	}

	fused := make([]*fusionState, 0, len(states))
	for _, s := range states {
		fused = append(fused, s)
	}
	sort.SliceStable(fused, func(i, j int) bool {
		if fused[i].fusion == fused[j].fusion {
			return fused[i].chunkID < fused[j].chunkID
		}
		return fused[i].fusion > fused[j].fusion
	})
	hydrateLimit := len(fused)
	if hydrateLimit > plan.CandidateLimit {
		hydrateLimit = plan.CandidateLimit
	}
	ids := make([]string, 0, hydrateLimit)
	for i := 0; i < hydrateLimit; i++ {
		ids = append(ids, fused[i].chunkID)
	}
	visible, err := r.repo.VisibleChunksByIDsAtContext(searchCtx, tenantID, ids, temporal)
	if err != nil {
		return resp, err
	}
	chunkMap := make(map[string]Chunk, len(visible))
	for _, c := range visible {
		chunkMap[c.ID] = c
	}

	minScore := req.MinScore
	if minScore < 0 {
		minScore = 0
	}

	// ACL/status/revision visibility has already been applied by
	// VisibleChunksByIDs. The remaining stages are pure ranking operations:
	// parent expansion -> duplicate suppression -> MMR/diversity selection.
	prepared := prepareEvidenceCandidates(fused, chunkMap, minScore, plan.ParentExpansionTokenBudget, func(parentID string) (*Chunk, error) {
		return r.repo.GetParentChunkAtContext(searchCtx, tenantID, parentID, temporal)
	})
	deduped := dedupeEvidenceCandidates(prepared)
	resolved := applySupersession(deduped, temporal)
	reranked := resolved
	rerankWarning := ""
	if plan.UseRerank {
		reranked, rerankWarning = r.rerankEvidenceCandidates(searchCtx, plan.EffectiveQuery, resolved)
	}
	if rerankWarning != "" {
		resp.Degraded = true
		resp.Warnings = append(resp.Warnings, rerankWarning)
	}
	traceCandidateCount = len(states)
	traceCandidates = append(traceCandidates[:0], reranked...)
	conflictGroups := conflictGroupsFromCandidates(reranked)
	selected := selectMMR(reranked, limit)
	selected = preserveConflictAlternatives(selected, reranked, limit, conflictGroups)
	resp.ConflictGroups = conflictGroups
	resp.Ambiguous = len(conflictGroups) > 0
	if resp.Ambiguous {
		resp.Warnings = append(resp.Warnings, "检索结果包含显式冲突组，已保留冲突证据供模型按引用说明不确定性")
	}
	for _, candidate := range selected {
		s := candidate.state
		c := candidate.matched
		readChunk := candidate.read
		readID := candidate.readID
		locator := unmarshalMap(c.LocatorJSON)
		metadata := unmarshalMap(c.MetadataJSON)
		metadata["unitId"] = c.UnitID
		metadata["chunkIndex"] = c.ChunkIndex
		metadata["matchedChunkId"] = c.ID
		if readID != c.ID {
			metadata["parentChunkId"] = readID
			metadata["retrievalExpansion"] = "parent"
		}
		metadata["exactRankScore"] = s.exactScore
		metadata["mmrScore"] = candidate.mmrScore
		metadata["mmrLambda"] = defaultMMRLambda
		metadata["temporalMode"] = temporal.Mode
		metadata["ingestedAt"] = c.CreatedAt.UTC().Format(time.RFC3339Nano)
		supersedes := unmarshalStringList(c.SupersedesJSON)
		if len(supersedes) == 0 && readID != c.ID {
			supersedes = unmarshalStringList(readChunk.SupersedesJSON)
		}
		conflictGroup := candidateConflictGroup(candidate)
		if conflictGroup != "" {
			metadata["conflictGroup"] = conflictGroup
			if resp.Ambiguous {
				for _, group := range conflictGroups {
					if group == conflictGroup {
						metadata["conflictState"] = "ambiguous"
						break
					}
				}
			}
		}
		if len(supersedes) > 0 {
			metadata["supersedes"] = supersedes
		}
		if len(candidate.duplicates) > 0 {
			metadata["duplicateEvidence"] = candidate.duplicates
		}
		evidenceKind := "chunk"
		if readID != c.ID {
			evidenceKind = "parent_chunk"
		}
		ev := Evidence{
			ID: readID, UnitID: c.UnitID, EvidenceKind: evidenceKind, CoverageKeys: coverageKeysForLocator(locator), Checksum: readChunk.Checksum,
			SourceType: "knowledge", SourceID: c.SourceID, SourceRevision: c.SourceRevision,
			Title: readChunk.Title, Content: readChunk.Content, Locator: locator,
			DenseScore: s.denseScore, SparseScore: s.sparseScore, FusionScore: s.fusion,
			RerankScore: candidate.rerankScore, Authority: 1, Confidence: confidenceFromFusion(s.fusion),
			Trust: "untrusted_data_only", ValidFrom: c.ValidFrom, ValidTo: c.ValidTo, Supersedes: supersedes,
			ConflictGroup: conflictGroup, Metadata: metadata,
		}
		resp.Evidence = append(resp.Evidence, ev)
	}
	// No forced TopK. A retrieval can legitimately return zero Evidence.
	decision := evaluateSufficiency(plan.OriginalQuery, resp.Evidence, resp.Ambiguous)
	resp.Sufficient = decision.Sufficient
	resp.AnswerState = decision.State
	resp.SufficiencyReason = decision.Reason
	if !decision.Sufficient {
		resp.Warnings = append(resp.Warnings, "知识证据不足，允许模型明确返回无答案而不是强行补齐")
	}
	resp.Warnings = dedupeStrings(resp.Warnings)
	return resp, nil
}

func coverageKeysForLocator(locator map[string]any) []string {
	if len(locator) == 0 {
		return nil
	}
	keys := make([]string, 0, len(locator))
	for key, value := range locator {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		switch key {
		case "page", "pageNumber", "sheet", "sheetName", "row", "rowStart", "rowEnd", "paragraph", "section", "sectionPath", "slide", "cell", "range":
			keys = append(keys, key+"="+fmt.Sprint(value))
		}
	}
	sort.Strings(keys)
	return keys
}

func confidenceFromFusion(score float64) float64 {
	// RRF is an ordering signal, not a calibrated probability. This field is
	// deliberately conservative and is never fed back into fusion.
	v := score * 15
	if v > 1 {
		return 1
	}
	if v < 0 {
		return 0
	}
	return v
}

type retrievalCandidateTrace struct {
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

type retrievalQueryPlanTrace struct {
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

func queryHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func safeQueryPlanTrace(result SearchResponse) retrievalQueryPlanTrace {
	history := make([]string, 0, len(result.QueryHistory))
	for _, query := range result.QueryHistory {
		if strings.TrimSpace(query) == "" {
			continue
		}
		history = append(history, queryHash(query))
	}
	return retrievalQueryPlanTrace{
		Level: result.RetrievalLevel, Complexity: result.QueryComplexity,
		OriginalHash: queryHash(result.OriginalQuery), EffectiveHash: queryHash(result.EffectiveQuery),
		QueryHistory: history, QueryCount: len(history), TimeoutMs: result.TraceBudgetMs, CandidateLimit: result.TraceCandidateLimit,
		ParentExpansionTokens: result.TraceParentExpansionTokens, RerankCandidateLimit: result.TraceRerankCandidateLimit, MaxAgenticRounds: result.TraceMaxAgenticRounds,
	}
}

type retrievalEvidenceTrace struct {
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

func locatorHashJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func locatorHashMap(locator map[string]any) string {
	if len(locator) == 0 {
		return ""
	}
	raw, err := json.Marshal(locator)
	if err != nil {
		return ""
	}
	return locatorHashJSON(string(raw))
}

func safeCandidateTrace(candidates []evidenceCandidate) []retrievalCandidateTrace {
	limit := len(candidates)
	if limit > 50 {
		limit = 50
	}
	out := make([]retrievalCandidateTrace, 0, limit)
	for i := 0; i < limit; i++ {
		candidate := candidates[i]
		state := candidate.state
		if state == nil {
			continue
		}
		out = append(out, retrievalCandidateTrace{
			Rank: i + 1, MatchedChunkID: candidate.matched.ID, EvidenceID: candidate.readID,
			SourceID: candidate.matched.SourceID, SourceRevision: candidate.matched.SourceRevision,
			DenseScore: state.denseScore, SparseScore: state.sparseScore, ExactScore: state.exactScore,
			FusionScore: state.fusion, Relevance: candidate.relevance, RerankScore: candidate.rerankScore, MMRScore: candidate.mmrScore,
			ConflictGroup: candidateConflictGroup(candidate), ValidFrom: candidate.matched.ValidFrom, ValidTo: candidate.matched.ValidTo,
			LocatorHash: locatorHashJSON(candidate.matched.LocatorJSON),
		})
	}
	return out
}

func safeEvidenceTrace(evidence []Evidence) []retrievalEvidenceTrace {
	out := make([]retrievalEvidenceTrace, 0, len(evidence))
	for i, item := range evidence {
		out = append(out, retrievalEvidenceTrace{
			Rank: i + 1, EvidenceID: item.ID, SourceID: item.SourceID, SourceRevision: item.SourceRevision,
			DenseScore: item.DenseScore, SparseScore: item.SparseScore, FusionScore: item.FusionScore,
			RerankScore: item.RerankScore, Confidence: item.Confidence, Trust: item.Trust,
			ConflictGroup: item.ConflictGroup, ValidFrom: item.ValidFrom, ValidTo: item.ValidTo,
			LocatorHash: locatorHashMap(item.Locator),
		})
	}
	return out
}

func (r *Repository) RecordRetrievalTrace(tenantID, query string, candidateCount int, candidates []evidenceCandidate, result SearchResponse, duration time.Duration, errorCode string) error {
	if r == nil || r.db == nil {
		return nil
	}
	sum := sha256.Sum256([]byte(query))
	sources, _ := json.Marshal([]string{"knowledge_exact", "knowledge_sparse", "knowledge_dense"})
	warnings, _ := json.Marshal(dedupeStrings(result.Warnings))
	candidateTrace, _ := json.Marshal(safeCandidateTrace(candidates))
	evidenceTrace, _ := json.Marshal(safeEvidenceTrace(result.Evidence))
	queryPlanTrace, _ := json.Marshal(safeQueryPlanTrace(result))
	traceID := uuid.NewString()
	var asOf any
	if result.AsOf != nil {
		asOf = result.AsOf.UTC()
	}
	mode := strings.TrimSpace(result.TemporalMode)
	if mode == "" {
		mode = TemporalModeCurrent
	}
	sloExceeded := result.TraceSLOTargetMs > 0 && duration.Milliseconds() > result.TraceSLOTargetMs
	return r.db.Exec(`INSERT INTO retrieval_traces
        (id, tenant_id, request_id, query_hash, sources_json, candidate_count, selected_count, sufficient, duration_ms, error_code,
         temporal_mode, as_of, degraded, ambiguous, warnings_json, candidate_trace_json, evidence_trace_json, query_plan_json,
         config_hash, slo_target_ms, slo_exceeded, policy_mode, route_reason, shadow_level, shadow_reason, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		traceID, tenantID, traceID, hex.EncodeToString(sum[:]), string(sources), candidateCount, len(result.Evidence), boolToInt(result.Sufficient),
		duration.Milliseconds(), strings.TrimSpace(errorCode), mode, asOf, boolToInt(result.Degraded), boolToInt(result.Ambiguous),
		string(warnings), string(candidateTrace), string(evidenceTrace), string(queryPlanTrace), strings.TrimSpace(result.TraceConfigHash), result.TraceSLOTargetMs, boolToInt(sloExceeded),
		strings.TrimSpace(result.TracePolicyMode), strings.TrimSpace(result.TraceRouteReason), strings.TrimSpace(result.TraceShadowLevel), strings.TrimSpace(result.TraceShadowReason), time.Now().UTC()).Error
}

type retrievalTraceRow struct {
	ID                 string     `gorm:"column:id"`
	RequestID          string     `gorm:"column:request_id"`
	QueryHash          string     `gorm:"column:query_hash"`
	CandidateCount     int        `gorm:"column:candidate_count"`
	SelectedCount      int        `gorm:"column:selected_count"`
	Sufficient         int        `gorm:"column:sufficient"`
	DurationMs         int64      `gorm:"column:duration_ms"`
	ErrorCode          string     `gorm:"column:error_code"`
	TemporalMode       string     `gorm:"column:temporal_mode"`
	AsOf               *time.Time `gorm:"column:as_of"`
	Degraded           int        `gorm:"column:degraded"`
	Ambiguous          int        `gorm:"column:ambiguous"`
	WarningsJSON       string     `gorm:"column:warnings_json"`
	CandidateTraceJSON string     `gorm:"column:candidate_trace_json"`
	EvidenceTraceJSON  string     `gorm:"column:evidence_trace_json"`
	QueryPlanJSON      string     `gorm:"column:query_plan_json"`
	ConfigHash         string     `gorm:"column:config_hash"`
	SLOTargetMs        int64      `gorm:"column:slo_target_ms"`
	SLOExceeded        int        `gorm:"column:slo_exceeded"`
	PolicyMode         string     `gorm:"column:policy_mode"`
	RouteReason        string     `gorm:"column:route_reason"`
	ShadowLevel        string     `gorm:"column:shadow_level"`
	ShadowReason       string     `gorm:"column:shadow_reason"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
}

// ListRetrievalTraces returns only the already-sanitized trace payloads stored by
// RecordRetrievalTrace. Raw query text, document body and locator values are not
// present in this table/view contract, so the developer UI cannot accidentally
// turn tracing into a data-exfiltration path.
func (r *Repository) ListRetrievalTraces(tenantID string, limit int) ([]RetrievalTraceSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if r == nil || r.db == nil {
		return []RetrievalTraceSummary{}, nil
	}
	var rows []retrievalTraceRow
	if err := r.db.Raw(`SELECT id, request_id, query_hash, candidate_count, selected_count, sufficient, duration_ms, error_code,
		temporal_mode, as_of, degraded, ambiguous, warnings_json, candidate_trace_json, evidence_trace_json, query_plan_json,
		config_hash, slo_target_ms, slo_exceeded, policy_mode, route_reason, shadow_level, shadow_reason, created_at
		FROM retrieval_traces
		WHERE tenant_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, strings.TrimSpace(tenantID), limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]RetrievalTraceSummary, 0, len(rows))
	for _, row := range rows {
		item := RetrievalTraceSummary{
			ID: row.ID, RequestID: row.RequestID, QueryHash: row.QueryHash,
			CandidateCount: row.CandidateCount, SelectedCount: row.SelectedCount, Sufficient: row.Sufficient != 0,
			DurationMs: row.DurationMs, ErrorCode: strings.TrimSpace(row.ErrorCode), TemporalMode: row.TemporalMode, AsOf: row.AsOf,
			Degraded: row.Degraded != 0, Ambiguous: row.Ambiguous != 0, ConfigHash: row.ConfigHash,
			SLOTargetMs: row.SLOTargetMs, SLOExceeded: row.SLOExceeded != 0,
			PolicyMode: row.PolicyMode, RouteReason: row.RouteReason, ShadowLevel: row.ShadowLevel, ShadowReason: row.ShadowReason, CreatedAt: row.CreatedAt,
			Warnings: []string{}, Candidates: []RetrievalCandidateTrace{}, Evidence: []RetrievalEvidenceTrace{},
			QueryPlan: RetrievalQueryPlanTrace{QueryHistory: []string{}},
		}
		_ = json.Unmarshal([]byte(row.WarningsJSON), &item.Warnings)
		_ = json.Unmarshal([]byte(row.CandidateTraceJSON), &item.Candidates)
		_ = json.Unmarshal([]byte(row.EvidenceTraceJSON), &item.Evidence)
		_ = json.Unmarshal([]byte(row.QueryPlanJSON), &item.QueryPlan)
		if item.Warnings == nil {
			item.Warnings = []string{}
		}
		if item.Candidates == nil {
			item.Candidates = []RetrievalCandidateTrace{}
		}
		if item.Evidence == nil {
			item.Evidence = []RetrievalEvidenceTrace{}
		}
		if item.QueryPlan.QueryHistory == nil {
			item.QueryPlan.QueryHistory = []string{}
		}
		out = append(out, item)
	}
	return out, nil
}

type retrievalSLOTraceRow struct {
	DurationMs    int64  `gorm:"column:duration_ms"`
	ErrorCode     string `gorm:"column:error_code"`
	QueryPlanJSON string `gorm:"column:query_plan_json"`
	SLOTargetMs   int64  `gorm:"column:slo_target_ms"`
	SLOExceeded   int    `gorm:"column:slo_exceeded"`
}

func percentileInt64(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if percentile <= 0 {
		return sorted[0]
	}
	if percentile >= 1 {
		return sorted[len(sorted)-1]
	}
	index := int(math.Ceil(float64(len(sorted))*percentile)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func (r *Repository) RetrievalSLOSummary(tenantID, configHash string, window time.Duration, minSamples int, targets map[string]time.Duration) (RetrievalSLOSummary, error) {
	now := time.Now().UTC()
	if window <= 0 {
		window = time.Hour
	}
	if minSamples <= 0 {
		minSamples = 1
	}
	from := now.Add(-window)
	summary := RetrievalSLOSummary{
		ConfigHash: strings.TrimSpace(configHash), WindowMinutes: int(window / time.Minute), MinSamples: minSamples,
		From: from, To: now, Levels: []RetrievalSLOLevelSummary{},
	}
	if r == nil || r.db == nil {
		return summary, nil
	}
	var rows []retrievalSLOTraceRow
	query := r.db.Raw(`SELECT duration_ms, error_code, query_plan_json, slo_target_ms, slo_exceeded
		FROM retrieval_traces
		WHERE tenant_id = ? AND created_at >= ? AND created_at <= ? AND config_hash = ?
		ORDER BY created_at ASC`, tenantID, from, now, strings.TrimSpace(configHash))
	if err := query.Scan(&rows).Error; err != nil {
		return summary, err
	}
	levels := []string{RetrievalLevelFast, RetrievalLevelDeep, RetrievalLevelAgentic}
	for _, level := range levels {
		item := RetrievalSLOLevelSummary{Level: level}
		if target := targets[level]; target > 0 {
			item.TargetMs = target.Milliseconds()
		}
		durations := make([]int64, 0)
		for _, row := range rows {
			var plan retrievalQueryPlanTrace
			if err := json.Unmarshal([]byte(row.QueryPlanJSON), &plan); err != nil || plan.Level != level {
				continue
			}
			item.Samples++
			if strings.TrimSpace(row.ErrorCode) != "" {
				item.ErrorCount++
			}
			if row.SLOExceeded != 0 {
				item.ExceededCount++
			}
			if row.DurationMs >= 0 {
				durations = append(durations, row.DurationMs)
			}
			if item.TargetMs <= 0 && row.SLOTargetMs > 0 {
				item.TargetMs = row.SLOTargetMs
			}
		}
		item.P50Ms = percentileInt64(durations, 0.50)
		item.P95Ms = percentileInt64(durations, 0.95)
		item.EnoughSamples = item.Samples >= minSamples
		item.TargetMet = item.EnoughSamples && item.TargetMs > 0 && item.P95Ms <= item.TargetMs
		summary.Levels = append(summary.Levels, item)
	}
	return summary, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
