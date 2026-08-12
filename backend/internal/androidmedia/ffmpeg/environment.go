package ffmpeg

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/u-ai/backend/internal/media/ffmpeg"
)

const (
	AndroidFFmpegUnavilable          = "ANDROID_MEDIA_FFMPEG_UNAVAILABLE"
	AndroidFFmpegRuntimePackageMissing = "ANDROID_MEDIA_FFMPEG_RUNTIME_PACKAGE_MISSING"
	AndroidFFmpegArchUnsupported     = "ANDROID_MEDIA_FFMPEG_ARCH_UNSUPPORTED"
)

type AndroidEnvironmentConfig struct {
	ExpectedArch    ffmpeg.Architecture
	Platform        string
	FFmpegPath      string
	FFprobePath     string
	BundleRootPath  string
}

func DefaultAndroidEnvironmentConfig() AndroidEnvironmentConfig {
	return AndroidEnvironmentConfig{
		ExpectedArch: ffmpeg.ArchARM64,
		Platform:     runtime.GOOS,
	}
}

func (c AndroidEnvironmentConfig) ResolvePaths() (ffmpegPath, ffprobePath string, source ffmpeg.BinarySource) {
	if c.FFmpegPath != "" && c.FFprobePath != "" {
		return c.FFmpegPath, c.FFprobePath, ffmpeg.BinarySourceConfigured
	}

	if c.BundleRootPath != "" {
		arch := archString(c.ExpectedArch)
		platform := c.Platform
		if platform == "" {
			platform = "linux"
		}

		candidates := []struct {
			ffmpeg  string
			ffprobe string
		}{
			{
				filepath.Join(c.BundleRootPath, "ffmpeg", platform, arch, "bin", "ffmpeg"),
				filepath.Join(c.BundleRootPath, "ffmpeg", platform, arch, "bin", "ffprobe"),
			},
			{
				filepath.Join(c.BundleRootPath, "packages", "ffmpeg", platform, arch, "bin", "ffmpeg"),
				filepath.Join(c.BundleRootPath, "packages", "ffmpeg", platform, arch, "bin", "ffprobe"),
			},
		}

		for _, cand := range candidates {
			if localFileExists(cand.ffmpeg) && localFileExists(cand.ffprobe) {
				return cand.ffmpeg, cand.ffprobe, ffmpeg.BinarySourceRuntimePackage
			}
		}
	}

	return "", "", ffmpeg.BinarySourceUnavailable
}

func localFileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func archString(a ffmpeg.Architecture) string {
	if a == "" {
		return "arm64"
	}
	return string(a)
}
