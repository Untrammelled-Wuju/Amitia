package ffmpeg

import (
	"context"

	"github.com/u-ai/backend/internal/media/ffmpeg"
)

type FFmpegCapabilityState struct {
	Supported bool `json:"supported"`

	FFmpegAvailable  bool `json:"ffmpegAvailable"`
	FFprobeAvailable bool `json:"ffprobeAvailable"`

	FFmpegVersion  string `json:"ffmpegVersion,omitempty"`
	FFprobeVersion string `json:"ffprobeVersion,omitempty"`

	RuntimeID string `json:"runtimeId,omitempty"`

	Platform     string `json:"platform,omitempty"`
	Architecture string `json:"architecture,omitempty"`

	Source string `json:"source,omitempty"`

	NetworkProtocolsAllowed bool `json:"networkProtocolsAllowed"`

	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type FFmpegProvider interface {
	CapabilityState(ctx context.Context) FFmpegCapabilityState
}

type androidFFmpegProvider struct {
	backend ffmpeg.Backend
}

func NewAndroidFFmpegProvider(backend ffmpeg.Backend) FFmpegProvider {
	return &androidFFmpegProvider{backend: backend}
}

func (p *androidFFmpegProvider) CapabilityState(ctx context.Context) FFmpegCapabilityState {
	state := p.backend.Capabilities(ctx)
	return FFmpegCapabilityState{
		Supported:               state.Supported,
		FFmpegAvailable:         state.FFmpegAvailable,
		FFprobeAvailable:        state.FFprobeAvailable,
		FFmpegVersion:           state.FFmpegVersion,
		FFprobeVersion:          state.FFprobeVersion,
		RuntimeID:               state.RuntimeID,
		Platform:                state.Platform,
		Architecture:            state.Architecture,
		Source:                  state.Source,
		NetworkProtocolsAllowed: state.NetworkProtocolsAllowed,
		State:                   state.State,
		Reason:                  state.Reason,
	}
}

type blockedFFmpegProvider struct{}

func NewBlockedFFmpegProvider() FFmpegProvider {
	return &blockedFFmpegProvider{}
}

func (b *blockedFFmpegProvider) CapabilityState(ctx context.Context) FFmpegCapabilityState {
	return FFmpegCapabilityState{
		Supported:               false,
		FFmpegAvailable:         false,
		FFprobeAvailable:        false,
		NetworkProtocolsAllowed: false,
		State:                   "unavailable",
		Reason:                  AndroidFFmpegUnavilable,
	}
}
