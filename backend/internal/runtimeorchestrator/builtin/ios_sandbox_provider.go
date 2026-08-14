//go:build ios
// +build ios

package builtin

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/resourceuri"
)

const (
	SandboxErrDisabled              = "IOS_SANDBOX_DISABLED"
	SandboxErrUnsupportedHost       = "IOS_SANDBOX_UNSUPPORTED_HOST"
	SandboxErrCapabilityUnavailable = "IOS_SANDBOX_CAPABILITY_UNAVAILABLE"
	SandboxErrBackendUnavailable    = "IOS_SANDBOX_BACKEND_UNAVAILABLE"
	SandboxErrRootfsNotConfigured   = "IOS_SANDBOX_ROOTFS_NOT_CONFIGURED"
	SandboxErrNotReady              = "IOS_SANDBOX_NOT_READY"
	SandboxErrInvalidWorkspaceURI   = "IOS_SANDBOX_INVALID_WORKSPACE_URI"
	SandboxErrInvalidRootfsURI      = "IOS_SANDBOX_INVALID_ROOTFS_URI"
	SandboxErrSecretInEnvironment   = "IOS_SANDBOX_SECRET_IN_ENVIRONMENT"
	SandboxErrInvalidEnvKey         = "IOS_SANDBOX_INVALID_ENV_KEY"
	SandboxErrHostRequired          = "IOS_SANDBOX_HOST_REQUIRED"
	SandboxErrConfigInvalid         = "IOS_SANDBOX_CONFIG_INVALID"
)

type SandboxError struct {
	Code  string
	Cause error
}

func (e *SandboxError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Cause.Error()
	}
	return e.Code
}

func (e *SandboxError) Unwrap() error {
	return e.Cause
}

var secretKeyPatterns = []string{
	"TOKEN", "SECRET", "PASSWORD", "API_KEY", "APIKEY",
	"AUTHORIZATION", "COOKIE", "PRIVATE_KEY", "PRIVATEKEY",
	"CREDENTIAL", "AUTH_TOKEN", "ACCESS_TOKEN", "REFRESH_TOKEN",
	"ACCESS_KEY", "SIGNATURE", "SESSION",
}

func isSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, pattern := range secretKeyPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

type IOSSandboxProviderConfig struct {
	Enabled      bool
	WorkspaceURI string
	RootfsURI    string
	Environment  map[string]string
}

func (c IOSSandboxProviderConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.WorkspaceURI != "" {
		if _, err := resourceuri.Parse(c.WorkspaceURI); err != nil {
			return &SandboxError{
				Code:  SandboxErrInvalidWorkspaceURI,
				Cause: err,
			}
		}
	}

	if c.RootfsURI != "" {
		if _, err := resourceuri.Parse(c.RootfsURI); err != nil {
			return &SandboxError{
				Code:  SandboxErrInvalidRootfsURI,
				Cause: err,
			}
		}
	}

	for key := range c.Environment {
		if key == "" {
			return &SandboxError{
				Code:  SandboxErrInvalidEnvKey,
				Cause: fmt.Errorf("environment key must not be blank"),
			}
		}
		if isSecretKey(key) {
			return &SandboxError{
				Code:  SandboxErrSecretInEnvironment,
				Cause: fmt.Errorf("environment key %q contains raw secret material", key),
			}
		}
	}

	return nil
}

func (c IOSSandboxProviderConfig) freeze() IOSSandboxProviderConfig {
	return IOSSandboxProviderConfig{
		Enabled:      c.Enabled,
		WorkspaceURI: c.WorkspaceURI,
		RootfsURI:    c.RootfsURI,
		Environment:  cloneStringMap(c.Environment),
	}
}

type IOSSandboxProviderFactory struct {
	config IOSSandboxProviderConfig

	newBackend func() (sandbox.SandboxBackend, error)
}

func NewIOSSandboxProviderFactory(
	config IOSSandboxProviderConfig,
) *IOSSandboxProviderFactory {
	return &IOSSandboxProviderFactory{
		config:     config.freeze(),
		newBackend: sandbox.NewIOSSandboxBackend,
	}
}

func (f *IOSSandboxProviderFactory) ProviderID() string {
	return sandbox.ProviderIDIOSSandbox
}

func (f *IOSSandboxProviderFactory) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotIOSSandbox
}

func (f *IOSSandboxProviderFactory) Requirements() []runtimehost.CapabilityRequirement {
	return []runtimehost.CapabilityRequirement{
		{
			ID:      runtimehost.CapRuntimeSandboxedExec,
			Minimum: runtimehost.SupportSupported,
		},
		{
			ID:      runtimehost.CapRuntimeNativeOffload,
			Minimum: runtimehost.SupportLimited,
		},
	}
}

func (f *IOSSandboxProviderFactory) Build(
	bc runtimeorchestrator.ProviderBuildContext,
) (runtimeorchestrator.ProviderInstance, error) {
	if bc.Host == nil {
		return nil, &SandboxError{
			Code:  SandboxErrHostRequired,
			Cause: fmt.Errorf("runtime host is required"),
		}
	}

	descriptor := bc.Host.Descriptor()

	if descriptor.Host != platform.HostPlatformIOS {
		return nil, &SandboxError{
			Code:  SandboxErrUnsupportedHost,
			Cause: fmt.Errorf("unsupported host platform: %s", descriptor.Host),
		}
	}

	if !f.config.Enabled {
		backend, err := f.newBackend()
		if err != nil {
			return nil, &SandboxError{
				Code:  SandboxErrBackendUnavailable,
				Cause: err,
			}
		}
		if backend == nil {
			return nil, &SandboxError{
				Code:  SandboxErrBackendUnavailable,
				Cause: fmt.Errorf("backend factory returned nil"),
			}
		}
		return newIOSSandboxProviderInstance(backend, bc.Host, f.config), nil
	}

	if err := f.config.Validate(); err != nil {
		return nil, &SandboxError{
			Code:  SandboxErrConfigInvalid,
			Cause: err,
		}
	}

	if f.config.RootfsURI == "" {
		return nil, &SandboxError{
			Code:  SandboxErrRootfsNotConfigured,
			Cause: fmt.Errorf("rootfsUri is required when provider is enabled"),
		}
	}

	if f.newBackend == nil {
		return nil, &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend factory is not configured"),
		}
	}

	backend, err := f.newBackend()
	if err != nil {
		return nil, &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: err,
		}
	}

	if backend == nil {
		return nil, &SandboxError{
			Code:  SandboxErrBackendUnavailable,
			Cause: fmt.Errorf("backend factory returned nil"),
		}
	}

	return newIOSSandboxProviderInstance(backend, bc.Host, f.config), nil
}

var _ runtimeorchestrator.ProviderFactory = (*IOSSandboxProviderFactory)(nil)
