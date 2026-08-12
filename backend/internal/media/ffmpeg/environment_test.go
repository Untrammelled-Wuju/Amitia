package ffmpeg

import "testing"

func TestEnvironment_HasFFmpeg(t *testing.T) {
	tests := []struct {
		name     string
		env      *Environment
		expected bool
	}{
		{
			name:     "empty",
			env:      &Environment{},
			expected: false,
		},
		{
			name:     "path only",
			env:      &Environment{FFmpegPath: "/usr/bin/ffmpeg"},
			expected: false,
		},
		{
			name:     "available no path",
			env:      &Environment{Available: true},
			expected: false,
		},
		{
			name:     "complete",
			env:      &Environment{FFmpegPath: "/usr/bin/ffmpeg", Available: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.env.HasFFmpeg(); got != tt.expected {
				t.Errorf("HasFFmpeg() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEnvironment_HasFFprobe(t *testing.T) {
	env := &Environment{FFprobePath: "/usr/bin/ffprobe", Available: true}
	if !env.HasFFprobe() {
		t.Error("expected HasFFprobe true")
	}

	env = &Environment{FFprobePath: "/usr/bin/ffprobe", Available: false}
	if env.HasFFprobe() {
		t.Error("expected HasFFprobe false when not available")
	}
}

func TestEnvironment_Complete(t *testing.T) {
	tests := []struct {
		name     string
		env      *Environment
		expected bool
	}{
		{
			name:     "nothing",
			env:      &Environment{},
			expected: false,
		},
		{
			name:     "ffmpeg only",
			env:      &Environment{FFmpegPath: "/usr/bin/ffmpeg", Available: true},
			expected: false,
		},
		{
			name:     "ffprobe only",
			env:      &Environment{FFprobePath: "/usr/bin/ffprobe", Available: true},
			expected: false,
		},
		{
			name: "all",
			env: &Environment{
				FFmpegPath:  "/usr/bin/ffmpeg",
				FFprobePath: "/usr/bin/ffprobe",
				Available:   true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.env.Complete()
			if got != tt.expected {
				t.Errorf("Complete() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestUnavailableEnvironment(t *testing.T) {
	env := UnavailableEnvironment("runtime-1", "not found")
	if env.Available {
		t.Error("expected not available")
	}
	if env.RuntimeID != "runtime-1" {
		t.Errorf("expected runtime-1, got %q", env.RuntimeID)
	}
	if env.Source != BinarySourceUnavailable {
		t.Errorf("expected unavailable source, got %v", env.Source)
	}
	if len(env.Diagnostics) != 1 || env.Diagnostics[0] != "not found" {
		t.Errorf("unexpected diagnostics: %v", env.Diagnostics)
	}
}
