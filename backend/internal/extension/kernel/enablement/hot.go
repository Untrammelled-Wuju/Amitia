//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrHotInvalid = errors.New("plugin hot: invalid request or policy")
	ErrHotDenied  = errors.New("plugin hot: residency denied")
)

type ProfileSource string

const (
	ProfileSourceHost   ProfileSource = "host"
	ProfileSourcePlugin ProfileSource = "plugin"
)

type HotProfile struct {
	MaxMemoryBytes int64
	MaxConcurrent  int
	Source         ProfileSource
}

type HotPolicy struct {
	AllowedCategories map[string]bool
	MinMemoryBytes    int64
	MinConcurrent     int
	MaxHot            int
}

func (p HotPolicy) Validate() error {
	if len(p.AllowedCategories) == 0 {
		return fmt.Errorf("%w: no allowed categories", ErrHotInvalid)
	}
	if p.MinMemoryBytes < 0 || p.MinConcurrent < 0 || p.MaxHot < 0 {
		return fmt.Errorf("%w: negative policy bound", ErrHotInvalid)
	}
	return nil
}

type HotRequest struct {
	PluginID string
	Category string
	Granted  bool
	Profile  HotProfile
}

type HotResidency struct {
	installer *Installer
	policy    HotPolicy
	mu        sync.Mutex
	hot       map[string]HotRequest
	denials   map[string]string
}

func NewHotResidency(installer *Installer, policy HotPolicy) (*HotResidency, error) {
	if installer == nil {
		return nil, fmt.Errorf("%w: installer is nil", ErrHotInvalid)
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &HotResidency{
		installer: installer,
		policy:    policy,
		hot:       map[string]HotRequest{},
		denials:   map[string]string{},
	}, nil
}

func (h *HotResidency) deny(pluginID string, reason string, cause error) error {
	h.mu.Lock()
	h.denials[pluginID] = reason
	h.mu.Unlock()
	if cause != nil {
		return fmt.Errorf("%w: %s: %s: %w", ErrHotDenied, pluginID, reason, cause)
	}
	return fmt.Errorf("%w: %s: %s", ErrHotDenied, pluginID, reason)
}

func (h *HotResidency) Acquire(ctx context.Context, request HotRequest) error {
	if h == nil {
		return fmt.Errorf("%w: residency is nil", ErrHotInvalid)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if request.PluginID == "" || request.Category == "" {
		return fmt.Errorf("%w: plugin id and category are required", ErrHotInvalid)
	}
	if !h.policy.AllowedCategories[request.Category] {
		return h.deny(request.PluginID, "category_not_allowed", nil)
	}
	if !request.Granted {
		return h.deny(request.PluginID, "authorization_missing", nil)
	}
	if request.Profile.Source != ProfileSourceHost {
		return h.deny(request.PluginID, "profile_source_not_host", nil)
	}
	if request.Profile.MaxMemoryBytes < h.policy.MinMemoryBytes {
		return h.deny(request.PluginID, "profile_memory_below_floor", nil)
	}
	if request.Profile.MaxConcurrent < h.policy.MinConcurrent {
		return h.deny(request.PluginID, "profile_concurrency_below_floor", nil)
	}
	state, ok := h.installer.State(request.PluginID)
	if !ok {
		return fmt.Errorf("%w: %s is not installed", ErrHotInvalid, request.PluginID)
	}
	if state != LifecycleActivated && state != LifecycleWarm && state != LifecycleRunning {
		return fmt.Errorf("%w: %s is %s", ErrHotInvalid, request.PluginID, state)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.hot[request.PluginID]; ok {
		if existing.Category != request.Category {
			return fmt.Errorf("%w: %s is already hot as %s", ErrHotInvalid, request.PluginID, existing.Category)
		}
		return nil
	}
	if h.policy.MaxHot > 0 && len(h.hot) >= h.policy.MaxHot {
		h.denials[request.PluginID] = "capacity_exhausted"
		return fmt.Errorf("%w: %s: capacity_exhausted", ErrHotDenied, request.PluginID)
	}
	h.hot[request.PluginID] = request
	delete(h.denials, request.PluginID)
	return nil
}

func (h *HotResidency) Release(pluginID string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.hot, pluginID)
}

func (h *HotResidency) IsHot(pluginID string) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.hot[pluginID]
	return ok
}

func (h *HotResidency) Len() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.hot)
}

func (h *HotResidency) Denial(pluginID string) (string, bool) {
	if h == nil {
		return "", false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	reason, ok := h.denials[pluginID]
	return reason, ok
}

func (r *IdleReaper) WithHotCheck(check func(string) bool) *IdleReaper {
	r.hotCheck = check
	return r
}
