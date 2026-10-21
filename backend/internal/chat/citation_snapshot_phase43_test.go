package chat

import (
	"reflect"
	"testing"

	"github.com/u-ai/backend/internal/knowledge"
)

func TestPhase43CitationMarkersDoNotCaptureMarkdownLinks(t *testing.T) {
	markdown := "出处 [K1]、[1]；链接 [K2](https://example.org)、[2](https://example.org)，图片 ![K3](https://example.org/logo.png)，引用式 [K4][document]，定义 [3]: https://example.org。\\[K5] 和 `[K6]`。"
	if got := citationMarkersInProse(markdown, knowledgeMarkerRE); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("knowledge markers: got %v, want [1]", got)
	}
	if got := citationMarkersInProse(markdown, numericCitationPattern); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("web markers: got %v, want [1]", got)
	}
}

func TestPhase43SnapshotAndAuditUseSameMarkerContract(t *testing.T) {
	recorder := &ownedCitationAudit{}
	for i := 1; i <= 6; i++ {
		recorder.registerKnowledgeCitation(i, knowledge.Evidence{ID: "ev", SourceID: "source", SourceRevision: 1}, "text")
	}
	recorder.captureCitationIDs("web_run", `{"citations":[{"index":1,"evidence_id":"web1","text":"released in 2026"},{"index":2,"evidence_id":"web2","text":"unused"}]}`)
	body := "报告 [K1] 和 [1]。网页 [2](https://example.org) 与图片 ![K2](https://example.org/a.png)。"
	snapshot := recorder.BuildCitationSnapshot(body)
	if len(snapshot) != 2 || snapshot[0].Marker != "[K1]" || snapshot[1].Marker != "[1]" {
		t.Fatalf("snapshot must omit markdown link labels: %+v", snapshot)
	}
	audit := recorder.AuditCitationMarkers(body)
	if !reflect.DeepEqual(audit.Used, []int{1}) || len(audit.Unknown) != 0 {
		t.Fatalf("audit used and unknown ids: used=%v unknown=%v", audit.Used, audit.Unknown)
	}
	if len(audit.Claims) != 1 || !reflect.DeepEqual(audit.Claims[0].Citations, []int{1}) {
		t.Fatalf("audit claimed link ids: %+v", audit.Claims)
	}
	if got := recorder.BuildCitationSnapshot("[K1](https://example.org) [1](https://example.org)"); len(got) != 0 {
		t.Fatalf("markdown links cannot become citation evidence: %+v", got)
	}
}
