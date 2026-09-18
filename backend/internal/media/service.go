package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func (s *Service) Materializer() ResourceMaterializer {
	return s.materializer
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

type FFmpegExecuteRequest struct {
	SourceURI string   `json:"sourceUri"`
	TargetURI string   `json:"targetUri"`
	Args      []string `json:"args"`
}

type FFmpegExecuteResult struct {
	ResourceURI string `json:"resourceUri"`
	ExitCode    int    `json:"exitCode"`
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	DurationMS  int64  `json:"durationMs"`
}

var ffmpegSchemePattern = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*:`)

// ExecuteFFmpeg exposes advanced FFmpeg filters/codecs while keeping filesystem
// access inside the ResourceURI boundary. Callers supply arguments that are
// inserted between one materialized input and one staging output; raw paths,
// URL protocols, response files and additional -i inputs are rejected.
func (s *Service) ExecuteFFmpeg(ctx context.Context, request FFmpegExecuteRequest) (*FFmpegExecuteResult, error) {
	if s.backend == nil || s.materializer == nil {
		return nil, fmt.Errorf("media service unavailable")
	}
	raw, ok := s.backend.(RawFFmpegBackend)
	if !ok {
		return nil, fmt.Errorf("media backend does not support raw ffmpeg execution")
	}
	request.SourceURI = strings.TrimSpace(request.SourceURI)
	request.TargetURI = strings.TrimSpace(request.TargetURI)
	if request.SourceURI == "" || request.TargetURI == "" {
		return nil, fmt.Errorf("sourceUri and targetUri are required")
	}
	if len(request.Args) > 128 {
		return nil, fmt.Errorf("ffmpeg args exceed 128 entries")
	}
	for _, arg := range request.Args {
		if err := validateFFmpegUserArg(arg); err != nil {
			return nil, err
		}
	}

	inputPath, cleanupInput, err := s.materializer.Materialize(ctx, request.SourceURI)
	if err != nil {
		return nil, err
	}
	defer cleanupInput()

	ext := filepath.Ext(request.TargetURI)
	if len(ext) > 16 || strings.ContainsAny(ext, `/\\`) {
		ext = ""
	}
	stagingPath, err := s.resourceIO.CreateStagingPath(ext)
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.resourceIO.CleanupStaging(stagingPath) }()

	args := []string{"-y", "-hide_banner", "-nostdin", "-loglevel", "error", "-i", inputPath}
	args = append(args, request.Args...)
	args = append(args, stagingPath)
	processResult, err := raw.ExecuteRaw(ctx, args)
	if err != nil {
		return nil, err
	}
	if processResult.ExitCode != 0 {
		return nil, fmt.Errorf("ffmpeg exited with code %d: %s", processResult.ExitCode, strings.TrimSpace(string(processResult.Stderr)))
	}
	info, err := os.Stat(stagingPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg output missing: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return nil, fmt.Errorf("ffmpeg produced no output")
	}
	if err := s.materializer.Publish(ctx, stagingPath, request.TargetURI); err != nil {
		return nil, err
	}
	return &FFmpegExecuteResult{
		ResourceURI: request.TargetURI,
		ExitCode:    processResult.ExitCode,
		Stdout:      string(processResult.Stdout),
		Stderr:      string(processResult.Stderr),
		DurationMS:  processResult.Duration.Milliseconds(),
	}, nil
}

func validateFFmpegUserArg(arg string) error {
	if arg == "" {
		return nil
	}
	if len(arg) > 8192 || strings.ContainsRune(arg, '\x00') {
		return fmt.Errorf("ffmpeg argument is invalid or too long")
	}
	lower := strings.ToLower(strings.TrimSpace(arg))
	if lower == "-i" || strings.HasPrefix(lower, "-i=") || lower == "-f concat" || lower == "-filter_script" || lower == "-filter_complex_script" {
		return fmt.Errorf("additional ffmpeg inputs/scripts are not allowed")
	}
	if strings.HasPrefix(lower, "@") {
		return fmt.Errorf("ffmpeg response files are not allowed")
	}
	if ffmpegSchemePattern.MatchString(lower) {
		return fmt.Errorf("ffmpeg protocol arguments are not allowed")
	}
	if filepath.IsAbs(arg) || strings.HasPrefix(arg, "../") || strings.Contains(arg, "/../") || strings.Contains(arg, `\\..\\`) {
		return fmt.Errorf("raw filesystem paths are not allowed in ffmpeg args")
	}
	// Filter arguments can themselves open files or network resources (for
	// example movie=/path, subtitles=/path, fontfile=/path). Keep raw mode
	// strictly single-input/single-output and ResourceURI bounded.
	for _, fragment := range []string{
		"file:", "http:", "https:", "ftp:", "tcp:", "udp:", "rtmp:", "rtmps:", "srt:", "concat:", "crypto:", "data:", "subfile:",
		"movie=", "amovie=", "subtitles=", "ass=", "fontfile=", "textfile=", "lut3d=", "lut1d=",
	} {
		if strings.Contains(lower, fragment) {
			return fmt.Errorf("ffmpeg argument may access an external resource: %s", fragment)
		}
	}
	if strings.Contains(lower, "=/") || strings.Contains(lower, `=\\`) || strings.Contains(lower, "=../") || strings.Contains(lower, `=..\\`) {
		return fmt.Errorf("filesystem paths inside ffmpeg filter arguments are not allowed")
	}
	// Options whose value would make FFmpeg read/write a second file are blocked
	// even when the following argument looks relative.
	for _, prefix := range []string{"-attach", "-dump_attachment", "-filter_script", "-filter_complex_script", "-passlogfile", "-vstats_file", "-progress"} {
		if lower == prefix || strings.HasPrefix(lower, prefix+"=") {
			return fmt.Errorf("ffmpeg option %s is not allowed in raw mode", prefix)
		}
	}
	return nil
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
