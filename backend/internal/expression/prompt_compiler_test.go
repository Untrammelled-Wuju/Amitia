package expression

import (
	"strings"
	"testing"
)

func TestCompileChannelPrompt_WechatShortPerLine(t *testing.T) {
	cp := CompileChannelPrompt(ChannelWechat)
	if !strings.Contains(cp.SystemInstruction, "每句话必须单独一行") {
		t.Fatal("wechat prompt should include short_per_line format instruction")
	}
	if !strings.Contains(cp.SystemInstruction, "不能使用markdown格式") {
		t.Fatal("wechat prompt should forbid markdown")
	}
	if !strings.Contains(cp.StyleInstruction, "不要客服腔") {
		t.Fatal("wechat style should be conversational")
	}
}

func TestCompileChannelPrompt_QQShortPerLine(t *testing.T) {
	cp := CompileChannelPrompt(ChannelQQ)
	if !strings.Contains(cp.SystemInstruction, "每句话必须单独一行") {
		t.Fatal("qq prompt should include short_per_line format instruction")
	}
	if !strings.Contains(cp.SystemInstruction, "不能使用markdown格式") {
		t.Fatal("qq prompt should forbid markdown")
	}
}

func TestCompileChannelPrompt_WebFullParagraph(t *testing.T) {
	cp := CompileChannelPrompt(ChannelWeb)
	if !strings.Contains(cp.SystemInstruction, "完整段落") {
		t.Fatal("web prompt should allow full paragraphs")
	}
	if strings.Contains(cp.SystemInstruction, "每句话必须单独一行") {
		t.Fatal("web prompt should not include short_per_line constraint")
	}
	if !strings.Contains(cp.SystemInstruction, "Markdown") {
		t.Fatal("web prompt should allow markdown")
	}
}

func TestCompileChannelPrompt_VoiceSingleUtterance(t *testing.T) {
	cp := CompileChannelPrompt(ChannelVoice)
	if !strings.Contains(cp.SystemInstruction, "单一简短语句") {
		t.Fatal("voice prompt should include single_utterance constraint")
	}
	if !strings.Contains(cp.SystemInstruction, "120字") {
		t.Fatal("voice prompt should mention 120 char limit")
	}
}

func TestCompileChannelPrompt_UnknownFallback(t *testing.T) {
	cp := CompileChannelPrompt("unknown")
	if !strings.Contains(cp.SystemInstruction, "简洁自然") {
		t.Fatal("unknown channel should use safe fallback")
	}
	if !strings.Contains(cp.StyleInstruction, "适度温暖") {
		t.Fatal("unknown channel style should use safe default")
	}
}

func TestApplyPostValidation_StripsMarkdownForWechat(t *testing.T) {
	raw := "**hello** *world* __test__"
	result := ApplyPostValidation(raw, ChannelWechat)
	if strings.Contains(result, "**") || strings.Contains(result, "*") || strings.Contains(result, "__") {
		t.Fatalf("wechat validation should strip markdown, got: %s", result)
	}
}

func TestApplyPostValidation_PreservesMarkdownForWeb(t *testing.T) {
	raw := "**hello** *world*"
	result := ApplyPostValidation(raw, ChannelWeb)
	if result != raw {
		t.Fatalf("web validation should preserve markdown, got: %s", result)
	}
}

func TestApplyPostValidation_TruncatesOverLimit(t *testing.T) {
	raw := strings.Repeat("a", 300)
	result := ApplyPostValidation(raw, ChannelWechat)
	runes := []rune(result)
	if len(runes) > 200 {
		t.Fatalf("wechat output should be truncated to 200, got %d chars", len(runes))
	}
}

func TestApplyPostValidation_VoiceKeepsShort(t *testing.T) {
	raw := strings.Repeat("b", 200)
	result := ApplyPostValidation(raw, ChannelVoice)
	runes := []rune(result)
	if len(runes) > 120 {
		t.Fatalf("voice output should be truncated to 120, got %d chars", len(runes))
	}
}
