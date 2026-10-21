package chat

import "testing"

func TestNormalizeTitleGenerationReply(t *testing.T) {
	got := normalizeTitleGenerationReply("第一段\n第二段\n第三段")
	if got != "第一段\n第二段\n第三段" {
		t.Fatalf("unexpected title generation reply: %q", got)
	}
}
