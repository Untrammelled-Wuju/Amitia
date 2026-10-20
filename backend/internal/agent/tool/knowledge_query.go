// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	knowledgesvc "github.com/u-ai/backend/internal/knowledge"
)

var toolKnowledgeService knowledgesvc.Service

// SetKnowledgeService wires the authenticated, tenant-global knowledge service into
// the built-in agent tool runtime. Character IDs are intentionally not part of the
// service call: Knowledge and WorldBook are separate domains.
func SetKnowledgeService(svc knowledgesvc.Service) {
	toolKnowledgeService = svc
}

type knowledgeToolEvidence struct {
	ID             string         `json:"id"`
	UnitID         string         `json:"unitId,omitempty"`
	EvidenceKind   string         `json:"evidenceKind,omitempty"`
	CoverageKeys   []string       `json:"coverageKeys,omitempty"`
	Checksum       string         `json:"checksum,omitempty"`
	SourceID       string         `json:"sourceId"`
	SourceRevision int64          `json:"sourceRevision"`
	Title          string         `json:"title,omitempty"`
	Content        string         `json:"content"`
	Locator        map[string]any `json:"locator,omitempty"`
	FusionScore    float64        `json:"fusionScore,omitempty"`
	Confidence     float64        `json:"confidence,omitempty"`
	Trust          string         `json:"trust,omitempty"`
	ValidFrom      *time.Time     `json:"validFrom,omitempty"`
	ValidTo        *time.Time     `json:"validTo,omitempty"`
	Supersedes     []string       `json:"supersedes,omitempty"`
	ConflictGroup  string         `json:"conflictGroup,omitempty"`
}

type knowledgeToolEnvelope struct {
	Query             string                  `json:"query"`
	RetrievalLevel    string                  `json:"retrievalLevel"`
	QueryComplexity   string                  `json:"queryComplexity"`
	Count             int                     `json:"count"`
	Sufficient        bool                    `json:"sufficient"`
	AnswerState       string                  `json:"answerState"`
	SufficiencyReason string                  `json:"sufficiencyReason"`
	Degraded          bool                    `json:"degraded"`
	TemporalMode      string                  `json:"temporalMode"`
	AsOf              *time.Time              `json:"asOf,omitempty"`
	Ambiguous         bool                    `json:"ambiguous"`
	ConflictGroups    []string                `json:"conflictGroups,omitempty"`
	Warnings          []string                `json:"warnings,omitempty"`
	Evidence          []knowledgeToolEvidence `json:"evidence"`
}

func init() {
	params, err := ParseParametersSchema(json.RawMessage(`{
		"type":"object",
		"required":["query"],
		"additionalProperties":false,
		"properties":{
			"query":{"type":"string","minLength":1,"maxLength":4096},
			"limit":{"type":"integer","minimum":1,"maximum":30},
			"source_ids":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":128}},
			"min_score":{"type":"number","minimum":0,"maximum":1},
			"retrieval_level":{"type":"string","enum":["auto","none","fast","deep","agentic"]},
			"temporal_mode":{"type":"string","enum":["current","historical","all"]},
			"as_of":{"type":"string","description":"RFC3339 timestamp required when temporal_mode=historical"}
		}
	}`))
	if err != nil {
		panic(err)
	}
	Register(Tool{Type: "function", Function: Function{
		Name:        "search_knowledge",
		Description: "Search the authenticated space's shared knowledge base with hybrid dense, lexical and exact retrieval, including current or explicit historical as-of queries. Knowledge is global to the current space and is never filtered by character.",
		Parameters:  params,
	}}, searchKnowledge)

	readParams, err := ParseParametersSchema(json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"properties":{
			"evidence_id":{"type":"string","maxLength":256},
			"unit_id":{"type":"string","maxLength":256},
			"source_id":{"type":"string","maxLength":256},
			"source_revision":{"type":"integer","minimum":1},
			"locator":{"type":"object","maxProperties":16,"additionalProperties":true},
			"before_units":{"type":"integer","minimum":0,"maximum":16},
			"after_units":{"type":"integer","minimum":0,"maximum":16},
			"max_units":{"type":"integer","minimum":1,"maximum":32},
			"max_runes":{"type":"integer","minimum":500,"maximum":64000}
		},
		"anyOf":[
			{"required":["evidence_id"]},
			{"required":["unit_id"]},
			{"required":["source_id","locator"]}
		]
	}`))
	if err != nil {
		panic(err)
	}
	Register(Tool{Type: "function", Function: Function{
		Name:        "read_knowledge",
		Description: "Read canonical source text around evidence, a parsed unit, or a structured source locator. Use after search_knowledge when exact wording, surrounding context, a specific page/row/section, or multi-document verification is needed. This cannot access arbitrary filesystem paths.",
		Parameters:  readParams,
	}}, readKnowledge)
}

func searchKnowledge(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	if toolKnowledgeService == nil {
		return ErrorResult("service_not_initialized", "ERROR: knowledge service not initialized")
	}
	spaceID, scopeErr := effectiveToolSpaceID(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	query := strings.TrimSpace(stringArg(args, "query"))
	if query == "" {
		return ErrorResult("invalid_args", "ERROR: query is required")
	}
	if utf8.RuneCountInString(query) > 4096 {
		return ErrorResult("invalid_args", "ERROR: query is too long")
	}
	limit := boundedIntArg(args, "limit", 8, 1, 30)
	sourceIDs := dedupeBoundedStrings(stringSliceArg(args, "source_ids", 50), 50, 128)
	minScore, err := boundedFloatArg(args, "min_score", 0, 0, 1)
	if err != nil {
		return ErrorResult("invalid_args", "ERROR: "+err.Error())
	}
	temporalMode := strings.ToLower(strings.TrimSpace(stringArg(args, "temporal_mode")))
	if temporalMode == "" {
		temporalMode = knowledgesvc.TemporalModeCurrent
	}
	if temporalMode != knowledgesvc.TemporalModeCurrent && temporalMode != knowledgesvc.TemporalModeHistorical && temporalMode != knowledgesvc.TemporalModeAll {
		return ErrorResult("invalid_args", "ERROR: temporal_mode must be current, historical, or all")
	}
	var asOf *time.Time
	if raw := strings.TrimSpace(stringArg(args, "as_of")); raw != "" {
		parsed, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil {
			return ErrorResult("invalid_args", "ERROR: as_of must be RFC3339")
		}
		parsed = parsed.UTC()
		asOf = &parsed
	}
	if temporalMode == knowledgesvc.TemporalModeHistorical && asOf == nil {
		return ErrorResult("invalid_args", "ERROR: historical temporal_mode requires as_of")
	}
	retrievalLevel := strings.ToLower(strings.TrimSpace(stringArg(args, "retrieval_level")))
	if retrievalLevel == "" {
		retrievalLevel = knowledgesvc.RetrievalLevelAuto
	}
	switch retrievalLevel {
	case knowledgesvc.RetrievalLevelAuto, knowledgesvc.RetrievalLevelNone, knowledgesvc.RetrievalLevelFast, knowledgesvc.RetrievalLevelDeep, knowledgesvc.RetrievalLevelAgentic:
	default:
		return ErrorResult("invalid_args", "ERROR: retrieval_level must be auto, none, fast, deep, or agentic")
	}

	response, err := toolKnowledgeService.Search(callCtx, spaceID, knowledgesvc.SearchRequest{
		Query: query, RetrievalLevel: retrievalLevel, Limit: limit, SourceIDs: sourceIDs, MinScore: minScore, TemporalMode: temporalMode, AsOf: asOf,
	})
	if err != nil {
		if callCtx.Err() != nil {
			return CancelledResult(callCtx.Err().Error())
		}
		return ErrorResult("knowledge_search_failed", "ERROR: knowledge search failed")
	}

	envelope := knowledgeToolEnvelope{
		Query: query, RetrievalLevel: response.RetrievalLevel, QueryComplexity: response.QueryComplexity, Sufficient: response.Sufficient, AnswerState: response.AnswerState, SufficiencyReason: response.SufficiencyReason, Degraded: response.Degraded,
		TemporalMode: response.TemporalMode, AsOf: response.AsOf, Ambiguous: response.Ambiguous,
		ConflictGroups: append([]string(nil), response.ConflictGroups...),
		Warnings:       append([]string(nil), response.Warnings...),
		Evidence:       make([]knowledgeToolEvidence, 0, len(response.Evidence)),
	}
	for _, evidence := range response.Evidence {
		if len(envelope.Evidence) >= limit {
			break
		}
		envelope.Evidence = append(envelope.Evidence, knowledgeToolEvidence{
			ID: evidence.ID, UnitID: evidence.UnitID, EvidenceKind: evidence.EvidenceKind, CoverageKeys: append([]string(nil), evidence.CoverageKeys...), Checksum: evidence.Checksum,
			SourceID: evidence.SourceID, SourceRevision: evidence.SourceRevision,
			Title: truncateRunes(evidence.Title, 512), Content: truncateRunes(evidence.Content, 12000),
			Locator: evidence.Locator, FusionScore: evidence.FusionScore, Confidence: evidence.Confidence, Trust: evidence.Trust,
			ValidFrom: evidence.ValidFrom, ValidTo: evidence.ValidTo, Supersedes: append([]string(nil), evidence.Supersedes...), ConflictGroup: evidence.ConflictGroup,
		})
	}
	envelope.Count = len(envelope.Evidence)
	body, err := json.Marshal(envelope)
	if err != nil {
		return ErrorResult("knowledge_result_encode_failed", "ERROR: failed to encode knowledge result")
	}
	result := TextResult(string(body))
	result.Audit = map[string]interface{}{
		"operation": "search", "space_id": spaceID, "evidence_count": envelope.Count,
		"retrieval_level": envelope.RetrievalLevel, "query_complexity": envelope.QueryComplexity,
		"degraded": envelope.Degraded, "sufficient": envelope.Sufficient, "answer_state": envelope.AnswerState, "sufficiency_reason": envelope.SufficiencyReason,
	}
	return result
}

func readKnowledge(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	if toolKnowledgeService == nil {
		return ErrorResult("service_not_initialized", "ERROR: knowledge service not initialized")
	}
	spaceID, scopeErr := effectiveToolSpaceID(execCtx)
	if scopeErr != nil {
		return *scopeErr
	}
	locator := map[string]any{}
	if raw, ok := args["locator"]; ok && raw != nil {
		switch typed := raw.(type) {
		case map[string]interface{}:
			locator = typed
		default:
			return ErrorResult("invalid_args", "ERROR: locator must be an object")
		}
	}
	req := knowledgesvc.ReadRequest{
		EvidenceID:     strings.TrimSpace(stringArg(args, "evidence_id")),
		UnitID:         strings.TrimSpace(stringArg(args, "unit_id")),
		SourceID:       strings.TrimSpace(stringArg(args, "source_id")),
		SourceRevision: int64(boundedIntArg(args, "source_revision", 0, 0, 1<<30)),
		Locator:        locator,
		BeforeUnits:    boundedIntArg(args, "before_units", 1, 0, 16),
		AfterUnits:     boundedIntArg(args, "after_units", 1, 0, 16),
		MaxUnits:       boundedIntArg(args, "max_units", 6, 1, 32),
		MaxRunes:       boundedIntArg(args, "max_runes", 16000, 500, 64000),
	}
	response, err := toolKnowledgeService.Read(callCtx, spaceID, req)
	if err != nil {
		if callCtx.Err() != nil {
			return CancelledResult(callCtx.Err().Error())
		}
		return ErrorResult("knowledge_read_failed", "ERROR: knowledge canonical read failed: "+err.Error())
	}
	body, err := json.Marshal(response)
	if err != nil {
		return ErrorResult("knowledge_result_encode_failed", "ERROR: failed to encode knowledge read result")
	}
	result := TextResult(string(body))
	result.Audit = map[string]interface{}{
		"operation": "read", "space_id": spaceID, "source_id": response.SourceID,
		"source_revision": response.SourceRevision, "anchor_unit_id": response.AnchorUnitID,
		"unit_count": len(response.Units), "total_runes": response.TotalRunes, "truncated": response.Truncated,
	}
	return result
}

func boundedFloatArg(args map[string]interface{}, key string, fallback, low, high float64) (float64, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return fallback, nil
	}
	var parsed float64
	switch typed := value.(type) {
	case float64:
		parsed = typed
	case float32:
		parsed = float64(typed)
	case int:
		parsed = float64(typed)
	case int64:
		parsed = float64(typed)
	case json.Number:
		v, err := typed.Float64()
		if err != nil {
			return 0, fmt.Errorf("%s must be a number", key)
		}
		parsed = v
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
	if parsed < low || parsed > high {
		return 0, fmt.Errorf("%s must be between %.2f and %.2f", key, low, high)
	}
	return parsed, nil
}

func dedupeBoundedStrings(values []string, maxItems, maxRunes int) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > maxRunes {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "…"
}
