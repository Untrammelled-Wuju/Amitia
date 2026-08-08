package sandbox

import (
	"context"
	"fmt"
	"runtime"
)

const (
	SlotIOSSandbox       = "ios-sandbox"
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
	state BackendAvailability
}

func newISHBackend() (SandboxBackend, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "ios" {
		return &unavailableBackend{reason: "iSH backend requires iOS/darwin platform"}, nil
	}
	return &ishBackend{state: BackendUnavailable}, nil
}

func NewIOSSandboxBackend() (SandboxBackend, error) {
	return newISHBackend()
}

func (b *ishBackend) Availability(_ context.Context) BackendAvailability {
	return b.state
}

func (b *ishBackend) Start(_ context.Context, _ SandboxConfig) error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "ios" {
		return fmt.Errorf("iSH backend not available on %s", runtime.GOOS)
	}
	b.state = BackendStarting
	return nil
}

func (b *ishBackend) Stop(_ context.Context) error {
	b.state = BackendUnavailable
	return nil
}

func (b *ishBackend) Execute(_ context.Context, cmd SandboxCommand) (SandboxResult, error) {
	return SandboxResult{
		Error: fmt.Sprintf("iSH native bridge not yet connected; command not executed: %v", cmd.Command),
	}, fmt.Errorf("iSH native bridge not connected")
}

func (b *ishBackend) Health(_ context.Context) SandboxHealth {
	return SandboxHealth{
		Healthy:         b.state == BackendRunning,
		Message:         b.state.String(),
		ISHInitialized:  b.state == BackendRunning,
		RootfsInstalled: false,
	}
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
