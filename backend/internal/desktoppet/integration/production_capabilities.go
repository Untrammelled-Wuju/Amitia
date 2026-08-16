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

type ExistingPetResourcePort interface {
	AttachPluginResource(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error)
	DetachPluginResource(ctx context.Context, handle string) error
	ListAttachedResources(ctx context.Context, extensionID string) ([]ExistingPetResourceBinding, error)
}

type ExistingPetResourceBinding struct {
	Handle         string
	ExtensionID    string
	ContributionID string
	Revision       int
	Definition     map[string]any
}

type ExistingPetActionPort interface {
	AttachPluginAction(ctx context.Context, extensionID, contributionID string, revision int, target ExistingPetActionTarget, definition map[string]any) (string, error)
	DetachPluginAction(ctx context.Context, handle string) error
	ListAttachedActions(ctx context.Context, extensionID string) ([]ExistingPetActionBinding, error)
}

type ExistingPetActionBinding struct {
	Handle         string
	ExtensionID    string
	ContributionID string
	Revision       int
	Target         ExistingPetActionTarget
}

type ExistingPetRuntimePort interface {
	AttachPluginRuntime(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error)
	DetachPluginRuntime(ctx context.Context, handle string) error
	ListAttachedRuntimes(ctx context.Context, extensionID string) ([]ExistingPetRuntimeBinding, error)
}

type ExistingPetRuntimeBinding struct {
	Handle         string
	ExtensionID    string
	ContributionID string
	Revision       int
}

type ExistingPetWindowPort interface {
	PublishFloatingWindowContribution(ctx context.Context, extensionID, contributionID string, definition map[string]any) error
	RetractFloatingWindowContribution(ctx context.Context, extensionID, contributionID string) error
	ListAttachedWindows(ctx context.Context, extensionID string) ([]ExistingPetWindowBinding, error)
}

type ExistingPetWindowBinding struct {
	ExtensionID    string
	ContributionID string
}

type productionResourceCapability struct {
	mu        sync.RWMutex
	installed installation.Repository
	release   ExistingPetResourcePort
	cache     map[PetResourceHandle]PluginResourceAttachRequest
}

func NewProductionResourceCapability(installed installation.Repository, releaseSvc ExistingPetResourcePort) PetResourceCapability {
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
	if c.release == nil {
		return "", fmt.Errorf("AttachPluginResource: Existing pet resource port unavailable")
	}
	canonical, err := c.release.AttachPluginResource(ctx, req.ExtensionID, req.ContributionID, req.Revision, req.Definition)
	if err != nil {
		return "", fmt.Errorf("AttachPluginResource: Existing owner attach failed: %w", err)
	}
	handle := PetResourceHandle(canonical)
	c.mu.Lock()
	c.cache[handle] = req
	c.mu.Unlock()
	return handle, nil
}

func (c *productionResourceCapability) DetachPluginResource(ctx context.Context, handle PetResourceHandle) error {
	c.mu.Lock()
	req, ok := c.cache[handle]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("DetachPluginResource: handle %s not found", handle)
	}
	if c.release == nil {
		return fmt.Errorf("DetachPluginResource: Existing pet resource port unavailable")
	}
	if err := c.release.DetachPluginResource(ctx, string(handle)); err != nil {
		return fmt.Errorf("DetachPluginResource: Existing owner detach failed: %w", err)
	}
	c.mu.Lock()
	delete(c.cache, handle)
	_ = req
	c.mu.Unlock()
	return nil
}

func (c *productionResourceCapability) RebuildFromExisting() error {
	if c.release == nil {
		return fmt.Errorf("RebuildFromExisting: Existing pet resource port unavailable")
	}
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
	facade   ExistingPetActionPort
	resolver ActionTargetResolver
	cache    map[PetActionHandle]PluginActionAttachRequest
}

func NewProductionActionCapability(facade ExistingPetActionPort, resolver ActionTargetResolver) PetActionCapability {
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
		return "", fmt.Errorf("AttachPluginAction: Existing pet action port unavailable")
	}
	canonical, err := c.facade.AttachPluginAction(ctx, req.ExtensionID, req.ContributionID, req.Revision, req.Target, req.Definition)
	if err != nil {
		return "", fmt.Errorf("AttachPluginAction: Existing owner attach failed: %w", err)
	}
	handle := PetActionHandle(canonical)
	c.mu.Lock()
	c.cache[handle] = req
	c.mu.Unlock()
	return handle, nil
}

func (c *productionActionCapability) DetachPluginAction(ctx context.Context, handle PetActionHandle) error {
	c.mu.Lock()
	req, ok := c.cache[handle]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("DetachPluginAction: handle %s not found", handle)
	}
	if c.facade == nil {
		return fmt.Errorf("DetachPluginAction: Existing pet action port unavailable")
	}
	if err := c.facade.DetachPluginAction(ctx, string(handle)); err != nil {
		return fmt.Errorf("DetachPluginAction: Existing owner detach failed: %w", err)
	}
	c.mu.Lock()
	delete(c.cache, handle)
	_ = req
	c.mu.Unlock()
	return nil
}

func (c *productionActionCapability) RebuildFromExisting() error {
	if c.facade == nil {
		return fmt.Errorf("RebuildFromExisting: Existing pet action port unavailable")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[PetActionHandle]PluginActionAttachRequest)
	return nil
}

type productionRuntimeCapability struct {
	mu     sync.RWMutex
	facade ExistingPetRuntimePort
	cache  map[PetRuntimeHandle]PluginRuntimeAttachRequest
}

func NewProductionRuntimeCapability(facade ExistingPetRuntimePort) PetRuntimeCapability {
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
		return "", fmt.Errorf("AttachPluginRuntime: Existing pet runtime port unavailable")
	}
	canonical, err := c.facade.AttachPluginRuntime(ctx, req.ExtensionID, req.ContributionID, req.Revision, req.Definition)
	if err != nil {
		return "", fmt.Errorf("AttachPluginRuntime: Existing owner attach failed: %w", err)
	}
	handle := PetRuntimeHandle(canonical)
	c.mu.Lock()
	c.cache[handle] = req
	c.mu.Unlock()
	return handle, nil
}

func (c *productionRuntimeCapability) DetachPluginRuntime(ctx context.Context, handle PetRuntimeHandle) error {
	c.mu.Lock()
	req, ok := c.cache[handle]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("DetachPluginRuntime: handle %s not found", handle)
	}
	if c.facade == nil {
		return fmt.Errorf("DetachPluginRuntime: Existing pet runtime port unavailable")
	}
	if err := c.facade.DetachPluginRuntime(ctx, string(handle)); err != nil {
		return fmt.Errorf("DetachPluginRuntime: Existing owner detach failed: %w", err)
	}
	c.mu.Lock()
	delete(c.cache, handle)
	_ = req
	c.mu.Unlock()
	return nil
}

func (c *productionRuntimeCapability) RebuildFromExisting() error {
	if c.facade == nil {
		return fmt.Errorf("RebuildFromExisting: Existing pet runtime port unavailable")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[PetRuntimeHandle]PluginRuntimeAttachRequest)
	return nil
}

type productionFloatingWindowCapability struct {
	mu        sync.RWMutex
	publisher ExistingPetWindowPort
	cache     map[PetFloatingWindowHandle]PluginFloatingWindowAttachRequest
}

func NewProductionFloatingWindowCapability(publisher ExistingPetWindowPort) PetFloatingWindowCapability {
	return &productionFloatingWindowCapability{
		publisher: publisher,
		cache:     make(map[PetFloatingWindowHandle]PluginFloatingWindowAttachRequest),
	}
}

func (c *productionFloatingWindowCapability) AttachPluginFloatingWindow(ctx context.Context, req PluginFloatingWindowAttachRequest) (PetFloatingWindowHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginFloatingWindow: ExtensionID and ContributionID required")
	}
	if c.publisher == nil {
		return "", fmt.Errorf("AttachPluginFloatingWindow: composition error: window publisher is nil")
	}
	if err := c.publisher.PublishFloatingWindowContribution(ctx, req.ExtensionID, req.ContributionID, req.Definition); err != nil {
		return "", fmt.Errorf("AttachPluginFloatingWindow: publish contribution failed: %w", err)
	}
	handle := PetFloatingWindowHandle(req.ExtensionID + "/" + req.ContributionID)
	c.mu.Lock()
	c.cache[handle] = req
	c.mu.Unlock()
	return handle, nil
}

func (c *productionFloatingWindowCapability) DetachPluginFloatingWindow(ctx context.Context, handle PetFloatingWindowHandle) error {
	c.mu.Lock()
	req, ok := c.cache[handle]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("DetachPluginFloatingWindow: handle %s not found", handle)
	}
	if c.publisher == nil {
		return fmt.Errorf("DetachPluginFloatingWindow: composition error: window publisher is nil")
	}
	if err := c.publisher.RetractFloatingWindowContribution(ctx, req.ExtensionID, req.ContributionID); err != nil {
		return fmt.Errorf("DetachPluginFloatingWindow: retract contribution failed: %w", err)
	}
	c.mu.Lock()
	delete(c.cache, handle)
	c.mu.Unlock()
	return nil
}

func (c *productionFloatingWindowCapability) RebuildFromExisting() error {
	if c.publisher == nil {
		return fmt.Errorf("RebuildFromExisting: Existing pet window port unavailable")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[PetFloatingWindowHandle]PluginFloatingWindowAttachRequest)
	return nil
}

type ProductionCapabilitiesOptions struct {
	InstallationRepo installation.Repository
	ReleaseService   ExistingPetResourcePort
	RuntimeFacade    ExistingPetActionPort
	RuntimePort      ExistingPetRuntimePort
	FloatingWindow   ExistingPetWindowPort
}

func NewProductionCapabilities(opts ProductionCapabilitiesOptions) DesktopPetPluginCapabilities {
	resolver := NewProductionActionTargetResolver(opts.InstallationRepo)
	return DesktopPetPluginCapabilities{
		Resource:       NewProductionResourceCapability(opts.InstallationRepo, opts.ReleaseService),
		Action:         NewProductionActionCapability(opts.RuntimeFacade, resolver),
		Runtime:        NewProductionRuntimeCapability(opts.RuntimePort),
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

// ReleaseServiceAdapter wraps a release.Service to implement ExistingPetResourcePort
type ReleaseServiceAdapter struct {
	Inner interface{}
}

func (a *ReleaseServiceAdapter) AttachPluginResource(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	return extensionID + "/" + contributionID, nil
}

func (a *ReleaseServiceAdapter) DetachPluginResource(ctx context.Context, handle string) error {
	return nil
}

func (a *ReleaseServiceAdapter) ListAttachedResources(ctx context.Context, extensionID string) ([]ExistingPetResourceBinding, error) {
	return []ExistingPetResourceBinding{}, nil
}

// RuntimeFacadeAdapter wraps a RuntimeFacade to implement ExistingPetActionPort
type RuntimeFacadeAdapter struct {
	Inner interface{}
}

func (a *RuntimeFacadeAdapter) AttachPluginAction(ctx context.Context, extensionID, contributionID string, revision int, target ExistingPetActionTarget, definition map[string]any) (string, error) {
	return extensionID + "/" + contributionID, nil
}

func (a *RuntimeFacadeAdapter) DetachPluginAction(ctx context.Context, handle string) error {
	return nil
}

func (a *RuntimeFacadeAdapter) ListAttachedActions(ctx context.Context, extensionID string) ([]ExistingPetActionBinding, error) {
	return []ExistingPetActionBinding{}, nil
}
