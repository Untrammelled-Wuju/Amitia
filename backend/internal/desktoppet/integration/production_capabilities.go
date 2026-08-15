package integration

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/desktoppet/installation"
)

type FloatingWindowEventPublisher interface {
	PublishFloatingWindowContribution(ctx context.Context, extensionID, contributionID string, definition map[string]any) error
}

type productionResourceCapability struct {
	mu        sync.RWMutex
	installed installation.Repository
	release   interface{}
	cache     map[PetResourceHandle]PluginResourceAttachRequest
}

func NewProductionResourceCapability(installed installation.Repository, releaseSvc interface{}) PetResourceCapability {
	return &productionResourceCapability{
		installed: installed,
		release:   releaseSvc,
		cache:     make(map[PetResourceHandle]PluginResourceAttachRequest),
	}
}

func (c *productionResourceCapability) AttachPluginResource(ctx context.Context, req PluginResourceAttachRequest) (PetResourceHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginResource: ExtensionID and ContributionID required")
	}
	if c.installed == nil {
		return "", fmt.Errorf("AttachPluginResource: installation repository unavailable")
	}
	handle := PetResourceHandle(req.ExtensionID + "/" + req.ContributionID)
	c.mu.Lock()
	c.cache[handle] = req
	c.mu.Unlock()
	return handle, nil
}

func (c *productionResourceCapability) DetachPluginResource(ctx context.Context, handle PetResourceHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.cache[handle]; !ok {
		return fmt.Errorf("DetachPluginResource: handle %s not found", handle)
	}
	delete(c.cache, handle)
	return nil
}

func (c *productionResourceCapability) RebuildFromExisting() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[PetResourceHandle]PluginResourceAttachRequest)
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
	facade   interface{}
	resolver ActionTargetResolver
	cache    map[PetActionHandle]PluginActionAttachRequest
}

func NewProductionActionCapability(facade interface{}, resolver ActionTargetResolver) PetActionCapability {
	return &productionActionCapability{
		facade:   facade,
		resolver: resolver,
		cache:    make(map[PetActionHandle]PluginActionAttachRequest),
	}
}

func (c *productionActionCapability) AttachPluginAction(ctx context.Context, req PluginActionAttachRequest) (PetActionHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginAction: ExtensionID and ContributionID required")
	}
	if req.Target.InstallationID == "" && c.resolver != nil {
		target, err := c.resolver.ResolveActionTarget(ctx, req.ExtensionID, req.ContributionID, req.Revision)
		if err != nil {
			return "", fmt.Errorf("AttachPluginAction: resolve target failed: %w", err)
		}
		req.Target = target
	}
	if c.facade == nil {
		return "", fmt.Errorf("AttachPluginAction: runtime facade unavailable")
	}
	handle := PetActionHandle(req.ExtensionID + "/" + req.ContributionID)
	c.mu.Lock()
	c.cache[handle] = req
	c.mu.Unlock()
	return handle, nil
}

func (c *productionActionCapability) DetachPluginAction(ctx context.Context, handle PetActionHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.cache[handle]; !ok {
		return fmt.Errorf("DetachPluginAction: handle %s not found", handle)
	}
	delete(c.cache, handle)
	return nil
}

func (c *productionActionCapability) RebuildFromExisting() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[PetActionHandle]PluginActionAttachRequest)
	return nil
}

type productionRuntimeCapability struct {
	mu     sync.RWMutex
	facade interface{}
	cache  map[PetRuntimeHandle]PluginRuntimeAttachRequest
}

func NewProductionRuntimeCapability(facade interface{}) PetRuntimeCapability {
	return &productionRuntimeCapability{
		facade: facade,
		cache:  make(map[PetRuntimeHandle]PluginRuntimeAttachRequest),
	}
}

func (c *productionRuntimeCapability) AttachPluginRuntime(ctx context.Context, req PluginRuntimeAttachRequest) (PetRuntimeHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginRuntime: ExtensionID and ContributionID required")
	}
	if c.facade == nil {
		return "", fmt.Errorf("AttachPluginRuntime: runtime facade unavailable")
	}
	handle := PetRuntimeHandle(req.ExtensionID + "/" + req.ContributionID)
	c.mu.Lock()
	c.cache[handle] = req
	c.mu.Unlock()
	return handle, nil
}

func (c *productionRuntimeCapability) DetachPluginRuntime(ctx context.Context, handle PetRuntimeHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.cache[handle]; !ok {
		return fmt.Errorf("DetachPluginRuntime: handle %s not found", handle)
	}
	delete(c.cache, handle)
	return nil
}

func (c *productionRuntimeCapability) RebuildFromExisting() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[PetRuntimeHandle]PluginRuntimeAttachRequest)
	return nil
}

type productionFloatingWindowCapability struct {
	mu        sync.RWMutex
	publisher FloatingWindowEventPublisher
	cache     map[PetFloatingWindowHandle]PluginFloatingWindowAttachRequest
}

func NewProductionFloatingWindowCapability(publisher FloatingWindowEventPublisher) PetFloatingWindowCapability {
	return &productionFloatingWindowCapability{
		publisher: publisher,
		cache:     make(map[PetFloatingWindowHandle]PluginFloatingWindowAttachRequest),
	}
}

func (c *productionFloatingWindowCapability) AttachPluginFloatingWindow(ctx context.Context, req PluginFloatingWindowAttachRequest) (PetFloatingWindowHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginFloatingWindow: ExtensionID and ContributionID required")
	}
	if c.publisher != nil {
		if err := c.publisher.PublishFloatingWindowContribution(ctx, req.ExtensionID, req.ContributionID, req.Definition); err != nil {
			return "", fmt.Errorf("AttachPluginFloatingWindow: publish contribution failed: %w", err)
		}
	}
	handle := PetFloatingWindowHandle(req.ExtensionID + "/" + req.ContributionID)
	c.mu.Lock()
	c.cache[handle] = req
	c.mu.Unlock()
	return handle, nil
}

func (c *productionFloatingWindowCapability) DetachPluginFloatingWindow(ctx context.Context, handle PetFloatingWindowHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.cache[handle]; !ok {
		return fmt.Errorf("DetachPluginFloatingWindow: handle %s not found", handle)
	}
	delete(c.cache, handle)
	return nil
}

func (c *productionFloatingWindowCapability) RebuildFromExisting() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[PetFloatingWindowHandle]PluginFloatingWindowAttachRequest)
	return nil
}

type ProductionCapabilitiesOptions struct {
	InstallationRepo installation.Repository
	ReleaseService   interface{}
	RuntimeFacade    interface{}
	FloatingWindow   FloatingWindowEventPublisher
}

func NewProductionCapabilities(opts ProductionCapabilitiesOptions) DesktopPetPluginCapabilities {
	resolver := NewProductionActionTargetResolver(opts.InstallationRepo)
	return DesktopPetPluginCapabilities{
		Resource:       NewProductionResourceCapability(opts.InstallationRepo, opts.ReleaseService),
		Action:         NewProductionActionCapability(opts.RuntimeFacade, resolver),
		Runtime:        NewProductionRuntimeCapability(opts.RuntimeFacade),
		FloatingWindow: NewProductionFloatingWindowCapability(opts.FloatingWindow),
		ActionTarget:   resolver,
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
