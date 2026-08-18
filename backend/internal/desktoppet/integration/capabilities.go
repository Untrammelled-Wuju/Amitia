package integration

import (
	"context"
	"fmt"
)

type AttachTransactionRequest struct {
	ExtensionID    string
	PluginID       string
	ContributionID string
	Revision       int
	Target         ExistingPetActionTarget
	Definition     map[string]any
}

type AttachTransactionResult struct {
	ResourceHandle       PetResourceHandle
	ActionHandle         PetActionHandle
	RuntimeHandle        PetRuntimeHandle
	FloatingWindowHandle PetFloatingWindowHandle
}

type DetachTransactionRequest struct {
	ExtensionID    string
	PluginID       string
	ContributionID string
	ResourceHandle PetResourceHandle
	ActionHandle   PetActionHandle
	RuntimeHandle  PetRuntimeHandle
	WindowHandle   PetFloatingWindowHandle
}

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
	ActionTarget   ActionTargetResolver
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
	if c.ActionTarget == nil {
		return fmt.Errorf("desktoppet integration: ActionTargetResolver is required")
	}
	return nil
}

func (c DesktopPetPluginCapabilities) TransactionalAttach(ctx context.Context, req AttachTransactionRequest) (AttachTransactionResult, error) {
	var result AttachTransactionResult
	var rollback []func() error

	resReq := PluginResourceAttachRequest{
		ExtensionID:    req.ExtensionID,
		PluginID:       req.PluginID,
		ContributionID: req.ContributionID,
		Revision:       req.Revision,
		Definition:     req.Definition,
	}
	resH, err := c.Resource.AttachPluginResource(ctx, resReq)
	if err != nil {
		return result, fmt.Errorf("transaction attach: resource failed: %w", err)
	}
	result.ResourceHandle = resH
	rollback = append(rollback, func() error {
		return c.Resource.DetachPluginResource(ctx, resH)
	})

	actionReq := PluginActionAttachRequest{
		ExtensionID:    req.ExtensionID,
		PluginID:       req.PluginID,
		ContributionID: req.ContributionID,
		Revision:       req.Revision,
		Target:         req.Target,
		Definition:     req.Definition,
	}
	actionH, err := c.Action.AttachPluginAction(ctx, actionReq)
	if err != nil {
		return result, c.rollbackAttach(ctx, rollback, fmt.Errorf("transaction attach: action failed: %w", err))
	}
	result.ActionHandle = actionH
	rollback = append(rollback, func() error {
		return c.Action.DetachPluginAction(ctx, actionH)
	})

	rtReq := PluginRuntimeAttachRequest{
		ExtensionID:    req.ExtensionID,
		PluginID:       req.PluginID,
		ContributionID: req.ContributionID,
		Revision:       req.Revision,
		Definition:     req.Definition,
	}
	rtH, err := c.Runtime.AttachPluginRuntime(ctx, rtReq)
	if err != nil {
		return result, c.rollbackAttach(ctx, rollback, fmt.Errorf("transaction attach: runtime failed: %w", err))
	}
	result.RuntimeHandle = rtH
	rollback = append(rollback, func() error {
		return c.Runtime.DetachPluginRuntime(ctx, rtH)
	})

	winReq := PluginFloatingWindowAttachRequest{
		ExtensionID:    req.ExtensionID,
		PluginID:       req.PluginID,
		ContributionID: req.ContributionID,
		Revision:       req.Revision,
		Definition:     req.Definition,
	}
	winH, err := c.FloatingWindow.AttachPluginFloatingWindow(ctx, winReq)
	if err != nil {
		return result, c.rollbackAttach(ctx, rollback, fmt.Errorf("transaction attach: floating window failed: %w", err))
	}
	result.FloatingWindowHandle = winH
	return result, nil
}

func (c DesktopPetPluginCapabilities) rollbackAttach(ctx context.Context, rollback []func() error, cause error) error {
	var rbErr error
	for i := len(rollback) - 1; i >= 0; i-- {
		if err := rollback[i](); err != nil {
			rbErr = err
		}
	}
	if rbErr != nil {
		return fmt.Errorf("%w (rollback also failed: %v)", cause, rbErr)
	}
	return cause
}

func (c DesktopPetPluginCapabilities) TransactionalDetach(ctx context.Context, req DetachTransactionRequest) error {
	if err := c.FloatingWindow.DetachPluginFloatingWindow(ctx, req.WindowHandle); err != nil {
		return fmt.Errorf("transaction detach: floating window failed: %w", err)
	}
	if err := c.Runtime.DetachPluginRuntime(ctx, req.RuntimeHandle); err != nil {
		return fmt.Errorf("transaction detach: runtime failed: %w", err)
	}
	if err := c.Action.DetachPluginAction(ctx, req.ActionHandle); err != nil {
		return fmt.Errorf("transaction detach: action failed: %w", err)
	}
	if err := c.Resource.DetachPluginResource(ctx, req.ResourceHandle); err != nil {
		return fmt.Errorf("transaction detach: resource failed: %w", err)
	}
	return nil
}

type ContributionExtensionSource interface {
	ListContributionsForExtension(ctx context.Context, extensionID string) ([]ContributionDefinition, error)
}

type ContributionDefinition struct {
	ExtensionID    string
	PluginID       string
	ContributionID string
	Revision       int
	Definition     map[string]any
}

func (c DesktopPetPluginCapabilities) RebuildFromExisting(ctx context.Context, extensionID string, source ContributionExtensionSource) error {
	if source == nil {
		return fmt.Errorf("RebuildFromExisting: contribution source required")
	}
	contribs, err := source.ListContributionsForExtension(ctx, extensionID)
	if err != nil {
		return fmt.Errorf("RebuildFromExisting: list contributions: %w", err)
	}

	// Correlation is explicitly rebuildable. Clear any stale in-memory handles first;
	// the canonical Extension Kernel contribution source is then replayed through the
	// same production capabilities used for normal lifecycle attach.
	if p, ok := c.Resource.(*productionResourceCapability); ok && p.release != nil {
		if err := p.release.RebuildFromExisting(); err != nil {
			return fmt.Errorf("RebuildFromExisting: resource reset: %w", err)
		}
		p.mu.Lock()
		p.cache = make(map[PetResourceHandle]PluginResourceAttachRequest)
		p.mu.Unlock()
	}
	if p, ok := c.Action.(*productionActionCapability); ok && p.facade != nil {
		if err := p.facade.RebuildFromExisting(); err != nil {
			return fmt.Errorf("RebuildFromExisting: action reset: %w", err)
		}
		p.mu.Lock()
		p.cache = make(map[PetActionHandle]PluginActionAttachRequest)
		p.mu.Unlock()
	}
	if p, ok := c.Runtime.(*productionRuntimeCapability); ok && p.facade != nil {
		if err := p.facade.RebuildFromExisting(); err != nil {
			return fmt.Errorf("RebuildFromExisting: runtime reset: %w", err)
		}
		p.mu.Lock()
		p.cache = make(map[PetRuntimeHandle]PluginRuntimeAttachRequest)
		p.mu.Unlock()
	}
	if p, ok := c.FloatingWindow.(*productionFloatingWindowCapability); ok && p.publisher != nil {
		if err := p.publisher.RebuildFromExisting(); err != nil {
			return fmt.Errorf("RebuildFromExisting: window reset: %w", err)
		}
		p.mu.Lock()
		p.cache = make(map[PetFloatingWindowHandle]PluginFloatingWindowAttachRequest)
		p.mu.Unlock()
	}

	for _, contrib := range contribs {
		if contrib.ExtensionID == "" {
			contrib.ExtensionID = extensionID
		}
		target := ExistingPetActionTarget{}
		if c.ActionTarget != nil {
			resolved, resolveErr := c.ActionTarget.ResolveActionTarget(ctx, contrib.ExtensionID, contrib.ContributionID, contrib.Revision)
			if resolveErr != nil {
				return fmt.Errorf("RebuildFromExisting: resolve action target %s: %w", contrib.ContributionID, resolveErr)
			}
			target = resolved
		}
		if _, err := c.TransactionalAttach(ctx, AttachTransactionRequest{
			ExtensionID:    contrib.ExtensionID,
			PluginID:       contrib.PluginID,
			ContributionID: contrib.ContributionID,
			Revision:       contrib.Revision,
			Target:         target,
			Definition:     contrib.Definition,
		}); err != nil {
			return fmt.Errorf("RebuildFromExisting: replay contribution %s: %w", contrib.ContributionID, err)
		}
	}
	return nil
}
