package knowledge

import (
	"encoding/json"
	"fmt"
	amitialog "log"
	"strconv"
	"strings"
	"time"
)

const (
	TemporalModeCurrent    = "current"
	TemporalModeHistorical = "historical"
	TemporalModeAll        = "all"
)

type temporalQuery struct {
	Mode string
	AsOf time.Time
}

func resolveTemporalQuery(req SearchRequest) (temporalQuery, error) {
	mode := strings.ToLower(strings.TrimSpace(req.TemporalMode))
	switch mode {
	case "", "current":
		return temporalQuery{Mode: TemporalModeCurrent, AsOf: time.Now().UTC()}, nil
	case "historical", "as_of", "asof":
		if req.AsOf == nil || req.AsOf.IsZero() {
			return temporalQuery{}, fmt.Errorf("historical temporalMode requires asOf")
		}
		return temporalQuery{Mode: TemporalModeHistorical, AsOf: req.AsOf.UTC()}, nil
	case "all":
		return temporalQuery{Mode: TemporalModeAll}, nil
	default:
		return temporalQuery{}, fmt.Errorf("unsupported temporalMode %q; use current, historical, or all", req.TemporalMode)
	}
}

type temporalFields struct {
	ValidFrom     *time.Time
	ValidTo       *time.Time
	Supersedes    []string
	ConflictGroup string
}

func deriveTemporalFields(metadata map[string]any, content string) temporalFields {
	merged := map[string]any{}
	for key, value := range metadata {
		merged[key] = value
	}
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var object map[string]any
		if json.Unmarshal([]byte(trimmed), &object) == nil {
			for key, value := range object {
				if _, exists := merged[key]; !exists {
					merged[key] = value
				}
			}
		}
	}
	out := temporalFields{}
	out.ValidFrom = firstTimeField(merged, "valid_from", "validFrom")
	out.ValidTo = firstTimeField(merged, "valid_to", "validTo")
	if out.ValidFrom != nil && out.ValidTo != nil && !out.ValidTo.After(*out.ValidFrom) {
		// Invalid temporal metadata must not accidentally hide otherwise valid
		// knowledge. Ignore the malformed end boundary and keep the canonical
		// ingestion deterministic; created_at remains ingestion time, not validity.
		out.ValidTo = nil
	}
	out.Supersedes = firstStringListField(merged, "supersedes", "supersedes_ids", "supersedesIds")
	out.ConflictGroup = firstStringField(merged, "conflict_group", "conflictGroup")
	return out
}

func firstTimeField(m map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if parsed := parseTemporalTime(value); parsed != nil {
				return parsed
			}
		}
	}
	return nil
}

func parseTemporalTime(value any) *time.Time {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		t := v.UTC()
		return &t
	case float64:
		if v <= 0 {
			return nil
		}
		t := time.Unix(int64(v), 0).UTC()
		return &t
	case json.Number:
		i, err := v.Int64()
		if err == nil && i > 0 {
			t := time.Unix(i, 0).UTC()
			return &t
		}
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		if unix, err := strconv.ParseInt(s, 10, 64); err == nil && unix > 0 {
			t := time.Unix(unix, 0).UTC()
			return &t
		}
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05Z07:00",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				u := t.UTC()
				return &u
			}
		}
	}
	return nil
}

func firstStringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			s := strings.TrimSpace(fmt.Sprint(value))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func firstStringListField(m map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		var raw []string
		switch v := value.(type) {
		case []string:
			raw = append(raw, v...)
		case []any:
			for _, item := range v {
				raw = append(raw, fmt.Sprint(item))
			}
		case string:
			s := strings.TrimSpace(v)
			if strings.HasPrefix(s, "[") {
				if err := json.Unmarshal([]byte(s), &raw); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
			} else {
				raw = strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '，' || r == '；' })
			}
		default:
			raw = []string{fmt.Sprint(v)}
		}
		out := make([]string, 0, len(raw))
		seen := map[string]struct{}{}
		for _, item := range raw {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, exists := seen[item]; exists {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
		return out
	}
	return nil
}

func marshalStringList(v []string) string {
	if len(v) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func unmarshalStringList(raw string) []string {
	var out []string
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &out) != nil {
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func chunkVisibleAt(c Chunk, q temporalQuery) bool {
	if q.Mode == TemporalModeAll {
		return true
	}
	at := q.AsOf
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if c.ValidFrom != nil && c.ValidFrom.After(at) {
		return false
	}
	if c.ValidTo != nil && !c.ValidTo.After(at) {
		return false
	}
	return true
}
