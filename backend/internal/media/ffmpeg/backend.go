package ffmpeg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sync"

	"github.com/u-ai/backend/internal/runtimehost"
)

type Backend interface {
	Capabilities(ctx context.Context) CapabilityState

	GetCapabilities(ctx context.Context) (*Capabilities, error)

	CheckVersion(ctx context.Context) (*Environment, error)

	Probe(ctx context.Context, localPath string) (*ProbeResult, error)

	ProbeFull(ctx context.Context, localPath string) (*FullProbeResult, error)

	CancelAll()
}

type sharedBackend struct {
	host     runtimehost.RuntimeHost
	resolver Resolver
	runner   *Runner
	config   Config

	mu       sync.RWMutex
	env      *Environment
}

func NewBackend(host runtimehost.RuntimeHost, config Config) Backend {
	resolver := NewBinaryResolver(config)
	runner := NewRunner(host, config)
	return &sharedBackend{
		host:     host,
		resolver: resolver,
		runner:   runner,
		config:   config,
	}
}

func (b *sharedBackend) Capabilities(ctx context.Context) CapabilityState {
	env, err := b.getEnvironment(ctx)
	if err != nil || env == nil {
		return DisabledCapabilityState(b.host.RuntimeInstanceID(), "environment detection failed")
	}

	if !env.Available {
		reason := "ffmpeg not available"
		if len(env.Diagnostics) > 0 {
			reason = env.Diagnostics[0]
		}
		return DisabledCapabilityState(b.host.RuntimeInstanceID(), reason)
	}

	return CapabilityState{
		Supported:               true,
		FFmpegAvailable:         env.HasFFmpeg(),
		FFprobeAvailable:        env.HasFFprobe(),
		FFmpegVersion:           env.Version,
		FFprobeVersion:          env.ProbeVersion,
		RuntimeID:               env.RuntimeID,
		Platform:                string(env.Platform),
		Architecture:            string(env.Architecture),
		Source:                  string(env.Source),
		NetworkProtocolsAllowed: b.config.AllowNetworkProtocols,
		State:                   "available",
	}
}

func (b *sharedBackend) GetCapabilities(ctx context.Context) (*Capabilities, error) {
	introspector := NewCapabilityIntrospector(b)
	return introspector.Introspect(ctx)
}

func (b *sharedBackend) CheckVersion(ctx context.Context) (*Environment, error) {
	env, err := b.getEnvironment(ctx)
	if err != nil {
		return nil, err
	}

	if !env.Available {
		return env, nil
	}

	args := BuildVersionArgs()
	result, err := b.runner.RunProcess(ctx, env.FFmpegPath, args)
	if err == nil && result.ExitCode == 0 {
		env.Version = ParseVersionOutput(result.Stdout)
	}

	if env.FFprobePath != "" {
		args := BuildVersionArgs()
		result, err := b.runner.RunProcess(ctx, env.FFprobePath, args)
		if err == nil && result.ExitCode == 0 {
			env.ProbeVersion = ParseVersionOutput(result.Stdout)
		}
	}

	return env, nil
}

func (b *sharedBackend) Probe(ctx context.Context, localPath string) (*ProbeResult, error) {
	env, err := b.getEnvironment(ctx)
	if err != nil {
		return nil, err
	}
	if !env.Available || env.FFprobePath == "" {
		return nil, NewError(FFMPEG_UNAVAILABLE, "ffprobe not available")
	}
	return Probe(ctx, b.runner, env.FFprobePath, localPath, b.config)
}

func (b *sharedBackend) ProbeFull(ctx context.Context, localPath string) (*FullProbeResult, error) {
	env, err := b.getEnvironment(ctx)
	if err != nil {
		return nil, err
	}
	if !env.Available || env.FFprobePath == "" {
		return nil, NewError(FFMPEG_UNAVAILABLE, "ffprobe not available")
	}
	return ProbeFull(ctx, b.runner, env.FFprobePath, localPath, b.config)
}

func (b *sharedBackend) CancelAll() {
	b.runner.CancelAll()
}

func (b *sharedBackend) getEnvironment(ctx context.Context) (*Environment, error) {
	b.mu.RLock()
	if b.env != nil {
		b.mu.RUnlock()
		return b.env, nil
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.env != nil {
		return b.env, nil
	}

	env, err := b.resolver.Resolve(ctx, b.host)
	if err != nil {
		return nil, err
	}

	b.env = env
	return env, nil
}

func (b *sharedBackend) invalidateCache() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.env = nil
}

func computeFileHash(path string) (string, error) {
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
