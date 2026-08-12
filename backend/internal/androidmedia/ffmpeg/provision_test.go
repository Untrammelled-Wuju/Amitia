package ffmpeg

import (
	"testing"

	mediaffmpeg "github.com/u-ai/backend/internal/media/ffmpeg"
)

func TestProvisionEnvironment_NoPaths(t *testing.T) {
	config := AndroidEnvironmentConfig{}
	result := ProvisionEnvironment(config)

	if result.Success {
		t.Error("expected failure when no paths available")
	}
	if result.Source != mediaffmpeg.BinarySourceUnavailable {
		t.Errorf("expected unavailable source, got %v", result.Source)
	}
	if result.Reason != AndroidFFmpegRuntimePackageMissing {
		t.Errorf("expected %q reason, got %q", AndroidFFmpegRuntimePackageMissing, result.Reason)
	}
}

func TestProvisionEnvironment_ExplicitPaths(t *testing.T) {
	config := AndroidEnvironmentConfig{
		FFmpegPath:  "/usr/bin/ffmpeg",
		FFprobePath: "/usr/bin/ffprobe",
	}
	result := ProvisionEnvironment(config)

	if !result.Success {
		t.Error("expected success with explicit paths")
	}
	if result.FFmpegPath != "/usr/bin/ffmpeg" {
		t.Errorf("expected /usr/bin/ffmpeg, got %q", result.FFmpegPath)
	}
	if result.FFprobePath != "/usr/bin/ffprobe" {
		t.Errorf("expected /usr/bin/ffprobe, got %q", result.FFprobePath)
	}
}

func TestCheckArchitecture_Match(t *testing.T) {
	err := CheckArchitecture(mediaffmpeg.ArchARM64)
	if err != nil {
		t.Errorf("expected no error for ARM64, got: %v", err)
	}
}

func TestCheckArchitecture_Mismatch(t *testing.T) {
	err := CheckArchitecture(mediaffmpeg.ArchX86_64)
	if err == nil {
		t.Error("expected error for x86_64 mismatch")
	}
	archErr, ok := err.(*ffmpegArchError)
	if !ok {
		t.Fatalf("expected *ffmpegArchError, got %T", err)
	}
	if archErr.expected != mediaffmpeg.ArchX86_64 {
		t.Errorf("expected x86_64 expected, got %v", archErr.expected)
	}
}

func TestFFmpegArchError_Error(t *testing.T) {
	err := &ffmpegArchError{
		expected: mediaffmpeg.ArchARM64,
		actual:   mediaffmpeg.ArchX86_64,
	}
	expected := AndroidFFmpegArchUnsupported + ": expected arm64, got x86_64"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestDetectHostArch(t *testing.T) {
	arch := detectHostArch()
	if arch != mediaffmpeg.ArchARM64 {
		t.Errorf("expected ARM64, got %v", arch)
	}
}
