// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package integration

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// These adapters intentionally keep only rebuildable correlation state in memory.
// DesktopPet plugin contributions are projections of the canonical Extension Kernel
// contribution source; they must never create plugin_*_attachments durable truth.
// RebuildFromExisting clears correlation and DesktopPetPluginCapabilities.RebuildFromExisting
// repopulates it from the canonical contribution source.

type correlationResourcePort struct {
	mu       sync.RWMutex
	bindings map[string]ExistingPetResourceBinding
}

func NewExistingResourceCorrelationPort() ExistingPetResourcePort {
	return &correlationResourcePort{bindings: make(map[string]ExistingPetResourceBinding)}
}

func (p *correlationResourcePort) AttachPluginResource(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("extensionID and contributionID required")
	}
	handle := correlationHandle("resource", extensionID, contributionID, revision)
	p.mu.Lock()
	p.bindings[handle] = ExistingPetResourceBinding{
		Handle: handle, ExtensionID: extensionID, ContributionID: contributionID,
		Revision: revision, Definition: cloneDefinition(definition),
	}
	p.mu.Unlock()
	return handle, nil
}

func (p *correlationResourcePort) DetachPluginResource(ctx context.Context, handle string) error {
	if handle == "" {
		return fmt.Errorf("handle required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.bindings[handle]; !ok {
		return fmt.Errorf("resource correlation %s not found", handle)
	}
	delete(p.bindings, handle)
	return nil
}

func (p *correlationResourcePort) ListAttachedResources(ctx context.Context, extensionID string) ([]ExistingPetResourceBinding, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]ExistingPetResourceBinding, 0)
	for _, b := range p.bindings {
		if extensionID == "" || b.ExtensionID == extensionID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handle < out[j].Handle })
	return out, nil
}

func (p *correlationResourcePort) RebuildFromExisting() error {
	p.mu.Lock()
	p.bindings = make(map[string]ExistingPetResourceBinding)
	p.mu.Unlock()
	return nil
}

type correlationActionPort struct {
	mu       sync.RWMutex
	bindings map[string]ExistingPetActionBinding
}

func NewExistingActionCorrelationPort() ExistingPetActionPort {
	return &correlationActionPort{bindings: make(map[string]ExistingPetActionBinding)}
}

func (p *correlationActionPort) AttachPluginAction(ctx context.Context, extensionID, contributionID string, revision int, target ExistingPetActionTarget, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("extensionID and contributionID required")
	}
	if target.InstallationID == "" {
		return "", fmt.Errorf("AttachPluginAction: target installationID required")
	}
	handle := correlationHandle("action", extensionID, contributionID, revision)
	p.mu.Lock()
	p.bindings[handle] = ExistingPetActionBinding{Handle: handle, ExtensionID: extensionID, ContributionID: contributionID, Revision: revision, Target: target}
	p.mu.Unlock()
	return handle, nil
}

func (p *correlationActionPort) DetachPluginAction(ctx context.Context, handle string) error {
	if handle == "" {
		return fmt.Errorf("handle required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.bindings[handle]; !ok {
		return fmt.Errorf("action correlation %s not found", handle)
	}
	delete(p.bindings, handle)
	return nil
}

func (p *correlationActionPort) ListAttachedActions(ctx context.Context, extensionID string) ([]ExistingPetActionBinding, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]ExistingPetActionBinding, 0)
	for _, b := range p.bindings {
		if extensionID == "" || b.ExtensionID == extensionID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handle < out[j].Handle })
	return out, nil
}

func (p *correlationActionPort) RebuildFromExisting() error {
	p.mu.Lock()
	p.bindings = make(map[string]ExistingPetActionBinding)
	p.mu.Unlock()
	return nil
}

type correlationRuntimePort struct {
	mu       sync.RWMutex
	bindings map[string]ExistingPetRuntimeBinding
}

func NewExistingRuntimeCorrelationPort() ExistingPetRuntimePort {
	return &correlationRuntimePort{bindings: make(map[string]ExistingPetRuntimeBinding)}
}

func (p *correlationRuntimePort) AttachPluginRuntime(ctx context.Context, extensionID, contributionID string, revision int, definition map[string]any) (string, error) {
	if extensionID == "" || contributionID == "" {
		return "", fmt.Errorf("extensionID and contributionID required")
	}
	handle := correlationHandle("runtime", extensionID, contributionID, revision)
	p.mu.Lock()
	p.bindings[handle] = ExistingPetRuntimeBinding{Handle: handle, ExtensionID: extensionID, ContributionID: contributionID, Revision: revision}
	p.mu.Unlock()
	return handle, nil
}

func (p *correlationRuntimePort) DetachPluginRuntime(ctx context.Context, handle string) error {
	if handle == "" {
		return fmt.Errorf("handle required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.bindings[handle]; !ok {
		return fmt.Errorf("runtime correlation %s not found", handle)
	}
	delete(p.bindings, handle)
	return nil
}

func (p *correlationRuntimePort) ListAttachedRuntimes(ctx context.Context, extensionID string) ([]ExistingPetRuntimeBinding, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]ExistingPetRuntimeBinding, 0)
	for _, b := range p.bindings {
		if extensionID == "" || b.ExtensionID == extensionID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handle < out[j].Handle })
	return out, nil
}

func (p *correlationRuntimePort) RebuildFromExisting() error {
	p.mu.Lock()
	p.bindings = make(map[string]ExistingPetRuntimeBinding)
	p.mu.Unlock()
	return nil
}

type correlationWindowPort struct {
	mu       sync.RWMutex
	bindings map[string]ExistingPetWindowBinding
}

func NewExistingWindowCorrelationPort() ExistingPetWindowPort {
	return &correlationWindowPort{bindings: make(map[string]ExistingPetWindowBinding)}
}

func (p *correlationWindowPort) PublishFloatingWindowContribution(ctx context.Context, extensionID, contributionID string, definition map[string]any) error {
	if extensionID == "" || contributionID == "" {
		return fmt.Errorf("extensionID and contributionID required")
	}
	key := extensionID + "/" + contributionID
	p.mu.Lock()
	p.bindings[key] = ExistingPetWindowBinding{ExtensionID: extensionID, ContributionID: contributionID}
	p.mu.Unlock()
	return nil
}

func (p *correlationWindowPort) RetractFloatingWindowContribution(ctx context.Context, extensionID, contributionID string) error {
	if extensionID == "" || contributionID == "" {
		return fmt.Errorf("extensionID and contributionID required")
	}
	key := extensionID + "/" + contributionID
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.bindings[key]; !ok {
		return fmt.Errorf("window correlation %s not found", key)
	}
	delete(p.bindings, key)
	return nil
}

func (p *correlationWindowPort) ListAttachedWindows(ctx context.Context, extensionID string) ([]ExistingPetWindowBinding, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]ExistingPetWindowBinding, 0)
	for _, b := range p.bindings {
		if extensionID == "" || b.ExtensionID == extensionID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ExtensionID+"/"+out[i].ContributionID < out[j].ExtensionID+"/"+out[j].ContributionID
	})
	return out, nil
}

func (p *correlationWindowPort) RebuildFromExisting() error {
	p.mu.Lock()
	p.bindings = make(map[string]ExistingPetWindowBinding)
	p.mu.Unlock()
	return nil
}

func correlationHandle(kind, extensionID, contributionID string, revision int) string {
	return fmt.Sprintf("%s:%s/%s@%d", kind, extensionID, contributionID, revision)
}

func cloneDefinition(def map[string]any) map[string]any {
	if def == nil {
		return nil
	}
	out := make(map[string]any, len(def))
	for k, v := range def {
		out[k] = v
	}
	return out
}
