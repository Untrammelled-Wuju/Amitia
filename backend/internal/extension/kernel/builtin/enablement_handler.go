package builtin

import (
	"context"
	"fmt"
)

// EnablementConstraints describes the hardware and capability prerequisites
// that must be satisfied before a Built-in Extension can be enabled on a
// given device or environment.
type EnablementConstraints struct {
	RequiredCapabilities []string
	HardwareRequirements []string
}

// CompatibleEnablementHandler defines the interface for checking whether a
// Built-in Extension definition is compatible with the current environment
// before it is enabled.
type CompatibleEnablementHandler interface {
	CheckCompatibility(ctx context.Context, def Definition) error
}

// defaultEnablementHandler is the default CompatibleEnablementHandler
// implementation. It skips compatibility checks for non-required extensions
// and performs basic validation for required ones.
type defaultEnablementHandler struct{}

// CheckCompatibility checks whether the given Definition is compatible with
// the current environment. Non-required extensions are always considered
// compatible. Required extensions may be rejected if environment constraints
// are not satisfied.
func (h *defaultEnablementHandler) CheckCompatibility(ctx context.Context, def Definition) error {
	if !def.Required {
		return nil
	}
	if ctx == nil {
		return fmt.Errorf("enablement: nil context")
	}
	return nil
}

// NewDefaultEnablementHandler returns a CompatibleEnablementHandler that
// performs basic compatibility checking for Built-in Extensions.
func NewDefaultEnablementHandler() CompatibleEnablementHandler {
	return &defaultEnablementHandler{}
}
