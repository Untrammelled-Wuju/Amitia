package chat

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// CitationSnapshot is a compact, immutable record of a citation used in a
// final assistant reply. It is not a substitute for the source ACL or document.
type CitationSnapshot struct {
	Marker         string         `json:"marker"`
	Kind           string         `json:"kind"`
	Index          int            `json:"index"`
	EvidenceID     string         `json:"evidenceId,omitempty"`
	RefID          string         `json:"refId,omitempty"`
	SourceID       string         `json:"sourceId,omitempty"`
	SourceRevision int64          `json:"sourceRevision,omitempty"`
	UnitID         string         `json:"unitId,omitempty"`
	Title          string         `json:"title,omitempty"`
	URL            string         `json:"url,omitempty"`
	Locator        map[string]any `json:"locator,omitempty"`
	Excerpt        string         `json:"excerpt,omitempty"`
	ContentHash    string         `json:"contentHash,omitempty"`
	Checksum       string         `json:"checksum,omitempty"`
}

func (m *Message) AfterFind(_ *gorm.DB) error {
	m.CitationSnapshot = nil
	if strings.TrimSpace(m.CitationSnapshotJSON) == "" || m.CitationSnapshotJSON == "[]" || len(m.CitationSnapshotJSON) > maxCitationSnapshotJSONBytes {
		return nil
	}
	var snapshots []CitationSnapshot
	if json.Unmarshal([]byte(m.CitationSnapshotJSON), &snapshots) == nil {
		m.CitationSnapshot = normalizeCitationSnapshotRecords(snapshots)
	}
	// Malformed rows must not make old conversation history unreadable.
	return nil
}

func (m *Message) BeforeSave(_ *gorm.DB) error {
	provided := m.CitationSnapshot != nil
	m.CitationSnapshot = normalizeCitationSnapshotRecords(m.CitationSnapshot)
	if len(m.CitationSnapshot) == 0 {
		if provided || strings.TrimSpace(m.CitationSnapshotJSON) == "" {
			m.CitationSnapshotJSON = "[]"
		}
		return nil
	}
	encoded, err := json.Marshal(m.CitationSnapshot)
	if err != nil {
		return err
	}
	m.CitationSnapshotJSON = string(encoded)
	return nil
}

var knowledgeMarkerRE = regexp.MustCompile(`\[K([1-9][0-9]{0,5})\]`)

// citationMarkersInProse excludes fenced and inline code. Numeric and K
// markers have separate namespaces and cannot be confused.
func citationMarkersInProse(markdown string, pattern *regexp.Regexp) []int {
	var ids []int
	var fenceByte byte
	fenceSize := 0
	inlineSize := 0
	for _, line := range strings.Split(markdown, "\n") {
		prefix := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimLeft(line, " ")
		if prefix <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			count := 0
			for count < len(trimmed) && trimmed[count] == trimmed[0] {
				count++
			}
			if count >= 3 {
				if fenceSize == 0 && inlineSize == 0 {
					fenceByte, fenceSize = trimmed[0], count
					continue
				}
				if trimmed[0] == fenceByte && count >= fenceSize && strings.TrimSpace(trimmed[count:]) == "" {
					fenceByte, fenceSize = 0, 0
					continue
				}
			}
		}
		if fenceSize != 0 || prefix >= 4 || strings.HasPrefix(line, "\t") {
			continue
		}
		var prose strings.Builder
		for offset := 0; offset < len(line); {
			if line[offset] != '`' {
				if inlineSize == 0 {
					prose.WriteByte(line[offset])
				}
				offset++
				continue
			}
			run := 0
			for offset+run < len(line) && line[offset+run] == '`' {
				run++
			}
			if inlineSize == 0 {
				inlineSize = run
			} else if inlineSize == run {
				inlineSize = 0
			}
			offset += run
		}
		text := prose.String()
		for _, offsets := range pattern.FindAllStringSubmatchIndex(text, -1) {
			if offsets[1] < len(text) && strings.ContainsRune("([:", rune(text[offsets[1]])) {
				continue
			}
			if offsets[0] > 0 && text[offsets[0]-1] == '!' {
				continue
			}
			backslashes := 0
			for index := offsets[0] - 1; index >= 0 && text[index] == '\\'; index-- {
				backslashes++
			}
			if backslashes%2 != 0 {
				continue
			}
			n, err := strconv.Atoi(text[offsets[2]:offsets[3]])
			if err == nil && n > 0 {
				ids = append(ids, n)
			}
		}
	}
	return ids
}

func citationExcerpt(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}
