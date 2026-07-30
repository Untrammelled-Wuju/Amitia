// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

import (
	"fmt"
	"sort"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type ActionOverride struct {
	ActionKey   string
	Enabled     bool
	Recommended bool
}

type OverrideProvider interface {
	FetchOverrides() (map[string]ActionOverride, error)
}

type Resolver interface {
	Resolve(actionKey string) (contracts.ActionSpec, error)
	ResolveMany(actionKeys []string) ([]contracts.ActionSpec, error)
	ResolveWithOverrides(actionKey string, override ActionOverride) (contracts.ActionSpec, error)
}

type catalogResolver struct {
	provider OverrideProvider
}

func NewResolver(provider OverrideProvider) Resolver {
	return &catalogResolver{provider: provider}
}

func (r *catalogResolver) Resolve(actionKey string) (contracts.ActionSpec, error) {
	spec, ok := CatalogGet(actionKey)
	if !ok {
		return contracts.ActionSpec{}, fmt.Errorf("action key not found in catalog: %s", actionKey)
	}

	if r.provider != nil {
		overrides, err := r.provider.FetchOverrides()
		if err != nil {
			return contracts.ActionSpec{}, fmt.Errorf("fetch overrides for %s: %w", actionKey, err)
		}
		if override, exists := overrides[actionKey]; exists {
			spec.Identity.Enabled = override.Enabled
			spec.Identity.Recommended = override.Recommended
		}
	}

	if !spec.Identity.Enabled {
		return contracts.ActionSpec{}, fmt.Errorf("action key is disabled: %s", actionKey)
	}

	return spec, nil
}

func (r *catalogResolver) ResolveMany(actionKeys []string) ([]contracts.ActionSpec, error) {
	seen := make(map[string]bool)
	deduped := make([]string, 0, len(actionKeys))
	for _, key := range actionKeys {
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, key)
	}

	overrides := make(map[string]ActionOverride)
	if r.provider != nil {
		fetched, err := r.provider.FetchOverrides()
		if err != nil {
			return nil, fmt.Errorf("fetch overrides: %w", err)
		}
		overrides = fetched
	}

	result := make([]contracts.ActionSpec, 0, len(deduped))
	var resolveErr error
	for _, key := range deduped {
		spec, err := r.resolveSingle(key, overrides)
		if err != nil {
			if resolveErr == nil {
				resolveErr = fmt.Errorf("resolve %s: %w", key, err)
			}
			continue
		}
		result = append(result, spec)
	}

	sortByCatalogOrder(result)

	if resolveErr != nil {
		return result, resolveErr
	}

	return result, nil
}

func (r *catalogResolver) ResolveWithOverrides(actionKey string, override ActionOverride) (contracts.ActionSpec, error) {
	spec, ok := CatalogGet(actionKey)
	if !ok {
		if override.ActionKey != "" {
			return contracts.ActionSpec{}, fmt.Errorf("action key not in current catalog: %s", actionKey)
		}
		return contracts.ActionSpec{}, fmt.Errorf("action key not found in catalog: %s", actionKey)
	}

	spec.Identity.Enabled = override.Enabled
	spec.Identity.Recommended = override.Recommended

	if !spec.Identity.Enabled {
		return contracts.ActionSpec{}, fmt.Errorf("action key is disabled: %s", actionKey)
	}

	return spec, nil
}

func (r *catalogResolver) resolveSingle(actionKey string, overrides map[string]ActionOverride) (contracts.ActionSpec, error) {
	spec, ok := CatalogGet(actionKey)
	if !ok {
		return contracts.ActionSpec{}, fmt.Errorf("action key not found in catalog: %s", actionKey)
	}

	if override, exists := overrides[actionKey]; exists {
		spec.Identity.Enabled = override.Enabled
		spec.Identity.Recommended = override.Recommended
	}

	if !spec.Identity.Enabled {
		return contracts.ActionSpec{}, fmt.Errorf("action key is disabled: %s", actionKey)
	}

	return spec, nil
}

func sortByCatalogOrder(specs []contracts.ActionSpec) {
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].Identity.CategorySortOrder != specs[j].Identity.CategorySortOrder {
			return specs[i].Identity.CategorySortOrder < specs[j].Identity.CategorySortOrder
		}
		return specs[i].Identity.ActionSortOrder < specs[j].Identity.ActionSortOrder
	})
}
