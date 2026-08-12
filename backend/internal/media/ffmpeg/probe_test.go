package ffmpeg

import (
	"testing"
)

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard",
			input:    "ffmpeg version 4.4.2 Copyright (c) 2000-2021 the FFmpeg developers\n",
			expected: "4.4.2",
		},
		{
			name:     "git version",
			input:    "ffmpeg version N-109123-g58607 Copyright\n",
			expected: "N-109123-g58607",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "no version line",
			input:    "some random text\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseVersionOutput([]byte(tt.input))
			if got != tt.expected {
				t.Errorf("ParseVersionOutput() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseProbeOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *ProbeResult
	}{
		{
			name:  "empty",
			input: "",
			expected: nil,
		},
		{
			name:  "invalid json",
			input: "not json",
			expected: nil,
		},
		{
			name: "valid probe",
			input: `{
				"format": {
					"filename": "/tmp/test.mp4",
					"nb_streams": 2,
					"format_name": "mov,mp4,m4a,3gp,3g2,mj2",
					"duration": "120.5"
				},
				"streams": [
					{"codec_name": "h264", "codec_type": "video", "width": 1920, "height": 1080},
					{"codec_name": "aac", "codec_type": "audio"}
				]
			}`,
			expected: &ProbeResult{
				Valid:        true,
				FormatNames:  []string{"mov", "mp4", "m4a", "3gp", "3g2", "mj2"},
				DurationMS:   120500,
				Streams: []StreamSummary{
					{CodecName: "h264", CodecType: "video", Width: 1920, Height: 1080},
					{CodecName: "aac", CodecType: "audio"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseProbeOutput([]byte(tt.input))

			if tt.expected == nil {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Valid != tt.expected.Valid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.expected.Valid)
			}

			if result.DurationMS != tt.expected.DurationMS {
				t.Errorf("DurationMS = %d, want %d", result.DurationMS, tt.expected.DurationMS)
			}

			if len(result.FormatNames) != len(tt.expected.FormatNames) {
				t.Errorf("FormatNames length = %d, want %d", len(result.FormatNames), len(tt.expected.FormatNames))
			}

			if len(result.Streams) != len(tt.expected.Streams) {
				t.Errorf("Streams length = %d, want %d", len(result.Streams), len(tt.expected.Streams))
			}
		})
	}
}

func TestParseFormatNames(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"mov,mp4,m4a", []string{"mov", "mp4", "m4a"}},
		{"matroska,webm", []string{"matroska", "webm"}},
		{"", nil},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFormatNames(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("parseFormatNames(%q) = %v, want %v", tt.input, got, tt.expected)
				return
			}
			for i, name := range got {
				if name != tt.expected[i] {
					t.Errorf("parseFormatNames(%q)[%d] = %q, want %q", tt.input, i, name, tt.expected[i])
				}
			}
		})
	}
}
