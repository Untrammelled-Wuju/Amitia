package chat

import (
	"strings"
	"testing"
)

func TestBubbleMessageInstructionScopedToRequest(t *testing.T) {
	for _, style := range []string{"", "flow", "bubble"} {
		messages, _ := buildProcessPromptMessages(processPromptInput{
			CharacterConfig: "你是测试角色", StyleInstruction: "保持简洁", UserContent: "你好", MessageStyle: style,
		})
		if len(messages) == 0 || messages[0]["role"] != "system" {
			t.Fatalf("missing system prompt for %q", style)
		}
		system, _ := messages[0]["content"].(string)
		if strings.Contains(system, "[AMITIA_BR]") != (style == "bubble") {
			t.Fatalf("incorrect delimiter instruction for %q", style)
		}
		if !strings.Contains(system, "你是测试角色") {
			t.Fatalf("character instructions were lost for %q", style)
		}
		channelKept := false
		for _, message := range messages[1:] {
			channelKept = channelKept || strings.Contains(message["content"].(string), "保持简洁")
			if strings.Contains(message["content"].(string), "[AMITIA_BR]") {
				t.Fatal("delimiter instruction leaked into user messages")
			}
		}
		if !channelKept {
			t.Fatal("channel instructions were lost")
		}
	}
}

func TestNormalizeMessageStyle(t *testing.T) {
	for input, expected := range map[string]string{"": "flow", "flow": "flow", " bubble ": "bubble", "BUBBLE": "bubble"} {
		actual, err := NormalizeMessageStyle(input)
		if err != nil || actual != expected {
			t.Fatalf("%q normalized to %q: %v", input, actual, err)
		}
	}
	if _, err := NormalizeMessageStyle("arbitrary prompt"); err == nil {
		t.Fatal("invalid style accepted")
	}
}
