package knowledge

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	amitialog "log"
)

func estimateTokens(s string) int {
	r := len([]rune(s))
	if r == 0 {
		return 0
	}
	return (r + 1) / 2
}
func marshalMap(v map[string]any) string {
	if v == nil {
		return "{}"
	}
	b, _ := json.Marshal(v)
	return string(b)
}
func unmarshalMap(raw string) map[string]any {
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	return out
}

func splitSemantic(text string, maxRunes, overlap int) []string {
	text = cleanText(text)
	if text == "" {
		return nil
	}
	r := []rune(text)
	if len(r) <= maxRunes {
		return []string{text}
	}
	chunks := make([]string, 0, (len(r)+maxRunes-1)/maxRunes)
	start := 0
	for start < len(r) {
		end := start + maxRunes
		if end > len(r) {
			end = len(r)
		} else {
			floor := start + maxRunes*2/3
			for i := end - 1; i >= floor; i-- {
				if unicode.IsSpace(r[i]) || strings.ContainsRune("。！？；.!?;\n", r[i]) {
					end = i + 1
					break
				}
			}
		}
		if end <= start {
			end = start + maxRunes
			if end > len(r) {
				end = len(r)
			}
		}
		part := strings.TrimSpace(string(r[start:end]))
		if part != "" {
			chunks = append(chunks, part)
		}
		if end >= len(r) {
			break
		}
		next := end - overlap
		if next <= start {
			next = end
		}
		start = next
	}
	return chunks
}

func buildChunks(tenantID, sourceID string, revision int64, unit DocumentUnit, indexVersion string, settings ...runtimeSettings) []Chunk {
	runtime := loadRuntimeSettings()
	if len(settings) > 0 {
		runtime = settings[0]
	}
	defaults := defaultRuntimeSettings()
	targetRunes, overlapRunes, parentRunes := defaults.ChunkTargetRunes, defaults.ChunkOverlapRunes, defaults.ParentMaxRunes
	if runtime.ChunkTargetRunes > 0 {
		targetRunes = runtime.ChunkTargetRunes
	}
	if runtime.ChunkOverlapRunes >= 0 && runtime.ChunkOverlapRunes < targetRunes {
		overlapRunes = runtime.ChunkOverlapRunes
	}
	if runtime.ParentMaxRunes >= targetRunes {
		parentRunes = runtime.ParentMaxRunes
	}
	parts := splitSemantic(unit.Content, targetRunes, overlapRunes)
	if len(parts) == 0 {
		return nil
	}
	now := time.Now().UTC()
	loc := unit.LocatorJSON
	if loc == "" {
		loc = "{}"
	}
	meta := unit.MetadataJSON
	if meta == "" {
		meta = "{}"
	}
	parentID := ""
	out := make([]Chunk, 0, len(parts)+1)
	if len(parts) > 2 {
		parentID = uuid.NewString()
		pr := []rune(unit.Content)
		if len(pr) > parentRunes {
			pr = pr[:parentRunes]
		}
		parentContent := strings.TrimSpace(string(pr))
		out = append(out, Chunk{ID: parentID, TenantID: tenantID, SourceID: sourceID, SourceRevision: revision, UnitID: unit.ID, ChunkIndex: -1, Title: unit.Title, Content: parentContent, EmbeddingText: "", TokenCount: estimateTokens(parentContent), LocatorJSON: loc, MetadataJSON: meta, Checksum: checksumText(parentContent), IndexVersion: indexVersion, Status: "parent", ValidFrom: unit.ValidFrom, ValidTo: unit.ValidTo, SupersedesJSON: unit.SupersedesJSON, ConflictGroup: unit.ConflictGroup, CreatedAt: now, UpdatedAt: now})
	}
	for i, part := range parts {
		title := unit.Title
		embedText := part
		if title != "" {
			embedText = title + "\n" + part
		}
		out = append(out, Chunk{ID: uuid.NewString(), TenantID: tenantID, SourceID: sourceID, SourceRevision: revision, UnitID: unit.ID, ChunkIndex: i, ParentChunkID: parentID, Title: title, Content: part, EmbeddingText: embedText, TokenCount: estimateTokens(part), LocatorJSON: loc, MetadataJSON: meta, Checksum: checksumText(part), IndexVersion: indexVersion, Status: "parsed", ValidFrom: unit.ValidFrom, ValidTo: unit.ValidTo, SupersedesJSON: unit.SupersedesJSON, ConflictGroup: unit.ConflictGroup, CreatedAt: now, UpdatedAt: now})
	}
	return out
}
