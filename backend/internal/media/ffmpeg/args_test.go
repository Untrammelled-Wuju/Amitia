package ffmpeg

import (
	"testing"
)

func TestBuildVersionArgs(t *testing.T) {
	args := BuildVersionArgs()
	if len(args) != 1 || args[0] != "-version" {
		t.Errorf("unexpected version args: %v", args)
	}
}

func TestBuildFFprobeArgs(t *testing.T) {
	args := BuildFFprobeArgs("/tmp/test.mp4")
	expected := []string{"-v", "error", "-show_streams", "-show_format", "-print_format", "json", "/tmp/test.mp4"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d", len(expected), len(args))
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("arg[%d] = %q, want %q", i, arg, expected[i])
		}
	}
}

func TestBuildBaseFlags(t *testing.T) {
	flags := BuildBaseFlags()
	if len(flags) != 4 {
		t.Errorf("expected 4 base flags, got %d", len(flags))
	}
}

func TestBuildProgressFlags(t *testing.T) {
	flags := BuildProgressFlags()
	if len(flags) != 3 {
		t.Errorf("expected 3 progress flags, got %d", len(flags))
	}
}

func TestIsNetworkProtocol(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"http://example.com/video.mp4", true},
		{"https://example.com/video.mp4", true},
		{"rtsp://camera.local/stream", true},
		{"rtmp://live.example.com/stream", true},
		{"tcp://localhost:8080", true},
		{"file:///tmp/video.mp4", false},
		{"/tmp/video.mp4", false},
		{"relative/path.mp4", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsNetworkProtocol(tt.input)
			if got != tt.expected {
				t.Errorf("IsNetworkProtocol(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsAllowedProtocol(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		allowed  []string
		expected bool
	}{
		{"empty allowed, file", "/tmp/video.mp4", nil, true},
		{"empty allowed, http", "http://x.com/v.mp4", nil, false},
		{"file allowed, file", "/tmp/video.mp4", []string{"file", "pipe"}, true},
		{"file allowed, http", "http://x.com/v.mp4", []string{"file", "pipe"}, false},
		{"http allowed", "http://x.com/v.mp4", []string{"http", "https"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAllowedProtocol(tt.input, tt.allowed)
			if got != tt.expected {
				t.Errorf("IsAllowedProtocol(%q, %v) = %v, want %v", tt.input, tt.allowed, got, tt.expected)
			}
		})
	}
}

func TestSanitizeInputPath(t *testing.T) {
	config := DefaultConfig()

	err := SanitizeInputPath("", config)
	if err != nil {
		t.Errorf("empty path should be ok, got: %v", err)
	}

	err = SanitizeInputPath("/tmp/video.mp4", config)
	if err != nil {
		t.Errorf("local file should be ok, got: %v", err)
	}

	err = SanitizeInputPath("http://example.com/video.mp4", config)
	if err == nil {
		t.Error("expected network protocol error")
	}
	if err.(*Error).Code != FFMPEG_PROTOCOL_FORBIDDEN {
		t.Errorf("expected FFMPEG_PROTOCOL_FORBIDDEN, got: %v", err.(*Error).Code)
	}

	config.AllowNetworkProtocols = true
	err = SanitizeInputPath("http://example.com/video.mp4", config)
	if err != nil {
		t.Errorf("network protocol should be allowed, got: %v", err)
	}
}
