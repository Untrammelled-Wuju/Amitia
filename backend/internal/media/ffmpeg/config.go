package ffmpeg

import "time"

type Config struct {
	FFmpegPath  string
	FFprobePath string

	MaxProcessDuration time.Duration

	MaxStdoutBytes int64
	MaxStderrBytes int64

	MaxConcurrentProcesses int

	MaxProbeOutputBytes int64

	AllowNetworkProtocols bool

	AllowedProtocols []string
}

func DefaultConfig() Config {
	return Config{
		MaxProcessDuration:     5 * time.Minute,
		MaxStdoutBytes:         4 * 1024 * 1024,
		MaxStderrBytes:         1 * 1024 * 1024,
		MaxConcurrentProcesses: 1,
		MaxProbeOutputBytes:    1 * 1024 * 1024,
		AllowNetworkProtocols:  false,
		AllowedProtocols:       []string{"file", "pipe"},
	}
}

func (c Config) EffectiveMaxStdout() int64 {
	if c.MaxStdoutBytes <= 0 {
		return 4 * 1024 * 1024
	}
	return c.MaxStdoutBytes
}

func (c Config) EffectiveMaxStderr() int64 {
	if c.MaxStderrBytes <= 0 {
		return 1 * 1024 * 1024
	}
	return c.MaxStderrBytes
}

func (c Config) EffectiveMaxConcurrent() int {
	if c.MaxConcurrentProcesses <= 0 {
		return 1
	}
	return c.MaxConcurrentProcesses
}
