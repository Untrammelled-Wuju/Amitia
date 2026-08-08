package sandbox

import (
	"context"
	"fmt"
	"runtime"
)

const (
	ProviderIDIOSSandbox = "ios.ish-sandbox"
)

type BackendAvailability int

const (
	BackendUnavailable BackendAvailability = iota
	BackendAvailable
	BackendStarting
	BackendRunning
	BackendError
)

func (a BackendAvailability) String() string {
	switch a {
	case BackendUnavailable:
		return "unavailable"
	case BackendAvailable:
		return "available"
	case BackendStarting:
		return "starting"
	case BackendRunning:
		return "running"
	case BackendError:
		return "error"
	default:
		return "unknown"
	}
}

type SandboxBackend interface {
	Availability(ctx context.Context) BackendAvailability
	Start(ctx context.Context, config SandboxConfig) error
	Stop(ctx context.Context) error
	Execute(ctx context.Context, cmd SandboxCommand) (SandboxResult, error)
	Health(ctx context.Context) SandboxHealth
}

type SandboxConfig struct {
	RuntimeID    string
	WorkspaceURI string
	RootfsURI    string
	Environment  map[string]string
}

type SandboxCommand struct {
	Command []string
	Stdin   string
	Timeout int
	WorkDir string
}

type SandboxResult struct {
	Stdout   string
	Stderr   string
	ExitCode int64
	Error    string
}

type SandboxHealth struct {
	Healthy         bool
	Message         string
	ISHInitialized  bool
	RootfsInstalled bool
}

type ishBackend struct {
	bridge NativeBridge
}

func newISHBackend() (SandboxBackend, error) {
	return &ishBackend{
		bridge: NewNativeBridge(),
	}, nil
}

func NewIOSSandboxBackend() (SandboxBackend, error) {
	return newISHBackend()
}

func (b *ishBackend) Availability(ctx context.Context) BackendAvailability {
	return b.bridge.Availability(ctx)
}

func (b *ishBackend) Start(ctx context.Context, cfg SandboxConfig) error {
	return b.bridge.Start(ctx, cfg)
}

func (b *ishBackend) Stop(ctx context.Context) error {
	return b.bridge.Stop(ctx)
}

func (b *ishBackend) Execute(ctx context.Context, cmd SandboxCommand) (SandboxResult, error) {
	return b.bridge.Execute(ctx, cmd)
}

func (b *ishBackend) Health(ctx context.Context) SandboxHealth {
	return b.bridge.Health(ctx)
}

type unavailableBackend struct {
	reason string
}

func (b *unavailableBackend) Availability(_ context.Context) BackendAvailability {
	return BackendUnavailable
}

func (b *unavailableBackend) Start(_ context.Context, _ SandboxConfig) error {
	return fmt.Errorf("iSH backend unavailable: %s", b.reason)
}

func (b *unavailableBackend) Stop(_ context.Context) error {
	return nil
}

func (b *unavailableBackend) Execute(_ context.Context, cmd SandboxCommand) (SandboxResult, error) {
	return SandboxResult{
		Error: fmt.Sprintf("iSH backend unavailable: %s; command not executed: %v", b.reason, cmd.Command),
	}, fmt.Errorf("iSH backend unavailable: %s", b.reason)
}

func (b *unavailableBackend) Health(_ context.Context) SandboxHealth {
	return SandboxHealth{
		Healthy: false,
		Message: fmt.Sprintf("unavailable: %s", b.reason),
	}
}

func minPlatformBackend() (SandboxBackend, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "ios" {
		return &unavailableBackend{reason: "iSH backend requires iOS/darwin platform"}, nil
	}
	return newISHBackend()
}
