package integration

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/desktoppet/installation"
)

type productionResourceCapability struct {
	mu       sync.RWMutex
	installed installation.Repository
	attached map[PetResourceHandle]*petResource
}

func NewProductionResourceCapability(installed installation.Repository) PetResourceCapability {
	return &productionResourceCapability{
		installed: installed,
		attached:  make(map[PetResourceHandle]*petResource),
	}
}

func (c *productionResourceCapability) AttachPluginResource(ctx context.Context, req PluginResourceAttachRequest) (PetResourceHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginResource: ExtensionID and ContributionID required")
	}
	if c.installed == nil {
		return "", fmt.Errorf("AttachPluginResource: installation repository unavailable")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	handle := PetResourceHandle(req.ExtensionID + "/" + req.ContributionID)
	c.attached[handle] = &petResource{
		handle:   handle,
		req:      req,
		metadata: make(map[string]any),
	}
	return handle, nil
}

func (c *productionResourceCapability) DetachPluginResource(ctx context.Context, handle PetResourceHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.attached[handle]; !ok {
		return fmt.Errorf("DetachPluginResource: handle %s not found", handle)
	}
	delete(c.attached, handle)
	return nil
}

type productionActionTargetResolver struct {
	installed installation.Repository
}

func NewProductionActionTargetResolver(installed installation.Repository) ActionTargetResolver {
	return &productionActionTargetResolver{installed: installed}
}

func (r *productionActionTargetResolver) ResolveActionTarget(ctx context.Context, extensionID string, contributionID string, revision int) (ExistingPetActionTarget, error) {
	if extensionID == "" || contributionID == "" {
		return ExistingPetActionTarget{}, fmt.Errorf("ResolveActionTarget: extensionID and contributionID required")
	}
	if r.installed == nil {
		return ExistingPetActionTarget{}, fmt.Errorf("ResolveActionTarget: installation repository unavailable")
	}
	install, err := r.installed.GetActiveInstallation(extensionID)
	if err != nil {
		if err == installation.ErrInstallationNotFound {
			return ExistingPetActionTarget{}, fmt.Errorf("ResolveActionTarget: no active installation for %s", extensionID)
		}
		return ExistingPetActionTarget{}, fmt.Errorf("ResolveActionTarget: %w", err)
	}
	return ExistingPetActionTarget{
		InstallationID: install.ID,
		UserID:         install.UserID,
	}, nil
}

type productionActionCapability struct {
	mu       sync.RWMutex
	attached map[PetActionHandle]*petAction
}

func NewProductionActionCapability() PetActionCapability {
	return &productionActionCapability{
		attached: make(map[PetActionHandle]*petAction),
	}
}

func (c *productionActionCapability) AttachPluginAction(ctx context.Context, req PluginActionAttachRequest) (PetActionHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginAction: ExtensionID and ContributionID required")
	}
	if req.Target.InstallationID == "" {
		return "", fmt.Errorf("AttachPluginAction: action target installationID required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	handle := PetActionHandle(req.ExtensionID + "/" + req.ContributionID)
	c.attached[handle] = &petAction{
		handle:   handle,
		req:      req,
		metadata: make(map[string]any),
	}
	return handle, nil
}

func (c *productionActionCapability) DetachPluginAction(ctx context.Context, handle PetActionHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.attached[handle]; !ok {
		return fmt.Errorf("DetachPluginAction: handle %s not found", handle)
	}
	delete(c.attached, handle)
	return nil
}

type productionRuntimeCapability struct {
	mu       sync.RWMutex
	attached map[PetRuntimeHandle]*petRuntime
}

func NewProductionRuntimeCapability() PetRuntimeCapability {
	return &productionRuntimeCapability{
		attached: make(map[PetRuntimeHandle]*petRuntime),
	}
}

func (c *productionRuntimeCapability) AttachPluginRuntime(ctx context.Context, req PluginRuntimeAttachRequest) (PetRuntimeHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginRuntime: ExtensionID and ContributionID required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	handle := PetRuntimeHandle(req.ExtensionID + "/" + req.ContributionID)
	c.attached[handle] = &petRuntime{
		handle:   handle,
		req:      req,
		metadata: make(map[string]any),
	}
	return handle, nil
}

func (c *productionRuntimeCapability) DetachPluginRuntime(ctx context.Context, handle PetRuntimeHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.attached[handle]; !ok {
		return fmt.Errorf("DetachPluginRuntime: handle %s not found", handle)
	}
	delete(c.attached, handle)
	return nil
}

type productionFloatingWindowCapability struct {
	mu       sync.RWMutex
	attached map[PetFloatingWindowHandle]*petFloatingWindow
}

func NewProductionFloatingWindowCapability() PetFloatingWindowCapability {
	return &productionFloatingWindowCapability{
		attached: make(map[PetFloatingWindowHandle]*petFloatingWindow),
	}
}

func (c *productionFloatingWindowCapability) AttachPluginFloatingWindow(ctx context.Context, req PluginFloatingWindowAttachRequest) (PetFloatingWindowHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginFloatingWindow: ExtensionID and ContributionID required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	handle := PetFloatingWindowHandle(req.ExtensionID + "/" + req.ContributionID)
	c.attached[handle] = &petFloatingWindow{
		handle:   handle,
		req:      req,
		metadata: make(map[string]any),
	}
	return handle, nil
}

func (c *productionFloatingWindowCapability) DetachPluginFloatingWindow(ctx context.Context, handle PetFloatingWindowHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.attached[handle]; !ok {
		return fmt.Errorf("DetachPluginFloatingWindow: handle %s not found", handle)
	}
	delete(c.attached, handle)
	return nil
}

type ProductionCapabilitiesOptions struct {
	InstallationRepo installation.Repository
}

func NewProductionCapabilities(opts ProductionCapabilitiesOptions) DesktopPetPluginCapabilities {
	return DesktopPetPluginCapabilities{
		Resource:       NewProductionResourceCapability(opts.InstallationRepo),
		Action:         NewProductionActionCapability(),
		Runtime:        NewProductionRuntimeCapability(),
		FloatingWindow: NewProductionFloatingWindowCapability(),
		ActionTarget:   NewProductionActionTargetResolver(opts.InstallationRepo),
	}
}

func NewFixtureCapabilities() DesktopPetPluginCapabilities {
	return DesktopPetPluginCapabilities{
		Resource:       NewDefaultResourceCapability(),
		Action:         NewDefaultActionCapability(),
		Runtime:        NewDefaultRuntimeCapability(),
		FloatingWindow: NewDefaultFloatingWindowCapability(),
		ActionTarget:   NewDefaultActionTargetResolver(),
	}
}
