// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package backgroundremoval

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sort"
	"sync"
)

type BackgroundMode string

const (
	ModeKeepAlpha        BackgroundMode = "keep_alpha"
	ModeRemoveBackground BackgroundMode = "remove_background"
	ModeUseExistingAlpha BackgroundMode = "use_existing_alpha"
)

var (
	ErrProviderUnavailable    = errors.New("background removal provider unavailable")
	ErrBackgroundRemovalFailed = errors.New("background removal failed")
	ErrAlphaChannelInvalid    = errors.New("alpha channel invalid")
	ErrSubjectNotFound        = errors.New("subject not found")
)

const (
	ErrCodeBackgroundRemovalUnavailable = "BACKGROUND_REMOVAL_UNAVAILABLE"
	ErrCodeBackgroundRemovalFailed      = "BACKGROUND_REMOVAL_FAILED"
	ErrCodeAlphaChannelInvalid          = "ALPHA_CHANNEL_INVALID"
	ErrCodeSubjectNotFound              = "SUBJECT_NOT_FOUND"
)

type ImageInput struct {
	Image  image.Image
	Width  int
	Height int
	Mode   BackgroundMode
}

type SubjectBox struct {
	MinX   int
	MinY   int
	MaxX   int
	MaxY   int
	Width  int
	Height int
	Empty  bool
}

type BackgroundRemovalResult struct {
	Image      image.Image
	Width      int
	Height     int
	SubjectBox SubjectBox
	AlphaValid bool
}

type ProviderError struct {
	Code    string
	Message string
	Err     error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type BackgroundRemovalProvider interface {
	Name() string
	SupportedModes() []BackgroundMode
	RemoveBackground(ctx context.Context, input ImageInput) (*BackgroundRemovalResult, error)
}

type Registry interface {
	Register(provider BackgroundRemovalProvider)
	Get(mode BackgroundMode) (BackgroundRemovalProvider, error)
	List() []BackgroundRemovalProvider
}

type defaultRegistry struct {
	mu        sync.RWMutex
	providers []BackgroundRemovalProvider
	modeIndex map[BackgroundMode]BackgroundRemovalProvider
}

func NewRegistry() Registry {
	return &defaultRegistry{
		providers: make([]BackgroundRemovalProvider, 0),
		modeIndex: make(map[BackgroundMode]BackgroundRemovalProvider),
	}
}

func (r *defaultRegistry) Register(provider BackgroundRemovalProvider) {
	if provider == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.providers {
		if existing.Name() == provider.Name() {
			return
		}
	}
	r.providers = append(r.providers, provider)
	for _, mode := range provider.SupportedModes() {
		if _, ok := r.modeIndex[mode]; !ok {
			r.modeIndex[mode] = provider
		}
	}
}

func (r *defaultRegistry) Get(mode BackgroundMode) (BackgroundRemovalProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.modeIndex[mode]; ok {
		return p, nil
	}
	return nil, &ProviderError{
		Code:    ErrCodeBackgroundRemovalUnavailable,
		Message: fmt.Sprintf("no provider registered for mode %q", mode),
		Err:     ErrProviderUnavailable,
	}
}

func (r *defaultRegistry) List() []BackgroundRemovalProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BackgroundRemovalProvider, len(r.providers))
	copy(out, r.providers)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name() < out[j].Name()
	})
	return out
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistryInst Registry
)

func DefaultRegistry() Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistryInst = NewRegistry()
	})
	return defaultRegistryInst
}
