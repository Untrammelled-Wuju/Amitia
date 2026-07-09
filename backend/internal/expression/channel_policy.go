package expression

import (
	"fmt"

	"github.com/u-ai/backend/internal/prompt/textlib"
)

type ChannelKind string

const (
	ChannelWechat ChannelKind = "wechat"
	ChannelQQ     ChannelKind = "qq"
	ChannelWeb    ChannelKind = "web"
	ChannelVoice  ChannelKind = "voice"
)

type ChannelCapability struct {
	SupportsMarkdown  bool `json:"supportsMarkdown"`
	SupportsMedia     bool `json:"supportsMedia"`
	SupportsVoice     bool `json:"supportsVoice"`
	SupportsSegmented bool `json:"supportsSegmented"`
}

type ChannelPolicy struct {
	Kind              ChannelKind       `json:"kind"`
	Version           string            `json:"version"`
	MaxCharacters     int               `json:"maxCharacters"`
	MinCharacters     int               `json:"minCharacters"`
	MaxSegments       int               `json:"maxSegments"`
	MinSegments       int               `json:"minSegments"`
	Capabilities      ChannelCapability `json:"capabilities"`
	SegmentHint       string            `json:"segmentHint,omitempty"`
	ShortRules        string            `json:"shortRules,omitempty"`
	AntiRepeatEnabled bool              `json:"antiRepeatEnabled"`
}

type ExpressionChannelPolicyVersion string

const (
	ExpressionChannelPolicyV1 ExpressionChannelPolicyVersion = "channel-policy-v1"
)

func defaultChannelPolicy(kind ChannelKind) ChannelPolicy {
	switch kind {
	case ChannelWechat:
		return ChannelPolicy{
			Kind:          ChannelWechat,
			Version:       string(ExpressionChannelPolicyV1),
			MaxCharacters: 200,
			MinCharacters: 10,
			MaxSegments:   3,
			MinSegments:   1,
			Capabilities: ChannelCapability{
				SupportsMarkdown:  false,
				SupportsMedia:     true,
				SupportsVoice:     false,
				SupportsSegmented: true,
			},
			SegmentHint:       "short_per_line",
			ShortRules:        textlib.RawChannelWechatShortRules,
			AntiRepeatEnabled: true,
		}
	case ChannelQQ:
		return ChannelPolicy{
			Kind:          ChannelQQ,
			Version:       string(ExpressionChannelPolicyV1),
			MaxCharacters: 200,
			MinCharacters: 10,
			MaxSegments:   3,
			MinSegments:   1,
			Capabilities: ChannelCapability{
				SupportsMarkdown:  false,
				SupportsMedia:     true,
				SupportsVoice:     false,
				SupportsSegmented: true,
			},
			SegmentHint:       "short_per_line",
			ShortRules:        textlib.RawChannelQQShortRules,
			AntiRepeatEnabled: true,
		}
	case ChannelWeb:
		return ChannelPolicy{
			Kind:          ChannelWeb,
			Version:       string(ExpressionChannelPolicyV1),
			MaxCharacters: 500,
			MinCharacters: 20,
			MaxSegments:   5,
			MinSegments:   0,
			Capabilities: ChannelCapability{
				SupportsMarkdown:  true,
				SupportsMedia:     true,
				SupportsVoice:     false,
				SupportsSegmented: true,
			},
			SegmentHint: "full_paragraph",
			ShortRules:  textlib.RawChannelWebDesktopRules,
		}
	case ChannelVoice:
		return ChannelPolicy{
			Kind:          ChannelVoice,
			Version:       string(ExpressionChannelPolicyV1),
			MaxCharacters: 120,
			MinCharacters: 5,
			MaxSegments:   1,
			MinSegments:   1,
			Capabilities: ChannelCapability{
				SupportsMarkdown:  false,
				SupportsMedia:     false,
				SupportsVoice:     true,
				SupportsSegmented: false,
			},
			SegmentHint: "single_utterance",
		}
	default:
		return ChannelPolicy{
			Kind:          kind,
			Version:       string(ExpressionChannelPolicyV1),
			MaxCharacters: 240,
			MinCharacters: 0,
			MaxSegments:   3,
			MinSegments:   0,
			Capabilities: ChannelCapability{
				SupportsMarkdown:  false,
				SupportsMedia:     false,
				SupportsVoice:     false,
				SupportsSegmented: false,
			},
			SegmentHint: "safe_text_fallback",
		}
	}
}

var builtinPolicies map[ChannelKind]ChannelPolicy

func init() {
	builtinPolicies = make(map[ChannelKind]ChannelPolicy, 4)
	for _, k := range []ChannelKind{ChannelWechat, ChannelQQ, ChannelWeb, ChannelVoice} {
		builtinPolicies[k] = defaultChannelPolicy(k)
	}
}

func GetChannelPolicy(kind ChannelKind) ChannelPolicy {
	p, ok := builtinPolicies[kind]
	if !ok {
		return defaultChannelPolicy(kind)
	}
	return p
}

func GetChannelPolicyVersion(kind ChannelKind, version string) (ChannelPolicy, error) {
	if version != string(ExpressionChannelPolicyV1) {
		return ChannelPolicy{}, fmt.Errorf("unknown channel policy version: %s", version)
	}
	return GetChannelPolicy(kind), nil
}

func RegisterChannelPolicy(kind ChannelKind, policy ChannelPolicy) {
	policy.Kind = kind
	if policy.Version == "" {
		policy.Version = string(ExpressionChannelPolicyV1)
	}
	builtinPolicies[kind] = policy
}

func KnownChannels() []ChannelKind {
	return []ChannelKind{ChannelWechat, ChannelQQ, ChannelWeb, ChannelVoice}
}

const ChannelStateVersionKey = "channel-state-version"

func ChannelStateVersionMapping() map[ChannelKind]string {
	return map[ChannelKind]string{
		ChannelWechat: "wechat-state-v1",
		ChannelQQ:     "qq-state-v1",
		ChannelWeb:    "web-state-v1",
		ChannelVoice:  "voice-state-v1",
	}
}

func GetChannelStateVersion(kind ChannelKind) string {
	mapping := ChannelStateVersionMapping()
	if v, ok := mapping[kind]; ok {
		return v
	}
	return "unknown-state-v1"
}
