package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/knowledge"
	amitialog "log"
)

const (
	defaultInvestigationMaxRounds      = 3
	maxInvestigationRounds             = 5
	defaultInvestigationSearchCalls    = 2
	defaultInvestigationReadCalls      = 6
	defaultInvestigationReadRunes      = 48000
	defaultInvestigationReadRunesCall  = 12000
	defaultInvestigationMaxSources     = 5
	defaultInvestigationPlannerTimeout = 3500 * time.Millisecond
	defaultInvestigationTotalTimeout   = 12 * time.Second
	investigationNoProgressLimit       = 2
)

type investigationPlannerFunc func(ctx context.Context, userGoal string, result knowledge.SearchResponse, state investigationState, budget investigationBudget) (investigationAction, error)

type investigationAction struct {
	Action         string         `json:"action"`
	Reason         string         `json:"reason,omitempty"`
	Query          string         `json:"query,omitempty"`
	SourceIDs      []string       `json:"sourceIds,omitempty"`
	EvidenceID     string         `json:"evidenceId,omitempty"`
	UnitID         string         `json:"unitId,omitempty"`
	SourceID       string         `json:"sourceId,omitempty"`
	SourceRevision int64          `json:"sourceRevision,omitempty"`
	Locator        map[string]any `json:"locator,omitempty"`
}

type investigationBudget struct {
	MaxRounds      int
	MaxSearchCalls int
	MaxReadCalls   int
	MaxReadRunes   int
	MaxSources     int
}

type investigationState struct {
	Round       int
	SearchCalls int
	ReadCalls   int
	ReadRunes   int
	NoProgress  int
	Queries     map[string]struct{}
	Reads       map[string]struct{}
	Sources     map[string]struct{}
	LastAction  string
	Termination string
}

func defaultInvestigationBudget(result knowledge.SearchResponse) investigationBudget {
	maxRounds := result.TraceMaxAgenticRounds
	if maxRounds <= 0 {
		maxRounds = defaultInvestigationMaxRounds
	}
	if maxRounds > maxInvestigationRounds {
		maxRounds = maxInvestigationRounds
	}
	return investigationBudget{
		MaxRounds:      maxRounds,
		MaxSearchCalls: defaultInvestigationSearchCalls,
		MaxReadCalls:   defaultInvestigationReadCalls,
		MaxReadRunes:   defaultInvestigationReadRunes,
		MaxSources:     defaultInvestigationMaxSources,
	}
}

func (s *service) runKnowledgeInvestigation(ctx context.Context, tenantID, userGoal string, result knowledge.SearchResponse, trace *ownedKnowledgeTrace, citations *ownedCitationAudit) knowledge.SearchResponse {
	if s == nil || s.knowledgePort == nil || result.RetrievalLevel != knowledge.RetrievalLevelAgentic || result.Sufficient {
		return result
	}
	if ctx == nil {
		ctx = context.Background()
	}
	budget := defaultInvestigationBudget(result)
	state := investigationState{
		Queries: map[string]struct{}{}, Reads: map[string]struct{}{}, Sources: map[string]struct{}{},
	}
	for _, q := range result.QueryHistory {
		if normalized := normalizeInvestigationKey(q); normalized != "" {
			state.Queries[normalized] = struct{}{}
		}
	}
	if normalized := normalizeInvestigationKey(userGoal); normalized != "" {
		state.Queries[normalized] = struct{}{}
	}
	for _, ev := range result.Evidence {
		if strings.TrimSpace(ev.SourceID) != "" {
			state.Sources[ev.SourceID] = struct{}{}
		}
	}

	totalTimeout := defaultInvestigationTotalTimeout
	if result.TraceBudgetMs > 0 {
		candidate := time.Duration(result.TraceBudgetMs) * time.Millisecond
		if candidate > totalTimeout {
			totalTimeout = candidate
		}
	}
	investigationCtx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()

	investigationTask, _ := trace.begin(context.WithoutCancel(ctx), "investigation.started", "正在调查知识证据", map[string]any{
		"maxRounds": budget.MaxRounds, "maxSearchCalls": budget.MaxSearchCalls, "maxReadCalls": budget.MaxReadCalls, "maxReadRunes": budget.MaxReadRunes,
	})

	for round := 1; round <= budget.MaxRounds; round++ {
		if err := investigationCtx.Err(); err != nil {
			state.Termination = investigationTerminationForContext(err)
			break
		}
		knowledge.ApplySufficiency(userGoal, &result)
		if result.Sufficient {
			state.Termination = "sufficient"
			break
		}
		state.Round = round
		if investigationTask != nil {
			if err := trace.progress(context.WithoutCancel(ctx), investigationTask, "investigation.round", map[string]any{
				"round": round, "maxRounds": budget.MaxRounds,
				"sufficiencyReason": result.SufficiencyReason, "evidenceCount": len(result.Evidence),
				"searchCalls": state.SearchCalls, "readCalls": state.ReadCalls, "readRunes": state.ReadRunes,
			}); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}

		action, err := s.nextKnowledgeInvestigationAction(investigationCtx, userGoal, result, state, budget)
		if err != nil {
			result.Degraded = true
			result.Warnings = appendUniqueString(result.Warnings, "Agentic planner 不可用，已保留当前可验证证据: "+err.Error())
			state.Termination = "planner_failed"
			break
		}
		state.LastAction = action.Action
		progressBefore := investigationProgressFingerprint(result)
		noProgressBefore := state.NoProgress
		stop := false

		switch action.Action {
		case "finish_insufficient":
			state.Termination = "planner_finish_insufficient"
			stop = true
		case "finish_sufficient":
			knowledge.ApplySufficiency(userGoal, &result)
			if result.Sufficient {
				state.Termination = "sufficient"
				stop = true
			} else {
				state.NoProgress++
			}
		case "search":
			if state.SearchCalls >= budget.MaxSearchCalls {
				state.NoProgress++
				result.Warnings = appendUniqueString(result.Warnings, "Agentic search budget exhausted")
				break
			}
			query := strings.TrimSpace(action.Query)
			key := normalizeInvestigationKey(query)
			if query == "" || key == "" {
				state.NoProgress++
				break
			}
			if _, duplicate := state.Queries[key]; duplicate {
				state.NoProgress++
				result.Warnings = appendUniqueString(result.Warnings, "Agentic planner proposed a duplicate search query")
				break
			}
			sourceIDs := boundedKnownSourceIDs(action.SourceIDs, result.Evidence, budget.MaxSources)
			if len(action.SourceIDs) > 0 && len(sourceIDs) == 0 {
				state.NoProgress++
				result.Warnings = appendUniqueString(result.Warnings, "Agentic search action rejected: requested sources are outside the current evidence ledger")
				break
			}
			state.SearchCalls++
			state.Queries[key] = struct{}{}
			follow, searchErr := s.knowledgePort.Search(investigationCtx, tenantID, knowledge.SearchRequest{
				Query: query, ContextualQuery: truncatePromptRunes(userGoal, 2500), RetrievalLevel: knowledge.RetrievalLevelDeep,
				Limit: 6, SourceIDs: sourceIDs, PolicyMode: "agentic_followup", RouteReason: strings.TrimSpace(action.Reason),
			})
			if searchErr != nil {
				result.Degraded = true
				result.Warnings = appendUniqueString(result.Warnings, "Agentic follow-up search failed: "+searchErr.Error())
				state.NoProgress++
				break
			}
			result = mergeKnowledgeSearchResponse(result, follow)
			for _, ev := range follow.Evidence {
				if ev.SourceID != "" && len(state.Sources) < budget.MaxSources {
					state.Sources[ev.SourceID] = struct{}{}
				}
			}
		case "read":
			if state.ReadCalls >= budget.MaxReadCalls || state.ReadRunes >= budget.MaxReadRunes {
				state.NoProgress++
				result.Warnings = appendUniqueString(result.Warnings, "Agentic canonical read budget exhausted")
				break
			}
			readReq, parent, validationErr := buildInvestigationReadRequest(action, result.Evidence, budget.MaxReadRunes-state.ReadRunes)
			if validationErr != nil {
				state.NoProgress++
				result.Warnings = appendUniqueString(result.Warnings, "Agentic read action rejected: "+validationErr.Error())
				break
			}
			readKey := investigationReadFingerprint(readReq)
			if _, duplicate := state.Reads[readKey]; duplicate {
				state.NoProgress++
				result.Warnings = appendUniqueString(result.Warnings, "Agentic planner proposed a duplicate canonical read")
				break
			}
			state.ReadCalls++
			state.Reads[readKey] = struct{}{}
			read, readErr := s.knowledgePort.Read(investigationCtx, tenantID, readReq)
			if readErr != nil {
				result.Degraded = true
				result.Warnings = appendUniqueString(result.Warnings, "Agentic canonical read failed: "+readErr.Error())
				state.NoProgress++
				break
			}
			state.ReadRunes += read.TotalRunes
			result.Evidence = mergeKnowledgeEvidence(result.Evidence, evidenceFromCanonicalRead(parent, read))
			if read.SourceID != "" && len(state.Sources) < budget.MaxSources {
				state.Sources[read.SourceID] = struct{}{}
			}
		default:
			state.NoProgress++
			result.Warnings = appendUniqueString(result.Warnings, "Agentic planner returned unsupported action")
		}

		knowledge.ApplySufficiency(userGoal, &result)
		progressAfter := investigationProgressFingerprint(result)
		if progressAfter == progressBefore && state.NoProgress == noProgressBefore && action.Action != "finish_sufficient" && action.Action != "finish_insufficient" {
			state.NoProgress++
		} else if progressAfter != progressBefore {
			state.NoProgress = 0
		}
		if stop || result.Sufficient {
			if result.Sufficient {
				state.Termination = "sufficient"
			}
			break
		}
		if state.NoProgress >= investigationNoProgressLimit {
			state.Termination = "no_progress"
			break
		}
	}
	if state.Termination == "" {
		state.Termination = "round_budget_exhausted"
	}
	if !result.Sufficient {
		result.Warnings = appendUniqueString(result.Warnings, "Agentic investigation stopped: "+state.Termination)
	}
	if investigationTask != nil {
		status := "completed"
		eventType := "investigation.completed"
		payloadStatus := "done"
		if state.Termination == "cancelled" || state.Termination == "deadline" {
			status = "interrupted"
			eventType = "investigation.interrupted"
			payloadStatus = "failed"
		} else if state.Termination == "planner_failed" {
			status = "failed"
			eventType = "investigation.failed"
			payloadStatus = "failed"
		}
		if err := trace.complete(ctx, investigationTask, eventType, status, "知识证据调查完成", map[string]any{
			"state": payloadStatus,
			"round": state.Round, "termination": state.Termination, "sufficient": result.Sufficient,
			"answerState": result.AnswerState, "sufficiencyReason": result.SufficiencyReason,
			"evidenceCount": len(result.Evidence), "searchCalls": state.SearchCalls, "readCalls": state.ReadCalls, "readRunes": state.ReadRunes,
		}); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	return result
}

func (s *service) nextKnowledgeInvestigationAction(ctx context.Context, userGoal string, result knowledge.SearchResponse, state investigationState, budget investigationBudget) (investigationAction, error) {
	if s != nil && s.investigationPlanner != nil {
		return s.investigationPlanner(ctx, userGoal, result, state, budget)
	}
	return s.planKnowledgeInvestigationAction(ctx, userGoal, result, state, budget)
}

func (s *service) planKnowledgeInvestigationAction(ctx context.Context, userGoal string, result knowledge.SearchResponse, state investigationState, budget investigationBudget) (investigationAction, error) {
	var action investigationAction
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return action, err
	}
	plannerCtx, cancel := context.WithTimeout(ctx, defaultInvestigationPlannerTimeout)
	defer cancel()

	evidence := make([]map[string]any, 0, minInt(len(result.Evidence), 12))
	for i, ev := range result.Evidence {
		if i >= 12 {
			break
		}
		evidence = append(evidence, map[string]any{
			"evidenceId": ev.ID, "unitId": ev.UnitID, "sourceId": ev.SourceID, "sourceRevision": ev.SourceRevision,
			"title": truncatePromptRunes(ev.Title, 240), "locator": ev.Locator, "coverageKeys": ev.CoverageKeys,
			"kind": ev.EvidenceKind, "confidence": ev.Confidence, "snippet": truncatePromptRunes(strings.TrimSpace(ev.Content), 900),
		})
	}
	statePayload := map[string]any{
		"goal": userGoal, "round": state.Round, "maxRounds": budget.MaxRounds,
		"sufficient": result.Sufficient, "answerState": result.AnswerState, "sufficiencyReason": result.SufficiencyReason,
		"evidence": evidence, "searchCalls": state.SearchCalls, "maxSearchCalls": budget.MaxSearchCalls,
		"readCalls": state.ReadCalls, "maxReadCalls": budget.MaxReadCalls,
		"readRunes": state.ReadRunes, "maxReadRunes": budget.MaxReadRunes,
	}
	rawState, _ := json.Marshal(statePayload)
	messages := []map[string]interface{}{
		{"role": "system", "content": `You are Amitia's bounded knowledge investigation planner. You do not answer the user. Choose exactly one next action using only the supplied untrusted evidence summary. Output one JSON object and nothing else. Allowed actions: read, search, finish_insufficient, finish_sufficient. Prefer read when exact wording/surrounding context is missing and an evidence/unit/source locator is available. Use search only when a materially different query is needed. Never invent IDs, file paths, tenants, revisions or locators. finish_sufficient is only a hint; the host will independently verify sufficiency. Schema: {"action":"read|search|finish_insufficient|finish_sufficient","reason":"short","query":"optional","sourceIds":["optional"],"evidenceId":"optional","unitId":"optional","sourceId":"optional","sourceRevision":1,"locator":{}}.`},
		{"role": "user", "content": string(rawState)},
	}
	reply, _, err := s.callLLMJSON(plannerCtx, cfg, messages)
	if err != nil {
		return action, err
	}
	jsonText := extractInvestigationJSONObject(reply)
	if jsonText == "" {
		return action, fmt.Errorf("planner returned no JSON object")
	}
	if err := json.Unmarshal([]byte(jsonText), &action); err != nil {
		return investigationAction{}, fmt.Errorf("planner JSON invalid: %w", err)
	}
	action.Action = strings.ToLower(strings.TrimSpace(action.Action))
	action.Reason = truncatePromptRunes(strings.TrimSpace(action.Reason), 400)
	action.Query = truncatePromptRunes(strings.TrimSpace(action.Query), 800)
	action.EvidenceID = strings.TrimSpace(action.EvidenceID)
	action.UnitID = strings.TrimSpace(action.UnitID)
	action.SourceID = strings.TrimSpace(action.SourceID)
	if action.Action != "read" && action.Action != "search" && action.Action != "finish_insufficient" && action.Action != "finish_sufficient" {
		return investigationAction{}, fmt.Errorf("unsupported planner action %q", action.Action)
	}
	return action, nil
}

func buildInvestigationReadRequest(action investigationAction, evidence []knowledge.Evidence, remainingRunes int) (knowledge.ReadRequest, knowledge.Evidence, error) {
	var parent knowledge.Evidence
	findEvidence := func() (knowledge.Evidence, bool) {
		for _, ev := range evidence {
			if action.EvidenceID != "" && ev.ID == action.EvidenceID {
				return ev, true
			}
			if action.UnitID != "" && ev.UnitID == action.UnitID {
				return ev, true
			}
		}
		return knowledge.Evidence{}, false
	}
	if ev, ok := findEvidence(); ok {
		parent = ev
		if strings.TrimSpace(ev.UnitID) != "" {
			action.UnitID = ev.UnitID
		}
		if action.SourceID != "" && action.SourceID != ev.SourceID {
			return knowledge.ReadRequest{}, parent, fmt.Errorf("sourceId does not match referenced evidence")
		}
		if action.SourceRevision > 0 && action.SourceRevision != ev.SourceRevision {
			return knowledge.ReadRequest{}, parent, fmt.Errorf("sourceRevision does not match referenced evidence")
		}
		action.SourceID = ev.SourceID
		action.SourceRevision = ev.SourceRevision
	} else if action.EvidenceID != "" || action.UnitID != "" {
		return knowledge.ReadRequest{}, parent, fmt.Errorf("referenced evidence/unit is not present in the current ledger")
	} else if action.SourceID != "" && len(action.Locator) > 0 {
		known := false
		for _, ev := range evidence {
			if ev.SourceID == action.SourceID && (action.SourceRevision <= 0 || ev.SourceRevision == action.SourceRevision) {
				parent = ev
				known = true
				break
			}
		}
		if !known {
			return knowledge.ReadRequest{}, parent, fmt.Errorf("source locator read must target a source already present in the evidence ledger")
		}
	} else {
		return knowledge.ReadRequest{}, parent, fmt.Errorf("read requires a known evidenceId/unitId or known sourceId+locator")
	}

	maxRunes := defaultInvestigationReadRunesCall
	if remainingRunes < maxRunes {
		maxRunes = remainingRunes
	}
	if maxRunes < 500 {
		return knowledge.ReadRequest{}, parent, fmt.Errorf("read rune budget exhausted")
	}
	return knowledge.ReadRequest{
		EvidenceID: action.EvidenceID, UnitID: action.UnitID, SourceID: action.SourceID, SourceRevision: action.SourceRevision,
		Locator: cloneAnyMap(action.Locator), BeforeUnits: 1, AfterUnits: 1, MaxUnits: 6, MaxRunes: maxRunes,
	}, parent, nil
}

func evidenceFromCanonicalRead(parent knowledge.Evidence, read knowledge.ReadResponse) []knowledge.Evidence {
	out := make([]knowledge.Evidence, 0, len(read.Units))
	for _, unit := range read.Units {
		content := strings.TrimSpace(unit.Content)
		if content == "" {
			continue
		}
		checksum := strings.TrimSpace(unit.Checksum)
		if checksum == "" {
			sum := sha256.Sum256([]byte(content))
			checksum = hex.EncodeToString(sum[:])
		}
		idSuffix := checksum
		if len(idSuffix) > 16 {
			idSuffix = idSuffix[:16]
		}
		id := "read:" + strings.TrimSpace(unit.ID) + ":" + idSuffix
		metadata := cloneAnyMap(unit.Metadata)
		if metadata == nil {
			metadata = map[string]any{}
		}
		if parent.ID != "" {
			metadata["parentEvidenceId"] = parent.ID
		}
		out = append(out, knowledge.Evidence{
			ID: id, UnitID: unit.ID, EvidenceKind: "canonical_read", CoverageKeys: coverageKeysFromLocator(unit.Locator), Checksum: checksum,
			SourceType: parent.SourceType, SourceID: read.SourceID, SourceRevision: read.SourceRevision,
			Title: firstNonEmptyString(unit.Title, parent.Title), Content: content, Locator: cloneAnyMap(unit.Locator),
			FusionScore: maxFloat(parent.FusionScore, 1.0/(60.0+1.0)), RerankScore: parent.RerankScore,
			Authority: parent.Authority, Confidence: parent.Confidence, Trust: parent.Trust,
			ValidFrom: parent.ValidFrom, ValidTo: parent.ValidTo, Supersedes: append([]string(nil), parent.Supersedes...),
			ConflictGroup: parent.ConflictGroup, Metadata: metadata,
		})
	}
	return out
}

func mergeKnowledgeSearchResponse(base, incoming knowledge.SearchResponse) knowledge.SearchResponse {
	base.Evidence = mergeKnowledgeEvidence(base.Evidence, incoming.Evidence)
	base.Degraded = base.Degraded || incoming.Degraded
	for _, warning := range incoming.Warnings {
		base.Warnings = appendUniqueString(base.Warnings, warning)
	}
	base.Ambiguous = base.Ambiguous || incoming.Ambiguous
	base.ConflictGroups = dedupePromptStrings(append(base.ConflictGroups, incoming.ConflictGroups...))
	base.QueryHistory = dedupePromptStrings(append(base.QueryHistory, incoming.QueryHistory...))
	return base
}

func mergeKnowledgeEvidence(base, incoming []knowledge.Evidence) []knowledge.Evidence {
	out := append([]knowledge.Evidence(nil), base...)
	index := map[string]int{}
	for i, ev := range out {
		index[evidenceMergeKey(ev)] = i
	}
	for _, ev := range incoming {
		key := evidenceMergeKey(ev)
		if i, exists := index[key]; exists {
			if ev.EvidenceKind == "canonical_read" || len([]rune(ev.Content)) > len([]rune(out[i].Content)) {
				out[i] = ev
			}
			continue
		}
		index[key] = len(out)
		out = append(out, ev)
	}
	return out
}

func evidenceMergeKey(ev knowledge.Evidence) string {
	if id := strings.TrimSpace(ev.ID); id != "" {
		return "id:" + id
	}
	if unit := strings.TrimSpace(ev.UnitID); unit != "" {
		return fmt.Sprintf("unit:%s:%d:%s", ev.SourceID, ev.SourceRevision, unit)
	}
	return fmt.Sprintf("source:%s:%d:%s", ev.SourceID, ev.SourceRevision, strings.TrimSpace(ev.Checksum))
}

func investigationProgressFingerprint(result knowledge.SearchResponse) string {
	keys := make([]string, 0, len(result.Evidence))
	for _, ev := range result.Evidence {
		keys = append(keys, evidenceMergeKey(ev)+"|"+strings.Join(ev.CoverageKeys, ",")+"|"+fmt.Sprintf("%d", len([]rune(ev.Content))))
	}
	sort.Strings(keys)
	return strings.Join(keys, "\n") + fmt.Sprintf("\n%s|%t", result.SufficiencyReason, result.Sufficient)
}

func investigationReadFingerprint(req knowledge.ReadRequest) string {
	locator, _ := json.Marshal(req.Locator)
	return fmt.Sprintf("%s|%s|%s|%d|%s", req.EvidenceID, req.UnitID, req.SourceID, req.SourceRevision, string(locator))
}

func boundedKnownSourceIDs(requested []string, evidence []knowledge.Evidence, limit int) []string {
	if len(requested) == 0 {
		return nil
	}
	known := map[string]struct{}{}
	for _, ev := range evidence {
		if id := strings.TrimSpace(ev.SourceID); id != "" {
			known[id] = struct{}{}
		}
	}
	out := make([]string, 0, len(requested))
	seen := map[string]struct{}{}
	for _, id := range requested {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := known[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func extractInvestigationJSONObject(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "```") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "```json"))
		value = strings.TrimSpace(strings.TrimPrefix(value, "```JSON"))
		value = strings.TrimSpace(strings.TrimPrefix(value, "```"))
		if idx := strings.LastIndex(value, "```"); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start < 0 || end < start {
		return ""
	}
	return value[start : end+1]
}

func normalizeInvestigationKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func investigationTerminationForContext(err error) string {
	if err == context.DeadlineExceeded {
		return "deadline"
	}
	return "cancelled"
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func cloneAnyMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncatePromptRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
func coverageKeysFromLocator(locator map[string]any) []string {
	keys := make([]string, 0, len(locator))
	for k, v := range locator {
		switch k {
		case "page", "pageNumber", "row", "rowStart", "rowEnd", "sheet", "sheetName", "paragraph", "section", "slide", "range":
			keys = append(keys, k+"="+fmt.Sprint(v))
		}
	}
	sort.Strings(keys)
	return keys
}
func dedupePromptStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
