package util

import "testing"

func TestSplitMessageSegmentsByAmitiaBreak(t *testing.T) {
	content := "消息1[AMITIA_BR] 消息2 [AMITIA_BR][AMITIA_BR]\n消息3"
	got := SplitMessageSegments(content)
	want := []string{"消息1", " 消息2 ", "", "\n消息3"}

	if len(got) != len(want) {
		t.Fatalf("expected %d segments, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segment %d mismatch: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitMessageSegmentsDoesNotSplitNewline(t *testing.T) {
	content := "第一行\n第二行\n第三行"
	got := SplitMessageSegments(content)
	if len(got) != 1 || got[0] != content {
		t.Fatalf("expected whole content, got %v", got)
	}
}

func TestSplitMessageSegmentsFallsBackToWholeContent(t *testing.T) {
	content := "  你好。今天怎么样？  "
	got := SplitMessageSegments(content)
	if len(got) != 1 || got[0] != content {
		t.Fatalf("expected whole content, got %v", got)
	}
}

func TestSplitLongMessageUsesOnlyAmitiaBreak(t *testing.T) {
	content := "你好。[AMITIA_BR]今天怎么样？"
	got := SplitLongMessage(content, MaxWebMessageLen)
	want := []string{"你好。", "今天怎么样？"}
	if len(got) != len(want) {
		t.Fatalf("expected %d segments, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segment %d mismatch: got %q, want %q", i, got[i], want[i])
		}
	}
}
