// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"fmt"

	"github.com/u-ai/backend/internal/asr"
)

type VoiceRoutePlanner interface {
	Plan(session VoiceSessionRequest, capabilities VoiceCapabilities) (VoiceExecutionPlan, error)
}

type defaultRoutePlanner struct {
	preferredASRType string
	preferredProvier string
}

func NewRoutePlanner() VoiceRoutePlanner {
	return &defaultRoutePlanner{}
}

func (p *defaultRoutePlanner) Plan(session VoiceSessionRequest, caps VoiceCapabilities) (VoiceExecutionPlan, error) {
	plan := VoiceExecutionPlan{
		Path: "unknown",
	}

	_, streamingOK := asr.ResolveStreamingProvider(p.preferredASRType)

	if session.Mode == ContinuousVoiceSessionModeProviderRealtime && caps.FullDuplexRealtime {
		plan.Path = "provider_realtime"
		plan.RealtimeProviderID = p.preferredProvier
		plan.RequiresNetwork = true
		if err := plan.Validate(); err != nil {
			return plan, err
		}
		return plan, nil
	}

	if streamingOK && caps.StreamingASR {
		plan.Path = "local_vad_streaming_asr_streaming_tts"
		plan.UseLocalVAD = caps.VAD
		plan.UseStreamingASR = true
		plan.UseStreamingTTS = caps.StreamingTTS
		plan.UseFullTTS = !caps.StreamingTTS
		plan.RequiresNetwork = true
		if err := plan.Validate(); err != nil {
			return plan, err
		}
		return plan, nil
	}

	if caps.VAD && caps.StreamingTTS {
		plan.Path = "local_vad_segment_asr_streaming_tts"
		plan.UseLocalVAD = true
		plan.UseSegmentASR = true
		plan.UseStreamingTTS = true
		plan.RequiresNetwork = true
		if err := plan.Validate(); err != nil {
			return plan, err
		}
		return plan, nil
	}

	if caps.VAD {
		plan.Path = "local_vad_segment_asr_full_tts"
		plan.UseLocalVAD = true
		plan.UseSegmentASR = true
		plan.UseFullTTS = true
		plan.RequiresNetwork = true
		if err := plan.Validate(); err != nil {
			return plan, err
		}
		return plan, nil
	}

	return plan, fmt.Errorf("voice route planner: no viable execution path for mode=%s", session.Mode)
}

func (p *defaultRoutePlanner) SetPreferredProvider(asrType, providerID string) {
	p.preferredASRType = asrType
	p.preferredProvier = providerID
}

type VoiceRoutePolicy struct {
	AllowCloudFallback   bool
	AllowSegmentFallback bool
	RequireNetwork       bool
	MaxLocalEndpoitMs    int
}

func DefaultRoutePolicy() VoiceRoutePolicy {
	return VoiceRoutePolicy{
		AllowCloudFallback:   true,
		AllowSegmentFallback: true,
		RequireNetwork:       false,
		MaxLocalEndpoitMs:    2000,
	}
}

func (p *defaultRoutePlanner) PlanWithPolicy(session VoiceSessionRequest, caps VoiceCapabilities, policy VoiceRoutePolicy) (VoiceExecutionPlan, error) {
	plan, err := p.Plan(session, caps)
	if err != nil {
		return plan, err
	}

	if plan.RequiresNetwork && policy.RequireNetwork {
		return plan, fmt.Errorf("voice route planner: network required but policy forbids")
	}

	if plan.UseSegmentASR && !policy.AllowSegmentFallback {
		return plan, fmt.Errorf("voice route planner: segment ASR required but policy forbids")
	}

	return plan, nil
}

