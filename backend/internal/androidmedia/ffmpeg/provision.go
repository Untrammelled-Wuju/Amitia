package ffmpeg

import (
	"github.com/u-ai/backend/internal/media/ffmpeg"
)

type ProvisionResult struct {
	Success bool

	FFmpegPath  string
	FFprobePath string

	Source ffmpeg.BinarySource

	Reason string
}

func ProvisionEnvironment(config AndroidEnvironmentConfig) ProvisionResult {
	ffmpegPath, ffprobePath, source := config.ResolvePaths()

	if ffmpegPath == "" || ffprobePath == "" {
		return ProvisionResult{
			Success: false,
			Source:  ffmpeg.BinarySourceUnavailable,
			Reason:  AndroidFFmpegRuntimePackageMissing,
		}
	}

	return ProvisionResult{
		Success:     true,
		FFmpegPath:  ffmpegPath,
		FFprobePath: ffprobePath,
		Source:      source,
	}
}

func CheckArchitecture(expected ffmpeg.Architecture) error {
	actual := detectHostArch()
	if actual != expected {
		return &ffmpegArchError{expected: expected, actual: actual}
	}
	return nil
}

type ffmpegArchError struct {
	expected ffmpeg.Architecture
	actual   ffmpeg.Architecture
}

func (e *ffmpegArchError) Error() string {
	return AndroidFFmpegArchUnsupported + ": expected " + string(e.expected) + ", got " + string(e.actual)
}

func detectHostArch() ffmpeg.Architecture {
	return ffmpeg.ArchARM64
}
