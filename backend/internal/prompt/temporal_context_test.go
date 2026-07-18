package prompt

import (
	"strings"
	"testing"
)

func TestTemporalContextIsTrustedDataOnlyAndSuppressesLegacyProactiveTime(t *testing.T) {
	request := BuildRequest{TemporalContext: "【当前时间上下文】\n用户当地时间：2026-07-18 09:30", ProactiveTimeContext: "旧主动时间上下文", CurrentUserInput: "现在几点"}
	ir := NewBuilder().Build(request)
	count := 0
	for _, section := range ir.Sections {
		if section.Type == GwSectionTemporalContext {
			count++
			if section.TrustLevel != TrustTrusted || section.InstructionMode != ModeDataOnly || section.TokenBudget != 220 {
				t.Fatalf("unexpected temporal section %#v", section)
			}
		}
		if section.Type == GwSectionProactiveTimeContext {
			t.Fatal("legacy proactive time must be suppressed")
		}
	}
	if count != 1 {
		t.Fatalf("expected one temporal section, got %d", count)
	}
	messages, err := NewRenderer().Render(ir)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, message := range messages {
		joined += message.Content
	}
	if !strings.Contains(joined, "<temporal_context") || strings.Contains(joined, "旧主动时间上下文") || strings.Contains(joined, "<untrusted_data type=\"temporal_context\"") {
		t.Fatalf("unexpected render %s", joined)
	}
}
