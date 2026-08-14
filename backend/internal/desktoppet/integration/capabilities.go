package integration

import (
	"context"
	"fmt"
)

type PetResourceHandle string

func (h PetResourceHandle) String() string { return string(h) }

type PluginResourceAttachRequest struct {
	ExtensionID    string
	PluginID       string
	ContributionID string
	Revision       int
	Definition     map[string]any
}

type PetResourceCapability interface {
	AttachPluginResource(ctx context.Context, req PluginResourceAttachRequest) (PetResourceHandle, error)
	DetachPluginResource(ctx context.Context, handle PetResourceHandle) error
}

type PetActionHandle string

func (h PetActionHandle) String() string { return string(h) }

type ExistingPetActionTarget struct {
	InstallationID string
	DeviceID       string
	UserID         string
}

type ActionTargetResolver interface {
	ResolveActionTarget(ctx context.Context, extensionID string, contributionID string, revision int) (ExistingPetActionTarget, error)
}

type PluginActionAttachRequest struct {
	ExtensionID    string
	PluginID       string
	ContributionID string
	Revision       int
	Target         ExistingPetActionTarget
	Definition     map[string]any
}

type PetActionCapability interface {
	AttachPluginAction(ctx context.Context, req PluginActionAttachRequest) (PetActionHandle, error)
	DetachPluginAction(ctx context.Context, handle PetActionHandle) error
}

type PetRuntimeHandle string

func (h PetRuntimeHandle) String() string { return string(h) }

type PluginRuntimeAttachRequest struct {
	ExtensionID    string
	PluginID       string
	ContributionID string
	Revision       int
	Definition     map[string]any
}

type PetRuntimeCapability interface {
	AttachPluginRuntime(ctx context.Context, req PluginRuntimeAttachRequest) (PetRuntimeHandle, error)
	DetachPluginRuntime(ctx context.Context, handle PetRuntimeHandle) error
}

type PetFloatingWindowHandle string

func (h PetFloatingWindowHandle) String() string { return string(h) }

type PluginFloatingWindowAttachRequest struct {
	ExtensionID    string
	PluginID       string
	ContributionID string
	Revision       int
	Definition     map[string]any
}

type PetFloatingWindowCapability interface {
	AttachPluginFloatingWindow(ctx context.Context, req PluginFloatingWindowAttachRequest) (PetFloatingWindowHandle, error)
	DetachPluginFloatingWindow(ctx context.Context, handle PetFloatingWindowHandle) error
}

type DesktopPetPluginCapabilities struct {
	Resource       PetResourceCapability
	Action         PetActionCapability
	Runtime        PetRuntimeCapability
	FloatingWindow PetFloatingWindowCapability
}

func (c DesktopPetPluginCapabilities) Validate() error {
	if c.Resource == nil {
		return fmt.Errorf("desktoppet integration: PetResourceCapability is required")
	}
	if c.Action == nil {
		return fmt.Errorf("desktoppet integration: PetActionCapability is required")
	}
	if c.Runtime == nil {
		return fmt.Errorf("desktoppet integration: PetRuntimeCapability is required")
	}
	if c.FloatingWindow == nil {
		return fmt.Errorf("desktoppet integration: PetFloatingWindowCapability is required")
	}
	return nil
}
