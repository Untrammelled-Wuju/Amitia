package business

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memorypolicy"
)

type MemoryConsolidationReport struct {
	Scope              coordination.ExecutionScope `json:"executionScope"`
	Total              int                         `json:"total"`
	DuplicateGroups    []MemoryConsolidationGroup  `json:"duplicateGroups"`
	KeyConflicts       []MemoryConsolidationGroup  `json:"keyConflicts"`
	DisputedCount      int                         `json:"disputedCount"`
	ExpiredCount       int                         `json:"expiredCount"`
	AutoMergedCount    int                         `json:"autoMergedCount"`
	ClusterGroups      []MemoryConsolidationGroup  `json:"clusterGroups"`
	ReviewSuggestions  int                         `json:"reviewSuggestions"`
	ReviewItems        []MemoryReviewItem          `json:"reviewItems"`
	SummarySuggestions []MemorySummarySuggestion   `json:"summarySuggestions"`
	Suggestions        int                         `json:"suggestions"`
	GeneratedAt        string                      `json:"generatedAt"`
}

type MemoryConsolidationGroup struct {
	Kind   string   `json:"kind"`
	Key    string   `json:"key"`
	Value  string   `json:"value,omitempty"`
	IDs    []string `json:"ids"`
	Reason string   `json:"reason"`
}

type memoryConsolidationEntry struct {
	id, kind, key, value, status, source, role string
	importance, confidence                     float64
	recallProbability                          float64
	nextReviewAt                               string
	usable, autoSafe                           bool
}

type MemoryReviewItem struct {
	ID                string  `json:"id"`
	Kind              string  `json:"kind"`
	Key               string  `json:"key"`
	RecallProbability float64 `json:"recallProbability"`
	NextReviewAt      string  `json:"nextReviewAt,omitempty"`
	Reason            string  `json:"reason"`
}

type MemorySummarySuggestion struct {
	Kind    string   `json:"kind"`
	IDs     []string `json:"ids"`
	Summary string   `json:"summary"`
}

func (e *Engine) OwnedMemoryConsolidation(ctx context.Context, request Request) (MemoryConsolidationReport, error) {
	return e.ownedMemoryConsolidation(ctx, request, false)
}

func (e *Engine) ConsolidateOwnedMemories(ctx context.Context, request Request) (MemoryConsolidationReport, error) {
	return e.ownedMemoryConsolidation(ctx, request, true)
}

func (e *Engine) ownedMemoryConsolidation(ctx context.Context, request Request, apply bool) (MemoryConsolidationReport, error) {
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryConsolidationReport{}, err
	}
	defer finish()
	snapshot, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return MemoryConsolidationReport{}, err
	}
	now := time.Now()
	entries := make([]memoryConsolidationEntry, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		var document map[string]json.RawMessage
		if err := json.Unmarshal(resource.Body, &document); err != nil {
			return MemoryConsolidationReport{}, err
		}
		var content map[string]json.RawMessage
		if err := json.Unmarshal(document["content"], &content); err != nil {
			return MemoryConsolidationReport{}, err
		}
		kind := MemoryText(document, "kind")
		if kind == "" {
			kind = "fact"
		}
		status := MemoryText(content, "verifiedStatus")
		if status == "" {
			status = MemoryText(document, "verifiedStatus")
		}
		source := MemoryText(content, "source")
		if source == "" {
			source = MemoryText(document, "source")
		}
		value := MemoryValue(content)
		lifecycle := memorypolicy.ReadLifecycle(document, now).Effective(now)
		autoSafe := source == "extraction" && status != "verified" && status != "user_corrected" && status != "disputed" && !lifecycle.Pinned && coordination.ResourceUsable(resource.Body, now) && len([]rune(value)) >= 8
		entries = append(entries, memoryConsolidationEntry{
			id: resource.ID, kind: kind, key: memoryNormalizeConsolidationText(MemoryText(document, "key")), value: memoryNormalizeConsolidationText(value),
			status: status, source: source, role: resource.RoleID, importance: memoryNumber(content["importance"], 0), confidence: memoryNumber(content["confidence"], 50),
			recallProbability: lifecycle.RecallProbability, nextReviewAt: lifecycle.NextReviewAt, usable: coordination.ResourceRecallable(resource.Body, now), autoSafe: autoSafe,
		})
	}
	duplicates := map[string][]memoryConsolidationEntry{}
	keys := map[string][]memoryConsolidationEntry{}
	clusters := map[string][]memoryConsolidationEntry{}
	report := MemoryConsolidationReport{Scope: scope, Total: len(entries), DuplicateGroups: []MemoryConsolidationGroup{}, KeyConflicts: []MemoryConsolidationGroup{}, ClusterGroups: []MemoryConsolidationGroup{}, ReviewItems: []MemoryReviewItem{}, SummarySuggestions: []MemorySummarySuggestion{}, GeneratedAt: now.UTC().Format(time.RFC3339Nano)}
	for _, item := range entries {
		if item.status == "disputed" {
			report.DisputedCount++
		}
		if !item.usable {
			report.ExpiredCount++
		}
		if item.value != "" {
			duplicates[item.kind+"\x00"+item.value] = append(duplicates[item.kind+"\x00"+item.value], item)
		}
		if item.key != "" {
			keys[item.kind+"\x00"+item.key] = append(keys[item.kind+"\x00"+item.key], item)
		}
		if len([]rune(item.value)) >= 4 {
			prefix := string([]rune(item.value)[:2])
			clusters[item.kind+"\x00"+prefix] = append(clusters[item.kind+"\x00"+prefix], item)
		}
		if item.importance >= 7 && item.recallProbability < .45 && item.usable {
			report.ReviewSuggestions++
			report.ReviewItems = append(report.ReviewItems, MemoryReviewItem{ID: item.id, Kind: item.kind, Key: item.key, RecallProbability: item.recallProbability, NextReviewAt: item.nextReviewAt, Reason: "high_value_low_recall_probability"})
		}
	}
	report.DuplicateGroups = consolidationGroups(duplicates, true)
	report.KeyConflicts = consolidationGroups(keys, false)
	report.ClusterGroups = consolidationGroups(clusters, false)
	for _, items := range clusters {
		if len(items) < 2 {
			continue
		}
		ids := make([]string, 0, len(items))
		values := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.id)
			values = append(values, item.value)
		}
		sort.Strings(ids)
		report.SummarySuggestions = append(report.SummarySuggestions, MemorySummarySuggestion{Kind: items[0].kind, IDs: ids, Summary: strings.Join(values, "；")})
	}
	report.Suggestions = len(report.DuplicateGroups) + len(report.KeyConflicts) + len(report.ClusterGroups)
	if apply {
		mutations, merged, err := automaticMemoryMergeMutations(snapshot.Resources, duplicates, now)
		if err != nil {
			return MemoryConsolidationReport{}, err
		}
		if len(mutations) > 0 {
			if _, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: mutations}); err != nil {
				return MemoryConsolidationReport{}, err
			}
		}
		report.AutoMergedCount = merged
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return MemoryConsolidationReport{}, err
	}
	return report, nil
}

func automaticMemoryMergeMutations(resources []coordination.Resource, duplicates map[string][]memoryConsolidationEntry, now time.Time) ([]coordination.Mutation, int, error) {
	mutations := []coordination.Mutation{}
	nowText := now.UTC().Format(time.RFC3339Nano)
	for _, group := range duplicates {
		if len(group) < 2 {
			continue
		}
		safe := true
		for _, item := range group {
			safe = safe && item.autoSafe
		}
		if !safe {
			continue
		}
		primary := group[0]
		for _, item := range group[1:] {
			if item.importance > primary.importance || item.importance == primary.importance && item.confidence > primary.confidence || item.importance == primary.importance && item.confidence == primary.confidence && item.id < primary.id {
				primary = item
			}
		}
		for _, item := range group {
			if item.id == primary.id {
				continue
			}
			resource := memoryConsolidationResource(resources, item.id)
			if resource == nil {
				continue
			}
			var document map[string]json.RawMessage
			if err := json.Unmarshal(resource.Body, &document); err != nil {
				return nil, 0, err
			}
			document["allowContextUse"], document["archivedAt"] = body(false), body(nowText)
			document["consolidatedInto"], document["consolidatedAt"], document["consolidationReason"] = body(primary.id), body(nowText), body("exact-duplicate-content")
			mutations = append(mutations, coordination.Mutation{Kind: "memory", ID: item.id, RoleID: item.role, ExpectedRevision: resource.Revision, Body: body(document)})
		}
	}
	return mutations, len(mutations), nil
}

func memoryConsolidationResource(resources []coordination.Resource, id string) *coordination.Resource {
	for index := range resources {
		if resources[index].Kind == "memory" && resources[index].ID == id {
			return &resources[index]
		}
	}
	return nil
}

func consolidationGroups(groups map[string][]memoryConsolidationEntry, includeValue bool) []MemoryConsolidationGroup {
	result := []MemoryConsolidationGroup{}
	for _, entries := range groups {
		if len(entries) < 2 {
			continue
		}
		ids := make([]string, 0, len(entries))
		for _, item := range entries {
			ids = append(ids, item.id)
		}
		sort.Strings(ids)
		group := MemoryConsolidationGroup{Kind: entries[0].kind, Key: entries[0].key, IDs: ids, Reason: "same-key-conflict"}
		if includeValue {
			group.Value, group.Key, group.Reason = entries[0].value, "", "duplicate-content"
		}
		result = append(result, group)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		if result[i].Key != result[j].Key {
			return result[i].Key < result[j].Key
		}
		return result[i].Value < result[j].Value
	})
	return result
}

func memoryNormalizeConsolidationText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
