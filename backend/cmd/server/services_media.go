package main

import (
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/media"
	"github.com/u-ai/backend/internal/media/ffmpeg"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/workspace"
	"github.com/u-ai/backend/pkg/resourceuri"
)

func buildMediaService(host runtimehost.RuntimeHost, dataDir string, resolver *resourceuri.PhysicalResolver, workspaceService *workspace.Service) *media.Service {
	tempDir := filepath.Join(dataDir, "tmp", "media")
	_ = os.MkdirAll(tempDir, 0o755)

	config := ffmpeg.DefaultConfig()
	var backend media.Backend
	if host != nil {
		ffmpegBackend := ffmpeg.NewBackend(host, config)
		runner := ffmpeg.NewRunner(host, config)
		backend = media.NewFFmpegBackend(ffmpegBackend, runner, config)
	}

	materializer := media.NewResourceMaterializer(resolver, workspaceService, tempDir)
	svc := media.NewService(backend, tempDir, materializer)
	return svc
}
