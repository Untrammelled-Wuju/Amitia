package integration

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type petResource struct {
	handle   PetResourceHandle
	req      PluginResourceAttachRequest
	metadata map[string]any
}

type petAction struct {
	handle   PetActionHandle
	req      PluginActionAttachRequest
	metadata map[string]any
}

type petRuntime struct {
	handle   PetRuntimeHandle
	req      PluginRuntimeAttachRequest
	metadata map[string]any
}

type petFloatingWindow struct {
	handle   PetFloatingWindowHandle
	req      PluginFloatingWindowAttachRequest
	metadata map[string]any
}

type defaultResourceCapability struct {
	mu       sync.RWMutex
	attached map[PetResourceHandle]*petResource
}

func NewDefaultResourceCapability() PetResourceCapability {
	return &defaultResourceCapability{
		attached: make(map[PetResourceHandle]*petResource),
	}
}

func (c *defaultResourceCapability) AttachPluginResource(ctx context.Context, req PluginResourceAttachRequest) (PetResourceHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginResource: ExtensionID and ContributionID required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	handle := PetResourceHandle(uuid.New().String())
	c.attached[handle] = &petResource{
		handle:   handle,
		req:      req,
		metadata: make(map[string]any),
	}
	return handle, nil
}

func (c *defaultResourceCapability) DetachPluginResource(ctx context.Context, handle PetResourceHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.attached[handle]; !ok {
		return fmt.Errorf("DetachPluginResource: handle %s not found", handle)
	}
	delete(c.attached, handle)
	return nil
}

type defaultActionCapability struct {
	mu       sync.RWMutex
	attached map[PetActionHandle]*petAction
}

func NewDefaultActionCapability() PetActionCapability {
	return &defaultActionCapability{
		attached: make(map[PetActionHandle]*petAction),
	}
}

func (c *defaultActionCapability) AttachPluginAction(ctx context.Context, req PluginActionAttachRequest) (PetActionHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginAction: ExtensionID and ContributionID required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	handle := PetActionHandle(uuid.New().String())
	c.attached[handle] = &petAction{
		handle:   handle,
		req:      req,
		metadata: make(map[string]any),
	}
	return handle, nil
}

func (c *defaultActionCapability) DetachPluginAction(ctx context.Context, handle PetActionHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.attached[handle]; !ok {
		return fmt.Errorf("DetachPluginAction: handle %s not found", handle)
	}
	delete(c.attached, handle)
	return nil
}

type defaultRuntimeCapability struct {
	mu       sync.RWMutex
	attached map[PetRuntimeHandle]*petRuntime
}

func NewDefaultRuntimeCapability() PetRuntimeCapability {
	return &defaultRuntimeCapability{
		attached: make(map[PetRuntimeHandle]*petRuntime),
	}
}

func (c *defaultRuntimeCapability) AttachPluginRuntime(ctx context.Context, req PluginRuntimeAttachRequest) (PetRuntimeHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginRuntime: ExtensionID and ContributionID required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	handle := PetRuntimeHandle(uuid.New().String())
	c.attached[handle] = &petRuntime{
		handle:   handle,
		req:      req,
		metadata: make(map[string]any),
	}
	return handle, nil
}

func (c *defaultRuntimeCapability) DetachPluginRuntime(ctx context.Context, handle PetRuntimeHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.attached[handle]; !ok {
		return fmt.Errorf("DetachPluginRuntime: handle %s not found", handle)
	}
	delete(c.attached, handle)
	return nil
}

type defaultFloatingWindowCapability struct {
	mu       sync.RWMutex
	attached map[PetFloatingWindowHandle]*petFloatingWindow
}

func NewDefaultFloatingWindowCapability() PetFloatingWindowCapability {
	return &defaultFloatingWindowCapability{
		attached: make(map[PetFloatingWindowHandle]*petFloatingWindow),
	}
}

func (c *defaultFloatingWindowCapability) AttachPluginFloatingWindow(ctx context.Context, req PluginFloatingWindowAttachRequest) (PetFloatingWindowHandle, error) {
	if req.ExtensionID == "" || req.ContributionID == "" {
		return "", fmt.Errorf("AttachPluginFloatingWindow: ExtensionID and ContributionID required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	handle := PetFloatingWindowHandle(uuid.New().String())
	c.attached[handle] = &petFloatingWindow{
		handle:   handle,
		req:      req,
		metadata: make(map[string]any),
	}
	return handle, nil
}

func (c *defaultFloatingWindowCapability) DetachPluginFloatingWindow(ctx context.Context, handle PetFloatingWindowHandle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.attached[handle]; !ok {
		return fmt.Errorf("DetachPluginFloatingWindow: handle %s not found", handle)
	}
	delete(c.attached, handle)
	return nil
}

type defaultActionTargetResolver struct {
}

func NewDefaultActionTargetResolver() ActionTargetResolver {
	return &defaultActionTargetResolver{}
}

func (r *defaultActionTargetResolver) ResolveActionTarget(ctx context.Context, extensionID string, contributionID string, revision int) (ExistingPetActionTarget, error) {
	if extensionID == "" || contributionID == "" {
		return ExistingPetActionTarget{}, fmt.Errorf("ResolveActionTarget: extensionID and contributionID required")
	}
	return ExistingPetActionTarget{}, fmt.Errorf("ResolveActionTarget: fixture resolver has no desktop pet target")
}

func DefaultCapabilities() DesktopPetPluginCapabilities {
	return DesktopPetPluginCapabilities{
		Resource:       NewDefaultResourceCapability(),
		Action:         NewDefaultActionCapability(),
		Runtime:        NewDefaultRuntimeCapability(),
		FloatingWindow: NewDefaultFloatingWindowCapability(),
		ActionTarget:   NewDefaultActionTargetResolver(),
	}
}
