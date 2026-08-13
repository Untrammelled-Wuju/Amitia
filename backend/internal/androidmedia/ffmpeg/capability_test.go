package ffmpeg

import (
	"context"
	"testing"

	mediaffmpeg "github.com/u-ai/backend/internal/media/ffmpeg"
)

type mockBackend struct {
	state mediaffmpeg.CapabilityState
}

func (m *mockBackend) Capabilities(ctx context.Context) mediaffmpeg.CapabilityState {
	return m.state
}

func (m *mockBackend) GetCapabilities(ctx context.Context) (*mediaffmpeg.Capabilities, error) {
	return nil, nil
}

func (m *mockBackend) CheckVersion(ctx context.Context) (*mediaffmpeg.Environment, error) {
	return nil, nil
}

func (m *mockBackend) Probe(ctx context.Context, localPath string) (*mediaffmpeg.ProbeResult, error) {
	return nil, nil
}

func (m *mockBackend) ProbeFull(ctx context.Context, localPath string) (*mediaffmpeg.FullProbeResult, error) {
	return nil, nil
}

func (m *mockBackend) CancelAll() {}

func TestAndroidFFmpegProvider_CapabilityState(t *testing.T) {
	backend := &mockBackend{
		state: mediaffmpeg.CapabilityState{
			Supported:        true,
			FFmpegAvailable:  true,
			FFprobeAvailable: true,
			FFmpegVersion:    "4.4.2",
			Source:           "runtime_package",
			State:            "available",
		},
	}

	provider := NewAndroidFFmpegProvider(backend)
	state := provider.CapabilityState(context.Background())

	if !state.Supported {
		t.Error("expected supported")
	}
	if !state.FFmpegAvailable {
		t.Error("expected ffmpeg available")
	}
	if !state.FFprobeAvailable {
		t.Error("expected ffprobe available")
	}
	if state.Source != "runtime_package" {
		t.Errorf("expected source runtime_package, got %s", state.Source)
	}
}
