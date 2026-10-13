package business

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type MemoryTimeFilter struct {
	Basis          string `json:"basis"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	At             string `json:"at,omitempty"`
	LocalDateFrom  string `json:"localDateFrom,omitempty"`
	LocalDateTo    string `json:"localDateTo,omitempty"`
	IncludeUnknown bool   `json:"includeUnknown,omitempty"`
}

func parseMemoryFilterTime(raw string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("记忆时间筛选格式无效")
}

func validateMemoryTimeFilter(filter *MemoryTimeFilter) error {
	if filter == nil {
		return nil
	}
	switch filter.Basis {
	case "", "occurred", "validity", "created", "updated", "last_used":
	default:
		return errors.New("记忆时间筛选依据无效")
	}
	for _, raw := range []string{filter.From, filter.To, filter.At} {
		if raw != "" {
			if _, err := parseMemoryFilterTime(raw); err != nil {
				return err
			}
		}
	}
	for _, raw := range []string{filter.LocalDateFrom, filter.LocalDateTo} {
		if raw != "" {
			if _, err := time.Parse("2006-01-02", raw); err != nil {
				return errors.New("记忆本地日期筛选无效")
			}
		}
	}
	if filter.From != "" && filter.To != "" {
		from, _ := parseMemoryFilterTime(filter.From)
		to, _ := parseMemoryFilterTime(filter.To)
		if !from.Before(to) {
			return errors.New("记忆时间筛选范围无效")
		}
	}
	if filter.LocalDateFrom != "" && filter.LocalDateTo != "" && filter.LocalDateFrom > filter.LocalDateTo {
		return errors.New("记忆本地日期范围无效")
	}
	return nil
}

func memoryTimeMatches(document, content map[string]json.RawMessage, filter *MemoryTimeFilter) bool {
	if filter == nil {
		return true
	}
	text := func(keys ...string) string {
		for _, key := range keys {
			if value := MemoryText(content, key); value != "" {
				return value
			}
			if value := MemoryText(document, key); value != "" {
				return value
			}
		}
		return ""
	}
	local := text("localDate", "occurredLocalDate")
	if filter.LocalDateFrom != "" || filter.LocalDateTo != "" {
		if local == "" {
			if !filter.IncludeUnknown {
				return false
			}
		} else if filter.LocalDateFrom != "" && local < filter.LocalDateFrom || filter.LocalDateTo != "" && local > filter.LocalDateTo {
			return false
		}
	}
	if filter.From == "" && filter.To == "" && filter.At == "" {
		return true
	}
	if filter.Basis == "validity" {
		start, end := text("validFromUtc", "validFrom"), text("validToUtc", "validTo", "expiresAt")
		if start == "" && end == "" {
			return filter.IncludeUnknown
		}
		var low, high time.Time
		if start != "" {
			var err error
			low, err = parseMemoryFilterTime(start)
			if err != nil {
				return filter.IncludeUnknown
			}
		}
		if end != "" {
			var err error
			high, err = parseMemoryFilterTime(end)
			if err != nil {
				return filter.IncludeUnknown
			}
		}
		if filter.At != "" {
			point, _ := parseMemoryFilterTime(filter.At)
			if !low.IsZero() && point.Before(low) || !high.IsZero() && !point.Before(high) {
				return false
			}
		}
		if filter.From != "" {
			point, _ := parseMemoryFilterTime(filter.From)
			if !high.IsZero() && !point.Before(high) {
				return false
			}
		}
		if filter.To != "" {
			point, _ := parseMemoryFilterTime(filter.To)
			if !low.IsZero() && !low.Before(point) {
				return false
			}
		}
		return true
	}
	var raw string
	switch filter.Basis {
	case "created":
		raw = text("createdAt", "createdAtUtc")
	case "updated":
		raw = text("updatedAt", "updatedAtUtc")
	case "last_used":
		raw = text("lastReinforcedAt", "lastUsedAt")
	default:
		raw = text("occurredAtUtc", "occurredAt")
	}
	if raw == "" {
		return filter.IncludeUnknown
	}
	value, err := parseMemoryFilterTime(raw)
	if err != nil {
		return filter.IncludeUnknown
	}
	if filter.From != "" {
		low, _ := parseMemoryFilterTime(filter.From)
		if value.Before(low) {
			return false
		}
	}
	if filter.To != "" {
		high, _ := parseMemoryFilterTime(filter.To)
		if !value.Before(high) {
			return false
		}
	}
	if filter.At != "" {
		point, _ := parseMemoryFilterTime(filter.At)
		if !value.Equal(point) {
			return false
		}
	}
	return true
}

func memorySetMatches(values []string, value string) bool {
	if len(values) == 0 {
		return true
	}
	for _, candidate := range values {
		if strings.EqualFold(strings.TrimSpace(candidate), value) {
			return true
		}
	}
	return false
}

func memoryToolStrings(args map[string]interface{}, key string) []string {
	if value, ok := args[key].([]string); ok {
		return value
	}
	values, _ := args[key].([]interface{})
	result := []string{}
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func memoryTimeFilterArgs(args map[string]interface{}) *MemoryTimeFilter {
	filter := &MemoryTimeFilter{Basis: MemoryToolString(args, "time_basis"), From: MemoryToolString(args, "from"), To: MemoryToolString(args, "to"), At: MemoryToolString(args, "at"), LocalDateFrom: MemoryToolString(args, "local_date_from"), LocalDateTo: MemoryToolString(args, "local_date_to")}
	filter.IncludeUnknown, _ = args["include_unknown_time"].(bool)
	if filter.From == "" && filter.To == "" && filter.At == "" && filter.LocalDateFrom == "" && filter.LocalDateTo == "" && filter.Basis == "" {
		return nil
	}
	return filter
}
