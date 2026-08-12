package ffmpeg

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.FFmpegPath != "" {
		t.Errorf("expected empty ffmpeg path, got %q", config.FFmpegPath)
	}

	if config.MaxProcessDuration != 5*time.Minute {
		t.Errorf("expected max process duration 5m, got %v", config.MaxProcessDuration)
	}

	if config.MaxStdoutBytes != 4*1024*1024 {
		t.Errorf("expected max stdout 4MiB, got %d", config.MaxStdoutBytes)
	}

	if config.MaxStderrBytes != 1*1024*1024 {
		t.Errorf("expected max stderr 1MiB, got %d", config.MaxStderrBytes)
	}

	if config.MaxConcurrentProcesses != 1 {
		t.Errorf("expected max concurrent 1, got %d", config.MaxConcurrentProcesses)
	}

	if config.AllowNetworkProtocols {
		t.Error("expected network protocols disabled by default")
	}

	if len(config.AllowedProtocols) != 2 {
		t.Errorf("expected 2 allowed protocols, got %d", len(config.AllowedProtocols))
	}
}

func TestEffectiveMaxStdout(t *testing.T) {
	config := Config{MaxStdoutBytes: 0}
	if got := config.EffectiveMaxStdout(); got != 4*1024*1024 {
		t.Errorf("expected default 4MiB, got %d", got)
	}

	config = Config{MaxStdoutBytes: 1024}
	if got := config.EffectiveMaxStdout(); got != 1024 {
		t.Errorf("expected 1024, got %d", got)
	}
}

func TestEffectiveMaxStderr(t *testing.T) {
	config := Config{MaxStderrBytes: 0}
	if got := config.EffectiveMaxStderr(); got != 1*1024*1024 {
		t.Errorf("expected default 1MiB, got %d", got)
	}

	config = Config{MaxStderrBytes: 512}
	if got := config.EffectiveMaxStderr(); got != 512 {
		t.Errorf("expected 512, got %d", got)
	}
}

func TestEffectiveMaxConcurrent(t *testing.T) {
	config := Config{MaxConcurrentProcesses: 0}
	if got := config.EffectiveMaxConcurrent(); got != 1 {
		t.Errorf("expected default 1, got %d", got)
	}

	config = Config{MaxConcurrentProcesses: 3}
	if got := config.EffectiveMaxConcurrent(); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}
