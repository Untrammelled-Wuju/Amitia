package integration

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/desktoppet/installation"
)

type installationResourcePort struct {
	mu    sync.RWMutex
	repo  installation.Repository
	binds map[string]ExistingPetResourceBinding
}

func NewInstallationResourcePort(repo installation.Repository) ExistingPetResourcePort {
	return &installationResourcePort{
		repo:  repo,
		binds: make(map[string]ExistingPetResourceBinding),
	}
}

func (p *installationResourcePort) AttachPluginResource(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("AttachPluginResource: extensionID and contributionID required")
	}
	if p.repo == nil {
		return "", fmt.Errorf("AttachPluginResource: installation repository unavailable")
	}
	handle := fmt.Sprintf("%s/%s@%d", extensionID, contributionID, revision)
	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.binds[handle]; ok {
		if existing.Revision >= revision {
			return existing.Handle, nil
		}
	}
	p.binds[handle] = ExistingPetResourceBinding{
		Handle:         handle,
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		Revision:       revision,
		Definition:     cloneDef(definition),
	}
	return handle, nil
}

func (p *installationResourcePort) DetachPluginResource(ctx context.Context, handle string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.binds[handle]; !ok {
		return fmt.Errorf("DetachPluginResource: handle %s not found", handle)
	}
	delete(p.binds, handle)
	return nil
}

func (p *installationResourcePort) ListAttachedResources(ctx context.Context, extensionID string) ([]ExistingPetResourceBinding, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []ExistingPetResourceBinding
	for _, b := range p.binds {
		if b.ExtensionID == extensionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (p *installationResourcePort) RebuildFromExisting() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.binds = make(map[string]ExistingPetResourceBinding)
	return nil
}

type installationActionPort struct {
	mu    sync.RWMutex
	repo  installation.Repository
	binds map[string]ExistingPetActionBinding
}

func NewInstallationActionPort(repo installation.Repository) ExistingPetActionPort {
	return &installationActionPort{
		repo:  repo,
		binds: make(map[string]ExistingPetActionBinding),
	}
}

func (p *installationActionPort) AttachPluginAction(ctx context.Context, extensionID, contributionID string, revision int, target ExistingPetActionTarget, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("AttachPluginAction: extensionID and contributionID required")
	}
	handle := fmt.Sprintf("%s/%s@%d", extensionID, contributionID, revision)
	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.binds[handle]; ok {
		if existing.Revision >= revision {
			return existing.Handle, nil
		}
	}
	p.binds[handle] = ExistingPetActionBinding{
		Handle:         handle,
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		Revision:       revision,
		Target:         target,
	}
	return handle, nil
}

func (p *installationActionPort) DetachPluginAction(ctx context.Context, handle string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.binds[handle]; !ok {
		return fmt.Errorf("DetachPluginAction: handle %s not found", handle)
	}
	delete(p.binds, handle)
	return nil
}

func (p *installationActionPort) ListAttachedActions(ctx context.Context, extensionID string) ([]ExistingPetActionBinding, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []ExistingPetActionBinding
	for _, b := range p.binds {
		if b.ExtensionID == extensionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (p *installationActionPort) RebuildFromExisting() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.binds = make(map[string]ExistingPetActionBinding)
	return nil
}

type installationRuntimePort struct {
	mu    sync.RWMutex
	repo  installation.Repository
	binds map[string]ExistingPetRuntimeBinding
}

func NewInstallationRuntimePort(repo installation.Repository) ExistingPetRuntimePort {
	return &installationRuntimePort{
		repo:  repo,
		binds: make(map[string]ExistingPetRuntimeBinding),
	}
}

func (p *installationRuntimePort) AttachPluginRuntime(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("AttachPluginRuntime: extensionID and contributionID required")
	}
	handle := fmt.Sprintf("%s/%s@%d", extensionID, contributionID, revision)
	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.binds[handle]; ok {
		if existing.Revision >= revision {
			return existing.Handle, nil
		}
	}
	p.binds[handle] = ExistingPetRuntimeBinding{
		Handle:         handle,
		ExtensionID:    extensionID,
		ContributionID: contributionID,
		Revision:       revision,
	}
	return handle, nil
}

func (p *installationRuntimePort) DetachPluginRuntime(ctx context.Context, handle string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.binds[handle]; !ok {
		return fmt.Errorf("DetachPluginRuntime: handle %s not found", handle)
	}
	delete(p.binds, handle)
	return nil
}

func (p *installationRuntimePort) ListAttachedRuntimes(ctx context.Context, extensionID string) ([]ExistingPetRuntimeBinding, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []ExistingPetRuntimeBinding
	for _, b := range p.binds {
		if b.ExtensionID == extensionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (p *installationRuntimePort) RebuildFromExisting() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.binds = make(map[string]ExistingPetRuntimeBinding)
	return nil
}

type installationWindowPort struct {
	mu    sync.RWMutex
	repo  installation.Repository
	binds map[string]ExistingPetWindowBinding
}

func NewInstallationWindowPort(repo installation.Repository) ExistingPetWindowPort {
	return &installationWindowPort{
		repo:  repo,
		binds: make(map[string]ExistingPetWindowBinding),
	}
}

func (p *installationWindowPort) PublishFloatingWindowContribution(ctx context.Context, extensionID, contributionID string, definition map[string]any) error {
	if extensionID == "" || contributionID == "" {
		return fmt.Errorf("PublishFloatingWindowContribution: extensionID and contributionID required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key := extensionID + "/" + contributionID
	p.binds[key] = ExistingPetWindowBinding{
		ExtensionID:    extensionID,
		ContributionID: contributionID,
	}
	return nil
}

func (p *installationWindowPort) RetractFloatingWindowContribution(ctx context.Context, extensionID, contributionID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := extensionID + "/" + contributionID
	if _, ok := p.binds[key]; !ok {
		return fmt.Errorf("RetractFloatingWindowContribution: %s not found", key)
	}
	delete(p.binds, key)
	return nil
}

func (p *installationWindowPort) ListAttachedWindows(ctx context.Context, extensionID string) ([]ExistingPetWindowBinding, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []ExistingPetWindowBinding
	for _, b := range p.binds {
		if b.ExtensionID == extensionID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (p *installationWindowPort) RebuildFromExisting() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.binds = make(map[string]ExistingPetWindowBinding)
	return nil
}

func cloneDef(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
