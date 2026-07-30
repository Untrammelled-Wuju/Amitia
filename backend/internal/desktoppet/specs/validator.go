// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package specs

import (
	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type CatalogValidator struct{}

func NewCatalogValidator() *CatalogValidator {
	return &CatalogValidator{}
}

func (v *CatalogValidator) ValidateAll() []contracts.ValidationError {
	return CatalogValidate()
}

func (v *CatalogValidator) ValidatePreset(preset contracts.ActionPreset) []contracts.ValidationError {
	var errs []contracts.ValidationError

	if preset.Key == "" {
		errs = append(errs, contracts.ValidationError{
			Field:   "preset.key",
			Message: "must be non-empty",
		})
	}

	seen := make(map[string]bool)
	for _, k := range preset.ActionKeys {
		if seen[k] {
			errs = append(errs, contracts.ValidationError{
				ActionKey: k,
				Field:     "actionKeys",
				Message:   "duplicate key",
			})
		}
		seen[k] = true
	}

	for _, k := range preset.ActionKeys {
		if _, ok := CatalogGet(k); !ok {
			errs = append(errs, contracts.ValidationError{
				ActionKey: k,
				Field:     "actionKey",
				Message:   "not found in catalog",
			})
		}
	}

	if len(preset.RequiredAnyOf) > 0 {
		satisfied := false
		for _, group := range preset.RequiredAnyOf {
			allPresent := true
			for _, k := range group {
				if !seen[k] {
					allPresent = false
					break
				}
			}
			if allPresent {
				satisfied = true
				break
			}
		}
		if !satisfied {
			errs = append(errs, contracts.ValidationError{
				Field:   "requiredAnyOf",
				Message: "no group fully satisfied in preset action list",
			})
		}
	}

	if preset.Key == "minimal" {
		hasEnabledIdle := false
		for _, k := range preset.ActionKeys {
			spec, ok := CatalogGet(k)
			if !ok {
				continue
			}
			if spec.Identity.SupportsDefaultIdle && spec.Identity.Enabled {
				hasEnabledIdle = true
				break
			}
		}
		if !hasEnabledIdle {
			errs = append(errs, contracts.ValidationError{
				Field:   "preset.minimal",
				Message: "must contain at least one enabled default idle",
			})
		}
	}

	return errs
}

func (v *CatalogValidator) ValidatePresetHierarchy() []contracts.ValidationError {
	var errs []contracts.ValidationError

	if !IsMinimalSubsetOfStandard() {
		errs = append(errs, contracts.ValidationError{
			Field:   "preset.hierarchy",
			Message: "minimal is not a subset of standard",
		})
	}

	completeSet := make(map[string]bool)
	for _, k := range CatalogEnabledKeys() {
		completeSet[k] = true
	}

	standard, ok := PresetByKey("standard")
	if !ok {
		errs = append(errs, contracts.ValidationError{
			Field:   "preset.standard",
			Message: "standard preset not found",
		})
	} else {
		for _, k := range standard.ActionKeys {
			if !completeSet[k] {
				errs = append(errs, contracts.ValidationError{
					ActionKey: k,
					Field:     "preset.hierarchy",
					Message:   "standard action not in complete (CatalogEnabledKeys)",
				})
			}
		}
	}

	return errs
}

func (v *CatalogValidator) ValidateProjection(overrides map[string]ActionOverride) []contracts.ValidationError {
	var errs []contracts.ValidationError

	if overrides == nil {
		return errs
	}

	for _, k := range CatalogKeys() {
		if _, ok := overrides[k]; !ok {
			errs = append(errs, contracts.ValidationError{
				ActionKey: k,
				Field:     "projection",
				Message:   "missing override entry for catalog action",
			})
		}
	}

	catalogSet := make(map[string]bool)
	for _, k := range CatalogKeys() {
		catalogSet[k] = true
	}

	for k, override := range overrides {
		if override.ActionKey != "" && !catalogSet[k] {
			errs = append(errs, contracts.ValidationError{
				ActionKey: k,
				Field:     "projection",
				Message:   "unknown builtin key not in catalog",
			})
		}
	}

	return errs
}
