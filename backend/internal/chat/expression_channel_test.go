package chat

import (
	"testing"

	"github.com/u-ai/backend/internal/expression"
)

func TestResolveExpressionChannelUsesVoiceForVoiceMessage(t *testing.T) {
	if got := resolveExpressionChannel("web", true); got != expression.ChannelVoice {
		t.Fatalf("expected voice channel for voice message, got %s", got)
	}
}

func TestResolveExpressionChannelKeepsTextChannels(t *testing.T) {
	cases := []struct {
		channel string
		want    expression.ChannelKind
	}{
		{"wechat", expression.ChannelWechat},
		{"qq", expression.ChannelQQ},
		{"web", expression.ChannelWeb},
	}
	for _, tc := range cases {
		if got := resolveExpressionChannel(tc.channel, false); got != tc.want {
			t.Fatalf("resolveExpressionChannel(%q) = %s, want %s", tc.channel, got, tc.want)
		}
	}
}
