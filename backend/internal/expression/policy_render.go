package expression

import (
	"github.com/u-ai/backend/internal/interaction"
)

func ApplyChannelPolicy(policy ChannelPolicy, plan interaction.ExpressionPlan) interaction.ExpressionPlan {
	constraints := buildConstraints(policy)
	result := interaction.RenderExpressionStrategy(plan, constraints)
	return result.Plan
}

func buildConstraints(policy ChannelPolicy) interaction.ExpressionRenderConstraints {
	c := interaction.ExpressionRenderConstraints{}

	if !policy.Capabilities.SupportsMarkdown {
		c.DisabledGuardClaims = append(c.DisabledGuardClaims, "markdown_formatting")
	}

	if !policy.Capabilities.SupportsVoice {
		c.DisabledEmotionKinds = append(c.DisabledEmotionKinds, "voice_emphasis")
	}

	if policy.MaxSegments <= 1 {
		c.DisabledGuardClaims = append(c.DisabledGuardClaims, "multi_segment_output")
	}

	if policy.SegmentHint == "short_per_line" {
		c.DisabledTones = append(c.DisabledTones, interaction.ExpressionToneIntimate)
	}

	return c
}

func NewChannelRenderConstraints(kind ChannelKind) interaction.ExpressionRenderConstraints {
	policy := GetChannelPolicy(kind)
	return buildConstraints(policy)
}

func RenderWithChannel(kind ChannelKind, plan interaction.ExpressionPlan) interaction.ExpressionPlan {
	policy := GetChannelPolicy(kind)
	return ApplyChannelPolicy(policy, plan)
}