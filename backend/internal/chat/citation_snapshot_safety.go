package chat

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxCitationSnapshotEntries = 24
const maxCitationSnapshotJSONBytes = 131072

func boundedCitationString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > limit {
		return string([]rune(value)[:limit])
	}
	return value
}

func safeCitationURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || utf8.RuneCountInString(raw) > 2048 || strings.ContainsAny(raw, "\r\n\t\\") {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	params := parsed.Query()
	for name := range params {
		key := strings.ToLower(name)
		for _, blocked := range []string{"token", "secret", "password", "passwd", "apikey", "api_key", "auth", "signature", "credential", "session", "access_key", "code"} {
			if strings.Contains(key, blocked) {
				params.Del(name)
				break
			}
		}
	}
	parsed.RawQuery = params.Encode()
	return parsed.String()
}

func safeCitationLocator(locator map[string]any) map[string]any {
	if len(locator) == 0 {
		return nil
	}
	result := make(map[string]any, 8)
	aliases := map[string][]string{
		"page":      {"page", "pageNumber", "page_number"},
		"row":       {"row", "rowStart", "row_start", "rowEnd", "row_end"},
		"sheet":     {"sheet", "sheetName", "sheet_name"},
		"paragraph": {"paragraph", "paragraphIndex", "paragraph_index"},
		"section":   {"section"},
		"index":     {"index"},
	}
	for _, key := range []string{"page", "row", "sheet", "paragraph", "section", "index"} {
		var value any
		var exists bool
		var selected string
		for _, alias := range aliases[key] {
			if value, exists = locator[alias]; exists {
				selected = alias
				break
			}
		}
		if !exists {
			continue
		}
		switch v := value.(type) {
		case int:
			if v >= 0 && v <= 1000000000 {
				result[key] = int(v)
			}
		case int64:
			if v >= 0 && v <= 1000000000 {
				result[key] = int(v)
			}
		case float64:
			if v >= 0 && v <= 1000000000 && v == float64(int64(v)) {
				result[key] = int(v)
			}
		case string:
			if key == "sheet" {
				trimmed := strings.TrimSpace(v)
				if !safeCitationLocatorLabel(trimmed) {
					continue
				}
				if selected == "sheet" && len(trimmed) <= 3 {
					if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil && n >= 0 && n <= 1000000000 {
						result[key] = int(n)
						continue
					}
				}
				result[key] = trimmed
				continue
			}
			n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err == nil && n >= 0 && n <= 1000000000 {
				result[key] = int(n)
			}
		}
	}
	for _, key := range []string{"kind", "field"} {
		value, ok := locator[key].(string)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if !safeCitationLocatorLabel(value) {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func safeCitationLocatorLabel(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.RuneCountInString(value) <= 128 && !strings.ContainsAny(value, "/\\:") && strings.IndexFunc(value, unicode.IsControl) < 0
}

func normalizeCitationSnapshotRecords(entries []CitationSnapshot) []CitationSnapshot {
	if len(entries) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, maxCitationSnapshotEntries)
	out := make([]CitationSnapshot, 0, maxCitationSnapshotEntries)
	for _, entry := range entries {
		if len(out) >= maxCitationSnapshotEntries {
			break
		}
		if entry.Index < 1 || entry.Index > 999999 || (entry.Kind != "knowledge" && entry.Kind != "web") {
			continue
		}
		marker := "[" + strconv.Itoa(entry.Index) + "]"
		if entry.Kind == "knowledge" {
			marker = "[K" + strconv.Itoa(entry.Index) + "]"
		}
		if marker != entry.Marker {
			continue
		}
		if _, exists := seen[marker]; exists {
			continue
		}
		seen[marker] = struct{}{}
		entry.EvidenceID = boundedCitationString(entry.EvidenceID, 256)
		entry.RefID = boundedCitationString(entry.RefID, 256)
		entry.SourceID = boundedCitationString(entry.SourceID, 256)
		entry.UnitID = boundedCitationString(entry.UnitID, 256)
		entry.Title = boundedCitationString(entry.Title, 200)
		entry.Excerpt = boundedCitationString(entry.Excerpt, 1200)
		entry.ContentHash = boundedCitationString(entry.ContentHash, 128)
		entry.Checksum = boundedCitationString(entry.Checksum, 128)
		entry.URL = safeCitationURL(entry.URL)
		entry.Locator = safeCitationLocator(entry.Locator)
		if entry.SourceRevision < 0 {
			entry.SourceRevision = 0
		}
		out = append(out, entry)
	}
	return out
}
