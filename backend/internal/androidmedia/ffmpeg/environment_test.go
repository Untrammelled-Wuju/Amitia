package ffmpeg

import (
	"testing"

	mediaffmpeg "github.com/u-ai/backend/internal/media/ffmpeg"
)

func TestDefaultAndroidEnvironmentConfig(t *testing.T) {
	config := DefaultAndroidEnvironmentConfig()
	if config.ExpectedArch != mediaffmpeg.ArchARM64 {
		t.Errorf("expected ARM64 arch, got %v", config.ExpectedArch)
	}
	if config.Platform == "" {
		t.Error("expected non-empty platform")
	}
}

func TestAndroidEnvironmentConfig_ResolvePaths_Explicit(t *testing.T) {
	config := AndroidEnvironmentConfig{
		FFmpegPath:  "/usr/bin/ffmpeg",
		FFprobePath: "/usr/bin/ffprobe",
	}

	fp, ffp, source := config.ResolvePaths()
	if fp != "/usr/bin/ffmpeg" {
		t.Errorf("expected /usr/bin/ffmpeg, got %q", fp)
	}
	if ffp != "/usr/bin/ffprobe" {
		t.Errorf("expected /usr/bin/ffprobe, got %q", ffp)
	}
	if source != mediaffmpeg.BinarySourceConfigured {
		t.Errorf("expected configured source, got %v", source)
	}
}

func TestAndroidEnvironmentConfig_ResolvePaths_Missing(t *testing.T) {
	config := AndroidEnvironmentConfig{}

	fp, ffp, source := config.ResolvePaths()
	if fp != "" {
		t.Errorf("expected empty path, got %q", fp)
	}
	if ffp != "" {
		t.Errorf("expected empty path, got %q", ffp)
	}
	if source != mediaffmpeg.BinarySourceUnavailable {
		t.Errorf("expected unavailable source, got %v", source)
	}
}

func TestArchString(t *testing.T) {
	tests := []struct {
		input    mediaffmpeg.Architecture
		expected string
	}{
		{mediaffmpeg.ArchARM64, "arm64"},
		{mediaffmpeg.ArchX86_64, "x86_64"},
		{mediaffmpeg.ArchARM, "arm"},
		{mediaffmpeg.ArchUnknown, "unknown"},
		{mediaffmpeg.Architecture(""), "arm64"},
	}

	for _, tt := range tests {
		got := archString(tt.input)
		if got != tt.expected {
			t.Errorf("archString(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestLocalFileExists(t *testing.T) {
	if localFileExists("") {
		t.Error("empty path should not exist")
	}
	if localFileExists("/tmp/non-existent-file-for-test") {
		t.Error("non-existent file should not exist")
	}
}
