package ffmpeg

import "testing"

func TestError_Error(t *testing.T) {
	err := NewError(FFMPEG_UNAVAILABLE, "not found")
	expected := "FFMPEG_UNAVAILABLE: not found"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestError_Nil(t *testing.T) {
	var err *Error
	if err.Error() != "" {
		t.Errorf("expected empty string for nil error, got %q", err.Error())
	}
}

func TestNewError(t *testing.T) {
	err := NewError(FFMPEG_BINARY_NOT_FOUND, "missing")
	if err.Code != FFMPEG_BINARY_NOT_FOUND {
		t.Errorf("expected code FFMPEG_BINARY_NOT_FOUND, got %q", err.Code)
	}
	if err.Message != "missing" {
		t.Errorf("expected message 'missing', got %q", err.Message)
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []string{
		FFMPEG_UNAVAILABLE,
		FFMPEG_BINARY_NOT_FOUND,
		FFMPEG_BINARY_INVALID,
		FFMPEG_BINARY_INTEGRITY_FAILED,
		FFMPEG_ARCH_MISMATCH,
		FFPROBE_NOT_FOUND,
		FFPROBE_INVALID,
		FFMPEG_RUNTIME_UNAVAILABLE,
		FFMPEG_PROCESS_SPAWN_UNSUPPORTED,
		FFMPEG_PROCESS_FAILED,
		FFMPEG_PROCESS_TIMEOUT,
		FFMPEG_PROCESS_CANCELLED,
		FFMPEG_PROTOCOL_FORBIDDEN,
		FFMPEG_OUTPUT_TOO_LARGE,
		FFMPEG_STDOUT_TOO_LARGE,
		FFMPEG_STDERR_TOO_LARGE,
		FFMPEG_INVALID_PROBE_OUTPUT,
	}

	for _, code := range codes {
		if code == "" {
			t.Error("error code should not be empty")
		}
	}
}
