//go:build ios
// +build ios

package builtin

import (
	"fmt"

	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
)

const (
	IOSNativeErrUnsupportedHost = "IOS_NATIVE_UNSUPPORTED_HOST"
	IOSNativeErrHostRequired    = "IOS_NATIVE_HOST_REQUIRED"
	IOSNativeErrBridgeRequired  = "IOS_NATIVE_BRIDGE_REQUIRED"
)

type IOSNativeError struct {
	Code  string
	Cause error
}

func (e *IOSNativeError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Cause.Error()
	}
	return e.Code
}

func (e *IOSNativeError) Unwrap() error {
	return e.Cause
}

type IOSNativeProviderConfig struct {
	Bridge nativebridge.Bridge
}

type IOSNativeProviderFactory struct {
	config IOSNativeProviderConfig
}

func NewIOSNativeProviderFactory(config IOSNativeProviderConfig) *IOSNativeProviderFactory {
	return &IOSNativeProviderFactory{
		config: config,
	}
}

func (f *IOSNativeProviderFactory) ProviderID() string {
	return "ios-native"
}

func (f *IOSNativeProviderFactory) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotIOSNative
}

func (f *IOSNativeProviderFactory) Requirements() []runtimehost.CapabilityRequirement {
	return []runtimehost.CapabilityRequirement{
		{
			ID:      runtimehost.CapRuntimeIOSNative,
			Minimum: runtimehost.SupportSupported,
		},
	}
}

func (f *IOSNativeProviderFactory) Build(
	bc runtimeorchestrator.ProviderBuildContext,
) (runtimeorchestrator.ProviderInstance, error) {
	if bc.Host == nil {
		return nil, &IOSNativeError{
			Code:  IOSNativeErrHostRequired,
			Cause: fmt.Errorf("runtime host is required"),
		}
	}

	descriptor := bc.Host.Descriptor()
	if descriptor.Host != platform.HostPlatformIOS {
		return nil, &IOSNativeError{
			Code:  IOSNativeErrUnsupportedHost,
			Cause: fmt.Errorf("unsupported host platform: %s", descriptor.Host),
		}
	}

	return newIOSNativeProviderInstance(f.config.Bridge, bc.Host), nil
}

var _ runtimeorchestrator.ProviderFactory = (*IOSNativeProviderFactory)(nil)
