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
	"time"
)

type BackgroundMode string

const (
	ModeKeepAlpha        BackgroundMode = "keep_alpha"
	ModeRemoveBackground BackgroundMode = "remove_background"
	ModeUseExistingAlpha BackgroundMode = "use_existing_alpha"
)

var (
	ErrProviderUnavailable     = errors.New("background removal provider unavailable")
	ErrBackgroundRemovalFailed = errors.New("background removal failed")
	ErrAlphaChannelInvalid     = errors.New("alpha channel invalid")
	ErrSubjectNotFound         = errors.New("subject not found")
	ErrProviderAlreadyExists   = errors.New("provider already registered")
	ErrProviderNotRegistered   = errors.New("provider not registered")
	ErrCapabilitiesMismatch    = errors.New("provider capabilities mismatch")
)

const (
	ErrCodeBackgroundRemovalUnavailable = "BACKGROUND_REMOVAL_UNAVAILABLE"
	ErrCodeBackgroundRemovalFailed      = "BACKGROUND_REMOVAL_FAILED"
	ErrCodeAlphaChannelInvalid          = "ALPHA_CHANNEL_INVALID"
	ErrCodeSubjectNotFound              = "SUBJECT_NOT_FOUND"
	ErrCodeProviderAlreadyRegistered    = "PROVIDER_ALREADY_REGISTERED"
	ErrCodeProviderNotRegistered        = "PROVIDER_NOT_REGISTERED"
	ErrCodeCapabilitiesMismatch         = "CAPABILITIES_MISMATCH"
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

type BackgroundMeasurements struct {
	CornerConsistency  float64
	BackgroundVariance float64
	RemovedRatio       float64
	BoundaryConnected  float64
	Confidence         float64
}

type BackgroundRemovalResult struct {
	Image      image.Image
	Width      int
	Height     int
	SubjectBox SubjectBox
	AlphaValid bool

	Foreground   *image.NRGBA
	Mask         *image.Gray
	Provider     string
	Degraded     bool
	Measurements BackgroundMeasurements
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

type BackgroundRemovalCapabilities struct {
	ProviderName         string
	ProviderVersion      string
	SupportedModes       []BackgroundMode
	SupportedMIMEs       []string
	MaxWidth             int
	MaxHeight            int
	MaxPixels            int64
	ReturnsMask          bool
	PreservesSemiAlpha   bool
	SupportsBatch        bool
	SupportsCancellation bool
	NetworkRequired      bool
}

type BackgroundRemovalRequest struct {
	RequestID       string
	Image           *image.NRGBA
	Mode            BackgroundMode
	ExpectedSubject string
	Timeout         time.Duration
}

type ProviderDescriptor struct {
	Name         string
	Capabilities BackgroundRemovalCapabilities
}

type ImageDescriptor struct {
	Width  int
	Height int
	MIME   string
	Pixels int64
}

type ResolvedProvider struct {
	Provider     BackgroundRemovalProvider
	Capabilities BackgroundRemovalCapabilities
	Degraded     bool
	DegradedFrom string
	Reason       string
}

type BackgroundPolicyConfig struct {
	ProviderName   string
	Mode           BackgroundMode
	FallbackPolicy string
	Timeout        time.Duration
	MaxRetries     int
}

type BackgroundRemovalProviderV2 interface {
	BackgroundRemovalProvider
	Capabilities() BackgroundRemovalCapabilities
	RemoveBackgroundV2(ctx context.Context, req BackgroundRemovalRequest) (*BackgroundRemovalResult, error)
}

type Registry interface {
	Register(provider BackgroundRemovalProvider, caps BackgroundRemovalCapabilities) error
	GetByName(name string) (BackgroundRemovalProvider, error)
	GetCapabilities(name string) (BackgroundRemovalCapabilities, error)
	Resolve(policy BackgroundPolicyConfig, input ImageDescriptor) (*ResolvedProvider, error)
	List() []ProviderDescriptor
	Get(mode BackgroundMode) (BackgroundRemovalProvider, error)
}

type defaultRegistry struct {
	mu        sync.RWMutex
	providers map[string]BackgroundRemovalProvider
	caps      map[string]BackgroundRemovalCapabilities
	order     []string
}

func NewRegistry() Registry {
	return &defaultRegistry{
		providers: make(map[string]BackgroundRemovalProvider),
		caps:      make(map[string]BackgroundRemovalCapabilities),
		order:     make([]string, 0),
	}
}

func (r *defaultRegistry) Register(provider BackgroundRemovalProvider, caps BackgroundRemovalCapabilities) error {
	if provider == nil {
		return &ProviderError{
			Code:    ErrCodeBackgroundRemovalFailed,
			Message: "cannot register nil provider",
			Err:     ErrBackgroundRemovalFailed,
		}
	}

	name := provider.Name()
	if name == "" {
		return &ProviderError{
			Code:    ErrCodeBackgroundRemovalFailed,
			Message: "provider name is empty",
			Err:     ErrBackgroundRemovalFailed,
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return &ProviderError{
			Code:    ErrCodeProviderAlreadyRegistered,
			Message: fmt.Sprintf("provider %q already registered", name),
			Err:     ErrProviderAlreadyExists,
		}
	}

	if caps.ProviderName == "" {
		caps.ProviderName = name
	}

	r.providers[name] = provider
	r.caps[name] = caps
	r.order = append(r.order, name)

	return nil
}

func (r *defaultRegistry) GetByName(name string) (BackgroundRemovalProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[name]
	if !ok {
		return nil, &ProviderError{
			Code:    ErrCodeBackgroundRemovalUnavailable,
			Message: fmt.Sprintf("provider %q not registered", name),
			Err:     ErrProviderUnavailable,
		}
	}
	return provider, nil
}

func (r *defaultRegistry) GetCapabilities(name string) (BackgroundRemovalCapabilities, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	caps, ok := r.caps[name]
	if !ok {
		return BackgroundRemovalCapabilities{}, &ProviderError{
			Code:    ErrCodeProviderNotRegistered,
			Message: fmt.Sprintf("capabilities for provider %q not found", name),
			Err:     ErrProviderNotRegistered,
		}
	}
	return caps, nil
}

func (r *defaultRegistry) Resolve(policy BackgroundPolicyConfig, input ImageDescriptor) (*ResolvedProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if policy.ProviderName != "" {
		provider, caps, ok := r.lookupLocked(policy.ProviderName)
		if ok && capabilitiesMatch(caps, policy, input) {
			return &ResolvedProvider{
				Provider:     provider,
				Capabilities: caps,
				Degraded:     false,
			}, nil
		}
	}

	if policy.FallbackPolicy == "keep_original" {
		return &ResolvedProvider{
			Provider:     nil,
			Capabilities: BackgroundRemovalCapabilities{},
			Degraded:     true,
			Reason:       "fallback policy is keep_original, no provider selected",
		}, nil
	}

	if policy.FallbackPolicy == "none" || policy.FallbackPolicy == "" {
		if policy.ProviderName != "" {
			_, _, ok := r.lookupLocked(policy.ProviderName)
			if ok {
				return nil, &ProviderError{
					Code:    ErrCodeCapabilitiesMismatch,
					Message: fmt.Sprintf("provider %q cannot handle input (mode=%s, mime=%s, %dx%d)", policy.ProviderName, policy.Mode, input.MIME, input.Width, input.Height),
					Err:     ErrCapabilitiesMismatch,
				}
			}
		}
		return nil, &ProviderError{
			Code:    ErrCodeBackgroundRemovalUnavailable,
			Message: fmt.Sprintf("no provider available for mode %q", policy.Mode),
			Err:     ErrProviderUnavailable,
		}
	}

	fallbackOrder := r.fallbackNamesLocked(policy)
	for _, name := range fallbackOrder {
		if name == policy.ProviderName {
			continue
		}
		caps := r.caps[name]
		if capabilitiesMatch(caps, policy, input) {
			provider := r.providers[name]
			resolved := &ResolvedProvider{
				Provider:     provider,
				Capabilities: caps,
				Degraded:     true,
				DegradedFrom: policy.ProviderName,
				Reason:       fmt.Sprintf("primary provider %q unavailable or incompatible, using %q", policy.ProviderName, name),
			}
			return resolved, nil
		}
	}

	return nil, &ProviderError{
		Code:    ErrCodeBackgroundRemovalUnavailable,
		Message: fmt.Sprintf("no provider available for mode %q after fallback", policy.Mode),
		Err:     ErrProviderUnavailable,
	}
}

func (r *defaultRegistry) List() []ProviderDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]ProviderDescriptor, len(r.order))
	for i, name := range r.order {
		out[i] = ProviderDescriptor{
			Name:         name,
			Capabilities: r.caps[name],
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (r *defaultRegistry) Get(mode BackgroundMode) (BackgroundRemovalProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range r.order {
		caps := r.caps[name]
		for _, m := range caps.SupportedModes {
			if m == mode {
				return r.providers[name], nil
			}
		}
	}

	if len(r.order) > 0 {
		for _, name := range r.order {
			provider := r.providers[name]
			for _, m := range provider.SupportedModes() {
				if m == mode {
					return provider, nil
				}
			}
		}
	}

	return nil, &ProviderError{
		Code:    ErrCodeBackgroundRemovalUnavailable,
		Message: fmt.Sprintf("no provider registered for mode %q", mode),
		Err:     ErrProviderUnavailable,
	}
}

func (r *defaultRegistry) lookupLocked(name string) (BackgroundRemovalProvider, BackgroundRemovalCapabilities, bool) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, BackgroundRemovalCapabilities{}, false
	}
	return provider, r.caps[name], true
}

func (r *defaultRegistry) fallbackNamesLocked(policy BackgroundPolicyConfig) []string {
	switch policy.FallbackPolicy {
	case "semantic_then_local":
		result := make([]string, 0, len(r.order))
		if _, ok := r.providers["semantic"]; ok {
			result = append(result, "semantic")
		}
		if _, ok := r.providers["local-color-key"]; ok {
			result = append(result, "local-color-key")
		}
		for _, name := range r.order {
			if name != "semantic" && name != "local-color-key" {
				result = append(result, name)
			}
		}
		return result
	case "existing_alpha_then_semantic":
		result := make([]string, 0, len(r.order))
		for _, name := range r.order {
			caps := r.caps[name]
			for _, m := range caps.SupportedModes {
				if m == ModeUseExistingAlpha {
					result = append(result, name)
					break
				}
			}
		}
		if _, ok := r.providers["semantic"]; ok {
			already := false
			for _, n := range result {
				if n == "semantic" {
					already = true
					break
				}
			}
			if !already {
				result = append(result, "semantic")
			}
		}
		for _, name := range r.order {
			already := false
			for _, n := range result {
				if n == name {
					already = true
					break
				}
			}
			if !already {
				result = append(result, name)
			}
		}
		return result
	default:
		result := make([]string, len(r.order))
		copy(result, r.order)
		return result
	}
}

func capabilitiesMatch(caps BackgroundRemovalCapabilities, policy BackgroundPolicyConfig, input ImageDescriptor) bool {
	modeSupported := false
	for _, m := range caps.SupportedModes {
		if m == policy.Mode {
			modeSupported = true
			break
		}
	}
	if !modeSupported && len(caps.SupportedModes) > 0 {
		return false
	}

	if input.MIME != "" && len(caps.SupportedMIMEs) > 0 {
		mimeSupported := false
		for _, m := range caps.SupportedMIMEs {
			if m == input.MIME {
				mimeSupported = true
				break
			}
		}
		if !mimeSupported {
			return false
		}
	}

	if caps.MaxWidth > 0 && input.Width > caps.MaxWidth {
		return false
	}
	if caps.MaxHeight > 0 && input.Height > caps.MaxHeight {
		return false
	}
	if caps.MaxPixels > 0 && input.Pixels > caps.MaxPixels {
		return false
	}

	return true
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
