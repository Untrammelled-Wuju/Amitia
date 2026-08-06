// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import (
	"context"
	"fmt"
	"strings"
)

type Resolved struct {
	ID       ID
	Mobile   bool
	Settings *Settings
}

type Resolver interface {
	Resolve(ctx context.Context, profile string) (Resolved, error)
}

type ResolveContext struct {
	DescriptorProvider DescriptorProvider
}

type profileResolver struct {
	classifier *runtimeClassifier
	provider   DescriptorProvider
}

func NewResolver(ctx ResolveContext) (Resolver, error) {
	if ctx.DescriptorProvider == nil {
		return nil, ErrRuntimeDescriptorUnavailable
	}
	return &profileResolver{
		classifier: NewRuntimeClassifier(),
		provider:   ctx.DescriptorProvider,
	}, nil
}

func (r *profileResolver) Resolve(ctx context.Context, profile string) (Resolved, error) {
	if err := ctx.Err(); err != nil {
		return Resolved{}, err
	}

	profile = strings.TrimSpace(profile)
	if profile == "" {
		profile = string(ProfileAuto)
	}

	id, err := ParseProfile(profile)
	if err != nil {
		return Resolved{}, err
	}

	if id != ProfileAuto {
		return r.resolveExplicit(id)
	}

	return r.resolveAuto()
}

func (r *profileResolver) resolveExplicit(id ID) (Resolved, error) {
	runtimeClass, err := r.classifier.Classify(r.provider)
	if err != nil {
		return Resolved{}, err
	}
	if runtimeClass == RuntimeClassRestricted {
		return Resolved{}, fmt.Errorf("%w: explicit profile %q not allowed in restricted runtime", ErrQdrantProcessUnsupported, id)
	}

	if id == ProfileDesktopDefault {
		return Resolved{ID: id, Mobile: false, Settings: nil}, nil
	}

	return r.settingsForID(id)
}

func (r *profileResolver) resolveAuto() (Resolved, error) {
	runtimeClass, err := r.classifier.Classify(r.provider)
	if err != nil {
		return Resolved{}, err
	}

	switch runtimeClass {
	case RuntimeClassAndroidProot:
		return r.settingsForID(ProfileMobileBalanced)
	case RuntimeClassDesktopProcess:
		return Resolved{ID: ProfileDesktopDefault, Mobile: false, Settings: nil}, nil
	case RuntimeClassRestricted:
		return Resolved{}, fmt.Errorf("%w: qdrant cannot run in restricted runtime", ErrQdrantProcessUnsupported)
	default:
		return Resolved{}, fmt.Errorf("%w: %s", ErrRuntimeClassificationFailed, runtimeClass)
	}
}

func (r *profileResolver) settingsForID(id ID) (Resolved, error) {
	var settings Settings
	switch id {
	case ProfileMobileCompact:
		settings = CompactSettings()
	case ProfileMobileBalanced:
		settings = BalancedSettings()
	case ProfileMobilePerformance:
		settings = PerformanceSettings()
	default:
		return Resolved{}, fmt.Errorf("%w: not a mobile profile: %q", ErrUnknownProfile, id)
	}
	if err := settings.Validate(); err != nil {
		return Resolved{}, err
	}
	return Resolved{ID: id, Mobile: true, Settings: &settings}, nil
}
