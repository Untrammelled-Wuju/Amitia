package ffmpeg

import "time"

type BinarySource string

const (
	BinarySourceBundled       BinarySource = "bundled"
	BinarySourceRuntimePackage BinarySource = "runtime_package"
	BinarySourceSystem        BinarySource = "system"
	BinarySourceConfigured    BinarySource = "configured"
	BinarySourceUnavailable   BinarySource = "unavailable"
)

type Architecture string

const (
	ArchARM64  Architecture = "arm64"
	ArchARM    Architecture = "arm"
	ArchX86_64 Architecture = "x86_64"
	ArchUnknown Architecture = "unknown"
)

type Environment struct {
	RuntimeID string

	FFmpegPath  string
	FFprobePath string

	Version      string
	ProbeVersion string

	Architecture Architecture
	Platform     string

	Source BinarySource

	Available bool

	Diagnostics []string

	HealthyAt time.Time
}

func (e *Environment) HasFFmpeg() bool {
	return e.FFmpegPath != "" && e.Available
}

func (e *Environment) HasFFprobe() bool {
	return e.FFprobePath != "" && e.Available
}

func (e *Environment) Complete() bool {
	return e.Available && e.HasFFmpeg() && e.HasFFprobe()
}

func UnavailableEnvironment(runtimeID string, reason string) *Environment {
	return &Environment{
		RuntimeID:    runtimeID,
		Source:       BinarySourceUnavailable,
		Available:    false,
		Diagnostics:  []string{reason},
	}
}
