package chat

import (
	"reflect"
	"testing"

	"github.com/u-ai/backend/internal/knowledge"
)

func TestPhase43KnowledgeCitationMarkersDoNotCaptureMarkdownLinks(t *testing.T) {
	markdown := "出处 [K1]、[1]；链接 [K2](https://example.org)、[2](https://example.org)，图片 ![K3](https://example.org/logo.png)，引用式 [K4][document]，定义 [3]: https://example.org。\\[K5] 和 `[K6]`。"
	if got := citationMarkersInProse(markdown, knowledgeMarkerRE); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("knowledge markers: got %v, want [1]", got)
	}
}

func TestPhase43KnowledgeSnapshotUsesMarkerContract(t *testing.T) {
	recorder := &ownedCitationAudit{}
	for i := 1; i <= 6; i++ {
		recorder.registerKnowledgeCitation(i, knowledge.Evidence{ID: "ev", SourceID: "source", SourceRevision: 1}, "text")
	}
	body := "报告 [K1] 和 [1]。网页 [2](https://example.org) 与图片 ![K2](https://example.org/a.png)。"
	snapshot := recorder.BuildCitationSnapshot(body)
	if len(snapshot) != 1 || snapshot[0].Marker != "[K1]" {
		t.Fatalf("snapshot must omit markdown link labels: %+v", snapshot)
	}
	if got := recorder.BuildCitationSnapshot("[K1](https://example.org) [1](https://example.org)"); len(got) != 0 {
		t.Fatalf("markdown links cannot become citation evidence: %+v", got)
	}
}
