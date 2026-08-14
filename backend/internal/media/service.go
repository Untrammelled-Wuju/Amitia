package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/u-ai/backend/internal/media/conversion"
	"github.com/u-ai/backend/internal/media/metadata"
)

type Service struct {
	backend      Backend
	resourceIO   *ResourceIO
	materializer ResourceMaterializer
	tempDir      string

	mu     sync.Mutex
	active int
}

func NewService(backend Backend, tempDir string, materializer ResourceMaterializer) *Service {
	return &Service{
		backend:      backend,
		resourceIO:   NewResourceIO(tempDir),
		materializer: materializer,
		tempDir:      tempDir,
	}
}

func (s *Service) GetMetadata(ctx context.Context, resourceURI string, req metadata.MetadataRequest) (*metadata.MediaMetadata, error) {
	if s.backend == nil || s.materializer == nil {
		return nil, fmt.Errorf("media service unavailable")
	}
	materialized, cleanup, err := s.materializer.Materialize(ctx, resourceURI)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	probeResult, err := s.backend.GetMetadata(ctx, materialized, req)
	if err != nil {
		return nil, err
	}

	probeResult.ResourceURI = resourceURI

	return probeResult, nil
}

func (s *Service) Convert(ctx context.Context, request conversion.ConvertRequest, opts conversion.ConvertOptions) (*conversion.ConversionResult, error) {
	if s.backend == nil || s.materializer == nil {
		return nil, fmt.Errorf("media service unavailable")
	}
	s.mu.Lock()
	s.active++
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
	}()

	materialized, cleanup, err := s.materializer.Materialize(ctx, request.SourceURI)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	metadataReq := metadata.MetadataRequest{
		SourceURI:        request.SourceURI,
		IncludeStreams:   true,
		IncludeChapters:  true,
		IncludeTags:      true,
		IncludeTechnical: true,
	}
	sourceMeta, err := s.backend.GetMetadata(ctx, materialized, metadataReq)
	if err != nil {
		return nil, err
	}

	planner := conversion.NewPlanner(nil)
	plan, err := planner.Plan(sourceMeta, request)
	if err != nil {
		return nil, err
	}

	stagingExt := request.Output.Extension
	if stagingExt == "" {
		stagingExt = request.Output.Container
	}
	if stagingExt != "" && !startsWithDot(stagingExt) {
		stagingExt = "." + stagingExt
	}

	stagingPath, err := s.resourceIO.CreateStagingPath(stagingExt)
	if err != nil {
		return nil, err
	}
	if request.TargetURI == "" {
		request.TargetURI, err = s.materializer.URIForLocalPath(stagingPath)
		if err != nil {
			_ = s.resourceIO.CleanupStaging(stagingPath)
			return nil, err
		}
	}

	result, err := s.backend.Convert(ctx, request, plan, materialized, stagingPath, opts)
	if err != nil {
		_ = s.resourceIO.CleanupStaging(stagingPath)
		return nil, err
	}
	if err := s.materializer.Publish(ctx, stagingPath, request.TargetURI); err != nil {
		_ = s.resourceIO.CleanupStaging(stagingPath)
		return nil, err
	}

	result.ResourceURI = request.TargetURI
	result.MetadataMode = request.Metadata.Mode
	if result.MetadataMode == "" {
		result.MetadataMode = conversion.MetadataModeSafe
	}

	return result, nil
}

func (s *Service) CancelAll() {
	if s.backend != nil {
		s.backend.CancelAll()
	}
}

func (s *Service) ActiveOperations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *Service) Capabilities(ctx context.Context) (*MediaBackendCapabilities, error) {
	if s.backend == nil {
		return &MediaBackendCapabilities{Available: false}, nil
	}
	caps, err := s.backend.Capabilities(ctx)
	if err != nil {
		return nil, err
	}

	result := &MediaBackendCapabilities{
		Available:            caps.Available,
		SupportsScale:        caps.SupportsScale,
		SupportsFPS:          caps.SupportsFPS,
		SupportsLoudnorm:     caps.SupportsLoudnorm,
		SupportsGIF:          caps.SupportsGIF,
		HardwareAcceleration: caps.HardwareAcceleration,
		Fingerprint:          caps.Fingerprint,
	}

	if len(caps.VideoEncodeCodecs) > 0 {
		result.VideoCodecs = caps.VideoEncodeCodecs
	}
	if len(caps.AudioEncodeCodecs) > 0 {
		result.AudioCodecs = caps.AudioEncodeCodecs
	}
	if len(caps.Containers) > 0 {
		result.Containers = caps.Containers
	}

	return result, nil
}

func (s *Service) GenerateTempPath(extension string) string {
	if extension == "" {
		extension = ".tmp"
	}
	if !startsWithDot(extension) {
		extension = "." + extension
	}
	return filepath.Join(s.tempDir, "media_"+randomID()+extension)
}

func startsWithDot(s string) bool {
	return len(s) > 0 && s[0] == '.'
}

func randomID() string {
	return fmt.Sprintf("%d", os.Getpid())
}
