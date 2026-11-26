package capability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const ExactRouteVersion = "exact-route-v1"

type ExactRouteIndex struct {
	mu       sync.RWMutex
	byKey    map[string][]string
	hits     atomic.Int64
	misses   atomic.Int64
	rebuilds atomic.Int64
	keys     atomic.Int64
}

func NewExactRouteIndex() *ExactRouteIndex {
	return &ExactRouteIndex{byKey: map[string][]string{}}
}

func normalizeExactKey(text string) string {
	trimmed := strings.ToLower(strings.TrimSpace(text))
	trimmed = strings.TrimRight(trimmed, "。.!！?？")
	return strings.Join(strings.Fields(trimmed), " ")
}

func exactKeysFor(tool ToolDefinition) []string {
	values := make([]string, 0, 6)
	for _, field := range []string{"routes", "route", "aliases", "alias", "types", "type"} {
		for _, item := range toolMetadataStrings(tool.Metadata, field) {
			values = append(values, item)
		}
	}
	return values
}

func (i *ExactRouteIndex) Rebuild(defs []ToolDefinition) {
	if i == nil {
		return
	}
	byKey := map[string][]string{}
	for _, tool := range defs {
		for _, value := range exactKeysFor(tool) {
			key := normalizeExactKey(value)
			if key == "" {
				continue
			}
			byKey[key] = append(byKey[key], tool.ID)
		}
	}
	for key, ids := range byKey {
		sort.Strings(ids)
		byKey[key] = dedupeStrings(ids)
	}
	i.mu.Lock()
	i.byKey = byKey
	i.mu.Unlock()
	i.rebuilds.Add(1)
	i.keys.Store(int64(len(byKey)))
}

func dedupeStrings(sorted []string) []string {
	result := sorted[:0]
	for _, value := range sorted {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func (i *ExactRouteIndex) Lookup(query string) ([]string, bool) {
	if i == nil {
		return nil, false
	}
	key := normalizeExactKey(query)
	if key == "" {
		i.misses.Add(1)
		return nil, false
	}
	i.mu.RLock()
	ids, ok := i.byKey[key]
	i.mu.RUnlock()
	if !ok {
		i.misses.Add(1)
		return nil, false
	}
	i.hits.Add(1)
	return append([]string{}, ids...), true
}

func (i *ExactRouteIndex) Hits() int64 {
	if i == nil {
		return 0
	}
	return i.hits.Load()
}

func (i *ExactRouteIndex) Misses() int64 {
	if i == nil {
		return 0
	}
	return i.misses.Load()
}

func (i *ExactRouteIndex) Keys() int {
	if i == nil {
		return 0
	}
	return int(i.keys.Load())
}

func (s *TaskSelector) WithExactRoutes(index *ExactRouteIndex) *TaskSelector {
	s.exact = index
	return s
}

func exactDigest(ids []string) string {
	sum := sha256.Sum256([]byte(ExactRouteVersion + "|" + strings.Join(ids, ",")))
	return hex.EncodeToString(sum[:])
}

func (s *TaskSelector) selectExact(ctx context.Context, profile TaskProfile, invocation ToolInvocationContext, ids []string, limit int) (SelectionReport, error) {
	report := SelectionReport{Profile: profile, Selected: []SelectedTool{}, Rejected: []RejectedTool{}}
	report.Strategy = StrategySnapshot{Version: ExactRouteVersion, Weights: RankingWeights{Explicit: 1}, Digest: exactDigest(ids)}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return SelectionReport{Profile: profile}, err
		}
		tool, ok := s.registry.Get(ctx, id)
		if !ok {
			report.Rejected = append(report.Rejected, RejectedTool{ToolID: id, Reasons: []AvailabilityReason{ReasonNotInstalled}})
			continue
		}
		reasons, cause, err := s.hardFilter(ctx, tool, invocation)
		if err != nil {
			return SelectionReport{Profile: profile}, err
		}
		if len(reasons) > 0 {
			report.Rejected = append(report.Rejected, RejectedTool{ToolID: tool.ID, ModelName: tool.ModelName, Reasons: reasons, Cause: cause})
			continue
		}
		report.Selected = append(report.Selected, SelectedTool{
			Tool:    tool,
			Score:   0,
			Rank:    1,
			Matched: []string{"exact:" + ExactRouteVersion},
			Domains: []TaskDomain{},
			Signals: map[string]float64{"exact": 1},
		})
	}
	if limit > 0 && len(report.Selected) > limit {
		report.Selected = report.Selected[:limit]
	}
	return report, nil
}

func (s *TaskSelector) exactLookup(ctx context.Context, query string) ([]string, bool, error) {
	if s == nil || s.exact == nil {
		return nil, false, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	ids, ok := s.exact.Lookup(query)
	if !ok {
		return nil, false, nil
	}
	if len(ids) == 0 {
		return nil, false, fmt.Errorf("%w: exact key resolved to no tools", ErrTaskSelectorInvalid)
	}
	return ids, true, nil
}
