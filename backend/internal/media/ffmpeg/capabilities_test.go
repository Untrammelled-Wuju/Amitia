package ffmpeg

import "testing"

func TestDisabledCapabilityState(t *testing.T) {
	state := DisabledCapabilityState("runtime-1", "not found")
	if state.Supported {
		t.Error("expected not supported")
	}
	if state.FFmpegAvailable {
		t.Error("expected ffmpeg not available")
	}
	if state.FFprobeAvailable {
		t.Error("expected ffprobe not available")
	}
	if state.RuntimeID != "runtime-1" {
		t.Errorf("expected runtime-1, got %q", state.RuntimeID)
	}
	if state.State != "disabled" {
		t.Errorf("expected disabled state, got %q", state.State)
	}
	if state.Reason != "not found" {
		t.Errorf("expected 'not found', got %q", state.Reason)
	}
}

func TestCapabilityState_IsUsable(t *testing.T) {
	tests := []struct {
		name     string
		state    CapabilityState
		expected bool
	}{
		{
			name:     "empty",
			state:    CapabilityState{},
			expected: false,
		},
		{
			name:     "supported only",
			state:    CapabilityState{Supported: true},
			expected: false,
		},
		{
			name:     "supported with ffmpeg",
			state:    CapabilityState{Supported: true, FFmpegAvailable: true},
			expected: true,
		},
		{
			name:     "ffmpeg without supported",
			state:    CapabilityState{FFmpegAvailable: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsUsable(); got != tt.expected {
				t.Errorf("IsUsable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCapabilities_Available(t *testing.T) {
	caps := Capabilities{
		Available: true,
		Probe:     true,
		StreamCopy: true,
	}

	if !caps.Available {
		t.Error("expected available")
	}
	if !caps.Probe {
		t.Error("expected probe support")
	}
	if !caps.StreamCopy {
		t.Error("expected stream copy support")
	}
}
