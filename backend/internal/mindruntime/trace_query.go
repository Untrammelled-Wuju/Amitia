package mindruntime

import (
	"sort"
	"strings"
	"time"
)

type CausalChainFilter struct {
	Kinds         []TraceEventKind
	Status        string
	Reason        string
	CharacterID   string
	InteractionID string
	DeliveryID    string
	ToolID        string
	EventID       string
	Scope         string
}

type MetricAggregation struct {
	Name  RuntimeMetricName `json:"name"`
	Count int               `json:"count"`
	Sum   int64             `json:"sum"`
	Avg   float64           `json:"avg"`
	Min   int64             `json:"min"`
	Max   int64             `json:"max"`
}

type CausalChainSummary struct {
	TotalEvents     int                    `json:"totalEvents"`
	ByKind          map[TraceEventKind]int `json:"byKind"`
	ByStatus        map[string]int         `json:"byStatus"`
	CancelledCount  int                    `json:"cancelledCount"`
	SupersededCount int                    `json:"supersededCount"`
	ToolCalls       int                    `json:"toolCalls"`
	Deliveries      int                    `json:"deliveries"`
	Compensations   int                    `json:"compensations"`
	Validations     int                    `json:"validations"`
	Duration        time.Duration          `json:"duration"`
}

type TraceQueryResult struct {
	Events   []RuntimeCausalEvent `json:"events"`
	Total    int                  `json:"total"`
	Filtered int                  `json:"filtered"`
}

type RuntimeExtendedQuery struct {
	Kind          TraceEventKind
	Status        string
	RequestID     string
	EventID       string
	InteractionID string
	DeliveryID    string
	ToolID        string
	CharacterID   string
	Scope         string
	MinQueueMs    int64
	MinBudgetUsed float64
	MinCandidates int
}

func FilterCausalChain(events []RuntimeCausalEvent, filter CausalChainFilter) TraceQueryResult {
	result := TraceQueryResult{Total: len(events)}
	if len(events) == 0 {
		return result
	}
	filtered := make([]RuntimeCausalEvent, 0, len(events))
	for _, event := range events {
		if !matchKindFilter(event.Kind, filter.Kinds) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(strings.TrimSpace(event.Status), filter.Status) {
			continue
		}
		if filter.Reason != "" && !strings.Contains(strings.ToLower(event.Reason), strings.ToLower(strings.TrimSpace(filter.Reason))) {
			continue
		}
		if filter.Scope != "" && !strings.EqualFold(strings.TrimSpace(event.Scope), filter.Scope) {
			continue
		}
		filtered = append(filtered, event)
	}
	result.Filtered = len(filtered)
	if len(filtered) == 0 {
		return result
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Index < filtered[j].Index
	})
	result.Events = filtered
	return result
}

func QueryCausalChain(events []RuntimeCausalEvent, query RuntimeExtendedQuery) TraceQueryResult {
	result := TraceQueryResult{Total: len(events)}
	if len(events) == 0 {
		return result
	}
	filtered := make([]RuntimeCausalEvent, 0, len(events))
	for _, event := range events {
		if query.Kind != "" && event.Kind != query.Kind {
			continue
		}
		if query.Status != "" && !strings.EqualFold(strings.TrimSpace(event.Status), query.Status) {
			continue
		}
		if query.Scope != "" && !strings.EqualFold(strings.TrimSpace(event.Scope), query.Scope) {
			continue
		}
		if query.MinQueueMs > 0 && event.QueueDurationMs < query.MinQueueMs {
			continue
		}
		if query.MinBudgetUsed > 0 && event.BudgetUsed < query.MinBudgetUsed {
			continue
		}
		if query.MinCandidates > 0 && event.CandidateCount < query.MinCandidates {
			continue
		}
		filtered = append(filtered, event)
	}
	result.Filtered = len(filtered)
	if len(filtered) == 0 {
		return result
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Index < filtered[j].Index
	})
	result.Events = filtered
	return result
}

func QueryCausalChainByIDs(events []RuntimeCausalEvent, query RuntimeExtendedQuery) TraceQueryResult {
	result := TraceQueryResult{Total: len(events)}
	if len(events) == 0 {
		return result
	}
	filtered := make([]RuntimeCausalEvent, 0, len(events))
	for _, event := range events {
		if query.RequestID != "" && event.ID != query.RequestID && event.Kind != TraceEventRequest {
			continue
		}
		if query.EventID != "" && event.ID != query.EventID {
			continue
		}
		filtered = append(filtered, event)
	}
	result.Filtered = len(filtered)
	if len(filtered) == 0 {
		return result
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Index < filtered[j].Index
	})
	result.Events = filtered
	return result
}

func matchKindFilter(kind TraceEventKind, kinds []TraceEventKind) bool {
	if len(kinds) == 0 {
		return true
	}
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func AggregateRuntimeMetrics(metrics []RuntimeMetric) []MetricAggregation {
	if len(metrics) == 0 {
		return nil
	}
	groups := make(map[RuntimeMetricName][]int64)
	for _, m := range metrics {
		groups[m.Name] = append(groups[m.Name], m.Value)
	}
	names := make([]RuntimeMetricName, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return string(names[i]) < string(names[j])
	})
	result := make([]MetricAggregation, 0, len(names))
	for _, name := range names {
		values := groups[name]
		agg := MetricAggregation{Name: name, Count: len(values)}
		if len(values) == 0 {
			result = append(result, agg)
			continue
		}
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		agg.Min = values[0]
		agg.Max = values[len(values)-1]
		var total int64
		for _, v := range values {
			total += v
		}
		agg.Sum = total
		agg.Avg = float64(total) / float64(len(values))
		result = append(result, agg)
	}
	return result
}

func BuildCausalChainSummary(events []RuntimeCausalEvent, totalDuration time.Duration) CausalChainSummary {
	summary := CausalChainSummary{
		TotalEvents: len(events),
		ByKind:      make(map[TraceEventKind]int),
		ByStatus:    make(map[string]int),
		Duration:    totalDuration,
	}
	for _, event := range events {
		summary.ByKind[event.Kind]++
		if event.Status != "" {
			summary.ByStatus[event.Status]++
		}
		switch event.Kind {
		case TraceEventCancel:
			summary.CancelledCount++
		case TraceEventSuperseded:
			summary.SupersededCount++
		case TraceEventTool:
			summary.ToolCalls++
		case TraceEventDelivery:
			summary.Deliveries++
		case TraceEventCompensation:
			summary.Compensations++
		case TraceEventValidation:
			summary.Validations++
		}
	}
	return summary
}

func FilterByCharacter(events []RuntimeCausalEvent, characterID string) []RuntimeCausalEvent {
	if len(events) == 0 || characterID == "" {
		return events
	}
	result := make([]RuntimeCausalEvent, 0, len(events))
	for _, e := range events {
		if strings.EqualFold(strings.TrimSpace(e.Scope), characterID) {
			result = append(result, e)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Index < result[j].Index
	})
	return result
}

func FilterByDelivery(events []RuntimeCausalEvent, deliveryID string) []RuntimeCausalEvent {
	if len(events) == 0 || deliveryID == "" {
		return events
	}
	result := make([]RuntimeCausalEvent, 0, len(events))
	for _, e := range events {
		if e.Kind == TraceEventDelivery && strings.EqualFold(strings.TrimSpace(e.ID), deliveryID) {
			result = append(result, e)
		}
	}
	return result
}

func FilterByTool(events []RuntimeCausalEvent, toolID string) []RuntimeCausalEvent {
	if len(events) == 0 || toolID == "" {
		return events
	}
	result := make([]RuntimeCausalEvent, 0, len(events))
	for _, e := range events {
		if e.Kind == TraceEventTool && strings.EqualFold(strings.TrimSpace(e.ID), toolID) {
			result = append(result, e)
		}
	}
	return result
}
