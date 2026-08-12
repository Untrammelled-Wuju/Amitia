package ffmpeg

import (
	"github.com/u-ai/backend/internal/media/ffmpeg"
	"github.com/u-ai/backend/internal/runtimehost"
)

type AndroidBindingConfig struct {
	EnvConfig     AndroidEnvironmentConfig
	BaseConfig    ffmpeg.Config
}

func DefaultAndroidBindingConfig() AndroidBindingConfig {
	return AndroidBindingConfig{
		EnvConfig:  DefaultAndroidEnvironmentConfig(),
		BaseConfig: ffmpeg.DefaultConfig(),
	}
}

func BuildAndroidConfig(bindingConfig AndroidEnvironmentConfig) ffmpeg.Config {
	ffmpegPath, ffprobePath, _ := bindingConfig.ResolvePaths()

	config := ffmpeg.DefaultConfig()
	config.FFmpegPath = ffmpegPath
	config.FFprobePath = ffprobePath

	return config
}

func NewAndroidBackend(host runtimehost.RuntimeHost, bindingConfig AndroidEnvironmentConfig) ffmpeg.Backend {
	ffmpegPath, ffprobePath, _ := bindingConfig.ResolvePaths()

	config := ffmpeg.DefaultConfig()
	config.FFmpegPath = ffmpegPath
	config.FFprobePath = ffprobePath

	return ffmpeg.NewBackend(host, config)
}

func NewAndroidBackendFromConfig(host runtimehost.RuntimeHost, baseConfig ffmpeg.Config, bindingConfig AndroidEnvironmentConfig) ffmpeg.Backend {
	ffmpegPath, ffprobePath, _ := bindingConfig.ResolvePaths()

	config := baseConfig
	if config.FFmpegPath == "" {
		config.FFmpegPath = ffmpegPath
	}
	if config.FFprobePath == "" {
		config.FFprobePath = ffprobePath
	}

	return ffmpeg.NewBackend(host, config)
}
