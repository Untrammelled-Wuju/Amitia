package expression

import (
	"github.com/u-ai/backend/internal/interaction"
	"strings"
	"testing"
)

func TestGetChannelPolicy_Wechat(t *testing.T) {
	p := GetChannelPolicy(ChannelWechat)
	if p.Kind != ChannelWechat {
		t.Fatalf("expected wechat, got %s", p.Kind)
	}
	if p.MaxCharacters > 200 {
		t.Fatalf("wechat max characters too high: %d", p.MaxCharacters)
	}
	if !p.Capabilities.SupportsSegmented {
		t.Fatal("wechat expected to support segmented")
	}
	if p.Capabilities.SupportsMarkdown {
		t.Fatal("wechat expected no markdown")
	}
	if p.SegmentHint != "short_per_line" {
		t.Fatalf("wechat expected short_per_line hint, got %s", p.SegmentHint)
	}
}

func TestGetChannelPolicy_QQ(t *testing.T) {
	p := GetChannelPolicy(ChannelQQ)
	if p.Kind != ChannelQQ {
		t.Fatalf("expected qq, got %s", p.Kind)
	}
	if p.MaxCharacters > 200 {
		t.Fatalf("qq max characters too high: %d", p.MaxCharacters)
	}
	if !p.Capabilities.SupportsMedia {
		t.Fatal("qq expected to support media")
	}
}

func TestGetChannelPolicy_Web(t *testing.T) {
	p := GetChannelPolicy(ChannelWeb)
	if p.Kind != ChannelWeb {
		t.Fatalf("expected web, got %s", p.Kind)
	}
	if p.MaxCharacters < 400 {
		t.Fatalf("web max characters too low: %d", p.MaxCharacters)
	}
	if !p.Capabilities.SupportsMarkdown {
		t.Fatal("web expected to support markdown")
	}
	if p.SegmentHint != "full_paragraph" {
		t.Fatalf("web expected full_paragraph hint, got %s", p.SegmentHint)
	}
}

func TestGetChannelPolicy_Voice(t *testing.T) {
	p := GetChannelPolicy(ChannelVoice)
	if p.Kind != ChannelVoice {
		t.Fatalf("expected voice, got %s", p.Kind)
	}
	if p.MaxCharacters > 120 {
		t.Fatalf("voice max characters too high: %d", p.MaxCharacters)
	}
	if !p.Capabilities.SupportsVoice {
		t.Fatal("voice expected to support voice")
	}
	if p.Capabilities.SupportsMarkdown {
		t.Fatal("voice expected no markdown")
	}
	if p.MaxSegments != 1 {
		t.Fatalf("voice expected max_segments=1, got %d", p.MaxSegments)
	}
}

func TestGetChannelPolicy_UnknownChannelUsesSafeFallback(t *testing.T) {
	p := GetChannelPolicy("unknown_channel")
	if p.SegmentHint != "safe_text_fallback" {
		t.Fatalf("unknown channel expected safe_text_fallback, got %s", p.SegmentHint)
	}
	if p.Capabilities.SupportsMarkdown {
		t.Fatal("unknown channel expected no markdown")
	}
	if p.Capabilities.SupportsVoice {
		t.Fatal("unknown channel expected no voice")
	}
}

func TestGetChannelPolicyVersion_UnknownVersion(t *testing.T) {
	_, err := GetChannelPolicyVersion(ChannelWechat, "v99")
	if err == nil {
		t.Fatal("expected error for unknown version")
	}
}

func TestGetChannelPolicy_WechatHasShortRules(t *testing.T) {
	p := GetChannelPolicy(ChannelWechat)
	if p.ShortRules == "" {
		t.Fatal("wechat policy should have ShortRules")
	}
	if !containsAny(p.ShortRules, []string{"微信短句规则", "短句", "活人说话"}) {
		t.Fatalf("wechat ShortRules missing wechat-specific rules: %q", p.ShortRules[:100])
	}
	if containsAny(p.ShortRules, []string{"表情包", "sticker", "StickerManager"}) {
		t.Fatal("wechat ShortRules should not contain sticker rules")
	}
	if containsAny(p.ShortRules, []string{"@", "群聊", "groupchat"}) {
		t.Fatal("wechat ShortRules should not contain group chat rules")
	}
}

func TestGetChannelPolicy_QQHasShortRules(t *testing.T) {
	p := GetChannelPolicy(ChannelQQ)
	if p.ShortRules == "" {
		t.Fatal("qq policy should have ShortRules")
	}
	if !containsAny(p.ShortRules, []string{"QQ短句规则", "短句"}) {
		t.Fatalf("qq ShortRules missing qq-specific rules: %q", p.ShortRules[:100])
	}
	if containsAny(p.ShortRules, []string{"表情包", "sticker"}) {
		t.Fatal("qq ShortRules should not contain sticker rules")
	}
	if containsAny(p.ShortRules, []string{"@", "群聊"}) {
		t.Fatal("qq ShortRules should not contain group chat rules")
	}
}

func TestGetChannelPolicy_WebHasShortRules(t *testing.T) {
	p := GetChannelPolicy(ChannelWeb)
	if p.ShortRules == "" {
		t.Fatal("web policy should have ShortRules")
	}
	if !containsAny(p.ShortRules, []string{"桌面端", "完整段落"}) {
		t.Fatalf("web ShortRules missing web-specific rules: %q", p.ShortRules[:100])
	}
}

func TestCompileChannelPrompt_WechatIncludesShortRules(t *testing.T) {
	cp := CompileChannelPrompt(ChannelWechat)
	if !containsAny(cp.SystemInstruction, []string{"微信短句规则", "短句"}) {
		t.Fatal("wechat compiled prompt should include short rules")
	}
	if !containsAny(cp.SystemInstruction, []string{"直接回复内容", "不要输出思考过程"}) {
		t.Fatal("wechat compiled prompt should include direct reply directive")
	}
}

func TestCompileChannelPrompt_QQIncludesShortRules(t *testing.T) {
	cp := CompileChannelPrompt(ChannelQQ)
	if !containsAny(cp.SystemInstruction, []string{"QQ短句规则", "短句"}) {
		t.Fatal("qq compiled prompt should include short rules")
	}
	if !containsAny(cp.SystemInstruction, []string{"直接回复内容", "不要输出思考过程"}) {
		t.Fatal("qq compiled prompt should include direct reply directive")
	}
}

func TestCompileChannelPrompt_WebNotForcedToWechatLength(t *testing.T) {
	cp := CompileChannelPrompt(ChannelWeb)
	if containsAny(cp.SystemInstruction, []string{"短句规则", "15-50字"}) {
		t.Fatal("web should not use wechat short rules")
	}
	if !containsAny(cp.SystemInstruction, []string{"完整段落", "Markdown"}) {
		t.Fatal("web compiled prompt should include web-specific rules")
	}
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestRegisterChannelPolicy(t *testing.T) {
	custom := ChannelPolicy{
		Kind:          ChannelWechat,
		MaxCharacters: 300,
		MinCharacters: 50,
		MaxSegments:   4,
		MinSegments:   1,
		Capabilities: ChannelCapability{
			SupportsMarkdown: true,
			SupportsMedia:    true,
			SupportsVoice:    false,
		},
		SegmentHint: "custom",
	}
	RegisterChannelPolicy(ChannelWechat, custom)
	restored := GetChannelPolicy(ChannelWechat)
	if restored.MaxCharacters != 300 {
		t.Fatalf("expected 300 after register, got %d", restored.MaxCharacters)
	}
	if !restored.Capabilities.SupportsMarkdown {
		t.Fatal("expected markdown support after register")
	}
	builtinPolicies[ChannelWechat] = defaultChannelPolicy(ChannelWechat)
}

func TestKnownChannels(t *testing.T) {
	channels := KnownChannels()
	if len(channels) != 4 {
		t.Fatalf("expected 4 known channels, got %d", len(channels))
	}
}

func TestRenderWithChannel_WechatDisablesIntimateTone(t *testing.T) {
	plan := interaction.ExpressionPlan{
		Policy: interaction.ExpressionPolicy{
			MaxCharacters: 200,
			MaxSentences:  3,
		},
		Tones: []interaction.ExpressionTone{
			interaction.ExpressionToneWarm,
			interaction.ExpressionToneIntimate,
		},
	}
	result := RenderWithChannel(ChannelWechat, plan)
	foundIntimate := false
	for _, tone := range result.Tones {
		if tone == interaction.ExpressionToneIntimate {
			foundIntimate = true
			break
		}
	}
	if foundIntimate {
		t.Fatal("wechat render should have removed intimate tone")
	}
}

func TestRenderWithChannel_WebPreservesMarkdown(t *testing.T) {
	plan := interaction.ExpressionPlan{
		Policy: interaction.ExpressionPolicy{
			MaxCharacters: 500,
			MaxSentences:  5,
		},
	}
	result := RenderWithChannel(ChannelWeb, plan)
	if result.Version != interaction.ExpressionPlanVersionV1 {
		t.Fatalf("expected plan version v1, got %s", result.Version)
	}
}

func TestRenderWithChannel_UnknownChannelSafeText(t *testing.T) {
	plan := interaction.ExpressionPlan{
		Policy: interaction.ExpressionPolicy{
			MaxCharacters: 0,
		},
		Tones: []interaction.ExpressionTone{
			interaction.ExpressionToneIntimate,
			interaction.ExpressionTonePlayful,
		},
	}
	result := RenderWithChannel("unknown_test_channel", plan)
	for _, tone := range result.Tones {
		if tone == interaction.ExpressionToneIntimate {
			return
		}
	}
}

func TestRenderWithChannel_VoiceSingleSegment(t *testing.T) {
	plan := interaction.ExpressionPlan{
		Policy: interaction.ExpressionPolicy{
			MaxCharacters: 120,
			MaxSentences:  1,
		},
		Tones: []interaction.ExpressionTone{
			interaction.ExpressionToneWarm,
		},
	}
	result := RenderWithChannel(ChannelVoice, plan)
	if result.Policy.MaxCharacters > 120 {
		t.Fatalf("voice expected max 120, got %d", result.Policy.MaxCharacters)
	}
}
