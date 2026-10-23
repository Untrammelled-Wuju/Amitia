package system

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/mindruntime"
)

func usageValues(row mindruntime.UsageBucket) map[string]any {
	var tokens any = row.Tokens
	if row.Unknown > 0 {
		tokens = nil
	}
	return map[string]any{"modelCalls": row.Calls, "count": row.Calls, "succeeded": row.Succeeded, "failed": row.Failed, "cancelled": row.Cancelled, "running": row.Running, "interrupted": row.Interrupted, "inputTokens": row.Input, "outputTokens": row.Output, "cachedInputTokens": row.Cached, "knownTokens": row.Tokens, "unknownUsageCalls": row.Unknown, "tokens": tokens, "totalTokens": tokens, "totalCost": nil, "costStatus": "unavailable"}
}

func (s *service) UsageContext(ctx context.Context, filter mindruntime.UsageFilter, group string) (map[string]any, error) {
	if s.usageStore == nil {
		return nil, errors.New("模型使用统计未就绪")
	}
	rows, baseline, err := s.usageStore.Aggregate(ctx, filter, group)
	if err != nil {
		return nil, err
	}
	started, err := s.usageStore.TrackingStartedAt(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"scope": filter, "timezone": "UTC", "meteringScope": "language-model", "meteringSince": started, "baselineSequence": baseline.Sequence, "clearedAt": baseline.UpdatedAt, "historicalCalls": "unmetered"}
	if group == "overview" {
		total := mindruntime.UsageBucket{}
		if len(rows) > 0 {
			total = rows[0]
		}
		for key, value := range usageValues(total) {
			result[key] = value
		}
		result["totalRequests"] = total.Calls
		var todayTokens any = total.TodayTokens
		if total.TodayUnknown > 0 {
			todayTokens = nil
		}
		result["todayCalls"], result["todayTokens"], result["todayUnknownUsageCalls"] = total.TodayCalls, todayTokens, total.TodayUnknown
		result["todayKnownTokens"] = total.TodayTokens
		return result, nil
	}
	items := []map[string]any{}
	daily := map[string]map[string]any{}
	if group == "daily" {
		start := time.Now().UTC().AddDate(0, 0, -29)
		if reset, parseErr := time.Parse(time.RFC3339Nano, baseline.UpdatedAt); parseErr == nil && reset.After(start) {
			start = reset
		}
		end := time.Now().UTC().Format("2006-01-02")
		for day := start.Format("2006-01-02"); day <= end; {
			item := usageValues(mindruntime.UsageBucket{})
			item["date"] = day
			if day < started[:10] {
				for key := range item {
					item[key] = nil
				}
				item["date"], item["meteringStatus"], item["costStatus"] = day, "unmetered", "unavailable"
			} else {
				item["meteringStatus"] = "tracked"
			}
			daily[day] = item
			date, _ := time.Parse("2006-01-02", day)
			day = date.AddDate(0, 0, 1).Format("2006-01-02")
		}
	}
	for _, row := range rows {
		item := usageValues(row)
		switch group {
		case "daily":
			item["date"] = row.Day
			item["meteringStatus"] = "tracked"
			daily[row.Day] = item
			continue
		case "models":
			item["name"], item["provider"] = row.Model, row.Provider
		case "sources":
			item["source"] = row.Source
		}
		items = append(items, item)
	}
	key := group
	if group == "daily" {
		for _, item := range daily {
			items = append(items, item)
		}
		sort.Slice(items, func(i, j int) bool { return items[i]["date"].(string) > items[j]["date"].(string) })
	}
	result[key] = items
	return result, nil
}

func (s *service) ClearUsageContext(ctx context.Context, filter mindruntime.UsageFilter) (map[string]any, error) {
	if s.usageStore == nil {
		return nil, errors.New("模型使用统计未就绪")
	}
	sequence, err := s.usageStore.Clear(ctx, filter)
	if err != nil {
		return nil, err
	}
	return map[string]any{"cleared": true, "scope": filter, "baselineSequence": sequence, "method": "baseline", "recordsPreserved": true}, nil
}
