package knowledge

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRetrievalTracePayloadDoesNotPersistDocumentContentOrLocator(t *testing.T) {
	candidate := evidenceCandidate{
		state: &fusionState{chunkID: "chunk-1", denseScore: 0.8, sparseScore: 0.5, exactScore: 1, fusion: 0.04},
		matched: Chunk{
			ID: "chunk-1", SourceID: "source-1", SourceRevision: 3,
			Content: "TOP SECRET DOCUMENT BODY", LocatorJSON: `{"page":42,"path":"private/location"}`,
		},
		read:      Chunk{ID: "parent-1", Content: "EXPANDED SECRET BODY"},
		readID:    "parent-1",
		relevance: 0.9,
	}
	candidateJSON, err := json.Marshal(safeCandidateTrace([]evidenceCandidate{candidate}))
	if err != nil {
		t.Fatalf("marshal candidate trace: %v", err)
	}
	candidateText := string(candidateJSON)
	for _, secret := range []string{"TOP SECRET", "EXPANDED SECRET", "private/location", `\"page\":42`} {
		if strings.Contains(candidateText, secret) {
			t.Fatalf("candidate trace leaked raw document/locator data %q: %s", secret, candidateText)
		}
	}
	if !strings.Contains(candidateText, "locatorHash") || !strings.Contains(candidateText, "chunk-1") {
		t.Fatalf("candidate trace lost safe diagnostic identity: %s", candidateText)
	}

	evidenceJSON, err := json.Marshal(safeEvidenceTrace([]Evidence{{
		ID: "parent-1", SourceID: "source-1", SourceRevision: 3,
		Content: "FINAL SECRET EVIDENCE", Locator: map[string]any{"page": 42, "path": "private/location"},
		DenseScore: 0.8, SparseScore: 0.5, FusionScore: 0.04, RerankScore: 0.7,
		Trust: "untrusted_data_only",
	}}))
	if err != nil {
		t.Fatalf("marshal evidence trace: %v", err)
	}
	evidenceText := string(evidenceJSON)
	for _, secret := range []string{"FINAL SECRET", "private/location", `\"page\":42`} {
		if strings.Contains(evidenceText, secret) {
			t.Fatalf("evidence trace leaked raw document/locator data %q: %s", secret, evidenceText)
		}
	}
	if !strings.Contains(evidenceText, "locatorHash") || !strings.Contains(evidenceText, "parent-1") {
		t.Fatalf("evidence trace lost safe diagnostic identity: %s", evidenceText)
	}
}
