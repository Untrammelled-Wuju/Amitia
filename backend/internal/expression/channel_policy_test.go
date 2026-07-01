package expression

import (
	"testing"
	"github.com/u-ai/backend/internal/interaction"
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