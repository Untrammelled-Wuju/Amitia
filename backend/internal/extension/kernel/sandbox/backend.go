// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sandbox

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/u-ai/backend/pkg/resourceuri"
)

const (
	ProviderIDIOSSandbox = "ios.ish-sandbox"

	MaxStdinSize  = 1 * 1024 * 1024  // 1 MiB
	MaxStdoutSize = 8 * 1024 * 1024  // 8 MiB
	MaxStderrSize = 8 * 1024 * 1024  // 8 MiB
	MaxTotalOutput = 16 * 1024 * 1024 // 16 MiB

	NativeErrRuntimeNotStarted  = "ISH_RUNTIME_NOT_STARTED"
	NativeErrRuntimeStarted     = "ISH_RUNTIME_ALREADY_STARTED"
	NativeErrRootfsUnavailable  = "ISH_ROOTFS_NOT_AVAILABLE"
	NativeErrExecutionInvalid   = "ISH_EXECUTION_INVALID"
	NativeErrExecutionBusy      = "ISH_EXECUTION_BUSY"
	NativeErrExecutionCancelled = "ISH_EXECUTION_CANCELLED"
	NativeErrExecutionTimeout   = "ISH_EXECUTION_TIMEOUT"
	NativeErrOutputLimit        = "ISH_OUTPUT_LIMIT_EXCEEDED"
	NativeErrNativeFailure      = "ISH_NATIVE_FAILURE"
	NativeErrKernelFailure      = "ISH_KERNEL_FAILURE"
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

type NativeBridgeError struct {
	Code    string
	Message string
}

func (e *NativeBridgeError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type SandboxBackend interface {
	Availability(ctx context.Context) BackendAvailability
	Start(ctx context.Context, config SandboxConfig) error
	Stop(ctx context.Context) error
	Execute(ctx context.Context, req SandboxExecuteRequest) (SandboxExecuteResult, error)
	Cancel(ctx context.Context, executionID string) error
	Health(ctx context.Context) SandboxHealth
}

type SandboxConfig struct {
	RuntimeID    string
	WorkspaceURI string
	RootfsURI    string
	Environment  map[string]string
}

func (c SandboxConfig) ValidateHostConfig() error {
	if c.RuntimeID == "" {
		return fmt.Errorf("sandbox: runtime ID is required")
	}
	if c.WorkspaceURI != "" {
		if _, err := resourceuri.Parse(c.WorkspaceURI); err != nil {
			return fmt.Errorf("sandbox: invalid workspaceUri: %w", err)
		}
	}
	if c.RootfsURI != "" {
		if _, err := resourceuri.Parse(c.RootfsURI); err != nil {
			return fmt.Errorf("sandbox: invalid rootfsUri: %w", err)
		}
	}
	return nil
}

func (c SandboxConfig) Clone() SandboxConfig {
	clone := SandboxConfig{
		RuntimeID:    c.RuntimeID,
		WorkspaceURI: c.WorkspaceURI,
		RootfsURI:    c.RootfsURI,
	}
	if c.Environment != nil {
		clone.Environment = make(map[string]string, len(c.Environment))
		for k, v := range c.Environment {
			clone.Environment[k] = v
		}
	}
	return clone
}

type SandboxExecuteRequest struct {
	ExecutionID          string
	Argv                 []string
	WorkingDirectoryURI  string
	Environment          map[string]string
	Stdin                []byte
	TimeoutSeconds       uint32
}

func (r SandboxExecuteRequest) Validate() error {
	if r.ExecutionID == "" {
		return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: "execution ID is required"}
	}
	if len(r.Argv) == 0 || r.Argv[0] == "" {
		return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: "argv must contain at least one element"}
	}
	for i, arg := range r.Argv {
		if containsNul(arg) {
			return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: fmt.Sprintf("argv[%d] contains NUL byte", i)}
		}
	}
	if containsNul(r.ExecutionID) {
		return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: "execution ID contains NUL byte"}
	}
	if len(r.Stdin) > MaxStdinSize {
		return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: fmt.Sprintf("stdin exceeds %d bytes", MaxStdinSize)}
	}
	for k, v := range r.Environment {
		if !isValidEnvKey(k) {
			return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: fmt.Sprintf("invalid environment key: %s", k)}
		}
		if containsNul(k) || containsNul(v) {
			return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: fmt.Sprintf("environment %s contains NUL byte", k)}
		}
	}
	if containsNul(r.WorkingDirectoryURI) {
		return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: "working directory contains NUL byte"}
	}
	return nil
}

type SandboxExecuteResult struct {
	ExecutionID string
	ExitCode    int
	Stdout      []byte
	Stderr      []byte
	StartedAt   time.Time
	FinishedAt  time.Time
	Generation  uint64
	Fatal       bool
}

type SandboxHealth struct {
	Healthy              bool
	Message              string
	ISHInitialized       bool
	RootfsInstalled      bool
	LifecycleState       string
	Generation           uint64
	DesiredRunning       bool
	RestartRequired      bool
	RecoveryPending      bool
	ActiveExecutionID    string
	RunningRootfsVersion string
	RunningRootfsDigest  string
	ActiveRootfsVersion  string
	ActiveRootfsDigest   string
	LastErrorCode        string
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

func (b *ishBackend) Execute(ctx context.Context, req SandboxExecuteRequest) (SandboxExecuteResult, error) {
	if err := req.Validate(); err != nil {
		return SandboxExecuteResult{}, err
	}
	if err := validateBackendState(ctx, b); err != nil {
		return SandboxExecuteResult{}, err
	}
	return b.bridge.Execute(ctx, req)
}

func (b *ishBackend) Cancel(ctx context.Context, executionID string) error {
	if executionID == "" {
		return &NativeBridgeError{Code: NativeErrExecutionInvalid, Message: "execution ID is required for cancel"}
	}
	return b.bridge.Cancel(ctx, executionID)
}

func (b *ishBackend) Health(ctx context.Context) SandboxHealth {
	return b.bridge.Health(ctx)
}

func validateBackendState(ctx context.Context, b *ishBackend) error {
	state := b.bridge.Availability(ctx)
	if state != BackendRunning {
		return &NativeBridgeError{
			Code:    NativeErrRuntimeNotStarted,
			Message: fmt.Sprintf("sandbox backend not running: current state=%s", state.String()),
		}
	}
	return nil
}

func isValidEnvKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	for i, r := range key {
		if i == 0 {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
	}
	return true
}

func containsNul(s string) bool {
	for _, r := range s {
		if r == 0 {
			return true
		}
	}
	return false
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

func (b *unavailableBackend) Execute(_ context.Context, _ SandboxExecuteRequest) (SandboxExecuteResult, error) {
	return SandboxExecuteResult{}, &NativeBridgeError{
		Code:    NativeErrRuntimeNotStarted,
		Message: fmt.Sprintf("iSH backend unavailable: %s", b.reason),
	}
}

func (b *unavailableBackend) Cancel(_ context.Context, _ string) error {
	return &NativeBridgeError{
		Code:    NativeErrRuntimeNotStarted,
		Message: fmt.Sprintf("iSH backend unavailable: %s", b.reason),
	}
}

func (b *unavailableBackend) Health(_ context.Context) SandboxHealth {
	return SandboxHealth{
		Healthy: false,
		Message: fmt.Sprintf("unavailable: %s", b.reason),
	}
}

func minPlatformBackend() (SandboxBackend, error) {
	if runtime.GOOS == "ios" {
		return newISHBackend()
	}
	return &unavailableBackend{reason: fmt.Sprintf("iSH backend requires iOS platform, current=%s", runtime.GOOS)}, nil
}
