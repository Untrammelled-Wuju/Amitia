// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package featuregate

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	NameRuntimeCanary  = "runtime-canary"
	NameRouteInventory = "route-inventory"
)

var (
	ErrUnknownFeature         = errors.New("featuregate: 未知功能开关")
	ErrInvalidValue           = errors.New("featuregate: 功能开关值非法")
	ErrConfigurationConflict  = errors.New("featuregate: 重复配置内容不一致")
	ErrConfigurationMalformed = errors.New("featuregate: 功能开关配置非法")
)

type Feature struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Applied bool   `json:"applied"`
}

type Report struct {
	Features []Feature `json:"features"`
}

type Hooks struct {
	Provider   func() error
	Subscriber func() error
	Worker     func(ctx context.Context)
}

type Outcome struct {
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	Duplicate  bool     `json:"duplicate"`
	Registered []string `json:"registered"`
}

type Registry struct {
	mu      sync.RWMutex
	states  map[string]bool
	applied map[string]bool
}

func catalog() []string {
	return []string{NameRouteInventory, NameRuntimeCanary}
}

func newStateMap() map[string]bool {
	states := make(map[string]bool, len(catalog()))
	for _, name := range catalog() {
		states[name] = false
	}
	return states
}

func normalizeName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func knownFeature(name string) bool {
	for _, item := range catalog() {
		if item == name {
			return true
		}
	}
	return false
}

func Parse(raw map[string]any) (*Registry, error) {
	registry := &Registry{
		states:  newStateMap(),
		applied: make(map[string]bool, len(catalog())),
	}
	seen := make(map[string]string, len(raw))
	for key, value := range raw {
		name := normalizeName(key)
		if !knownFeature(name) {
			return nil, fmt.Errorf("%w: %q（可用：%s）", ErrUnknownFeature, key, strings.Join(catalog(), ", "))
		}
		if previous, ok := seen[name]; ok {
			return nil, fmt.Errorf("%w: %q 与 %q 归一化后重复", ErrConfigurationMalformed, previous, key)
		}
		seen[name] = key
		enabled, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("%w: %q 必须是布尔值，实际 %T(%v)", ErrInvalidValue, key, value, value)
		}
		registry.states[name] = enabled
	}
	return registry, nil
}

func (r *Registry) Enabled(name string) bool {
	if r == nil {
		return false
	}
	normalized := normalizeName(name)
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.states[normalized]
}

func (r *Registry) Apply(ctx context.Context, name string, hooks Hooks) (Outcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized := normalizeName(name)
	if r == nil {
		return Outcome{Name: normalized, Enabled: false}, nil
	}

	r.mu.Lock()
	if _, ok := r.states[normalized]; !ok {
		r.mu.Unlock()
		return Outcome{}, fmt.Errorf("%w: %q", ErrUnknownFeature, name)
	}
	enabled := r.states[normalized]
	if !enabled {
		r.mu.Unlock()
		return Outcome{Name: normalized, Enabled: false}, nil
	}
	if r.applied[normalized] {
		r.mu.Unlock()
		return Outcome{Name: normalized, Enabled: true, Duplicate: true}, nil
	}
	r.applied[normalized] = true
	r.mu.Unlock()

	outcome := Outcome{Name: normalized, Enabled: true}
	registered := make([]string, 0, 3)
	if hooks.Provider != nil {
		if err := hooks.Provider(); err != nil {
			r.release(normalized)
			return Outcome{Name: normalized, Enabled: true, Registered: registered}, fmt.Errorf("featuregate: 激活 %q 的 provider 失败（已应用：%s）：%w", normalized, joinRegistered(registered), err)
		}
		registered = append(registered, "provider")
	}
	if hooks.Subscriber != nil {
		if err := hooks.Subscriber(); err != nil {
			r.release(normalized)
			return Outcome{Name: normalized, Enabled: true, Registered: registered}, fmt.Errorf("featuregate: 激活 %q 的 subscriber 失败（已应用：%s）：%w", normalized, joinRegistered(registered), err)
		}
		registered = append(registered, "subscriber")
	}
	if hooks.Worker != nil {
		go hooks.Worker(ctx)
		registered = append(registered, "worker")
	}
	outcome.Registered = registered
	return outcome, nil
}

func (r *Registry) release(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.applied, name)
}

func joinRegistered(registered []string) string {
	if len(registered) == 0 {
		return "none"
	}
	return strings.Join(registered, ",")
}

func (r *Registry) Snapshot() Report {
	if r == nil {
		return Report{Features: []Feature{}}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	features := make([]Feature, 0, len(r.states))
	for _, name := range catalog() {
		features = append(features, Feature{Name: name, Enabled: r.states[name], Applied: r.applied[name]})
	}
	return Report{Features: features}
}

func (r *Registry) fingerprint() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	parts := make([]string, 0, len(r.states))
	for _, name := range catalog() {
		parts = append(parts, fmt.Sprintf("%s=%t", name, r.states[name]))
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

var (
	globalMu          sync.RWMutex
	globalRegistry    *Registry
	globalFingerprint string
	globalConfigured  bool
)

func Configure(raw map[string]any) error {
	registry, err := Parse(raw)
	if err != nil {
		return err
	}
	fingerprint := registry.fingerprint()
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalConfigured && fingerprint != globalFingerprint {
		return fmt.Errorf("%w: 当前 %s，新配置 %s", ErrConfigurationConflict, globalFingerprint, fingerprint)
	}
	globalRegistry = registry
	globalFingerprint = fingerprint
	globalConfigured = true
	return nil
}

func Active() *Registry {
	globalMu.RLock()
	registry := globalRegistry
	globalMu.RUnlock()
	if registry == nil {
		return &Registry{states: newStateMap(), applied: make(map[string]bool, len(catalog()))}
	}
	return registry
}

func Enabled(name string) bool {
	return Active().Enabled(name)
}

func Apply(ctx context.Context, name string, hooks Hooks) (Outcome, error) {
	return Active().Apply(ctx, name, hooks)
}

func Snapshot() Report {
	return Active().Snapshot()
}
