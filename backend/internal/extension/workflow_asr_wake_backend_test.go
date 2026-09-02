package extension

import (
	"testing"

	"github.com/u-ai/backend/internal/realtime"
)

func TestMatchWorkflowWakePhrase(t *testing.T) {
	phrases := []realtime.WakePhrase{{ID: "amitia", DisplayText: "你好 Amitia"}}
	id, confidence, ok := matchWorkflowWakePhrase("你好，Amitia。", phrases)
	if !ok || id != "amitia" || confidence != 1 {
		t.Fatalf("exact normalized phrase should match: id=%q confidence=%v ok=%v", id, confidence, ok)
	}
	id, confidence, ok = matchWorkflowWakePhrase("助手，请听我说：你好 Amitia", phrases)
	if !ok || id != "amitia" || confidence < 0.9 {
		t.Fatalf("contained phrase should match: id=%q confidence=%v ok=%v", id, confidence, ok)
	}
	if _, _, ok = matchWorkflowWakePhrase("今天的天气怎么样", phrases); ok {
		t.Fatal("unrelated ASR transcript must not trigger wake")
	}
}

func TestNormalizeWorkflowWakeText(t *testing.T) {
	if got := normalizeWorkflowWakeText("  Hey, Amitia! "); got != "heyamitia" {
		t.Fatalf("unexpected normalized text: %q", got)
	}
}
