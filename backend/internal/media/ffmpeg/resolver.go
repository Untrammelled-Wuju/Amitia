package ffmpeg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/u-ai/backend/internal/runtimehost"
)

type Resolver interface {
	Resolve(ctx context.Context, host runtimehost.RuntimeHost) (*Environment, error)
}

type BinaryResolver struct {
	config Config
}

func NewBinaryResolver(config Config) *BinaryResolver {
	return &BinaryResolver{config: config}
}

func (r *BinaryResolver) Resolve(ctx context.Context, host runtimehost.RuntimeHost) (*Environment, error) {
	env := &Environment{
		RuntimeID: host.RuntimeInstanceID(),
	}

	runtimeID := host.RuntimeInstanceID()

	if !host.Capabilities().Supports(runtimehost.CapProcessSpawn) {
		return UnavailableEnvironment(runtimeID, "process spawn not supported"), nil
	}
	if !host.Capabilities().Supports(runtimehost.CapFilesystemExecutable) {
		return UnavailableEnvironment(runtimeID, "executable filesystem not supported"), nil
	}

	ffmpegPath := r.config.FFmpegPath
	ffprobePath := r.config.FFprobePath

	if ffmpegPath == "" || ffprobePath == "" {
		paths := r.discoverFromRuntimePackage(host)
		if paths != nil {
			ffmpegPath = paths.ffmpeg
			ffprobePath = paths.ffprobe
			env.Source = BinarySourceRuntimePackage
		}
	}

	if ffmpegPath == "" {
		ffmpegPath = r.config.FFmpegPath
		env.Source = BinarySourceConfigured
	}
	if ffprobePath == "" {
		ffprobePath = r.config.FFprobePath
		env.Source = BinarySourceConfigured
	}

	if ffmpegPath == "" || ffprobePath == "" {
		return UnavailableEnvironment(runtimeID, "ffmpeg/ffprobe binary not found"), nil
	}

	if !filepath.IsAbs(ffmpegPath) {
		return UnavailableEnvironment(runtimeID, "ffmpeg path must be absolute"), nil
	}
	if !filepath.IsAbs(ffprobePath) {
		return UnavailableEnvironment(runtimeID, "ffprobe path must be absolute"), nil
	}

	if err := r.validateBinary(ffmpegPath); err != nil {
		env.Diagnostics = append(env.Diagnostics, fmt.Sprintf("ffmpeg validation: %v", err))
		return UnavailableEnvironment(runtimeID, err.Error()), nil
	}
	if err := r.validateBinary(ffprobePath); err != nil {
		env.Diagnostics = append(env.Diagnostics, fmt.Sprintf("ffprobe validation: %v", err))
		return UnavailableEnvironment(runtimeID, err.Error()), nil
	}

	env.FFmpegPath = ffmpegPath
	env.FFprobePath = ffprobePath
	env.Architecture = detectArchitecture()
	env.Platform = runtime.GOOS
	env.Available = true

	return env, nil
}

type binaryPaths struct {
	ffmpeg  string
	ffprobe string
}

func (r *BinaryResolver) discoverFromRuntimePackage(host runtimehost.RuntimeHost) *binaryPaths {
	paths := host.Paths()
	if paths.Root == "" {
		return nil
	}

	arch := strings.ToLower(string(detectArchitecture()))
	platform := runtime.GOOS

	candidates := []struct {
		ffmpeg  string
		ffprobe string
	}{
		{
			filepath.Join(paths.Root, "ffmpeg", platform, arch, "bin", "ffmpeg"),
			filepath.Join(paths.Root, "ffmpeg", platform, arch, "bin", "ffprobe"),
		},
		{
			filepath.Join(paths.Root, "packages", "ffmpeg", platform, arch, "bin", "ffmpeg"),
			filepath.Join(paths.Root, "packages", "ffmpeg", platform, arch, "bin", "ffprobe"),
		},
	}

	for _, c := range candidates {
		if fileExists(c.ffmpeg) && fileExists(c.ffprobe) {
			return &binaryPaths{ffmpeg: c.ffmpeg, ffprobe: c.ffprobe}
		}
	}

	return nil
}

func (r *BinaryResolver) validateBinary(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewError(FFMPEG_BINARY_NOT_FOUND, "binary not found: "+path)
		}
		return NewError(FFMPEG_BINARY_INVALID, fmt.Sprintf("cannot stat binary: %v", err))
	}

	if info.IsDir() {
		return NewError(FFMPEG_BINARY_INVALID, "binary is a directory: "+path)
	}

	if !isExecutable(path) {
		return NewError(FFMPEG_BINARY_INVALID, "binary is not executable: "+path)
	}

	return nil
}

func (r *BinaryResolver) VerifyIntegrity(path string, expectedHash string) error {
	if expectedHash == "" {
		return nil
	}
	hash, err := computeSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(hash, expectedHash) {
		return NewError(FFMPEG_BINARY_INTEGRITY_FAILED,
			fmt.Sprintf("hash mismatch for %s: expected %s, got %s", path, expectedHash, hash))
	}
	return nil
}

func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.HasSuffix(strings.ToLower(path), ".exe")
	}
	return info.Mode()&0o111 != 0
}

func detectArchitecture() Architecture {
	switch runtime.GOARCH {
	case "arm64":
		return ArchARM64
	case "arm":
		return ArchARM
	case "amd64":
		return ArchX86_64
	default:
		return ArchUnknown
	}
}
